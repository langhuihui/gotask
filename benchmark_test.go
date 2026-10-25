package task

import (
	"errors"
	"testing"
)

// 本文件是主线验收门「基准证明无明显退化」的基线，覆盖本次正确性修复
// 触及的三类路径：
//
//  1. 单任务生命周期：AddTask（initContext 现在整体在 stopMu 下）+
//     Stop/dispose（stopOnce 换成 stopMu+stopped）。
//  2. Job 带存活子任务停止：waitChildrenDispose → waitChildStopped
//     （现在等子任务的 shutdown，而不是会被 Reject 短路掉的 WaitStopped）。
//  3. Job.Call：入队 + defer cancel 的背压路径。
//  4. 重复 Stop：stopped 标志的幂等快速路径（Once 的快速路径是原子读，
//     互斥量略贵，用本基准量化差异）。
//
// 只使用既有公开 API，可在修复前后的代码上直接对比运行：
//
//	go test -bench=. -benchmem -run='^$' -count=3 .
//
// 对比方法：git stash 掉未提交修复后跑同一命令，再 git stash pop。

// benchRunTask 启动后立即跑完 Run 自动结束，覆盖 start→run→dispose 全路径。
type benchRunTask struct {
	Task
}

func (t *benchRunTask) Run() error { return nil }

// benchGoTask 以独立 goroutine 等待停止信号，不占用事件循环。
type benchGoTask struct {
	Task
}

func (t *benchGoTask) Go() error {
	<-t.Done()
	return t.StopReason()
}

func BenchmarkTaskStartStop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		task := &benchRunTask{}
		root.AddTask(task)
		_ = task.WaitStopped()
	}
}

// BenchmarkJobStopWithLiveChildren 在子任务全部存活时停止 Job：
// 父任务 dispose 必须等齐每个子任务的 shutdown（waitChildStopped），
// 这是本次父子销毁顺序修复的直接路径。
func BenchmarkJobStopWithLiveChildren(b *testing.B) {
	const childCount = 8
	for i := 0; i < b.N; i++ {
		var job Job
		root.AddTask(&job)
		if err := job.WaitStarted(); err != nil {
			b.Fatal(err)
		}
		children := make([]*benchGoTask, childCount)
		for j := range children {
			children[j] = &benchGoTask{}
			job.AddTask(children[j])
			if err := children[j].WaitStarted(); err != nil {
				b.Fatal(err)
			}
		}
		job.Stop(ErrTaskComplete)
		_ = job.WaitStopped()
	}
}

// BenchmarkJobCall 走事件循环的 Call 路径（root 有常驻子任务，Size>0）。
func BenchmarkJobCall(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		root.Call(func() {})
	}
}

// BenchmarkStopIdempotent 对已停止任务重复 Stop：命中 stopped 标志直接返回。
func BenchmarkStopIdempotent(b *testing.B) {
	task := &benchRunTask{}
	root.AddTask(task)
	if err := task.WaitStopped(); !errors.Is(err, ErrTaskComplete) {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task.Stop(ErrTaskComplete)
	}
}
