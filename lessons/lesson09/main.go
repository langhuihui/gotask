package main

import (
	"fmt"

	task "github.com/langhuihui/gotask"
)

// WorkerTask 工作任务
type WorkerTask struct {
	task.Task
	WorkerID int
}

func (w *WorkerTask) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// w.Info("工作任务启动", "workerID", w.WorkerID)
	// fmt.Printf("工作任务 %d 已启动\n", w.WorkerID)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (w *WorkerTask) Run() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// w.Info("工作任务运行中", "workerID", w.WorkerID)
	// fmt.Printf("工作任务 %d 正在运行...\n", w.WorkerID)
	//
	// // 模拟工作
	// time.Sleep(2 * time.Second)
	// fmt.Printf("工作任务 %d 工作完成\n", w.WorkerID)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Run方法中的TODO注释")
	return fmt.Errorf("Run方法未完成")
}

func (w *WorkerTask) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// w.Info("工作任务清理", "workerID", w.WorkerID)
	// fmt.Printf("工作任务 %d 已清理\n", w.WorkerID)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// EventManager 事件管理任务
type EventManager struct {
	task.Job
	ManagerName  string
	StartCount   int
	DisposeCount int
}

func (e *EventManager) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// e.Info("事件管理任务启动", "managerName", e.ManagerName)
	// fmt.Printf("事件管理任务 %s 已启动\n", e.ManagerName)
	//
	// // 监听子任务启动事件
	// e.OnDescendantsStart(func(task task.ITask) {
	// 	e.StartCount++
	// 	fmt.Printf("事件管理任务 %s 监听到子任务启动: %s (总计: %d)\n",
	// 		e.ManagerName, task.GetOwnerType(), e.StartCount)
	// })
	//
	// // 监听子任务清理事件
	// e.OnDescendantsDispose(func(task task.ITask) {
	// 	e.DisposeCount++
	// 	fmt.Printf("事件管理任务 %s 监听到子任务清理: %s (总计: %d)\n",
	// 		e.ManagerName, task.GetOwnerType(), e.DisposeCount)
	// })
	//
	// // 设置任务启动后的回调
	// e.OnStart(func() {
	// 	fmt.Printf("事件管理任务 %s 启动完成回调\n", e.ManagerName)
	// })
	//
	// // 设置任务清理后的回调
	// e.OnDispose(func() {
	// 	fmt.Printf("事件管理任务 %s 清理完成回调\n", e.ManagerName)
	// })

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (e *EventManager) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// e.Info("事件管理任务清理", "managerName", e.ManagerName,
	// 	"startCount", e.StartCount, "disposeCount", e.DisposeCount)
	// fmt.Printf("事件管理任务 %s 已清理，启动事件: %d, 清理事件: %d\n",
	// 	e.ManagerName, e.StartCount, e.DisposeCount)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// 使用RootManager作为根任务管理器
type TaskManager = task.RootManager[uint32, *EventManager]

func main() {
	fmt.Println("=== GoTask Lesson 9: 事件监听与回调 ===")
	fmt.Println("本课程将教你如何使用事件监听和回调机制")
	fmt.Println("请按照TODO注释的提示，逐步取消注释来完成代码")
	fmt.Println()

	// 创建根任务管理器
	root := &TaskManager{}
	root.Init()

	// 创建事件管理任务
	eventManager := &EventManager{ManagerName: "事件管理器"}

	// 将事件管理任务添加到根管理器中
	root.AddTask(eventManager)

	// 等待事件管理任务启动
	err := eventManager.WaitStarted()
	if err != nil {
		fmt.Printf("❌ 事件管理任务启动失败: %v\n", err)
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 创建多个工作任务
	workers := make([]*WorkerTask, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask{WorkerID: i}
		// TODO: 取消下面的注释来添加工作任务到事件管理器中
		// eventManager.AddTask(workers[i-1])
	}

	// 验证：检查是否添加了工作任务
	if len(workers) > 0 && workers[0].GetTaskID() == 0 {
		fmt.Println("❌ 错误：请取消AddTask的TODO注释")
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 等待所有任务完成
	eventManager.WaitStopped()

	fmt.Println("=== 课程完成 ===")
	fmt.Println("如果看到事件监听的日志，说明你已经成功完成了Lesson 9!")

	// 优雅关闭
	root.Shutdown()
}
