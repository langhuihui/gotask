package task

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/langhuihui/gotask/util"
)

const TraceLevel = slog.Level(-8)
const OwnerTypeKey = "ownerType"
const DefaultEventLoopBufferSize = 256

var (
	ErrAutoStop        = errors.New("auto stop")
	ErrRetryRunOut     = errors.New("retry out")
	ErrStopByUser      = errors.New("stop by user")
	ErrRestart         = errors.New("restart")
	ErrTaskComplete    = errors.New("complete")
	ErrTimeout         = errors.New("timeout")
	ErrExit            = errors.New("exit")
	ErrPanic           = errors.New("panic")
	ErrTooManyChildren = errors.New("too many children in job")
	ErrDisposed        = errors.New("disposed")
)

const (
	TASK_STATE_INIT TaskState = iota
	TASK_STATE_STARTING
	TASK_STATE_STARTED
	TASK_STATE_RUNNING
	TASK_STATE_GOING
	TASK_STATE_DISPOSING
	TASK_STATE_DISPOSED
)

const (
	TASK_TYPE_TASK TaskType = iota
	TASK_TYPE_JOB
	TASK_TYPE_Work
	TASK_TYPE_CHANNEL
)

type (
	TaskState byte
	TaskType  byte
	ITask     interface {
		context.Context
		keepalive() bool
		GetParent() ITask
		GetTask() *Task
		GetTaskID() uint32
		GetSignal() any
		Stop(error)
		StopReason() error
		start() bool
		dispose()
		checkRetry(error) bool
		reset()
		IsStopped() bool
		GetTaskType() TaskType
		GetOwnerType() string
		GetDescriptions() map[string]string
		SetDescription(key string, value any)
		SetDescriptions(value Description)
		SetRetry(maxRetry int, retryInterval time.Duration)
		Using(resource ...any)
		OnStop(any)
		OnStart(func())
		OnDispose(func())
		GetState() TaskState
		GetLevel() byte
		WaitStopped() error
		WaitStarted() error
		getKey() any
	}
	IJob interface {
		ITask
		getJob() *Job
		AddTask(ITask, ...any) *Task
		RangeSubTask(func(yield ITask) bool)
		OnDescendantsDispose(func(ITask))
		OnDescendantsStart(func(ITask))
		Blocked() ITask
		EventLoopRunning() bool
		Call(func())
	}
	IChannelTask interface {
		ITask
		Tick(any)
	}
	TaskStarter interface {
		Start() error
	}
	TaskDisposal interface {
		Dispose()
	}
	// TaskPreDisposal is called before waiting for child tasks to stop.
	// Implement this on tasks that hold resources (e.g. ring buffer write locks)
	// that must be released so child goroutines can unblock and terminate.
	TaskPreDisposal interface {
		PreDispose()
	}
	TaskBlock interface {
		Run() error
	}
	TaskGo interface {
		Go() error
	}
	RetryConfig struct {
		MaxRetry         int
		RetryCount       int
		RetryInterval    time.Duration // Base interval for exponential backoff
		MaxRetryInterval time.Duration // Maximum interval (0 means no limit)
	}
	Description    = map[string]any
	TaskContextKey string
	Task           struct {
		ID          uint32
		StartTime   time.Time
		StartReason string
		Logger      *slog.Logger
		context.Context
		context.CancelCauseFunc
		handler                                    ITask
		retry                                      RetryConfig
		hookMu                                     sync.Mutex // 保护下面这组钩子字段，见 OnStart
		startHooksRun, disposeHooksRun             bool       // 本轮的对应遍历是否已经走过
		catchingUpStart, catchingUpDispose         bool       // 是否已有 goroutine 在排空补跑队列
		pendingStart, pendingDispose               []func()   // 错过本轮遍历、等待补跑的监听器
		afterStartListeners, afterDisposeListeners []func()
		closeOnStop                                []any
		resources                                  []any
		stopOnce                                   sync.Once
		description                                sync.Map
		startup, shutdown                          *util.Promise
		parent                                     *Job
		parentCtx                                  context.Context
		state                                      atomic.Uint32 // a TaskState; see GetState/setState
		level                                      byte
		loopGen                                    atomic.Uint32
	}
)

func FromPointer(pointer uintptr) *Task {
	return (*Task)(unsafe.Pointer(pointer))
}

func (*Task) keepalive() bool {
	return false
}

// GetState reports the task's lifecycle state. It is safe to call from any
// goroutine: the state is written by whichever event loop owns the task, and
// read by others (the parent loop during teardown, AddTask when resolving a key
// conflict, WorkCollection lookups).
func (task *Task) GetState() TaskState {
	return TaskState(task.state.Load())
}

func (task *Task) setState(state TaskState) {
	task.state.Store(uint32(state))
}

func (task *Task) GetLevel() byte {
	return task.level
}

func (task *Task) GetParent() ITask {
	if task.parent != nil {
		return task.parent.handler
	}
	return nil
}

func (task *Task) SetRetry(maxRetry int, retryInterval time.Duration) {
	task.retry.MaxRetry = maxRetry
	task.retry.RetryInterval = retryInterval
}

// SetMaxRetryInterval sets the maximum retry interval for exponential backoff
// If set to 0, there is no maximum limit (default behavior)
func (task *Task) SetMaxRetryInterval(maxRetryInterval time.Duration) {
	task.retry.MaxRetryInterval = maxRetryInterval
}

func (task *Task) GetTaskID() uint32 {
	return task.ID
}

func (task *Task) GetOwnerType() string {
	if ownerType, ok := task.description.Load(OwnerTypeKey); ok {
		return ownerType.(string)
	}
	return strings.TrimSuffix(reflect.TypeOf(task.handler).Elem().Name(), "Task")
}

func (*Task) GetTaskType() TaskType {
	return TASK_TYPE_TASK
}

func (task *Task) GetTask() *Task {
	return task
}

func (task *Task) GetTaskPointer() uintptr {
	return uintptr(unsafe.Pointer(task))
}

func (task *Task) GetKey() uint32 {
	return task.ID
}

func (task *Task) getKey() any {
	return reflect.ValueOf(task.handler).MethodByName("GetKey").Call(nil)[0].Interface()
}

func (task *Task) WaitStarted() error {
	if task.startup == nil {
		return nil
	}
	return task.startup.Await()
}

func (task *Task) WaitStopped() (err error) {
	err = task.WaitStarted()
	if err != nil {
		return err
	}
	return task.shutdown.Await()
}

func (task *Task) Trace(msg string, fields ...any) {
	if task.Logger == nil {
		slog.Default().Log(task.Context, TraceLevel, msg, fields...)
		return
	}
	task.Logger.Log(task.Context, TraceLevel, msg, fields...)
}

func (task *Task) IsStopped() bool {
	return task.Err() != nil
}

func (task *Task) StopReason() error {
	return context.Cause(task.Context)
}

func (task *Task) StopReasonIs(errs ...error) bool {
	stopReason := task.StopReason()
	for _, err := range errs {
		if errors.Is(err, stopReason) {
			return true
		}
	}
	return false
}

func (task *Task) Stop(err error) {
	if err == nil {
		task.Error("task stop with nil error", "taskId", task.ID, "taskType", task.GetTaskType(), "ownerType", task.GetOwnerType(), "parent", task.GetParent().GetOwnerType())
		panic("task stop with nil error")
	}
	_, file, line, _ := runtime.Caller(1)
	task.stopOnce.Do(func() {
		if task.CancelCauseFunc != nil {
			msg := "task cancel context"
			if task.startup != nil && task.startup.IsRejected() {
				msg = "task start failed"
			}
			task.Debug(msg, "caller", fmt.Sprintf("%s:%d", strings.TrimPrefix(file, sourceFilePathPrefix), line), "reason", err, "elapsed", time.Since(task.StartTime), "taskId", task.ID, "taskType", task.GetTaskType(), "ownerType", task.GetOwnerType())
			task.CancelCauseFunc(err)
		}
		task.stop()
	})
}

func (task *Task) stop() {
	task.WaitStarted() // wait for task to set closeOnStop
	// 锁内取走整份，锁外逐个处理。置 nil 而不是 [:0]：后者会把底层数组留给
	// 后续 append 复用，而那次 append 就会覆盖我们正在遍历的元素。
	task.hookMu.Lock()
	closeOnStop := task.closeOnStop
	task.closeOnStop = nil
	task.hookMu.Unlock()
	task.Debug("task stop", "taskId", task.ID, "ownerType", task.GetOwnerType(), "closeOnStop", len(closeOnStop))
	for _, resource := range closeOnStop {
		switch v := resource.(type) {
		case func():
			v()
		case func() error:
			v()
		case ITask:
			v.Stop(task.StopReason())
		}
	}
}

// OnStart 注册一个在任务启动后执行的回调。
//
// 四个钩子注册方法（OnStart/OnDispose/OnStop/Using）都往 Task 上的切片里
// append，而它们的消费端（start/stop/dispose）遍历同一批切片。注册端跑在任意
// goroutine 上——通常是调用 AddTask 的那个业务 goroutine——消费端跑在持有本
// 任务的事件循环 goroutine 上，两端必须由 hookMu 互斥。
//
// 约定：只在锁内取切片头快照，绝不持锁调用回调或资源清理函数——那些是调用方的
// 代码，可能反过来调用本任务的其它方法。
//
// 如果注册时本轮的遍历已经走过（即任务已经启动），本次注册不可能再被那次遍历
// 收录，于是补跑一次，而不是静默丢弃。
//
// 补跑走的是**队列 + 排空循环**，不是直接 listener()。两个原因：
//
//  1. 防止无界递归。回调体内再注册同类钩子是合法写法，直接同步调用会在调用方
//     栈上一层层套下去直到爆栈；改成排空循环之后，嵌套注册只是往队列里追加，由
//     外层循环接着跑，栈深度恒定。
//  2. 让遍历期间到达的注册仍然跑在事件循环上。dispose()/start() 自己也是通过这
//     个队列跑监听器，并在整个过程中持有 catchingUp 标志，所以那段时间内别的
//     goroutine 注册进来只会入队、由事件循环代跑。只有在遍历彻底结束之后才注册
//     的，才会落到注册方自己的 goroutine 上——那时事件循环已经不管这批回调了，
//     没有别的地方可跑。
//
// 仍然登记进 afterStartListeners 是为了重试：checkRetry → reset → 再次 start 时
// 这批监听器要再跑一遍，晚注册的那个也不该缺席。算上补跑的那一次，它与早注册的
// 监听器在整个重试序列里的执行次数是相等的。
func (task *Task) OnStart(listener func()) {
	task.hookMu.Lock()
	task.afterStartListeners = append(task.afterStartListeners, listener)
	if !task.startHooksRun {
		task.hookMu.Unlock() // 遍历还没开始，等它收录
		return
	}
	task.pendingStart = append(task.pendingStart, listener)
	if task.catchingUpStart {
		task.hookMu.Unlock() // 已有 goroutine 在排空，交给它，避免在本栈上递归
		return
	}
	task.catchingUpStart = true
	task.hookMu.Unlock()
	task.drainStartHooks()
}

// OnDispose 注册一个在任务销毁后执行的回调，机制与 OnStart 完全对称。
func (task *Task) OnDispose(listener func()) {
	task.hookMu.Lock()
	task.afterDisposeListeners = append(task.afterDisposeListeners, listener)
	if !task.disposeHooksRun {
		task.hookMu.Unlock()
		return
	}
	task.pendingDispose = append(task.pendingDispose, listener)
	if task.catchingUpDispose {
		task.hookMu.Unlock()
		return
	}
	task.catchingUpDispose = true
	task.hookMu.Unlock()
	task.drainDisposeHooks(nil)
}

// drainStartHooks 排空 pendingStart。调用前必须已在锁内把 catchingUpStart 置为
// true（表示"本 goroutine 负责跑"），本函数返回前一定会把它置回 false。
func (task *Task) drainStartHooks() {
	for {
		task.hookMu.Lock()
		queue := task.pendingStart
		task.pendingStart = nil
		if len(queue) == 0 {
			task.catchingUpStart = false
			task.hookMu.Unlock()
			return
		}
		task.hookMu.Unlock()
		for _, listener := range queue {
			if task.IsStopped() { // 与原先遍历里的 break 语义一致：停了就不再跑剩下的
				task.hookMu.Lock()
				task.pendingStart = nil
				task.catchingUpStart = false
				task.hookMu.Unlock()
				return
			}
			listener()
		}
	}
}

// drainDisposeHooks 排空 pendingDispose，约定同 drainStartHooks。
// yargs 为 dispose() 传入的日志上下文，补跑路径传 nil。
func (task *Task) drainDisposeHooks(yargs []any) {
	for i := 0; ; {
		task.hookMu.Lock()
		queue := task.pendingDispose
		task.pendingDispose = nil
		if len(queue) == 0 {
			task.catchingUpDispose = false
			task.hookMu.Unlock()
			return
		}
		task.hookMu.Unlock()
		for _, listener := range queue {
			if yargs != nil {
				task.SetDescription("disposeProcess", fmt.Sprintf("a:%d", i))
				task.Debug("task dispose listener begin", append(yargs, "listenerIndex", i)...)
			}
			listener()
			if yargs != nil {
				task.Debug("task dispose listener end", append(yargs, "listenerIndex", i)...)
			}
			i++
		}
	}
}

func (task *Task) Using(resource ...any) {
	task.hookMu.Lock()
	defer task.hookMu.Unlock()
	task.resources = append(task.resources, resource...)
}

func (task *Task) OnStop(resource any) {
	if t, ok := resource.(ITask); ok && t.GetTask() == task {
		panic("onStop resource is task itself")
	}
	task.hookMu.Lock()
	defer task.hookMu.Unlock()
	task.closeOnStop = append(task.closeOnStop, resource)
}

func (task *Task) GetSignal() any {
	return task.Done()
}

func (task *Task) checkRetry(err error) bool {
	if errors.Is(err, ErrTaskComplete) || errors.Is(err, ErrExit) || errors.Is(err, ErrStopByUser) {
		return false
	}
	if task.parent.IsStopped() {
		return false
	}
	if task.retry.MaxRetry < 0 || task.retry.RetryCount < task.retry.MaxRetry {
		task.retry.RetryCount++
		task.SetDescription("retryCount", task.retry.RetryCount)
		if task.retry.MaxRetry < 0 {
			task.Warn(fmt.Sprintf("retry %d/∞", task.retry.RetryCount), "taskId", task.ID)
		} else {
			task.Warn(fmt.Sprintf("retry %d/%d", task.retry.RetryCount, task.retry.MaxRetry), "taskId", task.ID)
		}

		// Calculate exponential backoff delay: baseInterval * 2^(retryCount-1)
		retryDelay := task.retry.RetryInterval
		if task.retry.RetryCount > 1 {
			// Calculate 2^(retryCount-1) using bit shift for better performance
			exponent := task.retry.RetryCount - 1
			if exponent < 30 { // Avoid overflow for very large retry counts
				retryDelay = task.retry.RetryInterval * time.Duration(1<<exponent)
			} else {
				// For very large retry counts, use maximum allowed duration
				retryDelay = task.retry.RetryInterval * time.Duration(1<<30)
			}

			// Apply maximum delay limit if set
			if task.retry.MaxRetryInterval > 0 && retryDelay > task.retry.MaxRetryInterval {
				retryDelay = task.retry.MaxRetryInterval
			}
		}

		task.SetDescription("retryDelay", retryDelay.String())
		if delta := time.Since(task.StartTime); delta < retryDelay {
			time.Sleep(retryDelay - delta)
		}
		// Re-check parent after sleep: parent may have been stopped while we were sleeping.
		// Without this check a stale retry would proceed with a cancelled context and could
		// call Publish() again, potentially kicking out a legitimate publisher that was
		// created by a replacement PullJob added during the sleep window.
		// if task.parent.IsStopped() {
		// 	task.Warn("parent stopped after retry sleep, abort retry", "taskId", task.ID, "ownerType", task.GetOwnerType())
		// 	return false
		// }
		return true
	} else {
		if task.retry.MaxRetry > 0 {
			task.Warn(fmt.Sprintf("max retry %d failed", task.retry.MaxRetry))
			return false
		}
	}
	return errors.Is(err, ErrRestart)
}

func (task *Task) start() bool {
	var err error
	if !ThrowPanic {
		defer func() {
			if r := recover(); r != nil {
				err = errors.New(fmt.Sprint(r))
				task.Error("panic", "error", err, "stack", string(debug.Stack()))
			}
		}()
	}
	for {
		// Guard against a race window: parent/task may be stopped right after
		// checkRetry returns true and before the next retry Start() call.
		//if task.IsStopped() || (task.parent != nil && task.parent.IsStopped()) {
		//	task.Warn("fix-B: task/parent stopped at retry loop entry, abort retry", "taskId", task.ID, "ownerType", task.GetOwnerType())
		//	return false
		//}
		task.StartTime = time.Now()
		task.Debug("task start", "taskId", task.ID, "taskType", task.GetTaskType(), "ownerType", task.GetOwnerType(), "reason", task.StartReason)
		task.setState(TASK_STATE_STARTING)
		if v, ok := task.handler.(TaskStarter); ok {
			err = v.Start()
		}
		if err == nil {
			task.setState(TASK_STATE_STARTED)
			task.startup.Fulfill(err)
			// 把本轮要跑的监听器灌进补跑队列，然后由同一个排空循环跑掉——这样
			// 遍历期间到达的 OnStart 也会被本 goroutine（事件循环）接手，而不是
			// 落到注册方的栈上。afterStartListeners 本身不清空：重试时要再跑一遍。
			task.hookMu.Lock()
			task.pendingStart = append(task.pendingStart, task.afterStartListeners...)
			task.startHooksRun = true
			task.catchingUpStart = true
			task.hookMu.Unlock()
			task.drainStartHooks()
			if task.IsStopped() {
				err = task.StopReason()
			} else {
				task.ResetRetryCount()
				if runHandler, ok := task.handler.(TaskBlock); ok {
					task.setState(TASK_STATE_RUNNING)
					task.Debug("task run", "taskId", task.ID, "taskType", task.GetTaskType(), "ownerType", task.GetOwnerType())
					err = runHandler.Run()
					if err == nil {
						err = ErrTaskComplete
					}
				}
			}
		}
		if err == nil {
			if goHandler, ok := task.handler.(TaskGo); ok {
				task.setState(TASK_STATE_GOING)
				task.Debug("task go", "taskId", task.ID, "taskType", task.GetTaskType(), "ownerType", task.GetOwnerType())
				go task.run(goHandler.Go)
			}
			return true
		} else {
			task.Stop(err)
			if task.parent != nil {
				task.parent.onChildDispose(task.handler)
			}
			if task.checkRetry(err) {
				task.reset()
			} else {
				return false
			}
		}
	}
}

func (task *Task) reset() {
	// 重试是新的一轮：两批监听器都要再跑一遍，标志随之复位，上一轮没排完的
	// 补跑队列作废（下一轮 start/dispose 会把完整名单重新灌进去）。
	// catchingUp 不动：它为 true 就意味着有 goroutine 正在排空，由那个
	// goroutine 自己置回 false——它下一次取到空队列就会退出。
	task.hookMu.Lock()
	task.startHooksRun, task.disposeHooksRun = false, false
	task.pendingStart, task.pendingDispose = nil, nil
	task.hookMu.Unlock()
	task.loopGen.Add(1)
	task.stopOnce = sync.Once{}
	task.Context, task.CancelCauseFunc = context.WithCancelCause(task.parentCtx)
	task.shutdown = util.NewPromise(context.Background())
	task.startup = util.NewPromise(task.Context)
}

func (task *Task) GetDescriptions() map[string]string {
	return maps.Collect(func(yield func(key, value string) bool) {
		task.description.Range(func(key, value any) bool {
			return yield(key.(string), fmt.Sprintf("%+v", value))
		})
	})
}

func (task *Task) GetDescription(key string) (any, bool) {
	return task.description.Load(key)
}

func (task *Task) SetDescription(key string, value any) {
	task.description.Store(key, value)
}

func (task *Task) RemoveDescription(key string) {
	task.description.Delete(key)
}

func (task *Task) SetDescriptions(value Description) {
	for k, v := range value {
		task.description.Store(k, v)
	}
}

func (task *Task) dispose() {
	taskType, ownerType := task.handler.GetTaskType(), task.GetOwnerType()
	if task.GetState() < TASK_STATE_STARTED {
		task.Debug("task dispose canceled", "taskId", task.ID, "taskType", taskType, "ownerType", ownerType, "state", task.GetState())
		task.setState(TASK_STATE_DISPOSED)
		task.shutdown.Fulfill(task.StopReason())
		return
	}
	reason := task.StopReason()
	task.setState(TASK_STATE_DISPOSING)
	yargs := []any{"reason", reason, "taskId", task.ID, "taskType", taskType, "ownerType", ownerType}
	task.Debug("task dispose", yargs...)
	defer task.Debug("task disposed", yargs...)
	if v, ok := task.handler.(TaskPreDisposal); ok {
		task.SetDescription("disposeProcess", "pre-dispose")
		task.Debug("task dispose pre-dispose begin", yargs...)
		v.PreDispose()
		task.Debug("task dispose pre-dispose end", yargs...)
	}
	if job, ok := task.handler.(IJob); ok {
		mt := job.getJob()
		task.SetDescription("disposeProcess", "wait children")
		task.Debug("task dispose wait children begin", append(yargs, "childCount", mt.Size.Load())...)
		mt.waitChildrenDispose(reason)
		task.Debug("task dispose wait children end", append(yargs, "childCount", mt.Size.Load())...)
	}
	task.SetDescription("disposeProcess", "self")
	if v, ok := task.handler.(TaskDisposal); ok {
		task.Debug("task dispose self begin", yargs...)
		v.Dispose()
		task.Debug("task dispose self end", yargs...)
	}
	task.SetDescription("disposeProcess", "resources")
	task.hookMu.Lock()
	resourceCount := len(task.resources)
	task.hookMu.Unlock()
	task.Debug("task dispose resources begin", append(yargs, "resourceCount", resourceCount)...)
	task.stopOnce.Do(task.stop)
	// 快照**必须取在 stop() 之后**：原来的 `for range task.resources` 是在
	// stop() 返回之后才求值 range 表达式的，而 stop() 会跑 closeOnStop 回调，
	// 那些回调可能再 Using() 新资源。把取快照提到 stop() 之前会把它们漏掉——
	// 这是加锁重构时极容易顺手改坏的一处顺序。
	task.hookMu.Lock()
	resources := task.resources
	task.resources = nil
	task.hookMu.Unlock()
	for _, resource := range resources {
		switch v := resource.(type) {
		case func():
			v()
		case ITask:
			v.Stop(task.StopReason())
		case util.Recyclable:
			v.Recycle()
		case io.Closer:
			v.Close()
		}
	}
	task.Debug("task dispose resources end", yargs...)
	// 同 start()：灌进补跑队列后由排空循环跑掉，遍历期间到达的 OnDispose 由本
	// goroutine（事件循环）接手。afterDisposeListeners 不清空，重试时要再跑一遍。
	task.hookMu.Lock()
	task.pendingDispose = append(task.pendingDispose, task.afterDisposeListeners...)
	task.disposeHooksRun = true
	task.catchingUpDispose = true
	task.hookMu.Unlock()
	task.drainDisposeHooks(yargs)
	task.SetDescription("disposeProcess", "done")
	task.setState(TASK_STATE_DISPOSED)
	task.shutdown.Fulfill(reason)
}

func (task *Task) ResetRetryCount() {
	task.retry.RetryCount = 0
}

func (task *Task) GetRetryCount() int {
	return task.retry.RetryCount
}

func (task *Task) GetMaxRetry() int {
	return task.retry.MaxRetry
}

func (task *Task) run(handler func() error) {
	var err error
	defer func() {
		if !ThrowPanic {
			if r := recover(); r != nil {
				err = errors.New(fmt.Sprint(r))
				task.Error("panic", "error", err, "stack", string(debug.Stack()))
			}
		}
		if err == nil {
			task.Stop(ErrTaskComplete)
		} else {
			task.Stop(err)
		}
	}()
	err = handler()
}

func (task *Task) Debug(msg string, args ...any) {
	if task.Logger == nil {
		slog.Default().Debug(msg, args...)
		return
	}
	task.Logger.Debug(msg, args...)
}

func (task *Task) Info(msg string, args ...any) {
	if task.Logger == nil {
		slog.Default().Info(msg, args...)
		return
	}
	task.Logger.Info(msg, args...)
}

func (task *Task) Warn(msg string, args ...any) {
	if task.Logger == nil {
		slog.Default().Warn(msg, args...)
		return
	}
	task.Logger.Warn(msg, args...)
}

func (task *Task) Error(msg string, args ...any) {
	if task.Logger == nil {
		slog.Default().Error(msg, args...)
		return
	}
	task.Logger.Error(msg, args...)
}

func (task *Task) TraceEnabled() bool {
	return task.Logger.Enabled(task.Context, TraceLevel)
}

func (task *Task) RunTask(t ITask, opt ...any) (err error) {
	tt := t.GetTask()
	tt.handler = t
	mt := task.parent
	if job, ok := task.handler.(IJob); ok {
		mt = job.getJob()
	}
	mt.initContext(tt, opt...)
	if mt.IsStopped() {
		err = mt.StopReason()
		task.startup.Reject(err)
		return
	}
	task.OnStop(t)
	started := tt.start()
	<-tt.Done()
	if started {
		tt.dispose()
	}
	return tt.StopReason()
}
