package main

import (
	"fmt"
	"time"

	task "github.com/langhuihui/gotask"
)

// WebServer 网络服务器
type WebServer struct {
	task.Work
	ServerName string
	Port       int
}

func (w *WebServer) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// w.Info("网络服务器启动", "serverName", w.ServerName, "port", w.Port)
	// fmt.Printf("网络服务器 %s 在端口 %d 上启动\n", w.ServerName, w.Port)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (w *WebServer) Go() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// w.Info("网络服务器运行中", "serverName", w.ServerName)
	// fmt.Printf("网络服务器 %s 正在运行...\n", w.ServerName)
	//
	// // 模拟服务器运行
	// ticker := time.NewTicker(3 * time.Second)
	// defer ticker.Stop()
	//
	// for {
	// 	select {
	// 	case <-ticker.C:
	// 		fmt.Printf("网络服务器 %s 状态检查正常\n", w.ServerName)
	// 	case <-w.Done():
	// 		fmt.Printf("网络服务器 %s 收到停止信号\n", w.ServerName)
	// 		return nil
	// 	}
	// }

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Go方法中的TODO注释")
	return fmt.Errorf("Go方法未完成")
}

func (w *WebServer) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// w.Info("网络服务器清理", "serverName", w.ServerName)
	// fmt.Printf("网络服务器 %s 已停止并清理\n", w.ServerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// DatabaseService 数据库服务
type DatabaseService struct {
	task.Task
	ServiceName string
	Connected   bool
}

func (d *DatabaseService) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// d.Info("数据库服务启动", "serviceName", d.ServiceName)
	// fmt.Printf("数据库服务 %s 正在启动...\n", d.ServiceName)
	//
	// // 模拟数据库连接
	// time.Sleep(1 * time.Second)
	// d.Connected = true
	// fmt.Printf("数据库服务 %s 连接成功\n", d.ServiceName)
	//
	// // 设置重试策略
	// d.SetRetry(3, 2*time.Second)
	//
	// // 设置清理回调
	// d.OnStop(func() {
	// 	d.Connected = false
	// 	fmt.Printf("数据库服务 %s 连接已断开\n", d.ServiceName)
	// })

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
	// fmt.Printf("数据库服务 %s 已清理\n", d.ServiceName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// CacheService 缓存服务
type CacheService struct {
	task.TickTask
	ServiceName string
	CacheHit    int
}

func (c *CacheService) GetTickInterval() time.Duration {
	// TODO: 取消下面的注释来设置定时器间隔
	// return 1 * time.Second

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消GetTickInterval方法中的TODO注释")
	return 0
}

func (c *CacheService) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// c.Info("缓存服务启动", "serviceName", c.ServiceName)
	// fmt.Printf("缓存服务 %s 已启动\n", c.ServiceName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (c *CacheService) Tick(tick any) {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// c.CacheHit++
	// c.Info("缓存服务执行", "serviceName", c.ServiceName, "cacheHit", c.CacheHit)
	// fmt.Printf("缓存服务 %s 缓存命中: %d\n", c.ServiceName, c.CacheHit)
	//
	// // 执行10次后自动停止
	// if c.CacheHit >= 10 {
	// 	fmt.Printf("缓存服务 %s 执行完成，自动停止\n", c.ServiceName)
	// 	c.Stop(task.ErrTaskComplete)
	// }

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Tick方法中的TODO注释")
}

func (c *CacheService) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// c.Info("缓存服务清理", "serviceName", c.ServiceName, "totalCacheHit", c.CacheHit)
	// fmt.Printf("缓存服务 %s 已清理，总缓存命中: %d\n", c.ServiceName, c.CacheHit)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// ApplicationManager 应用程序管理器
type ApplicationManager struct {
	task.Job
	AppName string
}

func (a *ApplicationManager) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// a.Info("应用程序管理器启动", "appName", a.AppName)
	// fmt.Printf("应用程序管理器 %s 已启动\n", a.AppName)
	//
	// // 监听子任务事件
	// a.OnDescendantsStart(func(task task.ITask) {
	// 	fmt.Printf("应用程序管理器 %s 监听到服务启动: %s\n", a.AppName, task.GetOwnerType())
	// })
	//
	// a.OnDescendantsDispose(func(task task.ITask) {
	// 	fmt.Printf("应用程序管理器 %s 监听到服务停止: %s\n", a.AppName, task.GetOwnerType())
	// })

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (a *ApplicationManager) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// a.Info("应用程序管理器清理", "appName", a.AppName)
	// fmt.Printf("应用程序管理器 %s 已清理\n", a.AppName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// 使用RootManager作为根任务管理器
type TaskManager = task.RootManager[uint32, *ApplicationManager]

func main() {
	fmt.Println("=== GoTask Lesson 10: 综合应用案例 ===")
	fmt.Println("本课程将演示一个完整的Web应用程序管理场景")
	fmt.Println("请按照TODO注释的提示，逐步取消注释来完成代码")
	fmt.Println()

	// 创建根任务管理器
	root := &TaskManager{}
	root.Init()

	// 创建应用程序管理器
	appManager := &ApplicationManager{AppName: "Web应用管理器"}

	// 将应用程序管理器添加到根管理器中
	root.AddTask(appManager)

	// 等待应用程序管理器启动
	err := appManager.WaitStarted()
	if err != nil {
		fmt.Printf("❌ 应用程序管理器启动失败: %v\n", err)
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 创建网络服务器
	webServer := &WebServer{
		ServerName: "HTTP服务器",
		Port:       8080,
	}

	// 创建数据库服务
	dbService := &DatabaseService{
		ServiceName: "MySQL数据库",
	}

	// 创建缓存服务
	cacheService := &CacheService{
		ServiceName: "Redis缓存",
	}

	// TODO: 取消下面的注释来添加服务到应用程序管理器中
	// appManager.AddTask(webServer)
	// appManager.AddTask(dbService)
	// appManager.AddTask(cacheService)

	// 验证：检查是否添加了服务
	if webServer.GetTaskID() == 0 || dbService.GetTaskID() == 0 || cacheService.GetTaskID() == 0 {
		fmt.Println("❌ 错误：请取消AddTask的TODO注释")
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 等待缓存服务完成
	// cacheService.WaitStopped()

	// 停止所有服务
	// webServer.Stop(task.ErrTaskComplete)
	// dbService.Stop(task.ErrTaskComplete)
	// webServer.WaitStopped()
	// dbService.WaitStopped()

	// 停止应用程序管理器
	appManager.Stop(task.ErrTaskComplete)
	appManager.WaitStopped()

	fmt.Println("=== 课程完成 ===")
	fmt.Println("如果看到完整的应用程序运行日志，说明你已经成功完成了Lesson 10!")
	fmt.Println("恭喜你完成了所有GoTask课程！")

	// 优雅关闭
	root.Shutdown()
}
