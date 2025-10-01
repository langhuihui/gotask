package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// WorkerTask09 Work task
type WorkerTask09 struct {
	task.Task
	WorkerID int
}

func (w *WorkerTask09) Start() error {
	w.Info("Work task started", "workerID", w.WorkerID)
	return nil
}

func (w *WorkerTask09) Run() error {
	w.Info("Work task running", "workerID", w.WorkerID)

	// Simulate work
	time.Sleep(2 * time.Second)
	w.Info("Work task completed", "workerID", w.WorkerID)
	return nil
}

func (w *WorkerTask09) Dispose() {
	w.Info("Work task cleaned up", "workerID", w.WorkerID)
}

// EventManager Event management task
type EventManager struct {
	task.Job
	ManagerName  string
	StartCount   int
	DisposeCount int
}

func (e *EventManager) Start() error {
	e.Info("Event management task started", "managerName", e.ManagerName)

	// Listen for child task start events
	e.OnDescendantsStart(func(task task.ITask) {
		e.StartCount++
		e.Info("Child task start event detected", "managerName", e.ManagerName, "taskType", task.GetOwnerType(), "total", e.StartCount)
	})

	// Listen for child task cleanup events
	e.OnDescendantsDispose(func(task task.ITask) {
		e.DisposeCount++
		e.Info("Child task cleanup event detected", "managerName", e.ManagerName, "taskType", task.GetOwnerType(), "total", e.DisposeCount)
	})
	return nil
}

func (e *EventManager) Dispose() {
	e.Info("Event management task cleaned up", "managerName", e.ManagerName,
		"startCount", e.StartCount, "disposeCount", e.DisposeCount)
}

// TestLesson09 Test event listening and callbacks
func TestLesson09(t *testing.T) {
	t.Log("=== Lesson 9: Event Listening and Callbacks ===")
	t.Log("Course Objective: Learn how to use event listening and callback mechanisms, understand task collaboration methods")
	t.Log("Core Concepts: OnDescendantsStart/OnDescendantsDispose listen for child task events, OnStart/OnDispose set callbacks")
	t.Log("Learning Content: Event-driven task coordination, loose coupling collaboration, task status monitoring")

	eventManagerStarted := false
	// Create event management task
	eventManager := &EventManager{ManagerName: "Event Manager"}
	eventManager.OnStart(func() {
		// TODO: Uncomment the code below to pass the course
		// eventManagerStarted = true
	})
	// Add event management task to root manager
	root.AddTask(eventManager)

	// Wait for event management task to start
	eventManager.WaitStarted()
	if eventManagerStarted == false {
		t.Error("Course not passed")
		return
	}
	// Create multiple work tasks
	workers := make([]*WorkerTask09, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask09{WorkerID: i}
		eventManager.AddTask(workers[i-1])
	}

	// Verification: Check if work tasks were added correctly
	if len(workers) > 0 && workers[0].GetTaskID() == 0 {
		t.Error("Work tasks not added correctly")
		return
	}

	// Wait for all tasks to complete
	eventManager.WaitStopped()

	t.Log("Lesson 9 test passed: Event listening and callbacks")
}
