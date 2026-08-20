package task

import (
	"testing"
)

// 本文件复现「钩子注册」与「钩子执行」之间的数据竞争。
//
// OnStart/OnDispose/OnStop/Using 都是裸 append，而消费端都是裸遍历，两端分处
// 不同 goroutine：注册通常发生在调用 AddTask 的那个业务 goroutine 上，消费则
// 发生在持有该任务的事件循环 goroutine 上。只要注册发生在 AddTask 之后——而这
// 一点没有任何东西能阻止，调用方甚至无从判断自己是不是晚了——两端就完全没有
// 同步。
//
//	写 task.go OnStart()   ← 业务 goroutine
//	读 task.go start()     遍历 afterStartListeners     ← 事件循环 goroutine
//	写 task.go OnDispose() ← 业务 goroutine
//	读 task.go dispose()   遍历 afterDisposeListeners   ← 事件循环 goroutine
//
// 需要强调最坏后果不止是"漏掉一个回调"：切片头是 ptr/len/cap 三个字长，写入
// 非原子，range 那次求值同样是三个非原子读，因此消费端可能读到 len 已更新、
// ptr 仍指向旧底层数组的撕裂头，越界取到的内存会被当作 func() 调用。
//
// 两个用例都不做构造性断言：是否发生竞争交给 -race 判定。它们也都是**确定性**
// 的——各自只在同一地址上制造一对访问，不依赖调度运气：
//
//   - start() 在 task.go 里先 startup.Fulfill() 再遍历 afterStartListeners，
//     所以 WaitStarted() 返回的位置精确地卡在那次读之前；
//   - dispose() 先调用 handler 的 Dispose()、之后才遍历 afterDisposeListeners，
//     所以从 Dispose() 里发信号同样卡在那次读之前。
//
// 注意信号方向只能是「事件循环 → 测试 goroutine」。反向的放行信号会建立
// happens-before 边，把竞争抹掉，用例就恒绿了。

// startRaceJob 用 Job 而不是 Task：Job 起来后会一直活着，保证 WaitStarted()
// 返回时任务仍停在 start() 里，还没走到 dispose。
type startRaceJob struct {
	Job
}

// Test_OnStartRegisterRace 证明任务启动之后再注册 OnStart 会与 start() 的遍历
// 打架。上游 event_loop_test.go 也是这个用法（先 AddTask 再 OnStart），只是那里
// 出现与否要看调度。
func Test_OnStartRegisterRace(t *testing.T) {
	var job startRaceJob
	root.AddTask(&job)
	// WaitStarted() 在 startup.Fulfill() 之后返回，而紧接的下一行才是
	// `for _, listener := range task.afterStartListeners`——注册与那次读并发。
	if err := job.WaitStarted(); err != nil {
		t.Fatalf("job 启动失败: %v", err)
	}
	job.OnStart(func() {})

	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
}

// disposeRaceTask 在自己的 Dispose() 里发信号：此刻 dispose() 才走到「self」
// 阶段，afterDisposeListeners 的遍历还在后面。
type disposeRaceTask struct {
	Task
	entered chan struct{}
}

func (t *disposeRaceTask) Run() error { return nil } // 立即完成，随即进入 dispose

func (t *disposeRaceTask) Dispose() { close(t.entered) }

// Test_OnDisposeRegisterRace 证明任务销毁过程中再注册 OnDispose 会与 dispose()
// 的遍历打架。这正是 monibuca 拉流代理踩到的形态：Pull() 在 AddTask 之后才注册
// OnDispose，而短命的 job 早已开始销毁。
func Test_OnDisposeRegisterRace(t *testing.T) {
	tk := &disposeRaceTask{entered: make(chan struct{})}
	root.AddTask(tk)

	<-tk.entered // dispose() 已进入 self 阶段，afterDisposeListeners 的遍历即将发生
	tk.OnDispose(func() {})

	_ = tk.WaitStopped()
}
