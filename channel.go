package task

import (
	"time"
)

// ITickTask 定时任务接口
type ITickTask interface {
	IChannelTask
	GetTickInterval() time.Duration
	GetTicker() *time.Ticker
}

// ChannelTask 通道任务
type ChannelTask struct {
	Task
	SignalChan any
}

func (*ChannelTask) GetTaskType() TaskType {
	return TASK_TYPE_CHANNEL
}

func (t *ChannelTask) GetSignal() any {
	return t.SignalChan
}

func (t *ChannelTask) Tick(any) {
}

// TickTask 定时任务
type TickTask struct {
	ChannelTask
	Ticker *time.Ticker
}

func (t *TickTask) GetTicker() *time.Ticker {
	return t.Ticker
}

func (t *TickTask) GetTickInterval() time.Duration {
	return time.Second
}

// tickHandler 取本任务绑定的 ITickTask handler。
//
// v2 之前 TickTask 族直接写 t.handler.(ITickTask)：handler 尚未绑定（在 AddTask
// 之前手动调用 Start 是最常见的触发方式）就是一次裸类型断言 panic，且 panic 点
// 落在调用方 goroutine 上——进程直接崩，拿不到任何任务上下文。v2 起统一返回
// ErrNotTickTask，让它走正常的启动失败路径（可记录、可重试、可被 FailurePolicy
// 感知）。
func (t *TickTask) tickHandler() (ITickTask, error) {
	h, ok := t.handler.(ITickTask)
	if !ok || h == nil {
		return nil, ErrNotTickTask
	}
	return h, nil
}

func (t *TickTask) Start() (err error) {
	h, err := t.tickHandler()
	if err != nil {
		return err
	}
	t.Ticker = time.NewTicker(h.GetTickInterval())
	t.SignalChan = t.Ticker.C
	t.OnStop(func() {
		t.Ticker.Reset(time.Millisecond)
	})
	return
}

// AsyncTickTask 异步定时任务
type AsyncTickTask struct {
	TickTask
}

func (t *AsyncTickTask) GetSignal() any {
	return t.Task.GetSignal()
}

// Run 在独立 goroutine 里消费 tick 源，直到本轮 context 结束。
//
// v2 起这是唯一的执行入口：v1 里同样的行为叫 Go()，而 Go 已随 TaskGo 一起删除
// （见 docs/重构-执行模型v2-大版本v2设计.md §4.1）。
func (t *AsyncTickTask) Run() error {
	h, err := t.tickHandler()
	if err != nil {
		return err
	}
	// tick 源由 TickTask.Start 建立。若使用者重写了 Start 却没有调用
	// TickTask.Start，Ticker 为 nil——v1 会在 select 里 nil 解引用 panic
	// （goroutine 内，被 task.run recover 成一条莫名的停止原因），v2 显式报错。
	if t.Ticker == nil {
		return ErrNotTickTask
	}
	h.Tick(nil)
	for {
		select {
		case c := <-t.Ticker.C:
			h.Tick(c)
		case <-t.Done():
			return nil
		}
	}
}