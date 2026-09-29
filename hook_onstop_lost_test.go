package task

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// 本文件针对 OnStop 的一处资源泄漏：Stop() 与 Start() 并发时，Start() 内部注册的
// 清理资源永远不会被执行。
//
// stop() 开头那句 `task.WaitStarted() // wait for task to set closeOnStop` 表达的
// 意图是对的——排空 closeOnStop 之前要等 Start() 把资源登记完。但它兑现不了：
// startup 是 util.NewPromise(task.Context) 建的，Await() 等的是 ctx.Done()，而
// Stop() 先 CancelCauseFunc(err) 再调 stop()，上下文此时已经取消，WaitStarted()
// 立即返回。于是 stop() 排到的是一份空切片。
//
// 随后 Start() 返回，把资源 append 进 closeOnStop，而这份切片再也不会有人遍历：
//
//	start()   Start() 返回 nil → IsStopped() 为真 → 跳过 Run() → 直接进 dispose
//	dispose() 唯一的收尾是 stopOnce.Do(task.stop)，而 once 已被那次并发 Stop 消费
//
// 后果取决于还有没有下一轮重试：有则晚一轮才关（reset() 不清 closeOnStop），
// 没有则永久泄漏。命中的是"资源在 Start() 里建好之后才注册清理钩子"这一类写法，
// 例如 monibuca 的 RTSP 客户端在 Connect() 成功后才 OnStop(TeardownAndClose)——
// 撞上这个窗口就是连接永不发 TEARDOWN，源端会话槽位被占死。

type stopDuringStartTask struct {
	Task
	startEntered chan struct{} // Start() 已进入，但还没注册资源
	releaseStart chan struct{} // 关闭后 Start() 才去注册资源
	closed       atomic.Bool   // 资源是否真的被关掉了
}

func (t *stopDuringStartTask) Start() error {
	close(t.startEntered)
	<-t.releaseStart
	// 生产形态：资源（连接/句柄）在 Start() 内部建立成功之后才注册清理钩子。
	t.OnStop(func() { t.closed.Store(true) })
	return nil
}

func (t *stopDuringStartTask) Run() error {
	<-t.Done()
	return t.StopReason()
}

func Test_OnStopLostWhenStoppedDuringStart(t *testing.T) {
	tk := &stopDuringStartTask{
		startEntered: make(chan struct{}),
		releaseStart: make(chan struct{}),
	}

	// 断言必须等到销毁走完才做，否则会跑在 Start() 恢复之前，红得不是地方。
	// 注意不能用 WaitStopped()：它开头的 WaitStarted() 在上下文已取消时直接带着
	// 错误返回，根本不等 shutdown。这里改用 dispose 末尾必然排空的 OnDispose 定序，
	// 顺带把"最终有没有关掉"的判据放宽到整个生命周期，而不是只认某一种修法。
	disposed := make(chan struct{})
	tk.OnDispose(func() { close(disposed) })

	root.AddTask(tk)

	<-tk.startEntered

	// Stop() 是同步的：它返回就意味着 stopOnce 已经消费、stop() 已经排空过
	// closeOnStop。此刻 Start() 还卡在 releaseStart 上，排到的必然是空切片。
	tk.Stop(errors.New("stopped while starting"))

	close(tk.releaseStart) // 现在才让 Start() 注册资源

	select {
	case <-disposed:
	case <-time.After(3 * time.Second):
		t.Fatal("任务没有在 3s 内完成销毁")
	}

	if !tk.closed.Load() {
		t.Fatal("Start() 里注册的 OnStop 资源没有被关闭：Stop() 与 Start() 并发时，" +
			"stop() 排到的是空的 closeOnStop，而 dispose() 里的 stopOnce.Do(task.stop) " +
			"已被那次 Stop 消费掉，退化成空操作——资源就此无人回收")
	}
}
