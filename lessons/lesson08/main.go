package main

import (
	"fmt"

	task "github.com/langhuihui/gotask"
)

// UnstableTask 不稳定任务
type UnstableTask struct {
	task.Task
	TaskName     string
	AttemptCount int
	MaxAttempts  int
}

func (u *UnstableTask) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// u.Info("不稳定任务启动", "taskName", u.TaskName)
	// fmt.Printf("不稳定任务 %s 已启动\n", u.TaskName)
	//
	// // 设置重试策略：最多重试3次，每次间隔1秒
	// u.SetRetry(3, 1*time.Second)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (u *UnstableTask) Run() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// u.AttemptCount++
	// u.Info("不稳定任务运行中", "taskName", u.TaskName, "attempt", u.AttemptCount)
	// fmt.Printf("不稳定任务 %s 第 %d 次尝试\n", u.TaskName, u.AttemptCount)
	//
	// // 模拟不稳定的操作：前两次失败，第三次成功
	// if u.AttemptCount < 3 {
	// 	fmt.Printf("不稳定任务 %s 第 %d 次尝试失败\n", u.TaskName, u.AttemptCount)
	// 	return fmt.Errorf("任务执行失败，尝试次数: %d", u.AttemptCount)
	// }
	//
	// fmt.Printf("不稳定任务 %s 第 %d 次尝试成功\n", u.TaskName, u.AttemptCount)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Run方法中的TODO注释")
	return fmt.Errorf("Run方法未完成")
}

func (u *UnstableTask) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// u.Info("不稳定任务清理", "taskName", u.TaskName, "totalAttempts", u.AttemptCount)
	// fmt.Printf("不稳定任务 %s 已清理，总共尝试了 %d 次\n", u.TaskName, u.AttemptCount)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// NetworkTask 网络任务
type NetworkTask struct {
	task.Task
	TaskName     string
	AttemptCount int
}

func (n *NetworkTask) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// n.Info("网络任务启动", "taskName", n.TaskName)
	// fmt.Printf("网络任务 %s 已启动\n", n.TaskName)
	//
	// // 设置重试策略：无限重试，每次间隔2秒
	// n.SetRetry(-1, 2*time.Second)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (n *NetworkTask) Run() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// n.AttemptCount++
	// n.Info("网络任务运行中", "taskName", n.TaskName, "attempt", n.AttemptCount)
	// fmt.Printf("网络任务 %s 第 %d 次尝试\n", n.TaskName, n.AttemptCount)
	//
	// // 模拟网络操作：前5次失败，第6次成功
	// if n.AttemptCount < 6 {
	// 	fmt.Printf("网络任务 %s 第 %d 次尝试失败\n", n.TaskName, n.AttemptCount)
	// 	return fmt.Errorf("网络连接失败，尝试次数: %d", n.AttemptCount)
	// }
	//
	// fmt.Printf("网络任务 %s 第 %d 次尝试成功\n", n.TaskName, n.AttemptCount)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Run方法中的TODO注释")
	return fmt.Errorf("Run方法未完成")
}

func (n *NetworkTask) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// n.Info("网络任务清理", "taskName", n.TaskName, "totalAttempts", n.AttemptCount)
	// fmt.Printf("网络任务 %s 已清理，总共尝试了 %d 次\n", n.TaskName, n.AttemptCount)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// RetryManager 重试管理任务
type RetryManager struct {
	task.Job
	ManagerName string
}

func (r *RetryManager) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// r.Info("重试管理任务启动", "managerName", r.ManagerName)
	// fmt.Printf("重试管理任务 %s 已启动\n", r.ManagerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (r *RetryManager) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// r.Info("重试管理任务清理", "managerName", r.ManagerName)
	// fmt.Printf("重试管理任务 %s 已清理\n", r.ManagerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// 使用RootManager作为根任务管理器
type TaskManager = task.RootManager[uint32, *RetryManager]

func main() {
	fmt.Println("=== GoTask Lesson 8: 重试机制 ===")
	fmt.Println("本课程将教你如何使用重试机制处理失败任务")
	fmt.Println("请按照TODO注释的提示，逐步取消注释来完成代码")
	fmt.Println()

	// 创建根任务管理器
	root := &TaskManager{}
	root.Init()

	// 创建重试管理任务
	retryManager := &RetryManager{ManagerName: "重试管理器"}

	// 将重试管理任务添加到根管理器中
	root.AddTask(retryManager)

	// 等待重试管理任务启动
	err := retryManager.WaitStarted()
	if err != nil {
		fmt.Printf("❌ 重试管理任务启动失败: %v\n", err)
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 创建不稳定任务
	unstableTask := &UnstableTask{TaskName: "不稳定任务"}

	// 创建网络任务
	networkTask := &NetworkTask{TaskName: "网络连接任务"}

	// TODO: 取消下面的注释来添加任务到重试管理器中
	// retryManager.AddTask(unstableTask)
	// retryManager.AddTask(networkTask)

	// 验证：检查是否添加了任务
	if unstableTask.GetTaskID() == 0 || networkTask.GetTaskID() == 0 {
		fmt.Println("❌ 错误：请取消AddTask的TODO注释")
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 等待所有任务完成
	// unstableTask.WaitStopped()
	// networkTask.WaitStopped()

	// 停止重试管理任务
	retryManager.Stop(task.ErrTaskComplete)
	retryManager.WaitStopped()

	fmt.Println("=== 课程完成 ===")
	fmt.Println("如果看到重试机制的日志，说明你已经成功完成了Lesson 8!")

	// 优雅关闭
	root.Shutdown()
}
