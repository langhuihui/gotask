//go:build lesson

package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask/v2"
)

// Use RootManager as root task manager
type TaskManager = task.RootManager[uint32, task.ManagerItem[uint32]]

// Create root task manager
var root TaskManager

func init() {
	root.Init()
}

// MyFirstTask First task example
type MyFirstTask struct {
	task.Task
	*testing.T
}

// Start Task start method
func (t *MyFirstTask) Start() error {
	t.Log("Task started executing", t.Name())
	return nil
}

// Dispose Task cleanup method
func (t *MyFirstTask) Dispose() {
	t.Log("Task cleanup", t.Name())
}

// TestLesson01 Test basic Task usage
func TestLesson01_1(t *testing.T) {
	t.Log("=== Lesson 1-1: Basic Task Usage ===")
	t.Skip("exercise: complete the TODO below (then delete this line) to check your work")
	t.Log("Course Objective: Learn the most basic Task usage methods in GoTask framework")
	t.Log("Key Concept: Tasks need to use AddTask to run (parent task drives child task execution)")

	// Create first task
	myTask := &MyFirstTask{T: t}

	//TODO: Uncomment below to complete task addition
	// root.AddTask(myTask)

	time.Sleep(1 * time.Second) // Wait for task startup

	if myTask.GetState() == task.TASK_STATE_STARTED {
		t.Log("Lesson 1-1 test passed: Basic Task Usage")
		return
	}
	t.Errorf("Course not passed")
}

// MyFirstTask2 Second task example, will inherit MyFirstTask's Start and Dispose methods
type MyFirstTask2 struct {
	MyFirstTask
}

// Run Task run method, TODO: Uncomment to complete task execution
// func (t *MyFirstTask2) Run() error {
// 	t.Info("Task is running", "name", t.Name())
// 	time.Sleep(2 * time.Second)
// 	return nil
// }

// TestLesson01_2 Run method usage
func TestLesson01_2(t *testing.T) {
	t.Log("=== Lesson 1-2: Run Method Usage ===")
	t.Skip("exercise: complete the TODO below (then delete this line) to check your work")
	t.Log("Course Objective: Learn inherited Task usage and Run method usage in GoTask framework")
	t.Log("Key Concept: Run method automatically enters RUNNING state")

	// Create second task
	myTask2 := &MyFirstTask2{MyFirstTask: MyFirstTask{T: t}}
	root.AddTask(myTask2)

	time.Sleep(1 * time.Second) // Wait for task startup

	if myTask2.GetState() == task.TASK_STATE_RUNNING {
		t.Log("Lesson 1-2 test passed: Run Method Usage")
		return
	}
	t.Errorf("Course not passed")
}

// MyFirstTask3 Third task example.
//
// v2 起 Go() 不再是任务的执行入口：TaskGo 已删除，Run 是唯一入口（见
// docs/重构-执行模型v2-大版本v2设计.md §4.1）。把它写成 Go() 不会编译报错，
// 但框架会以 ErrLegacyTaskGo 明确失败——而不是让任务「启动成功却永远不执行」。
type MyFirstTask3 struct {
	MyFirstTask
}

// Go 故意沿用 v1 的方法名，用来观察 v2 的迁移守卫。
func (t *MyFirstTask3) Go() error {
	t.Info("This must NOT run in v2", "name", t.Name())
	return nil
}

// TestLesson01_3 v2 migration guard: Go() is no longer an entry point
func TestLesson01_3(t *testing.T) {
	t.Log("=== Lesson 1-3: Go() Is No Longer a Task Entry Point (v2) ===")
	t.Skip("exercise: delete the Skip line above to check your work")
	t.Log("Course Objective: Understand why v2 removed TaskGo and how the migration guard protects you")
	t.Log("Key Concept: A task that only implements Go() stops with ErrLegacyTaskGo instead of silently idling")

	// Create third task
	myTask3 := &MyFirstTask3{MyFirstTask: MyFirstTask{T: t}}
	root.AddTask(myTask3)

	time.Sleep(1 * time.Second) // Wait for the guard to trip

	if myTask3.StopReasonIs(task.ErrLegacyTaskGo) {
		t.Log("Lesson 1-3 test passed: the migration guard caught the legacy Go() task")
		return
	}
	t.Errorf("Course not passed: expected ErrLegacyTaskGo, got %v", myTask3.StopReason())
}
