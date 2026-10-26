package task

import (
	"testing"
	"time"
)

// 本文件是重构线 Phase 3（3A 重试与监督分离）的基准，只在
// refactor/execution-model-v2 上有意义：它度量的是 C5 翻转后的能力——任务处于重试
// 退避等待时，同 Job 的 Call 仍能被及时处理。旧语义下退避内联在事件循环上 sleep，
// 这条基准的延迟会等于退避时长（无法作为基准运行，只能靠契约测试 C5′ 捕获）。
//
// 注意不要把退避设成 0：那样任务会在「失败 → 立刻重试」之间循环，基准实际测的是
// 重试吞吐而不是退避期间的循环响应。这里用 1h 退避把任务钉在退避状态。
//
// 运行方式：
//
//	go test -bench='BenchmarkCallDuringRetryBackoff' -benchmem -run='^$' -count=3 .
func BenchmarkCallDuringRetryBackoff(b *testing.B) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		b.Fatal(err)
	}
	task := &retryFailTask{}
	task.SetRetry(-1, time.Hour) // 首次失败后进入 1h 退避，整段基准期间都停在退避里
	job.AddTask(task)
	// 等首次 attempt 失败并进入退避（dispose 已完成）。
	deadline := time.Now().Add(5 * time.Second)
	for !(task.attempts.Load() == 1 && task.GetState() == TASK_STATE_DISPOSED) {
		if time.Now().After(deadline) {
			b.Fatal("task did not enter retry backoff")
		}
		time.Sleep(time.Millisecond)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job.Call(func() {})
	}
	b.StopTimer()
	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
}
