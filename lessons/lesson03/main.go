package main

import (
	"fmt"
	"time"

	task "github.com/langhuihui/gotask"
)

// ServiceTask 服务任务
type ServiceTask struct {
	task.Task
	ServiceName string
	Port        int
}

func (t *ServiceTask) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// t.Info("服务启动", "serviceName", t.ServiceName, "port", t.Port)
	// fmt.Printf("服务 %s 在端口 %d 上启动\n", t.ServiceName, t.Port)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (t *ServiceTask) Go() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// t.Info("服务运行中", "serviceName", t.ServiceName)
	// fmt.Printf("服务 %s 正在运行...\n", t.ServiceName)
	//
	// // 模拟服务运行
	// ticker := time.NewTicker(2 * time.Second)
	// defer ticker.Stop()
	//
	// for {
	// 	select {
	// 	case <-ticker.C:
	// 		fmt.Printf("服务 %s 心跳检测正常\n", t.ServiceName)
	// 	case <-t.Done():
	// 		fmt.Printf("服务 %s 收到停止信号\n", t.ServiceName)
	// 		return nil
	// 	}
	// }

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Go方法中的TODO注释")
	return fmt.Errorf("Go方法未完成")
}

func (t *ServiceTask) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// t.Info("服务清理", "serviceName", t.ServiceName)
	// fmt.Printf("服务 %s 已停止并清理\n", t.ServiceName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// ServerWork 服务器工作任务
type ServerWork struct {
	task.Work
	ServerName string
}

func (w *ServerWork) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// w.Info("服务器启动", "serverName", w.ServerName)
	// fmt.Printf("服务器 %s 已启动\n", w.ServerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (w *ServerWork) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// w.Info("服务器清理", "serverName", w.ServerName)
	// fmt.Printf("服务器 %s 已停止\n", w.ServerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// 使用RootManager作为根任务管理器
type TaskManager = task.RootManager[uint32, *ServerWork]

func main() {
	fmt.Println("=== GoTask Lesson 3: Work长期运行任务 ===")
	fmt.Println("本课程将教你如何使用Work来管理长期运行的任务")
	fmt.Println("请按照TODO注释的提示，逐步取消注释来完成代码")
	fmt.Println()

	// 创建根任务管理器
	root := &TaskManager{}
	root.Init()

	// 创建服务器工作任务
	server := &ServerWork{ServerName: "Web服务器"}

	// 将服务器任务添加到根管理器中
	root.AddTask(server)

	// 等待服务器启动
	err := server.WaitStarted()
	if err != nil {
		fmt.Printf("❌ 服务器启动失败: %v\n", err)
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 创建多个服务
	services := []struct {
		name string
		port int
	}{
		{"HTTP服务", 8080},
		{"HTTPS服务", 8443},
		{"WebSocket服务", 9000},
	}

	serviceTasks := make([]*ServiceTask, len(services))
	for i, svc := range services {
		serviceTasks[i] = &ServiceTask{
			ServiceName: svc.name,
			Port:        svc.port,
		}
		// TODO: 取消下面的注释来添加服务到服务器中
		// server.AddTask(serviceTasks[i])
	}

	// 验证：检查是否添加了服务
	if len(serviceTasks) > 0 && serviceTasks[0].GetTaskID() == 0 {
		fmt.Println("❌ 错误：请取消AddTask的TODO注释")
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 让服务运行一段时间
	time.Sleep(5 * time.Second)

	// 停止服务器
	server.Stop(task.ErrTaskComplete)
	server.WaitStopped()

	fmt.Println("=== 课程完成 ===")
	fmt.Println("如果看到所有服务的运行日志，说明你已经成功完成了Lesson 3!")

	// 优雅关闭
	root.Shutdown()
}
