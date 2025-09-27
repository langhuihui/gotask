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

func (t *WorkerTask) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// t.Info("工作线程启动", "workerID", t.WorkerID)
	// fmt.Printf("工作线程 %d 已启动\n", t.WorkerID)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (t *WorkerTask) Run() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// t.Info("工作线程运行中", "workerID", t.WorkerID)
	// fmt.Printf("工作线程 %d 正在工作...\n", t.WorkerID)
	// time.Sleep(1 * time.Second)
	// fmt.Printf("工作线程 %d 工作完成\n", t.WorkerID)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Run方法中的TODO注释")
	return fmt.Errorf("Run方法未完成")
}

func (t *WorkerTask) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// t.Info("工作线程清理", "workerID", t.WorkerID)
	// fmt.Printf("工作线程 %d 已清理\n", t.WorkerID)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// ManagerJob 管理任务容器
type ManagerJob struct {
	task.Job
	JobName string
}

func (j *ManagerJob) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// j.Info("管理任务启动", "jobName", j.JobName)
	// fmt.Printf("管理任务 %s 已启动\n", j.JobName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (j *ManagerJob) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// j.Info("管理任务清理", "jobName", j.JobName)
	// fmt.Printf("管理任务 %s 已清理\n", j.JobName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// 使用RootManager作为根任务管理器
type TaskManager = task.RootManager[uint32, *ManagerJob]

func main() {
	fmt.Println("=== GoTask Lesson 2: Job容器管理 ===")
	fmt.Println("本课程将教你如何使用Job来管理多个子任务")
	fmt.Println("请按照TODO注释的提示，逐步取消注释来完成代码")
	fmt.Println()

	// 创建根任务管理器
	root := &TaskManager{}
	root.Init()

	// 创建管理任务
	manager := &ManagerJob{JobName: "工作管理器"}

	// 将管理任务添加到根管理器中
	root.AddTask(manager)

	// 等待管理任务启动
	err := manager.WaitStarted()
	if err != nil {
		fmt.Printf("❌ 管理任务启动失败: %v\n", err)
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 创建多个工作线程
	workers := make([]*WorkerTask, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask{WorkerID: i}
		// TODO: 取消下面的注释来添加工作线程到管理任务中
		// manager.AddTask(workers[i-1])
	}

	// 验证：检查是否添加了工作线程
	if len(workers) > 0 && workers[0].GetTaskID() == 0 {
		fmt.Println("❌ 错误：请取消AddTask的TODO注释")
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 等待所有任务完成
	manager.WaitStopped()

	fmt.Println("=== 课程完成 ===")
	fmt.Println("如果看到所有任务的执行日志，说明你已经成功完成了Lesson 2!")

	// 优雅关闭
	root.Shutdown()
}
