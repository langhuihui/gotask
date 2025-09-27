package main

import (
	"fmt"
	"time"

	task "github.com/langhuihui/gotask"
)

// WebService 网络服务
type WebService struct {
	task.Task
	ServiceName string
	Port        int
}

func (w *WebService) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// w.Info("网络服务启动", "serviceName", w.ServiceName, "port", w.Port)
	// fmt.Printf("网络服务 %s 在端口 %d 上启动\n", w.ServiceName, w.Port)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (w *WebService) Go() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// w.Info("网络服务运行中", "serviceName", w.ServiceName)
	// fmt.Printf("网络服务 %s 正在运行...\n", w.ServiceName)
	//
	// // 模拟服务运行
	// ticker := time.NewTicker(3 * time.Second)
	// defer ticker.Stop()
	//
	// for {
	// 	select {
	// 	case <-ticker.C:
	// 		fmt.Printf("网络服务 %s 状态检查正常\n", w.ServiceName)
	// 	case <-w.Done():
	// 		fmt.Printf("网络服务 %s 收到停止信号\n", w.ServiceName)
	// 		return nil
	// 	}
	// }

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Go方法中的TODO注释")
	return fmt.Errorf("Go方法未完成")
}

func (w *WebService) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// w.Info("网络服务清理", "serviceName", w.ServiceName)
	// fmt.Printf("网络服务 %s 已停止并清理\n", w.ServiceName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// DatabaseService 数据库服务
type DatabaseService struct {
	task.Task
	ServiceName      string
	ConnectionString string
}

func (d *DatabaseService) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// d.Info("数据库服务启动", "serviceName", d.ServiceName)
	// fmt.Printf("数据库服务 %s 启动，连接字符串: %s\n", d.ServiceName, d.ConnectionString)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (d *DatabaseService) Go() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// d.Info("数据库服务运行中", "serviceName", d.ServiceName)
	// fmt.Printf("数据库服务 %s 正在运行...\n", d.ServiceName)
	//
	// // 模拟数据库服务运行
	// ticker := time.NewTicker(2 * time.Second)
	// defer ticker.Stop()
	//
	// for {
	// 	select {
	// 	case <-ticker.C:
	// 		fmt.Printf("数据库服务 %s 连接池状态正常\n", d.ServiceName)
	// 	case <-d.Done():
	// 		fmt.Printf("数据库服务 %s 收到停止信号\n", d.ServiceName)
	// 		return nil
	// 	}
	// }

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Go方法中的TODO注释")
	return fmt.Errorf("Go方法未完成")
}

func (d *DatabaseService) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// d.Info("数据库服务清理", "serviceName", d.ServiceName)
	// fmt.Printf("数据库服务 %s 已停止并清理\n", d.ServiceName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// 使用RootManager作为根任务管理器
type TaskManager = task.RootManager[uint32, *WebService]

func main() {
	fmt.Println("=== GoTask Lesson 6: RootManager根任务管理 ===")
	fmt.Println("本课程将教你如何使用RootManager管理整个应用程序")
	fmt.Println("请按照TODO注释的提示，逐步取消注释来完成代码")
	fmt.Println()

	// 创建根任务管理器
	root := &TaskManager{}
	// TODO: 取消下面的注释来初始化根任务管理器
	// root.Init()

	// 验证：检查是否初始化了根任务管理器
	if root.GetTaskID() == 0 {
		fmt.Println("❌ 错误：请取消root.Init()的TODO注释")
		fmt.Println("请检查并完成所有TODO注释")
		return
	}

	// 创建网络服务
	webService := &WebService{
		ServiceName: "HTTP服务",
		Port:        8080,
	}

	// 创建数据库服务
	dbService := &DatabaseService{
		ServiceName:      "MySQL数据库",
		ConnectionString: "mysql://localhost:3306/mydb",
	}

	// TODO: 取消下面的注释来添加服务到根管理器中
	// root.AddTask(webService)
	// root.AddTask(dbService)

	// 验证：检查是否添加了服务
	if webService.GetTaskID() == 0 || dbService.GetTaskID() == 0 {
		fmt.Println("❌ 错误：请取消AddTask的TODO注释")
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 等待服务启动
	// webService.WaitStarted()
	// dbService.WaitStarted()

	// 让服务运行一段时间
	time.Sleep(10 * time.Second)

	// 停止服务
	// webService.Stop(task.ErrTaskComplete)
	// dbService.Stop(task.ErrTaskComplete)
	// webService.WaitStopped()
	// dbService.WaitStopped()

	fmt.Println("=== 课程完成 ===")
	fmt.Println("如果看到所有服务的运行日志，说明你已经成功完成了Lesson 6!")

	// 优雅关闭
	// root.Shutdown()
}
