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
//
// Phase 3（3A 重试与监督分离）设计见 docs/重构-执行模型v2-Phase3设计.md：
//   - C5 已按新语义改写：退避不再占用 Owner 循环（旧语义被本测试在旧代码上捕获）；
//   - C12「退避中 Stop 及时收敛」/ C13「ShouldRetry 谓词生效」/
//     C14「Restart() 触发重试」/ C15「ErrTimeout 不重试」为本轮新增；
//   - C9/C10/C11 的重试序列必须继续通过（本轮只改等待位置，不改序列）。
//
// Phase 3（3B 生命周期超时与故障升级）：
//   - C16「停止超时默认只记录、继续等」/ C17「detach 为 opt-in」/
//     C18「销毁超时看门狗只记录不中断」/ C19「故障升级为 opt-in」/
//     C20「默认策略不升级」为本轮新增；默认配置下不得改变任何既有语义。
//
// Phase 4（外围）：C21「启动时间快照可跨 goroutine 读」（4B，dashboard 竞态）；
// C22「泛型函数可挂进任务树」/ C23「任务树与泛型的退避公式一致（含首轮封顶）」/
// C24「类型化 option 与裸值等价」（4C/4D）。

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

// waitUntil 轮询等待条件成立（超时即失败）。重试路径上的 attempt 由退避定时器与
// 执行 goroutine 驱动，固定 sleep 会引入调度窗口，一律轮询。
func waitUntil(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s (timeout %v)", msg, timeout)
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
	// v2：原 C2（「TaskGo 不占用父事件循环」）已随 TaskGo 删除并入本条——
	// v2 起 Run 是唯一执行入口，不存在第二条需要单独验证的执行路径。
	sibling := &blockingRunTask{entered: make(chan struct{}), release: make(chan struct{})}
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

// ---------- 契约 2：已删除【v2：TaskGo 不存在，语义并入 C1】 ----------
//
// v1 的 C2 验证「TaskGo 不占用父事件循环」。v2 删除 TaskGo 之后这条契约失去
// 被测对象——Run 是唯一执行入口，其循环外执行由 C1 覆盖。编号保留（不重排），
// 以免既有文档里的 C 编号指向漂移。

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

// ---------- 契约 5：重试退避不占用 Owner 事件循环【Phase 3 改写：旧语义为「占用」】 ----------
//
// 旧语义：checkRetry 的退避等待跑在 Job 自己的事件循环上，期间同 Job 的 Call
// 被堵住（本测试在 Phase 3 前的代码上以 "Call blocked during retry backoff"
// 失败）。Phase 3 后退避交给定时器，循环在等待期间继续处理其它事件，重试序列
// 与次数不变。
func TestContract_RetryBackoffDoesNotOccupyOwnerLoop(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	task := &retryFailTask{}
	task.SetRetry(1, 300*time.Millisecond)
	job.AddTask(task)

	// 等第一次 attempt 失败并进入退避（dispose 已完成、尚未进入下一轮）。
	waitUntil(t, 3*time.Second, func() bool {
		return task.attempts.Load() == 1 && task.GetState() == TASK_STATE_DISPOSED
	}, "task did not enter retry backoff")

	// 退避期间同 Job 的 Call 立即完成。
	begin := time.Now()
	callDone := callWithin(&job)
	select {
	case <-callDone:
		if elapsed := time.Since(begin); elapsed > 150*time.Millisecond {
			t.Fatalf("Call took %v during retry backoff: backoff must not occupy the owner loop", elapsed)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Call blocked during retry backoff: backoff must not occupy the owner loop")
	}

	// 退避到期后照常重试，序列不变。
	waitUntil(t, 3*time.Second, func() bool { return task.attempts.Load() == 2 }, "retry attempt did not run after backoff")
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
	children := make([]*blockingRunTask, childCount)
	for j := range children {
		children[j] = &blockingRunTask{entered: make(chan struct{}), release: make(chan struct{})}
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

// ---------- Phase 3（3A）新增辅助任务类型 ----------

// restartableTask：TaskBlock，每轮 Run 阻塞到自身 context 被取消后返回停止原因，
// 用于验证 Restart() 能否拉起新的一轮。
type restartableTask struct {
	Task
	attempts atomic.Int32
}

func (t *restartableTask) Run() error {
	t.attempts.Add(1)
	<-t.Done()
	return t.StopReason()
}

// timeoutFailTask：TaskBlock，每轮 Run 立刻以 ErrTimeout 结束。
type timeoutFailTask struct {
	Task
	attempts atomic.Int32
}

func (t *timeoutFailTask) Run() error {
	t.attempts.Add(1)
	return ErrTimeout
}

// ---------- 契约 12：退避期间停止，子任务被及时摘除【Phase 3 新增】 ----------
//
// C5 翻转后，退避不再由循环内联等待，循环必须能被「退避中的子任务被停止」唤醒：
// 否则它会为一个已停止的任务挂到退避结束（10s 退避会把收敛拖到 10s 之后）。
// Phase 3 前本测试同样通过（内联 sleep 靠 parent.Done 逃逸），但收敛路径不同；
// 改造后由 cancelRetry 令牌驱动。
func TestContract_StopDuringRetryBackoffIsPrompt(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	task := &retryFailTask{}
	task.SetRetry(-1, 10*time.Second) // 只有主动取消才能及时收敛
	job.AddTask(task)
	waitUntil(t, 3*time.Second, func() bool {
		return task.attempts.Load() == 1 && task.GetState() == TASK_STATE_DISPOSED
	}, "task did not enter retry backoff")

	job.Stop(ErrStopByUser)
	if err := job.WaitStopped(); !errors.Is(err, ErrStopByUser) {
		t.Fatalf("expected ErrStopByUser, got %v", err)
	}
	// 退避中的子任务必须被立刻摘除，而不是等完 10s 退避再继续重试。
	waitUntil(t, 2*time.Second, func() bool { return job.Size.Load() == 0 }, "retrying child was not removed on stop")
	if got := task.attempts.Load(); got != 1 {
		t.Fatalf("expected 1 attempt (stop cancelled the pending retry), got %d", got)
	}
}

// ---------- 契约 13：ShouldRetry 谓词每次失败都被询问【Phase 3 新增】 ----------
//
// 谓词与预算是正交的两件事：预算是「还有没有额度」，谓词是「这个错误值不值得」。
// 谓词拒绝时不消耗额度、不重试；允许时照常消耗额度重试。
func TestContract_ShouldRetryPredicateHonored(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	task := &retryFailTask{}
	task.SetRetry(3, 0) // 预算充足：拦截只能来自谓词
	// 只放行第一次失败：第 2 次失败时谓词拒绝。
	task.SetShouldRetry(func(error) bool { return task.attempts.Load() == 1 })
	job.AddTask(task)

	waitUntil(t, 3*time.Second, func() bool { return task.attempts.Load() >= 2 }, "predicate-allowed retry did not run")
	// 零退避下 attempt 可能在两次轮询之间跨过多轮，用「留足时间后看最终值」判定：
	// 谓词拒绝第二次失败，第 3 轮不得发生（被忽略时会看到 3 或 4）。
	time.Sleep(300 * time.Millisecond)
	if got := task.attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts (initial + predicate-allowed retry), got %d", got)
	}
	if got := task.GetRetryCount(); got != 1 {
		t.Fatalf("expected retryCount 1 (rejected attempt must not spend budget), got %d", got)
	}
	if err := job.WaitStopped(); !errors.Is(err, ErrAutoStop) {
		t.Fatalf("expected ErrAutoStop, got %v", err)
	}
}

// ---------- 契约 14：Restart() 触发下一轮【Phase 3 新增】 ----------
//
// ErrRestart 此前只有消费者没有生产者（F21），本契约锁定新补的 Task.Restart()：
// 结束本轮、立即重跑、消耗一次重试预算、不进指数退避。
func TestContract_RestartRequestTriggersRetry(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	task := &restartableTask{}
	task.SetRetry(-1, 0) // Restart 受 MaxRetry 约束，这里给无界预算
	job.AddTask(task)
	waitUntil(t, 3*time.Second, func() bool { return task.attempts.Load() == 1 }, "task did not start")

	task.Restart()
	waitUntil(t, 3*time.Second, func() bool { return task.attempts.Load() == 2 }, "Restart did not trigger a new attempt")

	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
	if got := task.GetRetryCount(); got < 1 {
		t.Fatalf("Restart must consume a retry slot, retryCount=%d", got)
	}
}

// ---------- 契约 15：ErrTimeout 不重试【Phase 3 新增：语义统一】 ----------
//
// Phase 3 前任务树只把完成/退出/用户停止视为终态，ErrTimeout 会被无限重试；
// generic 侧（shouldRetryGeneric）却把它排除在外。Phase 3 统一为「不重试」，
// 与 generic.WithTimeout 的语义一致（且 ErrTimeout 视为失败，供故障升级使用）。
func TestContract_TimeoutIsNotRetried(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	task := &timeoutFailTask{}
	task.SetRetry(-1, 0) // 若不把 ErrTimeout 视为终态，这里会无限热重试
	job.AddTask(task)

	select {
	case <-job.Done():
	case <-time.After(3 * time.Second):
		t.Fatalf("job did not stop: ErrTimeout must not be retried (attempts=%d)", task.attempts.Load())
	}
	if got := task.attempts.Load(); got != 1 {
		t.Fatalf("expected 1 attempt, got %d", got)
	}
	if err := job.WaitStopped(); !errors.Is(err, ErrAutoStop) {
		t.Fatalf("expected ErrAutoStop, got %v", err)
	}
}

// ---------- Phase 3（3B）新增辅助任务类型 ----------

// slowDisposeTask：TaskBlock，Run 阻塞到 context 取消；Dispose 阻塞到
// disposeRelease 关闭。用于观察「父等子销毁」的时序与超时策略。
type slowDisposeTask struct {
	Task
	entered        chan struct{} // Run 已进入
	disposeEntered chan struct{} // Dispose 已开始
	disposeRelease chan struct{} // 关闭后 Dispose 返回
	disposeDone    chan struct{} // Dispose 已返回
}

func (t *slowDisposeTask) Run() error {
	close(t.entered)
	<-t.Done()
	return t.StopReason()
}

func (t *slowDisposeTask) Dispose() {
	close(t.disposeEntered)
	<-t.disposeRelease
	close(t.disposeDone)
}

func newSlowDisposeTask() *slowDisposeTask {
	return &slowDisposeTask{
		entered:        make(chan struct{}),
		disposeEntered: make(chan struct{}),
		disposeRelease: make(chan struct{}),
		disposeDone:    make(chan struct{}),
	}
}

// ---------- 契约 16：停止超时默认只记录、继续等待【Phase 3 3B 新增】 ----------
//
// 默认档（不 detach）下，超时只产生 Error 日志 + stopTimeout description，
// 「子任务先于父任务销毁」的约定（C6）不受影响：父的 shutdown 仍然等到子任务
// dispose 完成才落定。
func TestContract_StopTimeoutRecordsAndKeepsWaiting(t *testing.T) {
	var parent Job
	parent.SetStopTimeout(50 * time.Millisecond)
	root.AddTask(&parent)
	if err := parent.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	child := newSlowDisposeTask()
	parent.AddTask(child)
	<-child.entered

	parent.Stop(ErrStopByUser)
	<-child.disposeEntered
	// 超过 stopTimeout：记录，但不放弃等待。
	waitUntil(t, 2*time.Second, func() bool {
		_, ok := parent.GetDescription("stopTimeout")
		return ok
	}, "stopTimeout was not recorded")
	select {
	case <-parent.shutdown.Done():
		t.Fatal("parent stopped waiting on the child by default: C6 must hold without opt-in detach")
	default:
	}

	close(child.disposeRelease)
	if err := parent.WaitStopped(); !errors.Is(err, ErrStopByUser) {
		t.Fatalf("expected ErrStopByUser, got %v", err)
	}
}

// ---------- 契约 17：停止超时 detach 为 opt-in【Phase 3 3B 新增】 ----------
//
// 开启 SetStopTimeoutDetach(true) 后，超时就放弃等待：父任务的 shutdown 在子任务
// dispose 完成之前落定（明确打破 C6，代价由使用方显式承担）。
func TestContract_StopTimeoutDetachOptIn(t *testing.T) {
	var parent Job
	parent.SetStopTimeout(50 * time.Millisecond)
	parent.SetStopTimeoutDetach(true)
	root.AddTask(&parent)
	if err := parent.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	child := newSlowDisposeTask()
	parent.AddTask(child)
	<-child.entered

	parent.Stop(ErrStopByUser)
	<-child.disposeEntered
	select {
	case <-parent.shutdown.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("parent kept waiting after stop timeout despite detach")
	}
	// 判定 detach 确实生效：此刻子任务还在 Dispose 里。
	select {
	case <-child.disposeDone:
		t.Fatal("child dispose finished before parent shutdown: detach did not take effect")
	default:
	}
	close(child.disposeRelease)
	<-child.disposeDone
}

// ---------- 契约 18：销毁超时看门狗只记录、不中断【Phase 3 3B 新增】 ----------
func TestContract_DisposeTimeoutWatchdog(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	task := newSlowDisposeTask()
	task.SetDisposeTimeout(50 * time.Millisecond)
	job.AddTask(task)
	<-task.entered

	job.Stop(ErrStopByUser)
	<-task.disposeEntered
	waitUntil(t, 2*time.Second, func() bool {
		v, ok := task.GetDescription("disposeTimeout")
		flagged, _ := v.(bool)
		return ok && flagged
	}, "dispose watchdog did not fire")
	// 只记录不中断：Dispose 仍卡在自己的实现里。
	if got := task.GetState(); got != TASK_STATE_DISPOSING {
		t.Fatalf("expected DISPOSING while Dispose is blocked, got %v", got)
	}
	close(task.disposeRelease)
	_ = job.WaitStopped()
	if got := task.GetState(); got != TASK_STATE_DISPOSED {
		t.Fatalf("expected DISPOSED after Dispose returned, got %v", got)
	}
}

// ---------- 契约 19：故障升级为 opt-in【Phase 3 3B 新增】 ----------
//
// 开启 FailurePolicy{Escalate:true} 后，子任务失败且无重试余地 → 父任务以
// errors.Join(childReason, ErrChildFailed) 停止，兄弟子任务随级联停止。
func TestContract_FailureEscalationOptIn(t *testing.T) {
	t.Run("event loop", func(t *testing.T) {
		var job Job
		job.SetFailurePolicy(FailurePolicy{Escalate: true})
		root.AddTask(&job)
		if err := job.WaitStarted(); err != nil {
			t.Fatal(err)
		}
		sibling := &blockingCtxRunTask{entered: make(chan struct{}), release: make(chan struct{})}
		job.AddTask(sibling)
		<-sibling.entered

		failing := &retryFailTask{}
		job.AddTask(failing)

		select {
		case <-job.Done():
		case <-time.After(3 * time.Second):
			t.Fatal("job did not escalate a child failure")
		}
		if err := job.WaitStopped(); !errors.Is(err, ErrChildFailed) {
			t.Fatalf("expected escalation reason to contain ErrChildFailed, got %v", err)
		}
		if err := job.StopReason(); !errors.Is(err, ErrChildFailed) {
			t.Fatalf("job stop reason lost ErrChildFailed: %v", err)
		}
		if got, ok := job.GetDescription("failureEscalated"); !ok || got == nil {
			t.Fatal("failureEscalated description not recorded")
		}
		// 兄弟任务被级联停止。
		select {
		case <-sibling.shutdown.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("sibling was not stopped by escalation")
		}
	})

	t.Run("RunTask", func(t *testing.T) {
		var owner Job
		owner.SetFailurePolicy(FailurePolicy{Escalate: true})
		root.AddTask(&owner)
		if err := owner.WaitStarted(); err != nil {
			t.Fatal(err)
		}
		child := &retryFailTask{}
		if err := owner.RunTask(child); err == nil {
			t.Fatal("expected RunTask to report the child failure")
		}
		select {
		case <-owner.Done():
		case <-time.After(3 * time.Second):
			t.Fatal("owner did not escalate a RunTask child failure")
		}
		if err := owner.WaitStopped(); !errors.Is(err, ErrChildFailed) {
			t.Fatalf("expected escalation reason to contain ErrChildFailed, got %v", err)
		}
	})
}

// ---------- 契约 21：启动时间快照可跨 goroutine 读【Phase 4 4B 新增】 ----------
//
// 公开字段 StartTime 写在事件循环（或 RunTask 的调用方）goroutine 上，其他
// goroutine 直接读它是数据竞争（dashboard 曾长期这么读）。GetStartTime/Elapsed
// 读的是 startNano 原子快照，本契约在 -race 下从测试 goroutine 读它，
// 同时锁定「未启动任务返回零值 / 0」。
func TestContract_StartTimeSnapshot(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	task := &blockingCtxRunTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(task)
	<-task.entered

	done := make(chan time.Time, 1)
	go func() { done <- task.GetStartTime() }()
	var snapshot time.Time
	select {
	case snapshot = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("GetStartTime blocked")
	}
	if snapshot.IsZero() {
		t.Fatal("GetStartTime returned the zero time for a started task")
	}
	// 系统时钟有粒度（Windows 上实测刚启动的 Elapsed() 可能正好为 0），
	// 轮询等它变正，而不是要求立刻 > 0。
	waitUntil(t, time.Second, func() bool { return task.Elapsed() > 0 }, "Elapsed() never became positive for a running task")

	var unstarted Task
	if !unstarted.GetStartTime().IsZero() || unstarted.Elapsed() != 0 {
		t.Fatalf("unstarted task must report a zero start time, got %v / %v", unstarted.GetStartTime(), unstarted.Elapsed())
	}

	close(task.release)
	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
}

// ---------- 契约 20：默认策略下子任务失败不升级【Phase 3 3B 新增】 ----------
func TestContract_FailurePolicyDefaultKeepsRunning(t *testing.T) {
	var job Job // 未设置 FailurePolicy
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	sibling := &blockingCtxRunTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(sibling)
	<-sibling.entered

	failing := &retryFailTask{}
	job.AddTask(failing)

	// 给失败处理留足时间：默认策略下父 Job 必须继续运行。
	time.Sleep(300 * time.Millisecond)
	select {
	case <-job.Done():
		t.Fatal("job stopped on child failure without escalation policy")
	default:
	}
	select {
	case <-sibling.shutdown.Done():
		t.Fatal("sibling was stopped without escalation policy")
	default:
	}
	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
}
