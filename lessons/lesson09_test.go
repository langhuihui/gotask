package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// WorkerTask09 工作任务
type WorkerTask09 struct {
	task.Task
	WorkerID int
}

func (w *WorkerTask09) Start() error {
	w.Info("工作任务启动", "workerID", w.WorkerID)
	return nil
}

func (w *WorkerTask09) Run() error {
	w.Info("工作任务运行中", "workerID", w.WorkerID)

	// 模拟工作
	time.Sleep(2 * time.Second)
	w.Info("工作任务工作完成", "workerID", w.WorkerID)
	return nil
}

func (w *WorkerTask09) Dispose() {
	w.Info("工作任务清理", "workerID", w.WorkerID)
}

// EventManager 事件管理任务
type EventManager struct {
	task.Job
	ManagerName  string
	StartCount   int
	DisposeCount int
}

func (e *EventManager) Start() error {
	e.Info("事件管理任务启动", "managerName", e.ManagerName)

	// 监听子任务启动事件
	e.OnDescendantsStart(func(task task.ITask) {
		e.StartCount++
		e.Info("监听到子任务启动", "managerName", e.ManagerName, "taskType", task.GetOwnerType(), "total", e.StartCount)
	})

	// 监听子任务清理事件
	e.OnDescendantsDispose(func(task task.ITask) {
		e.DisposeCount++
		e.Info("监听到子任务清理", "managerName", e.ManagerName, "taskType", task.GetOwnerType(), "total", e.DisposeCount)
	})
	return nil
}

func (e *EventManager) Dispose() {
	e.Info("事件管理任务清理", "managerName", e.ManagerName,
		"startCount", e.StartCount, "disposeCount", e.DisposeCount)
}

// TestLesson09 测试事件监听与回调
func TestLesson09(t *testing.T) {
	t.Log("=== Lesson 9: 事件监听与回调 ===")
	t.Log("课程目标：学习如何使用事件监听和回调机制，了解任务间的协作方式")
	t.Log("核心概念：OnDescendantsStart/OnDescendantsDispose监听子任务事件，OnStart/OnDispose设置回调")
	t.Log("学习内容：事件驱动的任务协调、松耦合协作、任务状态监控")

	eventManagerStarted := false
	// 创建事件管理任务
	eventManager := &EventManager{ManagerName: "事件管理器"}
	eventManager.OnStart(func() {
		// TODO: 取消注释下面的代码来通过课程
		// eventManagerStarted = true
	})
	// 将事件管理任务添加到根管理器中
	root.AddTask(eventManager)

	// 等待事件管理任务启动
	eventManager.WaitStarted()
	if eventManagerStarted == false {
		t.Error("课程未通过")
		return
	}
	// 创建多个工作任务
	workers := make([]*WorkerTask09, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask09{WorkerID: i}
		eventManager.AddTask(workers[i-1])
	}

	// 验证：检查是否添加了工作任务
	if len(workers) > 0 && workers[0].GetTaskID() == 0 {
		t.Error("工作任务未正确添加")
		return
	}

	// 等待所有任务完成
	eventManager.WaitStopped()

	t.Log("Lesson 9 测试通过：事件监听与回调")
}
