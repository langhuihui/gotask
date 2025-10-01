package lessons

import (
	"fmt"
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// UnstableTask 不稳定任务
type UnstableTask struct {
	task.Task
	TaskName  string
	hasFailed bool
}

func (u *UnstableTask) Start() error {
	u.Info("不稳定任务启动", "taskName", u.TaskName)
	return nil
}

func (u *UnstableTask) Run() error {
	u.Info("不稳定任务运行中", "taskName", u.TaskName)

	// 第一次必定失败，第二次必定成功
	if !u.hasFailed {
		u.hasFailed = true
		u.Info("不稳定任务尝试失败", "taskName", u.TaskName)
		return fmt.Errorf("任务执行失败，第一次尝试")
	}

	u.Info("不稳定任务尝试成功", "taskName", u.TaskName)
	time.Sleep(5*time.Second)
	return nil
}

func (u *UnstableTask) Dispose() {
	u.Info("不稳定任务清理", "taskName", u.TaskName)
}

// RetryManager 重试管理任务
type RetryManager struct {
	task.Job
	ManagerName string
}

// TestLesson08 测试重试机制
func TestLesson08(t *testing.T) {
	t.Log("=== Lesson 8: 重试机制 ===")
	t.Log("课程目标：学习如何使用重试机制处理失败任务，了解错误恢复策略")
	t.Log("核心概念：SetRetry方法设置重试策略，支持有限重试和无限重试")
	t.Log("学习内容：重试间隔设置、重试状态监控、错误恢复策略")

	// 创建重试管理任务
	retryManager := &RetryManager{ManagerName: "重试管理器"}

	// 将重试管理任务添加到根管理器中
	root.AddTask(retryManager)
	
	// 创建不稳定任务
	unstableTask := &UnstableTask{TaskName: "不稳定任务"}

	// TODO: 取消注释下面的代码来通过课程
	// unstableTask.SetRetry(2, 1*time.Second)

	retryManager.AddTask(unstableTask)

	time.Sleep(3 * time.Second)
	if unstableTask.GetState() != task.TASK_STATE_RUNNING {
		t.Fatal("课程未通过")
		return
	}
	t.Log("Lesson 8 测试通过：重试机制")
}
