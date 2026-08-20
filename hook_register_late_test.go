package task

import (
	"runtime"
	"sync/atomic"
	"testing"
)

// 本文件针对的是「晚注册的钩子被静默丢弃」。
//
// 加锁之后竞争没有了，但语义问题还在：调用方拿到任务的时刻（AddTask 返回）与
// 任务真正启动/跑完的时刻之间没有先后保证，短命任务随时可能跑在前面。此时
// OnStart/OnDispose 只是往一个再也不会被读的切片里 append——不执行、不返回错误、
// 不打日志，调用方完全无从察觉。
//
// 这不是假想场景。monibuca 的拉流代理就是这个形态：Pull() 在 AddTask 之后才注册
// OnDispose，而拉流 job 在上游不可达时会立刻失败销毁。丢掉的那个回调负责复位
// "已开始拉流"标志，丢了就再也不会自动重拉。

// completeTask 起来即结束：Run() 立刻返回，start() 把它归一成 ErrTaskComplete。
type completeTask struct {
	Task
}

func (t *completeTask) Run() error { return nil }

// Test_OnStartAfterStarted：任务已启动之后再注册 OnStart，回调应当立即执行。
func Test_OnStartAfterStarted(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatalf("job 启动失败: %v", err)
	}
	defer func() {
		job.Stop(ErrStopByUser)
		_ = job.WaitStopped()
	}()

	var called atomic.Bool
	job.OnStart(func() { called.Store(true) })
	if !called.Load() {
		t.Fatal("在任务启动之后注册的 OnStart 回调被静默丢弃了")
	}
}

// Test_OnDisposeAfterDisposed：任务已销毁之后再注册 OnDispose，回调应当立即执行。
func Test_OnDisposeAfterDisposed(t *testing.T) {
	var tk completeTask
	root.AddTask(&tk)
	_ = tk.WaitStopped() // 到这里 dispose() 已经走完 afterDisposeListeners

	var called atomic.Bool
	tk.OnDispose(func() { called.Store(true) })
	if !called.Load() {
		t.Fatal("在任务销毁之后注册的 OnDispose 回调被静默丢弃了")
	}
}

// Test_SelfReRegisterDispose 钉住补跑机制的重入行为。
//
// 「在 dispose 回调里再注册一个 dispose 回调」是合法写法。如果补跑实现成
// `if 已遍历过 { listener() }` 这样的直接同步调用，这段代码就会在注册方栈上
// 一层层套下去：回调 → 注册 → 又满足条件 → 回调……直到爆栈。基线因为静默丢弃
// 反而不会（回调一次都不跑），所以这属于**补跑机制可能引入的新故障模式**，
// 必须单独钉死。
//
// 排空循环的正确表现是栈深度恒定：嵌套注册只入队，由外层循环接着跑。
func Test_SelfReRegisterDispose(t *testing.T) {
	var tk completeTask
	root.AddTask(&tk)
	_ = tk.WaitStopped()

	var count atomic.Int32
	var deepest atomic.Int32
	var f func()
	f = func() {
		// 用调用栈深度而不是次数来判定：次数由下面的 500 上限控制，
		// 深度才是"有没有递归"的直接证据。
		if d := stackDepth(); d > deepest.Load() {
			deepest.Store(d)
		}
		if count.Add(1) < 500 {
			tk.OnDispose(f)
		}
	}
	tk.OnDispose(f)

	if count.Load() != 500 {
		t.Fatalf("回调只跑了 %d 次，补跑机制没生效", count.Load())
	}
	base := stackDepth()
	if grew := deepest.Load() - base; grew > 50 {
		t.Fatalf("嵌套注册导致栈深度增长 %d 层（500 次重注册），说明补跑在调用方栈上递归了", grew)
	}
}

// Test_SelfReRegisterStart 是 start 侧的对称用例。
func Test_SelfReRegisterStart(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatalf("job 启动失败: %v", err)
	}
	defer func() {
		job.Stop(ErrStopByUser)
		_ = job.WaitStopped()
	}()

	var count atomic.Int32
	var deepest atomic.Int32
	var f func()
	f = func() {
		if d := stackDepth(); d > deepest.Load() {
			deepest.Store(d)
		}
		if count.Add(1) < 500 {
			job.OnStart(f)
		}
	}
	job.OnStart(f)

	if count.Load() != 500 {
		t.Fatalf("回调只跑了 %d 次，补跑机制没生效", count.Load())
	}
	base := stackDepth()
	if grew := deepest.Load() - base; grew > 50 {
		t.Fatalf("嵌套注册导致栈深度增长 %d 层（500 次重注册），说明补跑在调用方栈上递归了", grew)
	}
}

// stackDepth 返回当前 goroutine 的调用栈帧数。
func stackDepth() int32 {
	pcs := make([]uintptr, 512)
	return int32(runtime.Callers(0, pcs))
}

// lateDisposeTask 借一个「早注册」的监听器把测试 goroutine 精确地放进
// dispose() 的排空循环中间。
//
// 这里用的是**阻塞式**卡位：钩子发完信号就停住，等测试 goroutine 注册完再放行。
// 非阻塞卡位在这里不够——dispose() 从那个钩子返回到 setState(TASK_STATE_DISPOSED)
// 只隔几条语句，测试 goroutine 还没被调度回来它就跑完了，于是注册实际落在
// 「已 DISPOSED」而不是「排空途中」，用例就悄悄测了另一个窗口。
//
// 反向的放行信号会建立 happens-before 边，但本用例考的是执行次数、不是 -race，
// 所以无妨；race 用例在 hook_register_race_test.go 里，那边严禁反向信号。
type lateDisposeTask struct {
	Task
	entered chan struct{}
	proceed chan struct{}
}

func (t *lateDisposeTask) Run() error { return nil }

// Test_OnDisposeDuringDispose 卡的是最容易两头不着的那个交界：注册发生在
// dispose() 的排空循环正在逐个回调的当口（此时 state == TASK_STATE_DISPOSING）。
//
// 要求是**不多不少正好一次**——既不能丢掉（修复前就是 0 次），也不能因为既补跑
// 又被本轮排空收录而跑两遍。
//
// 这个窗口也是「拿现成的任务状态当判据」会翻车的地方，值得单独钉住：
//
//	判据 GetState() >= TASK_STATE_DISPOSED  → 本窗口 0 次（漏执行）
//	判据 GetState() >= TASK_STATE_DISPOSING → PreDispose 窗口 2 次（重复执行）
//
// 原因是这两个状态的跃迁点都不在监听器开跑的那一刻：DISPOSING 在 dispose() 开头
// 就置了，中间还隔着 PreDispose / 等子任务 / handler.Dispose / 资源释放；DISPOSED
// 又在监听器全部跑完之后。
//
// 需要说清楚的是这**不是原理上不可能**：把 setState(TASK_STATE_DISPOSED) 挪进
// 「监听器已排空」的同一个临界区，`>= TASK_STATE_DISPOSED` 也能对齐。本 PR 没那么
// 做，是因为挪动状态跃迁点会牵动 GetState 的其它使用方（AddTask 的同 key 冲突判定、
// WorkCollection 的可见性），用一个只服务于钩子的内部标志代价更小、影响面更窄。
func Test_OnDisposeDuringDispose(t *testing.T) {
	tk := &lateDisposeTask{entered: make(chan struct{}), proceed: make(chan struct{})}

	// 这一个注册在 AddTask 之前，必然进快照；它执行时说明遍历已经开始，
	// 然后停住不返回，把遍历过程按在原地。
	tk.OnDispose(func() {
		close(tk.entered)
		<-tk.proceed
	})
	root.AddTask(tk)

	<-tk.entered
	var count atomic.Int32
	tk.OnDispose(func() { count.Add(1) })
	if state := tk.GetState(); state != TASK_STATE_DISPOSING {
		t.Fatalf("卡位失败：注册时 state=%d，期望 TASK_STATE_DISPOSING(%d)——"+
			"本用例已经没在测它声称的那个窗口了", state, TASK_STATE_DISPOSING)
	}
	close(tk.proceed)

	_ = tk.WaitStopped()
	if got := count.Load(); got != 1 {
		t.Fatalf("dispose 途中注册的 OnDispose 回调执行了 %d 次，期望 1 次", got)
	}
}
