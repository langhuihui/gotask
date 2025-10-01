package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// WorkerTask03 工作任务（会自动完成）
type WorkerTask03 struct {
	task.Task
	WorkerID int
}

func (t *WorkerTask03) Start() error {
	t.Info("工作线程启动", "workerID", t.WorkerID)
	return nil
}

func (t *WorkerTask03) Run() error {
	t.Info("工作线程运行", "workerID", t.WorkerID)
	return nil
}

// ServerWork 服务器工作任务
type ServerWork struct {
	task.Work
	ServerName string
}

// TestLesson03 测试Work长期运行任务
func TestLesson03(t *testing.T) {
	t.Log("=== Lesson 3: Work长期运行任务 ===")
	t.Log("课程目标：学习Work和Job的区别，理解长期运行任务容器的特性")
	t.Log("核心概念：Work的keepalive返回true，即使所有子任务完成，Work也不会自动停止")
	t.Log("对比Job：Job在所有子任务完成后会自动停止，而Work会持续运行")
	t.Log("学习内容：Work容器管理、keepalive机制、手动停止Work任务")

	// 创建服务器工作任务
	server := &ServerWork{ServerName: "Web服务器"}

	// 将服务器任务添加到根管理器中
	root.AddTask(server)

	// 创建多个工作线程（与Lesson02_1相同，都有Run方法会自动完成）
	workers := make([]*WorkerTask03, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask03{WorkerID: i}
		server.AddTask(workers[i-1])
	}

	// 等待所有子任务完成
	for _, worker := range workers {
		worker.WaitStopped()
	}
	t.Log("所有子任务已完成")

	// 等待一段时间，观察Work的状态
	time.Sleep(1 * time.Second)

	// 需要手动停止Work（这是Work和Job的关键区别）
	// TODO: 取消注释来完成手动停止
	// server.Stop(task.ErrStopByUser)
	// server.WaitStopped()

	if server.GetState() == task.TASK_STATE_DISPOSED {
		t.Log("✓ Lesson 3 测试通过：Work长期运行任务")
	} else {
		t.Errorf("课程未通过")
	}
}
