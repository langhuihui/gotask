package task

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// shutdown 关闭接口
type shutdown interface {
	Shutdown()
}

// OSSignal 操作系统信号处理任务
type OSSignal struct {
	ChannelTask
	root shutdown
}

func (o *OSSignal) Start() error {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	o.SignalChan = signalChan
	o.OnStop(func() {
		signal.Stop(signalChan)
		close(signalChan)
	})
	return nil
}

func (o *OSSignal) Tick(any) {
	go o.root.Shutdown()
}

// ManagerItem 管理器项目接口
type ManagerItem[K comparable] interface {
	ITask
	GetKey() K
}

// RootManager 根任务管理器
type RootManager[K comparable, T ManagerItem[K]] struct {
	WorkCollection[K, T]
}

// Init 初始化根任务管理器
func (m *RootManager[K, T]) Init() {
	m.parentCtx = context.Background()
	m.reset()
	m.handler = m
	m.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	m.StartTime = time.Now()
	m.AddTask(&OSSignal{root: m}).WaitStarted()
	m.state = TASK_STATE_STARTED
}

// Shutdown 关闭根任务管理器
func (m *RootManager[K, T]) Shutdown() {
	fmt.Println("RootManager Shutdown...")
	m.Stop(ErrExit)
	m.dispose()
	fmt.Println("RootManager Shutdown done")
}
