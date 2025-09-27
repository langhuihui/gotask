package main

import (
	"fmt"
	"time"

	task "github.com/langhuihui/gotask"
)

// TimerTask 定时任务
type TimerTask struct {
	task.TickTask
	TaskName string
	Counter  int
}

func (t *TimerTask) GetTickInterval() time.Duration {
	// TODO: 取消下面的注释来设置定时器间隔
	// return 1 * time.Second

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消GetTickInterval方法中的TODO注释")
	return 0
}

func (t *TimerTask) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// t.Info("定时任务启动", "taskName", t.TaskName)
	// fmt.Printf("定时任务 %s 已启动，间隔: %v\n", t.TaskName, t.GetTickInterval())

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (t *TimerTask) Tick(tick any) {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// t.Counter++
	// t.Info("定时任务执行", "taskName", t.TaskName, "counter", t.Counter)
	// fmt.Printf("定时任务 %s 第 %d 次执行\n", t.TaskName, t.Counter)
	//
	// // 执行5次后自动停止
	// if t.Counter >= 5 {
	// 	fmt.Printf("定时任务 %s 执行完成，自动停止\n", t.TaskName)
	// 	t.Stop(task.ErrTaskComplete)
	// }

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Tick方法中的TODO注释")
}

func (t *TimerTask) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// t.Info("定时任务清理", "taskName", t.TaskName)
	// fmt.Printf("定时任务 %s 已清理，总共执行了 %d 次\n", t.TaskName, t.Counter)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// SchedulerTask 调度器任务
type SchedulerTask struct {
	task.Task
	SchedulerName string
}

func (s *SchedulerTask) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// s.Info("调度器启动", "schedulerName", s.SchedulerName)
	// fmt.Printf("调度器 %s 已启动\n", s.SchedulerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (s *SchedulerTask) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// s.Info("调度器清理", "schedulerName", s.SchedulerName)
	// fmt.Printf("调度器 %s 已清理\n", s.SchedulerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// 使用RootManager作为根任务管理器
type TaskManager = task.RootManager[uint32, *SchedulerTask]

func main() {
	fmt.Println("=== GoTask Lesson 5: TickTask定时任务 ===")
	fmt.Println("本课程将教你如何使用TickTask创建定时任务")
	fmt.Println("请按照TODO注释的提示，逐步取消注释来完成代码")
	fmt.Println()

	// 创建根任务管理器
	root := &TaskManager{}
	root.Init()

	// 创建调度器任务
	scheduler := &SchedulerTask{SchedulerName: "定时调度器"}

	// 将调度器任务添加到根管理器中
	root.AddTask(scheduler)

	// 等待调度器启动
	err := scheduler.WaitStarted()
	if err != nil {
		fmt.Printf("❌ 调度器启动失败: %v\n", err)
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 创建定时任务
	timerTask := &TimerTask{TaskName: "计数器任务"}

	// TODO: 取消下面的注释来添加定时任务到调度器中
	// scheduler.AddTask(timerTask)

	// 验证：检查是否添加了定时任务
	if timerTask.GetTaskID() == 0 {
		fmt.Println("❌ 错误：请取消AddTask的TODO注释")
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 等待定时任务完成
	timerTask.WaitStopped()

	// 停止调度器
	scheduler.Stop(task.ErrTaskComplete)
	scheduler.WaitStopped()

	fmt.Println("=== 课程完成 ===")
	fmt.Println("如果看到定时任务的执行日志，说明你已经成功完成了Lesson 5!")

	// 优雅关闭
	root.Shutdown()
}
