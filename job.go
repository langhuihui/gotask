package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/langhuihui/gotask/util"
)

var idG sync.Mutex
var taskIDCounter uint32
var sourceFilePathPrefix string

type ExistTaskError struct {
	Task ITask
}

func (e ExistTaskError) Error() string {
	return fmt.Sprintf("%v exist", e.Task.getKey())
}

func init() {
	if _, file, _, ok := runtime.Caller(0); ok {
		sourceFilePathPrefix = strings.TrimSuffix(file, "job.go")
	}
}

func GetNextTaskID() uint32 {
	idG.Lock()
	defer idG.Unlock()
	taskIDCounter++
	return taskIDCounter
}

// Job 任务容器，可以包含和管理多个子任务
type Job struct {
	Task
	children                    sync.Map
	descendantsDisposeListeners []func(ITask)
	descendantsStartListeners   []func(ITask)
	blocked                     ITask
	eventLoop                   EventLoop
	Size                        atomic.Int32
	// stopTimeout / stopTimeoutDetach 控制「本 Job 等待子任务销毁」的容忍时长与
	// 超时后的动作，由 SetStopTimeout/SetStopTimeoutDetach 在 Job 启动前设置。
	stopTimeout       time.Duration
	stopTimeoutDetach bool
	// failurePolicy 由 SetFailurePolicy 在启动前设置，默认零值=不升级。
	failurePolicy FailurePolicy
}

// SetStopTimeout 设置本 Job 停止时等待每个子任务完成销毁的时限；0（默认）表示
// 无限等待。超时后只记录（Error 日志 + stopTimeout description）并继续等——
// 默认档不改变「子任务先于父任务销毁」的约定（C6）。
//
// 必须在 Job 启动前调用（与 SetEventLoopBufferSize 同约定）。
func (mt *Job) SetStopTimeout(d time.Duration) {
	mt.stopTimeout = d
}

// SetStopTimeoutDetach 决定停止超时到点后的动作。false（默认）＝只记录并继续等；
// true（opt-in）＝放弃等待、父任务继续销毁。**代价**：打破「子任务先于父任务
// 销毁」的约定，父任务的 OnDispose 可能在子任务 shutdown 落定之前跑完。
// 只在业务明确接受该代价时开启。
func (mt *Job) SetStopTimeoutDetach(detach bool) {
	mt.stopTimeoutDetach = detach
}

// SetFailurePolicy 设置子任务失败的处理策略（Phase 3 3B，默认不升级）。
// 必须在 Job 启动前调用。
func (mt *Job) SetFailurePolicy(p FailurePolicy) {
	mt.failurePolicy = p
}

// maybeEscalate 在子任务以失败结束且没有重试余地时，按策略升级为本 Job 的失败。
//
// 走的是既有 Stop 链路：父停止 → 父 dispose → waitChildrenDispose 让兄弟子任务
// 收到同一个原因停下来，不引入新的停止路径。升级是显式的 Stop（不是自动停止），
// 所以 Work 的 keepalive 不豁免它——常驻容器也不该静默吞掉子任务失败。
func (mt *Job) maybeEscalate(child ITask, reason error) {
	if !mt.failurePolicy.Escalate || !isFailureReason(reason) {
		return
	}
	if mt.IsStopped() {
		// 已经在停（用户停止/上一级级联）：不覆盖既有停止原因。
		return
	}
	mt.Warn("escalating child failure", "childId", child.GetTaskID(), "childType", child.GetTaskType(), "ownerType", child.GetOwnerType(), "reason", reason)
	mt.SetDescription("failureEscalated", child.GetTaskID())
	mt.Stop(errors.Join(reason, ErrChildFailed))
}

// isFailureReason reports whether a stop reason means "this task failed" rather
// than a normal lifecycle outcome. ErrTimeout counts as a failure (Phase 3 评审 Q4):
// a timeout is an abnormal end even though it is never retried.
func isFailureReason(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, ErrTaskComplete) && !errors.Is(err, ErrAutoStop) &&
		!errors.Is(err, ErrStopByUser) && !errors.Is(err, ErrExit)
}

func (*Job) GetTaskType() TaskType {
	return TASK_TYPE_JOB
}

func (mt *Job) getJob() *Job {
	return mt
}

func (mt *Job) Blocked() ITask {
	return mt.blocked
}

func (mt *Job) EventLoopRunning() bool {
	return mt.eventLoop.running.Load()
}

// SetEventLoopBufferSize sets the capacity for the job's event loop channel.
// Call before the job starts to ensure the new buffer size is used.
func (mt *Job) SetEventLoopBufferSize(size int) {
	mt.eventLoop.SetBufferSize(size)
}

func (mt *Job) waitChildStopped(child ITask) error {
	// 等的必须是子任务的 shutdown——dispose 整个跑完（含 OnDispose 钩子）之后
	// 才落定的那个 promise。不能图省事用 child.WaitStopped()：它开头的
	// WaitStarted() 在 startup 被 Reject 时带着错误提前返回，根本不等 dispose。
	// 子任务的 start 失败路径是先 Reject startup、再跑 dispose 的，父任务从
	// 这里提前返回，自己的 OnDispose 就会与子任务的 dispose 并发甚至抢先执行，
	// 打破"子任务先于父任务销毁"的约定（Test_ParentStop 实测父钩子跑到了子钩子
	// 前面）。startup 正常落定的任务两条路等价（都要等 shutdown），受影响的只有
	// 被拒绝的那一类。
	done := make(chan error, 1)
	go func() {
		done <- child.GetTask().shutdown.Await()
	}()

	// 心跳与超时是两件事：心跳（30s）是常态可观测性；stopTimeout 是业务配置的
	// 容忍上限，默认 0 = 无限等（与 Phase 3 之前的语义一致）。
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()
	var deadline <-chan time.Time
	if mt.stopTimeout > 0 {
		timer := time.NewTimer(mt.stopTimeout)
		defer timer.Stop()
		deadline = timer.C
	}
	for {
		select {
		case err := <-done:
			return err
		case <-heartbeat.C:
			mt.Debug(
				"wait child dispose stalled",
				"childId", child.GetTaskID(),
				"childType", child.GetTaskType(),
				"childOwner", child.GetOwnerType(),
				"childState", child.GetState(),
				"childDescriptions", child.GetDescriptions(),
			)
		case <-deadline:
			// 只报一次：超时后关掉这个分支，避免每个心跳都刷 Error。
			deadline = nil
			mt.Error(
				"wait child dispose timeout",
				"childId", child.GetTaskID(),
				"childType", child.GetTaskType(),
				"childOwner", child.GetOwnerType(),
				"childState", child.GetState(),
				"timeout", mt.stopTimeout,
				"childDescriptions", child.GetDescriptions(),
			)
			mt.SetDescription("stopTimeout", mt.stopTimeout.String())
			if mt.stopTimeoutDetach {
				// opt-in：放弃等待，父任务继续销毁。代价是打破 C6 ——父的
				// OnDispose 可能先于子任务的 shutdown 落定。返回 ErrTimeout 只是
				// 让外层的 Debug 日志带上原因，不作为停止原因传播。
				mt.SetDescription("stopTimeoutDetach", child.GetTaskID())
				mt.Warn("giving up waiting for child dispose", "childId", child.GetTaskID(), "timeout", mt.stopTimeout)
				return ErrTimeout
			}
		}
	}
}

func (mt *Job) waitChildrenDispose(stopReason error) {
	mt.Debug("wait children dispose begin", "reason", stopReason, "childCount", mt.Size.Load())
	defer mt.Debug("wait children dispose end", "reason", stopReason, "childCount", mt.Size.Load())
	mt.eventLoop.active(mt)
	mt.children.Range(func(key, value any) bool {
		child := value.(ITask)
		mt.Debug(
			"wait child dispose begin",
			"childId", child.GetTaskID(),
			"childType", child.GetTaskType(),
			"childOwner", child.GetOwnerType(),
			"childState", child.GetState(),
			"childDescriptions", child.GetDescriptions(),
		)
		child.Stop(stopReason)
		mt.Debug(
			"wait child stop signaled",
			"childId", child.GetTaskID(),
			"childType", child.GetTaskType(),
			"childOwner", child.GetOwnerType(),
			"childState", child.GetState(),
			"childStopReason", child.StopReason(),
		)
		mt.SetDescription("waitChildDispose", child.GetTaskID())
		err := mt.waitChildStopped(child)
		mt.Debug(
			"wait child dispose end",
			"childId", child.GetTaskID(),
			"childType", child.GetTaskType(),
			"childOwner", child.GetOwnerType(),
			"childState", child.GetState(),
			"childStopReason", child.StopReason(),
			"err", err,
			"childDescriptions", child.GetDescriptions(),
		)
		mt.RemoveDescription("waitChildDispose")
		return true
	})
}

// OnDescendantsDispose 注册后代任务销毁时的回调。
//
// 与 Task 上那四个钩子同理，用内嵌 Task 的 hookMu 保护——注册端在业务
// goroutine，消费端在事件循环 goroutine。这里的波及面还更大一些：
// onDescendantsDispose 会一路向上传播到祖先，也就是**祖先的切片会被后代的事件
// 循环 goroutine 读**。向上传播时取的是各自 Job 的锁，只上行不回环，不会死锁。
func (mt *Job) OnDescendantsDispose(listener func(ITask)) {
	mt.hookMu.Lock()
	defer mt.hookMu.Unlock()
	mt.descendantsDisposeListeners = append(mt.descendantsDisposeListeners, listener)
}

func (mt *Job) onDescendantsDispose(descendants ITask) {
	mt.hookMu.Lock()
	listeners := mt.descendantsDisposeListeners
	mt.hookMu.Unlock()
	for _, listener := range listeners {
		listener(descendants)
	}
	if mt.parent != nil {
		mt.parent.onDescendantsDispose(descendants)
	}
}

func (mt *Job) onChildDispose(child ITask) {
	mt.onDescendantsDispose(child)
	child.dispose()
}

func (mt *Job) removeChild(child ITask) {
	if mt.children.CompareAndDelete(child.getKey(), child) {
		remains := mt.Size.Add(-1)
		mt.Debug("remove child", "id", child.GetTaskID(), "remains", remains)
	}
}

// OnDescendantsStart 注册后代任务启动时的回调，同步方式见 OnDescendantsDispose。
func (mt *Job) OnDescendantsStart(listener func(ITask)) {
	mt.hookMu.Lock()
	defer mt.hookMu.Unlock()
	mt.descendantsStartListeners = append(mt.descendantsStartListeners, listener)
}

func (mt *Job) onDescendantsStart(descendants ITask) {
	mt.hookMu.Lock()
	listeners := mt.descendantsStartListeners
	mt.hookMu.Unlock()
	for _, listener := range listeners {
		listener(descendants)
	}
	if mt.parent != nil {
		mt.parent.onDescendantsStart(descendants)
	}
}

func (mt *Job) onChildStart(child ITask) {
	mt.onDescendantsStart(child)
}

func (mt *Job) RangeSubTask(callback func(task ITask) bool) {
	mt.children.Range(func(key, value any) bool {
		return callback(value.(ITask))
	})
}

func (mt *Job) AddDependTask(t ITask, opt ...any) (task *Task) {
	t.Using(mt)
	opt = append(opt, 1)
	return mt.AddTask(t, opt...)
}

// initContext 为一个新入队（或重入队）的任务补齐全部按任务初始化的字段。
//
// 整个函数体在 task.stopMu 保护下执行：这些字段（Context/CancelCauseFunc/
// startup/shutdown/handler/ID/Logger……）会被任意 goroutine 的 Stop() 经
// Debug 变参饥饿求值读到（Test_TaskStateRace 的注释预警过这类读取），而
// Stop() 自己持 stopMu——写入端不拿同一把锁，那些读取就是数据竞争
// （Test_Block 的定时器 goroutine 与 AddTask 曾实测竞争 startup 及其
// promise 内部）。本函数内没有用户代码（getKey 由调用方在锁外调用），
// 持锁时间只有字段赋值与 NewPromise。
//
// handler 也在这里赋值：调用方（AddTask/RunTask）原先在锁外写它，同样
// 与 Stop() 的 Debug 读取竞争。
func (mt *Job) initContext(task *Task, handler ITask, opt ...any) {
	task.stopMu.Lock()
	defer task.stopMu.Unlock()
	callDepth := 2
	for _, o := range opt {
		switch v := o.(type) {
		case context.Context:
			task.parentCtx = v
		case Description:
			task.SetDescriptions(v)
		case RetryConfig:
			task.retry = v
		case *slog.Logger:
			task.Logger = v
		case int:
			callDepth += v
		}
	}
	_, file, line, ok := runtime.Caller(callDepth)
	if ok {
		task.StartReason = fmt.Sprintf("%s:%d", strings.TrimPrefix(file, sourceFilePathPrefix), line)
	}
	task.handler = handler
	task.parent = mt
	if task.parentCtx == nil {
		task.parentCtx = mt.Context
	}
	task.level = mt.level + 1
	if task.ID == 0 {
		task.ID = GetNextTaskID()
	}
	task.Context, task.CancelCauseFunc = context.WithCancelCause(task.parentCtx)
	// Stop() 可能在 AddTask 之前就被调用（那时 CancelCauseFunc 还是 nil，
	// 停止请求只被记住）。这里补上取消，让新 context 出生即已停止；否则
	// start() 会把任务当正常任务跑完整个生命周期，且它的信号通道永远不会
	// 关闭，事件循环与 WaitStopped 一起永久挂住。
	startupCtx := task.Context
	if task.stopped {
		task.CancelCauseFunc(task.stopErr)
		// context 已出生即取消，startup promise 不能再挂在它上面：promise
		// 的 Fulfill 走 CancelCauseFunc，对已取消的父 context 是空操作，
		// WaitStarted 会立刻返回停止原因，WaitStopped 随之跳过 shutdown
		// 等待——而 OnStart/OnDispose 钩子还没来得及跑（它们由事件循环
		// 后续执行），调用方却已经拿到返回值。Test_Hooks 正是这条路径。
		startupCtx = context.Background()
	}
	task.startup = util.NewPromise(startupCtx)
	task.shutdown = util.NewPromise(context.Background())
	// 退避握手通道随生命周期重建：任务对象可能被重新 AddTask（initContext 再跑
	// 一次），上一轮的等待者、令牌与退出信号必须一并作废，见 armRetry/reset。
	if task.retryCancel != nil {
		close(task.retryCancel)
		task.retryCancel = nil
	}
	task.retrying.Store(false)
	task.retryCh = make(chan struct{}, 1)
	task.attemptDone = nil
	if task.Logger == nil {
		task.Logger = mt.Logger
	}
}

func (mt *Job) AddTask(t ITask, opt ...any) (task *Task) {
	task = t.GetTask()
	mt.initContext(task, t, opt...)
	if mt.IsStopped() {
		mt.Warn("[fix1] AddTask rejected: parent already stopped", "key", t.getKey(), "newId", task.ID, "reason", mt.StopReason())
		task.startup.Reject(mt.StopReason())
		task.CancelCauseFunc(mt.StopReason()) // also cancel task.ctx so IsStopped() returns true
		return
	}
	actual, loaded := mt.children.LoadOrStore(t.getKey(), t)
	if loaded {
		existingTask := actual.(ITask)
		if existingTask.GetState() >= TASK_STATE_DISPOSING || existingTask.IsStopped() {
			// Existing task is stopped/disposing/disposed, replace it with the new one
			if !mt.children.CompareAndSwap(t.getKey(), actual, t) {
				// CAS failed: old task was already removed by removeChild, retry LoadOrStore
				if actual, loaded = mt.children.LoadOrStore(t.getKey(), t); loaded {
					mt.Warn("[fix1] AddTask rejected: CAS failed, key still occupied", "key", t.getKey(), "newId", task.ID, "existingId", actual.(ITask).GetTask().ID)
					task.startup.Reject(ExistTaskError{
						Task: actual.(ITask),
					})
					task.CancelCauseFunc(ExistTaskError{Task: actual.(ITask)}) // also cancel task.ctx
					return
				}
			} else {
				// CAS succeeded: compensate Size for the replaced task whose removeChild
				// CompareAndDelete will fail (map value is now the new task)
				mt.Size.Add(-1)
			}
		} else {
			mt.Warn("[fix1] AddTask rejected: key conflict, existing task still running", "key", t.getKey(), "newId", task.ID, "existingId", existingTask.GetTask().ID)
			task.startup.Reject(ExistTaskError{
				Task: existingTask,
			})
			task.CancelCauseFunc(ExistTaskError{Task: existingTask}) // also cancel task.ctx so IsStopped() returns true
			return
		}
	}
	var err error
	defer func() {
		if err != nil {
			mt.children.Delete(t.getKey())
			mt.Warn("[fix1] AddTask rejected: eventLoop/stopped after insert", "key", t.getKey(), "newId", task.ID, "reason", err)
			task.startup.Reject(err)
			task.CancelCauseFunc(err) // also cancel task.ctx so IsStopped() returns true
		}
	}()
	if err = mt.eventLoop.add(mt, t); err != nil {
		// add 失败意味着任务永远不会进入事件循环：dispose 不会跑，shutdown
		// 没有别的落定机会。父任务销毁时 waitChildStopped 等的就是 shutdown
		// （见那里），不落定就是永久挂起。这里补上——任务虽没跑起来，生命
		// 周期却已经终结，两个 promise 都必须兑现。（startup 由下面的 defer
		// 落定；post-add 的 IsStopped 分支不归这里管——消息已在通道里，循环
		// 最终会处理它并跑 dispose，由 dispose 落定 shutdown。）
		task.shutdown.Fulfill(err)
		return
	}
	if mt.IsStopped() {
		err = mt.StopReason()
		return
	}
	remains := mt.Size.Add(1)
	mt.Debug("child added", "id", task.ID, "remains", remains)
	return
}

func (mt *Job) Call(callback func()) {
	_, file, line, _ := runtime.Caller(1)
	caller := fmt.Sprintf("%s:%d", strings.TrimPrefix(file, sourceFilePathPrefix), line)
	if mt.Size.Load() <= 0 {
		mt.Debug("call immediately", "caller", caller)
		startTime := time.Now()
		callback()
		mt.Debug("call immediately done", "caller", caller, "elapsed", time.Since(startTime))
		return
	}
	ctx, cancel := context.WithCancel(mt)
	defer cancel()
	if err := mt.eventLoop.add(mt, func() {
		startTime := time.Now()
		mt.Debug("call", "caller", caller)
		defer cancel()
		callback()
		mt.Debug("call done", "caller", caller, "elapsed", time.Since(startTime))
	}); err != nil {
		mt.Warn("call rejected", "caller", caller, "reason", err)
		cancel()
	}
	<-ctx.Done()
}
