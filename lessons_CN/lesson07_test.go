package lessons

import (
	"sync"
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// FileHandle 文件句柄资源
type FileHandle struct {
	FileName string
	Opened   bool
}

func (fh *FileHandle) Close() error {
	fh.Opened = false
	return nil
}

// LogService 日志服务
type LogService struct {
	task.Task
	ServiceName string
}


type DataService07_1 struct {
	task.Task
	ServiceName string
}

func (s *DataService07_1) Start() error {
	s.Info("数据服务启动", "serviceName", s.ServiceName)

	// Using 示例：文件资源管理
	configFile := &FileHandle{FileName: "config.json", Opened: true}
	s.Using(configFile)
	s.Info("读取配置文件", "fileName", configFile.FileName)

	return nil
}

func (s *DataService07_1) Run() error {
	s.Info("数据服务运行中")
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (s *DataService07_1) Dispose() {
	s.Info("Dispose：Using已自动清理所有资源")
}

// TestLesson07_1 测试Using方法 - 文件资源和关联关闭
func TestLesson07_1(t *testing.T) {
	t.Log("=== Lesson 7-1: Using方法 - 文件资源和关联关闭 ===")
	t.Log("课程目标：学习Using方法的两种主要用法")
	t.Log("")
	t.Log("📝 Using 两种用法：")
	t.Log("  1. 文件资源管理：配置文件、日志文件等")
	t.Log("  2. 关联任务关闭：一个任务停止时，关联任务也自动停止")
	t.Log("")
	t.Log("📝 优势：")
	t.Log("  - 简化资源清理：无需手动管理文件关闭")
	t.Log("  - 自动关联关闭：避免忘记关闭关联服务")

	service := &DataService07_1{ServiceName: "数据服务"}
	root.AddTask(service)
	

		// Using 示例：关联任务关闭
	logService := &LogService{ServiceName: service.ServiceName + "-logger"}
	root.AddTask(logService)
	// TODO: 取消注释以完成课程
	// service.Using(logService) // 关联关闭：当数据服务停止时，日志服务也自动停止
	service.Stop(task.ErrTaskComplete)
	time.Sleep(time.Second)
	if logService.GetState() != task.TASK_STATE_DISPOSED {
		t.Fatal("课程未通过")
		return
	}
	t.Log("\n✓ Lesson 7-1 测试通过：Using方法文件资源和关联关闭")
}

type NetworkService07_2_OnStop struct {
	task.Task
	ServiceName string
	wg sync.WaitGroup
}

func (n *NetworkService07_2_OnStop) Start() error {
	n.wg.Add(1)
	// TODO: 取消注释以完成课程
	// n.OnStop(n.wg.Done)
	return nil
}

func (n *NetworkService07_2_OnStop) Run() error {
	n.Info("网络服务运行中")
	n.wg.Wait() // 等待阻塞资源释放,模拟阻塞资源
	return nil
}

func (n *NetworkService07_2_OnStop) Dispose() {
	n.Info("Dispose：OnStop已立即释放阻塞资源")
}

// TestLesson07_2 测试OnStop方法
func TestLesson07_2(t *testing.T) {
	t.Log("=== Lesson 7-2: OnStop方法 ===")
	t.Log("课程目标：学习OnStop方法的使用")
	t.Log("")
	t.Log("📝 OnStop 使用场景：")
	t.Log("  - 处理阻塞性资源（网络连接、端口监听）")
	t.Log("  - 任务停止时立即释放资源")
	t.Log("")
	t.Log("📝 实际场景：")
	t.Log("  服务器服务：OnStop处理网络连接和端口监听")


	service2 := &NetworkService07_2_OnStop{ServiceName: "服务器服务"}
	root.AddTask(service2)
	service2.WaitStarted()
	service2.Stop(task.ErrTaskComplete)
	time.Sleep(time.Second)
	if service2.GetState() != task.TASK_STATE_DISPOSED {
		t.Fatal("课程未通过")
		return
	}
	t.Log("\n✓ Lesson 7-2 测试通过：OnStop方法")
}
