package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// TimerTask Timer Task
type TimerTask struct {
	task.TickTask
	TaskName string
	Counter  int
}

func (t *TimerTask) GetTickInterval() time.Duration {
	return 1 * time.Second
}

func (t *TimerTask) Start() (err error) {
	t.Info("Timer task started", "taskName", t.TaskName)
	// TODO: Uncomment the code below to correctly execute the timer task
	// err = t.TickTask.Start()
	return
}

func (t *TimerTask) Tick(tick any) {
	t.Counter++

	// Automatically stop after 5 executions
	if t.Counter >= 3 {
		t.Info("Timer task completed, automatically stopping", "taskName", t.TaskName)
		t.Stop(task.ErrTaskComplete)
	}
}

// TestLesson05 Test TickTask Timer Task
func TestLesson05(t *testing.T) {
	t.Log("=== Lesson 5: TickTask Timer Task ===")
	t.Log("Learning Objective: Understand TickTask's timing execution mechanism")
	t.Log("Task: Uncomment the relevant code in TimerTask.Start() to implement timer task logic")

	// Create timer task
	timerTask := &TimerTask{TaskName: "Counter Task"}

	root.AddTask(timerTask)

	time.Sleep(4 * time.Second)

	// Verification: Check if the expected number of executions occurred
	if timerTask.Counter != 3 {
		t.Fatalf("Expected 5 executions, but executed %d times", timerTask.Counter)
	}

	t.Logf("Success! Timer task executed %d times", timerTask.Counter)
	t.Log("Lesson 3 completed! Timer task working correctly")
}
