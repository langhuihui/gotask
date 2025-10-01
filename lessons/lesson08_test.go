package lessons

import (
	"fmt"
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// UnstableTask Unstable task
type UnstableTask struct {
	task.Task
	TaskName  string
	hasFailed bool
}

func (u *UnstableTask) Start() error {
	u.Info("Unstable task started", "taskName", u.TaskName)
	return nil
}

func (u *UnstableTask) Run() error {
	u.Info("Unstable task running", "taskName", u.TaskName)

	// First attempt must fail, second attempt must succeed
	if !u.hasFailed {
		u.hasFailed = true
		u.Info("Unstable task attempt failed", "taskName", u.TaskName)
		return fmt.Errorf("Task execution failed, first attempt")
	}

	u.Info("Unstable task attempt succeeded", "taskName", u.TaskName)
	time.Sleep(5 * time.Second)
	return nil
}

func (u *UnstableTask) Dispose() {
	u.Info("Unstable task cleaned up", "taskName", u.TaskName)
}

// RetryManager Retry management task
type RetryManager struct {
	task.Job
	ManagerName string
}

// TestLesson08 Test retry mechanism
func TestLesson08(t *testing.T) {
	t.Log("=== Lesson 8: Retry Mechanism ===")
	t.Log("Course Objective: Learn how to use retry mechanism to handle failed tasks, understand error recovery strategies")
	t.Log("Core Concepts: SetRetry method sets retry strategy, supports limited retries and unlimited retries")
	t.Log("Learning Content: Retry interval settings, retry status monitoring, error recovery strategies")

	// Create retry management task
	retryManager := &RetryManager{ManagerName: "Retry Manager"}

	// Add retry management task to root manager
	root.AddTask(retryManager)

	// Create unstable task
	unstableTask := &UnstableTask{TaskName: "Unstable Task"}

	// TODO: Uncomment the code below to pass the course
	// unstableTask.SetRetry(2, 1*time.Second)

	retryManager.AddTask(unstableTask)

	time.Sleep(3 * time.Second)
	if unstableTask.GetState() != task.TASK_STATE_RUNNING {
		t.Fatal("Course not passed")
		return
	}
	t.Log("Lesson 8 test passed: Retry mechanism")
}
