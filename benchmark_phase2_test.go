package task

import (
	"testing"
)

// 本文件是重构线 Phase 2（控制与执行分离）的基准，只在 refactor/execution-model-v2
// 上有意义：BenchmarkCallDuringBlockingRun 度量的正是 Phase 2 修复的病理——旧语义
// 下子任务 Run 占住父事件循环，同 Job 的 Call 延迟 ≈ Run 时长；Phase 2 后循环只做
// 控制，延迟应为微秒级。不放进 benchmark_test.go：那个文件与 main 线同步，本基准
// 在主线语义下没有意义。
//
// 运行方式（与主线基线对比时用 git stash 法，见 benchmark_test.go 头部注释）：
//
//	go test -bench='BenchmarkCallDuringBlockingRun|BenchmarkTaskStartStop' -benchmem -run='^$' -count=3 .
func BenchmarkCallDuringBlockingRun(b *testing.B) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		b.Fatal(err)
	}
	// blockingRunTask 见 execution_model_contract_test.go：Run 阻塞到 release 关闭。
	blocker := &blockingRunTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(blocker)
	<-blocker.entered // Run 正在执行（且不再占用 job 的事件循环）
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job.Call(func() {})
	}
	b.StopTimer()
	close(blocker.release)
	_ = job.WaitStopped()
}
