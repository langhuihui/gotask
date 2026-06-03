package task

import (
	"testing"
	"time"
)

// bgKeepTask 是一个常驻子任务,用 Go()(独立 goroutine,不阻塞父循环)阻塞到被 Stop,
// 用来保证父 Job 的 Size>0、事件循环处于运行态(让 Call 走"入队"路径而非 Size<=0 内联)。
// 注意:必须用 Go() 而非 Run() —— Run() 内联跑在父循环 goroutine 上会把循环堵死。
type bgKeepTask struct {
	Task
}

func (t *bgKeepTask) Go() error {
	<-t.Done()
	return ErrTaskComplete
}

// reentrantStartTask 在自己的 Start() 里(此刻正跑在父 Job 的事件循环 goroutine 上)
// 再调用父 Job 的 Call —— 这就是同 goroutine 重入。
//
// 修复前:Call 在 Size>0 时无条件把 callback 入队、然后 <-ctx.Done() 阻塞等事件
// 循环处理它;但事件循环此刻正阻塞在这次 Call 里(它在跑我们的 Start()),永远
// 处理不到入队的 callback → 永久死锁。
// 修复后:Call 识别"调用者就是本事件循环 goroutine",内联执行 callback,不死锁。
type reentrantStartTask struct {
	Task
	job  *Job
	done chan struct{}
}

func (t *reentrantStartTask) Start() error {
	executed := false
	t.job.Call(func() { executed = true })
	if executed {
		close(t.done)
	}
	return ErrTaskComplete
}

func Test_Call_ReentrantFromLoopGoroutine(t *testing.T) {
	var job Job
	root.AddTask(&job)
	_ = job.WaitStarted()
	// 非阻塞收尾:死锁场景下事件循环卡住,不能 WaitStopped 否则挂住整个用例。
	defer job.Stop(ErrTaskComplete)

	// 常驻任务(Go 后台,不堵循环),确保 Size>0 + 循环在跑。
	keep := &bgKeepTask{}
	job.AddTask(keep)
	_ = keep.WaitStarted()

	done := make(chan struct{})
	rt := &reentrantStartTask{job: &job, done: done}
	job.AddTask(rt)

	select {
	case <-done:
		// 通过:重入 Call 内联完成,callback 已执行。
	case <-time.After(3 * time.Second):
		t.Fatal("re-entrant Job.Call deadlocked: callback never ran within 3s")
	}
}
