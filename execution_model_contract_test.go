package task

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// 本文件是重构线 Phase 1「语义基线」的契约测试：把**当前**执行模型的关键
// 语义固化下来，作为 Phase 2（控制与执行分离）的兼容边界。每条契约标注了
// 它在 Phase 2/3 中的命运，详见 docs/重构-执行模型v2.md。
//
// 这些测试在当前代码上必须全部通过；Phase 2 改造时：
//   - 标注「保持」的契约不允许失败，失败即回归；
//   - 标注「可改变」的契约在语义被有意改变时同步改写，并在文档里记录理由。

// ---------- 辅助任务类型 ----------

// blockingRunTask：TaskBlock，Run 阻塞直到 release 关闭。
type blockingRunTask struct {
	Task
	entered chan struct{}
	release chan struct{}
}

func (t *blockingRunTask) Run() error {
	close(t.entered)
	<-t.release
	return nil
}

// blockingGoTask：TaskGo，在独立 goroutine 里阻塞直到 release 关闭。
type blockingGoTask struct {
	Task
	entered chan struct{}
	release chan struct{}
}

func (t *blockingGoTask) Go() error {
	close(t.entered)
	<-t.release
	return nil
}

// blockingStartTask：Start 阻塞直到 release 关闭。
type blockingStartTask struct {
	Task
	entered chan struct{}
	release chan struct{}
}

func (t *blockingStartTask) Start() error {
	close(t.entered)
	<-t.release
	return nil
}

// retryFailTask：Run 必然失败，用于观察重试路径。
type retryFailTask struct {
	Task
	attempts atomic.Int32
}

func (t *retryFailTask) Run() error {
	t.attempts.Add(1)
	return errors.New("run failed")
}

// callWithin 在 goroutine 里执行 job.Call，返回完成通道。
func callWithin(job *Job) chan struct{} {
	done := make(chan struct{})
	go func() {
		job.Call(func() { close(done) })
	}()
	return done
}

// ---------- 契约 1：Run 占用父事件循环【可改变：Phase 2 的核心目标】 ----------

func TestContract_RunOccupiesParentEventLoop(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	blocker := &blockingRunTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(blocker)
	<-blocker.entered // Run 正在占用 job 的事件循环

	// 当前语义：同 Job 的 Call 要等 Run 让出循环才能执行。
	callDone := callWithin(&job)
	select {
	case <-callDone:
		t.Fatal("Call completed while Run occupies the parent event loop")
	case <-time.After(100 * time.Millisecond):
	}
	close(blocker.release)
	select {
	case <-callDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Call did not complete after Run released the loop")
	}
	_ = job.WaitStopped() // 子任务跑完后 Job 自动停止
}

// ---------- 契约 2：Go 不占用父事件循环【保持】 ----------

func TestContract_GoRunsOffParentEventLoop(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	blocker := &blockingGoTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(blocker)
	<-blocker.entered

	// 当前语义：TaskGo 跑在独立 goroutine 上，不堵同 Job 的 Call。
	callDone := callWithin(&job)
	select {
	case <-callDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Call blocked by a Go task: TaskGo must run off the event loop")
	}
	close(blocker.release)
	_ = job.WaitStopped()
}

// ---------- 契约 3：Start 占用父事件循环【可改变：Phase 2 评估】 ----------

func TestContract_StartOccupiesParentEventLoop(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	blocker := &blockingStartTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(blocker)
	<-blocker.entered // Start 正在占用 job 的事件循环

	callDone := callWithin(&job)
	select {
	case <-callDone:
		t.Fatal("Call completed while Start occupies the parent event loop")
	case <-time.After(100 * time.Millisecond):
	}
	close(blocker.release)
	select {
	case <-callDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Call did not complete after Start released the loop")
	}
	job.Stop(ErrTaskComplete)
	_ = job.WaitStopped()
}

// ---------- 契约 4：跨循环隔离【保持】 ----------

// 子 Job 里阻塞的执行不拖累父 Job（及兄弟 Job）的事件循环。每个 Job 有
// 独立的事件循环 goroutine，这是 Phase 2 改造后仍然必须成立的性质。
func TestContract_BlockingExecutionDoesNotBlockAncestorLoop(t *testing.T) {
	var parent Job
	root.AddTask(&parent)
	if err := parent.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	var childJob Job
	parent.AddTask(&childJob)
	if err := childJob.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	blocker := &blockingRunTask{entered: make(chan struct{}), release: make(chan struct{})}
	childJob.AddTask(blocker)
	<-blocker.entered // childJob 的事件循环被 Run 占住

	// 父 Job 的 Call 不受影响。
	callDone := callWithin(&parent)
	select {
	case <-callDone:
	case <-time.After(2 * time.Second):
		t.Fatal("ancestor event loop blocked by a descendant Run")
	}
	close(blocker.release)
	_ = parent.WaitStopped()
}

// ---------- 契约 5：重试退避占用 Owner 事件循环【可改变：Phase 3】 ----------

// checkRetry 的退避等待跑在 Job 自己的事件循环上（start 的错误路径里），
// 期间同 Job 的 Call 被堵住。Phase 3「重试与监督策略分离」时可改变此项。
func TestContract_RetryBackoffOccupiesOwnerLoop(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	task := &retryFailTask{}
	task.SetRetry(1, 300*time.Millisecond)
	job.AddTask(task)

	// 等第一次 Run 失败并进入退避（300ms）。
	time.Sleep(50 * time.Millisecond)
	callDone := callWithin(&job)
	select {
	case <-callDone:
		t.Fatal("Call completed during retry backoff: backoff runs on the owner loop")
	case <-time.After(150 * time.Millisecond):
	}
	select {
	case <-callDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Call did not complete after the retry cycle finished")
	}
	if got := task.attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts (initial + 1 retry), got %d", got)
	}
	_ = job.WaitStopped()
}

// ---------- 契约 6：子任务先于父任务销毁【保持】 ----------

// 与 task_test.go 的 Test_ParentStop 同义，这里从执行模型视角再锁一次：
// Job 的 OnDispose 钩子必须跑在全部子任务 dispose 完成之后。
func TestContract_ChildrenDisposeBeforeParent(t *testing.T) {
	var parent Job
	root.AddTask(&parent)
	var order atomic.Int32
	var childSaw, parentSaw atomic.Int32
	disposed := make(chan struct{}, 2)
	var task Task
	task.OnDispose(func() {
		childSaw.Store(order.Add(1))
		disposed <- struct{}{}
	})
	parent.OnDispose(func() {
		parentSaw.Store(order.Add(1))
		disposed <- struct{}{}
	})
	parent.AddTask(&task)
	parent.Stop(ErrAutoStop)
	<-disposed
	<-disposed
	if childSaw.Load() != 1 || parentSaw.Load() != 2 {
		t.Fatalf("dispose order wrong: child=%d parent=%d (want 1, 2)", childSaw.Load(), parentSaw.Load())
	}
}

// ---------- 契约 7：全部子任务完成后 Job 自动停止【保持】 ----------

func TestContract_JobAutoStopsWhenChildrenComplete(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	const childCount = 4
	children := make([]*blockingGoTask, childCount)
	for j := range children {
		children[j] = &blockingGoTask{entered: make(chan struct{}), release: make(chan struct{})}
		job.AddTask(children[j])
		<-children[j].entered
	}
	for _, c := range children {
		close(c.release)
	}
	// 不显式 Stop：等 Job 自动停止。
	select {
	case <-job.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("job did not auto-stop after all children completed")
	}
	if err := job.WaitStopped(); !errors.Is(err, ErrAutoStop) {
		t.Fatalf("expected ErrAutoStop, got %v", err)
	}
}
