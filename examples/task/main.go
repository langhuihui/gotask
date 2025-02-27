package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/langhuihui/gotask/pkg/task"
)

var root task.RootTask

func init() {
	root.Init()
}

// Define a custom task
type MyTask struct {
	task.Task
	Name string
}

type MyTickTask struct {
	task.TickTask
	Count int
}

func (t *MyTickTask) Tick(any) {
	t.Count--
	if t.Count <= 0 {
		t.Stop(task.ErrTaskComplete)
	}
}

func (t *MyTickTask) GetTickInterval() time.Duration {
	return time.Second / 2
}

// Implement the Run method
func (t *MyTask) Run() error {
	fmt.Printf("Task %s is running\n", t.Name)
	time.Sleep(time.Second)
	fmt.Printf("Task %s completed\n", t.Name)
	return nil
}

// Main function to demonstrate task usage
func main() {
	// 创建日志记录器
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	logger.Info("Starting task examples")

	// 监听系统信号，用于优雅关闭
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		<-signalChan
		logger.Info("Received shutdown signal")
		root.Stop(task.ErrExit)
	}()

	// 简单任务，带重试
	taskWithRetry := &MyTask{Name: "RetryTask"}
	task1 := root.AddTask(taskWithRetry)
	task1.SetRetry(3, time.Second)
	task1.OnStart(func() {
		logger.Info("RetryTask started")
	})
	task1.OnDispose(func() {
		logger.Info("RetryTask disposed")
	})

	// 等待第一个任务启动
	_ = task1.WaitStarted()

	// 添加一个定时器任务，运行5次
	tickerTask := &MyTickTask{
		Count: 5,
	}
	tickerTaskRef := root.AddTask(tickerTask)

	// 添加一个依赖于定时器任务的任务
	dependentTask := &MyTask{Name: "DependentTask"}
	task2 := root.AddTask(dependentTask)
	task2.Depend(tickerTaskRef)
	task2.OnStart(func() {
		logger.Info("DependentTask started - this will only happen after ticker completes")
	})

	// 等待所有任务完成
	time.Sleep(10 * time.Second)

	// 关闭所有任务
	logger.Info("Shutting down")
}
