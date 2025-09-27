package main

import (
	"fmt"
	"os"

	task "github.com/langhuihui/gotask"
)

// FileProcessor 文件处理任务
type FileProcessor struct {
	task.Task
	FileName   string
	FileHandle *os.File
	Processed  bool
}

func (f *FileProcessor) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// f.Info("文件处理任务启动", "fileName", f.FileName)
	// fmt.Printf("文件处理任务 %s 已启动\n", f.FileName)
	//
	// // 打开文件
	// var err error
	// f.FileHandle, err = os.Open(f.FileName)
	// if err != nil {
	// 	return fmt.Errorf("无法打开文件 %s: %v", f.FileName, err)
	// }
	//
	// // 使用Using方法注册资源，任务停止时会自动清理
	// f.Using(f.FileHandle)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (f *FileProcessor) Run() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// f.Info("文件处理任务运行中", "fileName", f.FileName)
	// fmt.Printf("文件处理任务 %s 正在处理文件...\n", f.FileName)
	//
	// // 模拟文件处理
	// time.Sleep(2 * time.Second)
	// f.Processed = true
	// fmt.Printf("文件处理任务 %s 处理完成\n", f.FileName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Run方法中的TODO注释")
	return fmt.Errorf("Run方法未完成")
}

func (f *FileProcessor) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// f.Info("文件处理任务清理", "fileName", f.FileName, "processed", f.Processed)
	// fmt.Printf("文件处理任务 %s 已清理，处理状态: %v\n", f.FileName, f.Processed)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// NetworkConnection 网络连接任务
type NetworkConnection struct {
	task.Task
	ConnectionName string
	Connected      bool
}

func (n *NetworkConnection) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// n.Info("网络连接任务启动", "connectionName", n.ConnectionName)
	// fmt.Printf("网络连接任务 %s 已启动\n", n.ConnectionName)
	//
	// // 模拟网络连接
	// time.Sleep(1 * time.Second)
	// n.Connected = true
	// fmt.Printf("网络连接任务 %s 连接成功\n", n.ConnectionName)
	//
	// // 使用OnStop方法注册清理函数
	// n.OnStop(func() {
	// 	n.Connected = false
	// 	fmt.Printf("网络连接任务 %s 连接已断开\n", n.ConnectionName)
	// })

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (n *NetworkConnection) Run() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// n.Info("网络连接任务运行中", "connectionName", n.ConnectionName)
	// fmt.Printf("网络连接任务 %s 正在运行...\n", n.ConnectionName)
	//
	// // 模拟网络活动
	// time.Sleep(3 * time.Second)
	// fmt.Printf("网络连接任务 %s 运行完成\n", n.ConnectionName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Run方法中的TODO注释")
	return fmt.Errorf("Run方法未完成")
}

func (n *NetworkConnection) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// n.Info("网络连接任务清理", "connectionName", n.ConnectionName)
	// fmt.Printf("网络连接任务 %s 已清理\n", n.ConnectionName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// ResourceManager 资源管理任务
type ResourceManager struct {
	task.Job
	ManagerName string
}

func (r *ResourceManager) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// r.Info("资源管理任务启动", "managerName", r.ManagerName)
	// fmt.Printf("资源管理任务 %s 已启动\n", r.ManagerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (r *ResourceManager) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// r.Info("资源管理任务清理", "managerName", r.ManagerName)
	// fmt.Printf("资源管理任务 %s 已清理\n", r.ManagerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// 使用RootManager作为根任务管理器
type TaskManager = task.RootManager[uint32, *ResourceManager]

func main() {
	fmt.Println("=== GoTask Lesson 7: 资源管理与清理 ===")
	fmt.Println("本课程将教你如何正确管理任务资源")
	fmt.Println("请按照TODO注释的提示，逐步取消注释来完成代码")
	fmt.Println()

	// 创建根任务管理器
	root := &TaskManager{}
	root.Init()

	// 创建资源管理任务
	resourceManager := &ResourceManager{ManagerName: "资源管理器"}

	// 将资源管理任务添加到根管理器中
	root.AddTask(resourceManager)

	// 等待资源管理任务启动
	err := resourceManager.WaitStarted()
	if err != nil {
		fmt.Printf("❌ 资源管理任务启动失败: %v\n", err)
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 创建文件处理任务
	fileProcessor := &FileProcessor{FileName: "test.txt"}

	// 创建网络连接任务
	networkConnection := &NetworkConnection{ConnectionName: "数据库连接"}

	// TODO: 取消下面的注释来添加任务到资源管理器中
	// resourceManager.AddTask(fileProcessor)
	// resourceManager.AddTask(networkConnection)

	// 验证：检查是否添加了任务
	if fileProcessor.GetTaskID() == 0 || networkConnection.GetTaskID() == 0 {
		fmt.Println("❌ 错误：请取消AddTask的TODO注释")
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 等待所有任务完成
	// fileProcessor.WaitStopped()
	// networkConnection.WaitStopped()

	// 停止资源管理任务
	resourceManager.Stop(task.ErrTaskComplete)
	resourceManager.WaitStopped()

	fmt.Println("=== 课程完成 ===")
	fmt.Println("如果看到资源清理的日志，说明你已经成功完成了Lesson 7!")

	// 优雅关闭
	root.Shutdown()
}
