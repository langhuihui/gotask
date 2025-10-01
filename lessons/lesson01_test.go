package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
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

// MyFirstTask3 Third task example
type MyFirstTask3 struct {
	MyFirstTask
}

// Go Task run method, using Go method for asynchronous execution, TODO: Uncomment to complete task execution
// func (t *MyFirstTask3) Go() error {
// 	t.Info("Task is running in goroutine", "name", t.Name())
// 	time.Sleep(2 * time.Second)
// 	return nil
// }

// TestLesson01_3 Go method usage
func TestLesson01_3(t *testing.T) {
	t.Log("=== Lesson 1-3: Go Method Usage ===")
	t.Log("Course Objective: Learn Go method usage in GoTask framework, understand difference between Go and Run methods")
	t.Log("Key Concept: Go method automatically enters GOING state, runs in independent goroutine")

	// Create third task
	myTask3 := &MyFirstTask3{MyFirstTask: MyFirstTask{T: t}}
	root.AddTask(myTask3)

	time.Sleep(1 * time.Second) // Wait for task startup

	if myTask3.GetState() == task.TASK_STATE_GOING {
		t.Log("Lesson 1-3 test passed: Go Method Usage")
		return
	}
	t.Errorf("Course not passed")
}
