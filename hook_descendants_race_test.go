package task

import (
	"testing"
)

// Job 上还有第五、第六个同形态的裸切片：descendantsStartListeners 与
// descendantsDisposeListeners（job.go）。注册端 OnDescendantsStart /
// OnDescendantsDispose 是裸 append，消费端 onDescendantsStart /
// onDescendantsDispose 是裸 range，跑在事件循环 goroutine 上。
//
// 这两个比 Task 上那四个波及面更大：onDescendantsDispose 遍历完自己的之后还会
// `mt.parent.onDescendantsDispose(descendants)` 一路向上——也就是说**祖先的切片
// 是被后代的事件循环 goroutine 读的**，而祖先自己的业务 goroutine 随时可能在
// 往里 append。
//
// 卡位要点：信号必须发在**被测的那次 range 之前**。如果从被遍历的监听器内部发
// 信号，range 早已求值完毕，close→receive 这条 happens-before 边会把读排在写
// 之前，竞争就被抹掉、用例恒绿。所以这里从子任务的 Start()/Run() 里发信号——
// 那时父 Job 的 onChildStart/onChildDispose 还没被调用。
//
// 同样不能用反向放行信号，理由同 hook_register_race_test.go。

type descendantsRaceJob struct {
	Job
}

// startSignalChild 只实现 Start()：start() 返回 true，父 Job 随后才在
// event_loop.go 里调 onChildStart → 遍历 descendantsStartListeners。
//
// 这里**不能**实现 Run()：Run() 一返回，start() 就会走 Stop→onChildDispose 并
// 返回 false，于是 onChildStart 根本不会被调用，用例会恒绿。
type startSignalChild struct {
	Task
	started chan struct{}
}

func (c *startSignalChild) Start() error {
	close(c.started)
	return nil
}

// disposeSignalChild 实现 Run() 并立刻返回：父 Job 随后才调 onChildDispose →
// 遍历 descendantsDisposeListeners。
type disposeSignalChild struct {
	Task
	ran chan struct{}
}

func (c *disposeSignalChild) Run() error {
	close(c.ran)
	return nil
}

func Test_OnDescendantsStartRegisterRace(t *testing.T) {
	job := &descendantsRaceJob{}
	root.AddTask(job)
	if err := job.WaitStarted(); err != nil {
		t.Fatalf("job 启动失败: %v", err)
	}

	child := &startSignalChild{started: make(chan struct{})}
	job.AddTask(child)

	<-child.started
	job.OnDescendantsStart(func(ITask) {}) // 与 onDescendantsStart 的遍历并发

	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
}

func Test_OnDescendantsDisposeRegisterRace(t *testing.T) {
	job := &descendantsRaceJob{}
	root.AddTask(job)
	if err := job.WaitStarted(); err != nil {
		t.Fatalf("job 启动失败: %v", err)
	}

	child := &disposeSignalChild{ran: make(chan struct{})}
	job.AddTask(child)

	<-child.ran
	job.OnDescendantsDispose(func(ITask) {}) // 与 onDescendantsDispose 的遍历并发

	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
}
