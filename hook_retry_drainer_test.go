package task

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 本文件针对上游评审指出的一处代际干扰：补跑排空（drainStartHooks /
// drainDisposeHooks）可能跑在**任意** goroutine 上——晚注册时就是注册方自己的
// goroutine——而 `reset()` 只复位标志与队列，不动 catchingUp。
//
// 于是上一轮那个还没退出的排空 goroutine 会活到下一轮，与下一轮由事件循环发起的
// 排空并存。两者共用同一组字段，上一轮那个只要先取到锁、看到队列空，就会把
// catchingUp 置回 false —— 此后到达的注册不再由事件循环代跑，而是落到注册方自己的
// 栈上，与本轮排空并发。
//
// 触发前提只有重试路径才有：`start()` 失败分支里是 Stop → onChildDispose →
// checkRetry → reset() → 下一轮 start()。

type retryDrainTask struct {
	Task
	mu            sync.Mutex
	rounds        int
	round1Entered chan struct{} // 第 1 轮 Run() 已进入（此时本轮排空必已结束）
	round1Fail    chan struct{} // 关闭后第 1 轮 Run() 返回可重试错误
	done          chan struct{} // 第 2 轮 Run() 挂在这里，测试结束时放行
}

func (t *retryDrainTask) Start() error {
	t.mu.Lock()
	t.rounds++
	t.mu.Unlock()
	return nil
}

func (t *retryDrainTask) Run() error {
	t.mu.Lock()
	round := t.rounds
	t.mu.Unlock()
	if round == 1 {
		close(t.round1Entered)
		<-t.round1Fail
		return errors.New("round 1 failed on purpose") // 可重试错误
	}
	<-t.done
	return ErrTaskComplete
}

func Test_RetryDrainerGenerationInterference(t *testing.T) {
	tk := &retryDrainTask{
		round1Entered: make(chan struct{}),
		round1Fail:    make(chan struct{}),
		done:          make(chan struct{}),
	}
	defer close(tk.done)
	tk.SetRetry(3, 0) // 退避 0，reset() 紧跟着上一轮发生
	root.AddTask(tk)

	// 等第 1 轮 Run() 进入：此刻 startHooksRun=true 且本轮排空已结束
	// （catchingUpStart 已被置回 false），晚注册才会真的自己起一个排空 goroutine。
	<-tk.round1Entered

	var invocations atomic.Int32
	inv1Begun := make(chan struct{})
	inv2Begun := make(chan struct{})
	wedge1 := make(chan struct{})
	wedge2 := make(chan struct{})

	listener := func() {
		switch invocations.Add(1) {
		case 1: // 第 1 轮，跑在注册方 goroutine 上（D_stale）
			close(inv1Begun)
			<-wedge1
		case 2: // 第 2 轮，跑在事件循环 goroutine 上（D_new）
			close(inv2Begun)
			<-wedge2
		}
	}

	// 晚注册：任务已启动，本次注册由注册方自己补跑 —— 这就是 D_stale。
	go tk.OnStart(listener)
	<-inv1Begun

	// 让第 1 轮失败：Stop → onChildDispose → checkRetry → reset() → 第 2 轮 start()，
	// 第 2 轮把 afterStartListeners 重新灌进队列，由事件循环排空（D_new）。
	close(tk.round1Fail)
	<-inv2Begun

	// 此刻 D_new 正卡在 listener 里、排空未结束。放行上一轮的 D_stale，
	// 它会回到循环顶部取锁、看到空队列。
	close(wedge1)
	time.Sleep(200 * time.Millisecond) // 给 D_stale 充分时间退出

	tk.hookMu.Lock()
	catchingUp := tk.catchingUpStart
	tk.hookMu.Unlock()
	close(wedge2)

	if !catchingUp {
		t.Fatalf("上一轮的排空 goroutine 把 catchingUpStart 置回了 false，" +
			"而本轮的排空仍在进行：此后到达的 OnStart 会落到注册方自己的栈上、" +
			"与本轮排空并发，而不是由事件循环代跑")
	}
}

// dispose 侧同形态。dispose() 在重试路径上每轮都会跑一次（start() 失败分支里的
// Stop → onChildDispose → dispose），所以上一轮遗留的排空者同样会活到下一轮。
//
// 这里退避取 200ms 而不是 0：晚注册必须落在「本轮 dispose 已排空完」与「reset()」
// 之间，退避窗口就是这段可控的间隙。

type retryDisposeDrainTask struct {
	Task
	mu     sync.Mutex
	rounds int
	done   chan struct{}
}

func (t *retryDisposeDrainTask) Start() error {
	t.mu.Lock()
	t.rounds++
	t.mu.Unlock()
	return nil
}

func (t *retryDisposeDrainTask) Run() error {
	t.mu.Lock()
	round := t.rounds
	t.mu.Unlock()
	if round <= 2 {
		return errors.New("failed on purpose") // 可重试错误，每轮都会走一次 dispose()
	}
	<-t.done
	return ErrTaskComplete
}

func Test_RetryDisposeDrainerGenerationInterference(t *testing.T) {
	tk := &retryDisposeDrainTask{done: make(chan struct{})}
	defer close(tk.done)
	tk.SetRetry(3, 200*time.Millisecond)

	round1Drained := make(chan struct{})
	var early atomic.Int32
	// 早注册的监听器：它跑在本轮 dispose 的排空循环里，跑完就说明该轮排空已经开始，
	// 紧接着排空循环会取到空队列并把 catchingUpDispose 置回 false。
	tk.OnDispose(func() {
		if early.Add(1) == 1 {
			close(round1Drained)
		}
	})
	root.AddTask(tk)

	<-round1Drained
	time.Sleep(30 * time.Millisecond) // 等第 1 轮排空循环收尾（退避还有 200ms，余量充足）

	var invocations atomic.Int32
	inv1Begun := make(chan struct{})
	inv2Begun := make(chan struct{})
	wedge1 := make(chan struct{})
	wedge2 := make(chan struct{})

	listener := func() {
		switch invocations.Add(1) {
		case 1: // 第 1 轮补跑，跑在注册方 goroutine 上（D_stale）
			close(inv1Begun)
			<-wedge1
		case 2: // 第 2 轮 dispose，跑在事件循环 goroutine 上（D_new）
			close(inv2Begun)
			<-wedge2
		}
	}

	go tk.OnDispose(listener) // 晚注册：本轮 dispose 已排空过，注册方自己补跑
	<-inv1Begun
	<-inv2Begun               // 第 2 轮 dispose 的排空已经在跑 listener

	close(wedge1)                      // 放行上一轮的 D_stale
	time.Sleep(200 * time.Millisecond) // 给它充分时间退出

	tk.hookMu.Lock()
	catchingUp := tk.catchingUpDispose
	tk.hookMu.Unlock()
	close(wedge2)

	if !catchingUp {
		t.Fatalf("上一轮的排空 goroutine 把 catchingUpDispose 置回了 false，" +
			"而本轮的排空仍在进行：此后到达的 OnDispose 会落到注册方自己的栈上、" +
			"与本轮排空并发，而不是由事件循环代跑")
	}
}
