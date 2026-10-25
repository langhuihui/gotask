package task

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// 本文件是重构线的执行模型契约测试：Phase 1「语义基线」把**当前**执行模型的
// 关键语义固化下来，作为 Phase 2（控制与执行分离）的兼容边界。每条契约标注了
// 它在 Phase 2/3 中的命运，详见 docs/重构-执行模型v2.md。
//
// Phase 2 设计见 docs/重构-执行模型v2-Phase2设计.md（已评审通过：仅 Run 移出
// 循环、onDescendantsStart 每 attempt 触发、RunTask 统一重试、基准退化记录理由）。
// Phase 2 后本文件的契约集：
//   - C1 已按新语义改写为 TestContract_RunDoesNotOccupyParentEventLoop；
//   - C8 锁定新语义（Run 执行期间停止信号仍被及时处理）；
//   - C9/C10 是行为保持类（重试序列、RunTask 驱动重试）：改造前后都应通过，
//     用于证明「只改执行位置、不改可观测序列」；
//   - C11 锁定一处顺带修复的缺陷：旧代码里 Run panic 被 start() 的 defer 恢复，
//     但错误路径（startup.Reject/Stop/onChildDispose/checkRetry）整个被 panic
//     展开跳过，start() 返回零值 false，任务被 removeChild 却永不停止、永不
//     销毁（WaitStopped 永久挂起）。Phase 2 后 panic 在执行 goroutine 上经
//     task.run 恢复并 Stop，回到正常信号路径。

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

// ---------- 契约 1：Run 不占用父事件循环【Phase 2 改写：旧语义为「占用」】 ----------
//
// Phase 2（控制与执行分离）后：TaskBlock.Run 跑在独立 goroutine 上，父 Job
// 事件循环在 Run 执行期间保持响应——Call、兄弟任务启动都不再被拖延。
// 旧代码上本测试失败（Call 被 Run 堵住），改造后通过。
func TestContract_RunDoesNotOccupyParentEventLoop(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	blocker := &blockingRunTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(blocker)
	<-blocker.entered // Run 正在执行，但不再占用 job 的事件循环

	// 同 Job 的 Call 不被 Run 拖延。
	callDone := callWithin(&job)
	select {
	case <-callDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Call blocked by Run: Run must not occupy the parent event loop")
	}

	// 兄弟任务在 Run 执行期间也能及时启动。
	sibling := &blockingGoTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(sibling)
	select {
	case <-sibling.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("sibling task did not start while Run is in flight")
	}
	close(sibling.release)
	close(blocker.release)
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
	// 不能假设 callDone 触发时第二次 attempt 已跑完：Phase 2 后 Run 在独立
	// goroutine 上执行，循环处理完排队的 Call 回调时，重试的执行 goroutine 可能
	// 还没被调度（旧代码里重试内联在 start() 中，无此窗口）。轮询等待。
	deadline := time.Now().Add(2 * time.Second)
	for task.attempts.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
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

// ---------- Phase 2 新增辅助任务类型 ----------

// blockingCtxRunTask：TaskBlock，Run 阻塞直到 release 关闭或自身 context 被
// 取消——模拟"会观察停止信号"的业务执行。
type blockingCtxRunTask struct {
	Task
	entered chan struct{}
	release chan struct{}
}

func (t *blockingCtxRunTask) Run() error {
	close(t.entered)
	select {
	case <-t.release:
		return nil
	case <-t.Done():
		return t.StopReason()
	}
}

// failThenSucceedTask：TaskBlock，前 failUntil 次 attempt 失败、之后成功；
// Dispose 计数用于校验「每次 attempt 一次 dispose」。
type failThenSucceedTask struct {
	Task
	attempts  atomic.Int32
	disposes  atomic.Int32
	failUntil int32
}

func (t *failThenSucceedTask) Run() error {
	if t.attempts.Add(1) <= t.failUntil {
		return errors.New("run failed")
	}
	return nil
}

func (t *failThenSucceedTask) Dispose() {
	t.disposes.Add(1)
}

// panicThenSucceedTask：TaskBlock，前 panics 次 attempt panic、之后成功。
type panicThenSucceedTask struct {
	Task
	attempts atomic.Int32
	panics   int32
}

func (t *panicThenSucceedTask) Run() error {
	if t.attempts.Add(1) <= t.panics {
		panic("run panic")
	}
	return nil
}

// ---------- 契约 8：Run 执行期间停止信号仍被及时处理【Phase 2 新增：新语义】 ----------
//
// 旧语义下 Run 占住循环：停止请求虽能取消 context（Stop 可从任意 goroutine
// 调用），父循环却要等 Run 返回才能处理其他事件。Phase 2 后循环始终响应；
// 子任务在 Run 观察 context 返回后完成 dispose，父任务随之收敛，且子任务
// dispose 先于父任务 shutdown 落定（C6）。
func TestContract_StopDuringBlockingRunIsPrompt(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	blocker := &blockingCtxRunTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(blocker)
	<-blocker.entered

	// Run 执行期间循环保持响应。
	callDone := callWithin(&job)
	select {
	case <-callDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Call blocked while Run is in flight")
	}

	// 停止父 Job：级联取消 blocker 的 context，Run 观察后返回。
	job.Stop(ErrStopByUser)
	select {
	case <-job.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("job did not stop while Run is in flight")
	}
	if err := job.WaitStopped(); !errors.Is(err, ErrStopByUser) {
		t.Fatalf("expected ErrStopByUser, got %v", err)
	}
	// 子任务 dispose 必须先于父任务 shutdown 落定。
	select {
	case <-blocker.shutdown.Done():
	case <-time.After(time.Second):
		t.Fatal("child dispose did not complete before parent shutdown")
	}
}

// ---------- 契约 9：Run 失败重试序列保持【Phase 2 新增：行为保持类】 ----------
//
// 旧语义（Run 内联在 start 的错误路径里重试）与 Phase 2 后（Run 经信号回到
// 路径 B 重试）必须给出同一条可观测序列：attempt 次数、每次 attempt 一次
// dispose、最终停止原因。本测试在改造前后都应通过。
//
// 注意不能直接等 task.WaitStopped()：重试时 reset() 会换新的 shutdown
// promise，早到的等待者持有的是第一次 attempt 的那个（落定值为首次失败
// 原因）——这是 WaitStopped 对重试任务的一贯语义，与改造无关。改为等 Job
// 自动停止（子任务全部结束）后校验最终状态。
func TestContract_RetryAfterRunFailureKeepsSequence(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	task := &failThenSucceedTask{failUntil: 2}
	task.SetRetry(2, 10*time.Millisecond)
	job.AddTask(task)
	deadline := time.Now().Add(5 * time.Second)
	for task.attempts.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if err := job.WaitStopped(); !errors.Is(err, ErrAutoStop) {
		t.Fatalf("expected ErrAutoStop, got %v", err)
	}
	if got := task.attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts (initial + 2 retries), got %d", got)
	}
	if got := task.disposes.Load(); got != 3 {
		t.Fatalf("expected 3 disposes (one per attempt), got %d", got)
	}
	if err := task.StopReason(); !errors.Is(err, ErrTaskComplete) {
		t.Fatalf("expected final stop reason ErrTaskComplete, got %v", err)
	}
}

// ---------- 契约 10：RunTask 驱动 TaskBlock 的重试【Phase 2 新增：行为保持类】 ----------
//
// RunTask 不经过事件循环。Phase 2 前它靠 start() 内联跑完整个重试序列；
// Phase 2 后 start() 只负责拉起执行 goroutine，RunTask 自己在调用方 goroutine
// 上复刻事件循环的信号驱动协议（<-Done → dispose → checkRetry → reset → start）。
// 可观测结果必须与改造前一致。
func TestContract_RunTaskDrivesRetryForBlockTask(t *testing.T) {
	var owner Job
	root.AddTask(&owner)
	if err := owner.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	task := &failThenSucceedTask{failUntil: 2}
	task.SetRetry(2, 10*time.Millisecond)
	if err := owner.RunTask(task); !errors.Is(err, ErrTaskComplete) {
		t.Fatalf("expected ErrTaskComplete, got %v", err)
	}
	if got := task.attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts (initial + 2 retries), got %d", got)
	}
	if got := task.disposes.Load(); got != 3 {
		t.Fatalf("expected 3 disposes (one per attempt), got %d", got)
	}
	owner.Stop(ErrTaskComplete)
	_ = owner.WaitStopped()
}

// ---------- 契约 11：Run panic 被恢复且可重试【Phase 2 新增：顺带修复的缺陷】 ----------
//
// 旧代码缺陷：Run panic 被 start() 的 defer 恢复后错误路径整个被跳过，任务
// 既不 Stop 也不 dispose（WaitStopped 永久挂起，本测试在旧代码上以超时收场）。
// Phase 2 后 panic 在执行 goroutine 上被 task.run 恢复（与 TaskGo 同构），
// 经 Stop 回到路径 B：dispose → checkRetry → 重试。
func TestContract_RunPanicRecoveredAndRetried(t *testing.T) {
	if ThrowPanic {
		t.Skip("panic propagation is enabled")
	}
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	task := &panicThenSucceedTask{panics: 1}
	task.SetRetry(1, 0)
	job.AddTask(task)
	// 同 C9：不能等 task.WaitStopped()——早到的等待者持有首次 attempt 的
	// shutdown promise，落定值为 panic 原因（重试的既有语义，与改造无关）。
	// 旧代码上 attempts 永远停在 1（panic 后错误路径被跳过，任务被静默丢弃），
	// 这里会以 "expected 2 attempts, got 1" 失败。
	deadline := time.Now().Add(5 * time.Second)
	for task.attempts.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if err := job.WaitStopped(); !errors.Is(err, ErrAutoStop) {
		t.Fatalf("expected ErrAutoStop, got %v", err)
	}
	if got := task.attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts (panic + retry), got %d", got)
	}
	if err := task.StopReason(); !errors.Is(err, ErrTaskComplete) {
		t.Fatalf("expected final stop reason ErrTaskComplete, got %v", err)
	}
}
