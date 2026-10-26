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
		// ShouldRetry is an optional predicate consulted on every failed attempt
		// before spending a retry slot (nil means "decide by the default rules").
		// It mirrors generic.RetryPolicy.ShouldRetry so both retry implementations
		// agree on what "worth retrying" means (Phase 3 design §3.2).
		ShouldRetry func(error) bool
	}
	Description    = map[string]any
	TaskContextKey string
	Task           struct {
		ID        uint32
		StartTime time.Time
		// startNano 是 StartTime.UnixNano() 的原子快照。StartTime 本身是公开
		// 字段、写在事件循环（或 RunTask 的调用方）goroutine 上，而 Stop() 会在
		// 任意 goroutine 上经 Debug 变参读它算 elapsed——裸字段跨 goroutine 读写
		// 就是数据竞争（Test_Block 实测）。Debug 改读快照；StartTime 保留原类型，
		// 同 goroutine 的 checkRetry 继续直接读它。
		startNano   atomic.Int64
		StartReason string
		Logger      *slog.Logger
		context.Context
		context.CancelCauseFunc
		handler                                    ITask
		retry                                      RetryConfig
		// retrying 表示本任务正处于「重试退避等待」中。此时 GetSignal() 返回
		// retryCh 而不是 Done()：事件循环把这个子任务的信号 case 绑到退避定时器
		// 上，而不是绑到已经结束（Done 已关闭）的本轮 context。见 armRetry。
		retrying atomic.Bool
		// retryCh 是退避到期的握手通道（容量 1）：定时器投递令牌，循环收到后
		// 执行 reset + start。每次 initContext/reset 重建，旧周期的令牌随之作废。
		retryCh chan struct{}
		// retryCancel 关闭即取消在途退避等待（cancelRetryLocked），受 stopMu 保护。
		retryCancel chan struct{}
		// attemptDone 是**本轮执行 goroutine** 的退出信号，由 start() 在拉起
		// goroutine 前创建、由 task.run 在 handler 返回后关闭。
		//
		// 重试必须先等它：reset() 会替换 task.Context，而上一轮的 handler 可能
		// 还在返回路上读它（race detector 实测 reset 写 vs Run 读 StopReason）。
		// 等它同时保证了不会出现「上一轮 handler 还在跑、下一轮已经起来」的并发
		// 执行——重试的语义本来就是「上一轮结束之后再来一轮」。
		attemptDone chan struct{}
		// attemptGen 是执行代际：reset() 每开一轮 +1。task.run 的 defer 拿启动时
		// 的代际来判定「我这一轮还是不是当前轮」，避免迟到的 Stop 取消下一轮的
		// context（见 stopIfCurrentAttempt）。
		attemptGen atomic.Uint32
		hookMu                                     sync.Mutex // 保护下面这组钩子字段，见 OnStart
		hookGen                                    uint32     // 钩子代际，reset() 每轮 +1，见 drainStartHooks
		startHooksRun, disposeHooksRun             bool       // 本轮的对应遍历是否已经走过
		closeOnStopRun                             bool       // 本轮 stop() 是否已经排空过 closeOnStop，见 OnStop
		catchingUpStart, catchingUpDispose         bool       // 是否已有 goroutine 在排空补跑队列
		pendingStart, pendingDispose               []func()   // 错过本轮遍历、等待补跑的监听器
		afterStartListeners, afterDisposeListeners []func()
		closeOnStop                                []any
		resources                                  []any
		stopMu                                     sync.Mutex // 保护 stopped/stopErr，见 Stop
		stopped                                    bool       // 本任务是否已被请求停止（含 InitContext 之前的提前 Stop）
		stopErr                                    error      // stopped 时记录停止原因，供 InitContext 补取消 context
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

// SetShouldRetry installs a predicate consulted on every failed attempt. It runs
// after the built-in "terminal stop reason" filter and before a retry slot is
// spent: returning false means this error is not worth retrying, no matter how
// much budget is left. nil restores the default rules.
//
// Mirrors generic.RetryPolicy.ShouldRetry so the task tree and the generic
// orchestration layer finally agree on retry semantics (Phase 3 design §3.2).
func (task *Task) SetShouldRetry(shouldRetry func(error) bool) {
	task.retry.ShouldRetry = shouldRetry
}

func (task *Task) GetTaskID() uint32 {
	return task.ID
}

func (task *Task) GetOwnerType() string {
	if ownerType, ok := task.description.Load(OwnerTypeKey); ok {
		return ownerType.(string)
	}
	if task.handler == nil {
		return "Task"
	}
	handlerType := reflect.TypeOf(task.handler)
	if handlerType.Kind() == reflect.Pointer {
		handlerType = handlerType.Elem()
	}
	return strings.TrimSuffix(handlerType.Name(), "Task")
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

// Stop 请求停止任务。幂等：只有第一次调用执行取消 context 与 stop() 排空，
// 后续调用直接返回——与旧实现 stopOnce.Do 的语义一致。
//
// 为什么用 stopMu + stopped 而不是 sync.Once：Once.Do 有快速路径
// （done.Load()==1 时直接返回，完全不碰互斥量）。Go 任务的 goroutine 在
// defer 里调 Stop()，其 Once 的 f 返回后、m.Unlock() 执行前有一个窗口；
// 事件循环的 dispose() 走 Once.Do 快速路径旁路了互斥量，随后的 reset()
// 又会整体覆写 stopOnce 字段——与那个尚未执行的 m.Unlock() 构成无序竞争
// （race detector 实测：reset 的 8 字节结构体写 vs goroutine 的 sema 写）。
// 改用显式互斥量后，dispose()/reset() 都必须拿锁，永远排在在途 Stop 临界区
// 之后，整个竞争类随之消失。
//
// CancelCauseFunc 为 nil 说明任务还没有 context（尚未 AddTask/InitContext）。
// 此时不能静默吞掉这次停止：记住 stopped 与 stopErr，InitContext 建好
// context 后立即补上取消，否则任务会带着一个永不取消的新 context 跑完
// 整个生命周期（Test_Hooks 曾因此永久挂住）。
//
// 整个函数体（含 nil 检查的 Error 日志与 Debug 日志）都在 stopMu 下：这些
// 日志的参数是**饥饿求值**的，会读 ID/handler/parent/startup/startNano 等
// 字段，而它们的写入端（initContext/reset/start）与 Stop 并发执行——不拿同
// 一把锁，race detector 就会报读写竞争（Test_Block 的定时器 goroutine 与
// AddTask 曾实测）。StartTime 是唯一的例外：它写在事件循环上，改成原子
// 快照 startNano 供这里读取，见字段注释。
func (task *Task) Stop(err error) {
	_, file, line, _ := runtime.Caller(1)
	task.stopMu.Lock()
	defer task.stopMu.Unlock()
	task.stopLocked(err, file, line)
}

// stopIfCurrentAttempt 是本轮执行 goroutine 专用的 Stop：只有当前代际仍是它启动
// 的那一轮时才真正停止。
//
// 为什么需要：task.run 的 defer 在 **Run 返回之后**才调 Stop，而这个 Stop 与
// 事件循环的处理是并发的——循环可能已经 dispose → retryDecision → reset() 换好
// 了下一轮的 context 并把 stopped 清零。迟到的 Stop 会把新一轮的 context 取消掉，
// 让新 attempt 刚出生就以旧一轮的原因退出（实测：Restart() 下 attempt 2 立刻以
// "restart" 返回，重试次数被白烧一次）。代际检查与 reset() 的递增都在 stopMu
// 下，二者串行，没有窗口。
func (task *Task) stopIfCurrentAttempt(gen uint32, err error) {
	task.stopMu.Lock()
	defer task.stopMu.Unlock()
	if task.attemptGen.Load() != gen {
		return
	}
	_, file, line, _ := runtime.Caller(1)
	task.stopLocked(err, file, line)
}

// stopLocked 是 Stop 的锁内实现，调用方必须已持有 stopMu。
func (task *Task) stopLocked(err error, file string, line int) {
	if err == nil {
		task.Error("task stop with nil error", "taskId", task.ID, "taskType", task.GetTaskType(), "ownerType", task.GetOwnerType(), "parent", task.GetParent().GetOwnerType())
		panic("task stop with nil error")
	}
	// 先幂等取消在途退避再判 stopped：父任务停止时子任务早就是 stopped（失败那轮
	// 已经 Stop 过），下面会直接返回；不在这里发令牌，循环就会为空等退避。
	task.cancelRetryLocked()
	if task.stopped {
		return
	}
	task.stopped = true
	task.stopErr = err
	if task.CancelCauseFunc != nil {
		msg := "task cancel context"
		if task.startup != nil && task.startup.IsRejected() {
			msg = "task start failed"
		}
		task.Debug(msg, "caller", fmt.Sprintf("%s:%d", strings.TrimPrefix(file, sourceFilePathPrefix), line), "reason", err, "elapsed", time.Since(time.Unix(0, task.startNano.Load())), "taskId", task.ID, "taskType", task.GetTaskType(), "ownerType", task.GetOwnerType())
		task.CancelCauseFunc(err)
	}
	task.stop()
}

// Restart 请求重启当前任务：结束本轮执行，然后经重试通道立即重跑。
//
// 与 Stop 的区别：Stop 是终止（ErrStopByUser 不重试）；Restart 是「这一轮结束，
// 再来一轮」，因此以 ErrRestart 停止，走重试通道。它会消耗一次重试预算（受
// MaxRetry 约束——默认 MaxRetry=0 时不会重跑，需要显式 SetRetry(n, …) 或
// SetRetry(-1, 0) 获得无界重启），但不进指数退避：ErrRestart 的退避为 0。
//
// 在 Phase 3 之前 ErrRestart 只是 checkRetry 末尾的一个死分支（全仓库没有
// 生产者，见 docs/重构-执行模型v2-Phase3设计.md F21），本方法补上生产者，
// 并让该分支遵守 MaxRetry 预算。
func (task *Task) Restart() {
	task.Stop(ErrRestart)
}

func (task *Task) stop() {
	// 等不到 Start() 结束，这句只在"任务已经启动完"的情形下有意义：startup 是
	// util.NewPromise(task.Context) 建的，Await() 等的是 ctx.Done()，而 Stop() 先
	// CancelCauseFunc 再调 stop()，所以 Stop 与 Start 并发时它立即返回，下面取到的
	// 是空切片。晚到的那些注册由 OnStop 自己就地关闭，见那里的注释。
	task.WaitStarted()
	// 锁内取走整份，锁外逐个处理。置 nil 而不是 [:0]：后者会把底层数组留给
	// 后续 append 复用，而那次 append 就会覆盖我们正在遍历的元素。
	task.hookMu.Lock()
	closeOnStop := task.closeOnStop
	task.closeOnStop = nil
	task.closeOnStopRun = true
	task.hookMu.Unlock()
	task.Debug("task stop", "taskId", task.ID, "ownerType", task.GetOwnerType(), "closeOnStop", len(closeOnStop))
	for _, resource := range closeOnStop {
		task.closeResource(resource)
	}
}

// closeResource 关闭一个 OnStop 资源。stop() 的批量排空与 OnStop 的就地补跑共用它，
// 免得两处的类型分支各自漂移。
func (task *Task) closeResource(resource any) {
	switch v := resource.(type) {
	case func():
		v()
	case func() error:
		v()
	case ITask:
		v.Stop(task.StopReason())
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
	gen := task.hookGen
	task.hookMu.Unlock()
	task.drainStartHooks(gen)
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
	gen := task.hookGen
	task.hookMu.Unlock()
	task.drainDisposeHooks(nil, gen)
}

// drainStartHooks 排空 pendingStart。调用前必须已在锁内把 catchingUpStart 置为
// true（表示"本 goroutine 负责跑"）并取走当时的 hookGen。
//
// gen 是代际护栏。补跑排空跑在**任意** goroutine 上（晚注册时就是注册方自己的），
// 而 reset() 可能在它还没退出时就开启下一轮。没有护栏的话，上一轮遗留的排空者会与
// 下一轮由事件循环发起的排空并存，共用同一组字段：它只要先取到锁，就会取走下一轮
// 的队列（跑在错误的 goroutine 上），或看到队列已被取空而把 catchingUpStart 置回
// false（此后到达的注册不再由事件循环代跑）。
//
// 所以代际不符时必须**原样退出**：既不取队列，也不动 catchingUpStart——当前代际的
// 排空者会自己收尾。已经取到手的那批仍然跑完，中途丢弃才是真的漏执行。
func (task *Task) drainStartHooks(gen uint32) {
	for {
		task.hookMu.Lock()
		if task.hookGen != gen {
			task.hookMu.Unlock()
			return
		}
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
				if task.hookGen == gen { // 同上：换代之后这两个字段已经不归本 goroutine 管
					task.pendingStart = nil
					task.catchingUpStart = false
				}
				task.hookMu.Unlock()
				return
			}
			listener()
		}
	}
}

// drainDisposeHooks 排空 pendingDispose，约定与代际护栏同 drainStartHooks。
// yargs 为 dispose() 传入的日志上下文，补跑路径传 nil。
func (task *Task) drainDisposeHooks(yargs []any, gen uint32) {
	for i := 0; ; {
		task.hookMu.Lock()
		if task.hookGen != gen {
			task.hookMu.Unlock()
			return
		}
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
	if task.closeOnStopRun {
		// 本轮 stop() 已经排空过 closeOnStop，此时再 append 就再也没人会遍历它：
		// dispose() 里唯一的收尾是 stopOnce.Do(task.stop)，而那次 once 已被触发排空
		// 的 Stop() 消费掉，退化成空操作；有下一轮重试才会晚一轮被带走，没有就永久
		// 泄漏。典型形态是 Stop() 与 Start() 并发——资源在 Start() 里建好之后才注册，
		// 注册时 stop() 早已跑完。这里就地关掉，动作与 stop() 遍历到它完全一致，
		// 只是换了个 goroutine 执行。
		task.hookMu.Unlock()
		task.closeResource(resource)
		return
	}
	task.closeOnStop = append(task.closeOnStop, resource)
	task.hookMu.Unlock()
}

func (task *Task) GetSignal() any {
	// 退避等待期间对外暴露 retryCh：本轮 attempt 已经结束、Done 已关闭，循环若
	// 继续绑 Done 会立刻收到一次虚假的就绪信号并重复 dispose。retryCh 收到令牌
	// 才是「退避到期」或「父任务停止」的真实信号，见 armRetry/cancelRetryLocked。
	if task.retrying.Load() {
		return task.retryCh
	}
	return task.Done()
}

// isTerminalStopReason reports whether err is a lifecycle outcome that must not
// trigger a retry: normal completion, explicit exit, user stop, and timeout.
//
// Phase 3（3A）把它抽成任务树与泛型编排共用的判定：此前 checkRetry 只看前三者、
// generic 侧的 shouldRetryGeneric 还额外排除 ErrTimeout，两套重试语义漂移
// （docs/重构-执行模型v2-Phase3设计.md F22）。统一后 ErrTimeout 一律不重试，
// 与 generic.WithTimeout 的语义一致。
func isTerminalStopReason(err error) bool {
	return errors.Is(err, ErrTaskComplete) || errors.Is(err, ErrExit) ||
		errors.Is(err, ErrStopByUser) || errors.Is(err, ErrTimeout)
}

// retryDecision 判定一次失败是否值得重试：需要时消耗一次重试预算，并算出剩余的
// 退避时长。**只判定，不等待**——路径 B（事件循环）随后用 armRetry 把退避交给
// 定时器，循环在等待期间保持响应（Phase 3 / C5）。
//
// 判定顺序：终态原因 → 父任务已停止 → ShouldRetry 谓词 → 预算。
// 谓词在预算之前：它回答「这个错误值不值得重试」，与「还有没有额度」正交；
// 被谓词拒绝的失败不消耗预算。
func (task *Task) retryDecision(err error) (delay time.Duration, ok bool) {
	if isTerminalStopReason(err) {
		return 0, false
	}
	// 没有父任务时无处安放重试（RootManager 之类的特权根任务不会走到这里）。
	if task.parent == nil || task.parent.IsStopped() {
		return 0, false
	}
	if task.retry.ShouldRetry != nil && !task.retry.ShouldRetry(err) {
		return 0, false
	}
	if task.retry.MaxRetry < 0 || task.retry.RetryCount < task.retry.MaxRetry {
		task.retry.RetryCount++
		task.SetDescription("retryCount", task.retry.RetryCount)
		if task.retry.MaxRetry < 0 {
			task.Warn(fmt.Sprintf("retry %d/∞", task.retry.RetryCount), "taskId", task.ID)
		} else {
			task.Warn(fmt.Sprintf("retry %d/%d", task.retry.RetryCount, task.retry.MaxRetry), "taskId", task.ID)
		}

		// ErrRestart 是调用方显式的重启请求（Task.Restart）：立即重跑，不进指数
		// 退避，但仍然消耗一次预算，避免无界重启悄悄绕过 MaxRetry（F21）。
		if errors.Is(err, ErrRestart) {
			task.SetDescription("retryDelay", "0s")
			return 0, true
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
		// 退避起点是本轮 attempt 的开始时间（不是失败时刻），已经执行掉的时间
		// 要从退避里扣掉——与 Phase 3 之前的内联等待完全一致。
		if delta := time.Since(task.StartTime); delta < retryDelay {
			return retryDelay - delta, true
		}
		return 0, true
	}
	if task.retry.MaxRetry > 0 {
		task.Warn(fmt.Sprintf("max retry %d failed", task.retry.MaxRetry))
	}
	return 0, false
}

// checkRetry 是 retryDecision 的阻塞版：判定后把退避睡完，返回是否可以重启。
//
// 只给两条路径用：start() 的失败路径（TaskStarter 的 Start 失败链，短且罕见）
// 与 RunTask（调用方专用 goroutine，没有共享循环可占）。事件循环走
// retryDecision + armRetry，退避期间循环继续处理其它事件。
func (task *Task) checkRetry(err error) bool {
	delay, ok := task.retryDecision(err)
	if !ok {
		return false
	}
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-task.parent.Done():
			return false
		}
	}
	if task.parent.IsStopped() {
		return false
	}
	return true
}

// armRetry 安排一次退避后的重启：等待「退避到期」与「本轮执行 goroutine 退出」
// 两个条件都满足后，向 retryCh 投递令牌（容量 1，非阻塞）；事件循环的信号 case
// （此刻 GetSignal 返回 retryCh）收到后执行 reset + start。
//
// 这是 C5 翻转的关键：退避不再由循环内联 sleep，循环在等待期间可以处理 Call、
// 兄弟任务与其它子任务信号。等待在独立 goroutine 上做，循环本身不被占用。
//
// 为什么还要等 attemptDone：reset() 会替换 task.Context 等字段，而 handler 可能
// 在 Run 返回后仍在读它们（例如 `return t.StopReason()`），二者并发就是数据竞争；
// 同时也能避免上一轮 handler 未退出就并发跑下一轮。若 handler 无视 context 长期
// 不返回，重试就不该发生——这正是 Go「无法强杀 goroutine」的诚实表达：父任务停止
// 时 cancelRetryLocked 会让等待者立刻放手。
func (task *Task) armRetry(delay time.Duration) {
	task.stopMu.Lock()
	defer task.stopMu.Unlock()
	if task.retryCh == nil {
		task.retryCh = make(chan struct{}, 1)
	}
	task.retrying.Store(true)
	task.SetDescription("retryPending", true)
	// 上一轮可能还有等待者：先让它放手，再挂新的。
	if task.retryCancel != nil {
		close(task.retryCancel)
		task.retryCancel = nil
	}
	cancel := make(chan struct{})
	task.retryCancel = cancel
	ch := task.retryCh
	retired := task.attemptDone
	parentDone := task.parent.Done()
	// 清掉可能残留的令牌（上一次 Stop 的唤醒令牌，属于已作废的周期）：留在缓冲区
	// 里会让本次退避立刻被当成「到期」，跳过整个退避窗口。
drain:
	for {
		select {
		case <-ch:
		default:
			break drain
		}
	}
	go func() {
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-cancel:
				return
			case <-parentDone:
				return
			}
		}
		if retired != nil {
			select {
			case <-retired:
			case <-cancel:
				return
			case <-parentDone:
				return
			}
		}
		select {
		case ch <- struct{}{}:
		default: // 已经有令牌在队列里（或本轮已作废），丢弃即可
		}
	}()
}

// cancelRetryLocked 取消在途退避并唤醒正在等它的循环。调用方必须已持有 stopMu
//（Stop 与 dispose 都在锁内调它）。幂等。
//
// 必须唤醒了才有效：退避期间子任务的 stop 早就是 true（失败那轮已经 Stop 过），
// 父任务停止时 child.Stop 会走幂等快路径直接返回，若只关掉等待者不发令牌，
// 循环会为一个已停的任务空等到退避结束（循环也就无法退出）。
func (task *Task) cancelRetryLocked() {
	if task.retryCancel != nil {
		close(task.retryCancel)
		task.retryCancel = nil
	}
	if task.retryCh != nil {
		select {
		case task.retryCh <- struct{}{}:
		default:
		}
	}
}

// setAttemptDone 登记本轮的退出信号（start 在拉起 goroutine 前调用）。
func (task *Task) setAttemptDone(done chan struct{}) {
	task.stopMu.Lock()
	task.attemptDone = done
	task.stopMu.Unlock()
}

// waitAttemptRetired 等待本轮执行 goroutine 退出（handler 已返回）。仅供
// RunTask 这种「调用方 goroutine 上驱动重试」的路径在 reset 之前调用。
func (task *Task) waitAttemptRetired() {
	task.stopMu.Lock()
	done := task.attemptDone
	task.stopMu.Unlock()
	if done != nil {
		<-done
	}
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
		err = nil
		// Guard against a race window: parent/task may be stopped right after
		// checkRetry returns true and before the next retry Start() call.
		//if task.IsStopped() || (task.parent != nil && task.parent.IsStopped()) {
		//	task.Warn("fix-B: task/parent stopped at retry loop entry, abort retry", "taskId", task.ID, "ownerType", task.GetOwnerType())
		//	return false
		//}
		now := time.Now()
		task.StartTime = now
		task.startNano.Store(now.UnixNano())
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
			startGen := task.hookGen
			task.hookMu.Unlock()
			task.drainStartHooks(startGen)
			if task.IsStopped() {
				err = task.StopReason()
			} else {
				if runHandler, ok := task.handler.(TaskBlock); ok {
					task.setState(TASK_STATE_RUNNING)
					task.Debug("task run", "taskId", task.ID, "taskType", task.GetTaskType(), "ownerType", task.GetOwnerType())
					// Phase 2（控制与执行分离，见 docs/重构-执行模型v2-Phase2设计.md）：
					// Run 移出父 Job 事件循环。执行 goroutine 与 TaskGo 完全同构
					// （task.run：panic 恢复 + 返回即 Stop），完成/失败经 context
					// 取消汇入子任务信号，由事件循环的信号分支驱动 dispose 与重试；
					// start() 就此返回 true，父循环继续处理 Call、兄弟任务与其他
					// 子任务信号。契约：TestContract_RunDoesNotOccupyParentEventLoop、
					// TestContract_StopDuringBlockingRunIsPrompt。
					//
					// 旧实现在这里内联执行 Run：长 Run 堵住整个父循环，且失败重试
					// 序列（dispose→退避→reset→再 start）也全部占着循环；运行中的
					// 子任务还不在 e.children 里，循环与 initCases 的重启重建都看不
					// 见它。另一个顺带修复的缺陷：Run panic 被本函数的 defer 恢复后，
					// 下面的错误路径整个被展开跳过，任务既不 Stop 也不 dispose
					// （WaitStopped 永久挂起，见 TestContract_RunPanicRecoveredAndRetried）。
					// 本轮退出信号：重试的等待者要等它关闭才允许 reset()（见 armRetry）。
					attemptDone := make(chan struct{})
					task.setAttemptDone(attemptDone)
					go task.run(runHandler.Run, attemptDone)
					return true
				}
			}
		}
		if err == nil {
			if goHandler, ok := task.handler.(TaskGo); ok {
				task.setState(TASK_STATE_GOING)
				task.Debug("task go", "taskId", task.ID, "taskType", task.GetTaskType(), "ownerType", task.GetOwnerType())
				attemptDone := make(chan struct{})
				task.setAttemptDone(attemptDone)
				go task.run(goHandler.Go, attemptDone)
			}
			return true
		} else {
			// 显式落定 startup：出生即被停止的任务其 promise 挂在 Background 上
			// （见 InitContext），等不到 context 取消来代劳；其余任务这里与 Stop
			// 内的取消竞争同一个 promise，先到者胜，落定的值相同。
			task.startup.Reject(err)
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
	//
	// hookGen +1 是给上一轮那些可能还没退出的排空 goroutine 立的界碑：它们跑在
	// 任意 goroutine 上，reset() 拦不住，只能让它们下次取锁时自己发现换代了并
	// 原样退出，别去碰下一轮的队列与 catchingUp。详见 drainStartHooks。
	//
	// catchingUp 不动：换代后它归下一轮的排空者管——下一轮 start()/dispose() 会
	// 重新置 true 并在排空结束时置回 false。
	task.hookMu.Lock()
	task.hookGen++
	task.startHooksRun, task.disposeHooksRun = false, false
	// closeOnStopRun 也复位：下一轮 Start() 注册的资源要重新攒进 closeOnStop，
	// 由下一轮 stop() 排空，而不是一注册就被当成"晚到"立刻关掉。
	task.closeOnStopRun = false
	task.pendingStart, task.pendingDispose = nil, nil
	task.hookMu.Unlock()
	task.loopGen.Add(1)
	// stopped 也必须在锁内清除：上一轮可能还有一个来自任务 goroutine 的
	// Stop() 临界区没走完（其 defer 停在 Once 时代的 done.Store 与 Unlock
	// 之间，今天则是 stopMu 的临界区尾部），不拿锁就复位的标志会与那个在途
	// 临界区再次抢同一个任务。详见 Stop 的注释。
	//
	// Context/CancelCauseFunc/shutdown/startup 的重建同样必须在锁内：它们是
	// Stop() 在 stopMu 下会经 Debug 变参饥饿求值读到的字段（startNano 之外，
	// 这四个都是裸字段），写入端不拿同一把锁就是数据竞争。这里没有用户代码，
	// 持锁时间只有字段赋值与两次 NewPromise。
	task.stopMu.Lock()
	task.stopped = false
	task.stopErr = nil
	// 新一代的退避握手：上一轮遗留的等待者与令牌一并作废（重置可能在
	// cancelRetry 之后就发生，旧令牌不能留到下一轮的 armRetry 里）。
	if task.retryCancel != nil {
		close(task.retryCancel)
		task.retryCancel = nil
	}
	task.retrying.Store(false)
	task.RemoveDescription("retryPending")
	task.retryCh = make(chan struct{}, 1)
	// 代际 +1：上一轮执行 goroutine 的 defer 若还停在 Stop 门口，会因代际不符而
	// 放手，不再取消本轮刚建好的 context（见 stopIfCurrentAttempt）。
	task.attemptGen.Add(1)
	task.Context, task.CancelCauseFunc = context.WithCancelCause(task.parentCtx)
	task.shutdown = util.NewPromise(context.Background())
	task.startup = util.NewPromise(task.Context)
	task.stopMu.Unlock()
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
	// 与 Stop() 共用 stopMu/stopped：Stop 从未执行过时（例如停止来自父级
	// context 的级联取消，本任务自己的 Stop 未被调用过）这里负责补一次
	// stop() 排空；已经 Stop 过则跳过。Once 版本靠 done 标志做同样的判断，
	// 但它的快速路径会旁路互斥量，与 reset() 的结构体写竞争（见 Stop 注释）。
	task.stopMu.Lock()
	if !task.stopped {
		task.stopped = true
		task.stop()
	}
	task.stopMu.Unlock()
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
	disposeGen := task.hookGen
	task.hookMu.Unlock()
	task.drainDisposeHooks(yargs, disposeGen)
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

func (task *Task) run(handler func() error, attemptDone chan struct{}) {
	gen := task.attemptGen.Load()
	var err error
	defer func() {
		if !ThrowPanic {
			if r := recover(); r != nil {
				err = errors.New(fmt.Sprint(r))
				task.Error("panic", "error", err, "stack", string(debug.Stack()))
			}
		}
		if err == nil {
			err = ErrTaskComplete
		}
		// 代际守卫：本轮若已被 reset() 接管（重试/重启），Stop 必须放手，否则
		// 会把新一轮的 context 取消掉（stopIfCurrentAttempt 的注释有完整时序）。
		task.stopIfCurrentAttempt(gen, err)
		// 最后一步才关：重试等待者据此确认「handler 已返回」，reset() 才允许
		// 替换 task.Context（否则与 handler 的返回路上的读取构成数据竞争）。
		close(attemptDone)
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
	mt := task.parent
	if job, ok := task.handler.(IJob); ok {
		mt = job.getJob()
	}
	// handler 的赋值已内嵌到 initContext（锁内）：原先在锁外写它，与 Stop()
	// 的 Debug 变参饥饿求值读取竞争（GetOwnerType 会解引用 handler）。
	mt.initContext(tt, t, opt...)
	if mt.IsStopped() {
		err = mt.StopReason()
		tt.startup.Reject(err)
		tt.CancelCauseFunc(err)
		return
	}
	task.OnStop(t)
	if !tt.start() {
		// start 内部已走完销毁（Start 失败或无重试余地），与 Phase 2 前一致。
		return tt.StopReason()
	}
	// Phase 2（控制与执行分离）：start() 对 TaskBlock 只负责拉起执行 goroutine
	// 并立即返回，完成、失败与重试的驱动职责移交给调用方——事件循环的信号分支
	// 是同款协议（onChildDispose → checkRetry → reset → start），这里在调用方
	// goroutine 上复刻它。<-tt.Done() 每轮读当前 context：reset() 换了新
	// context，自然等到新 attempt 的信号。契约：TestContract_RunTaskDrives-
	// RetryForBlockTask。
	for {
		<-tt.Done()
		tt.dispose()
		if !tt.checkRetry(tt.StopReason()) {
			return tt.StopReason()
		}
		// reset() 会替换 Context 等字段：必须等本轮 handler 真正返回，否则与
		// 它的返回路径并发（详见 armRetry 的注释）。事件循环那边的等待在
		// armRetry 的 goroutine 上，这里就在调用方 goroutine 上等。
		tt.waitAttemptRetired()
		tt.reset()
		if !tt.start() {
			return tt.StopReason()
		}
	}
}
