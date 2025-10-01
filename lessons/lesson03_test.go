package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// WorkerTask03 Work task (automatically completes)
type WorkerTask03 struct {
	task.Task
	WorkerID int
}

func (t *WorkerTask03) Start() error {
	t.Info("Worker thread started", "workerID", t.WorkerID)
	return nil
}

func (t *WorkerTask03) Run() error {
	t.Info("Worker thread running", "workerID", t.WorkerID)
	return nil
}

// ServerWork Server work task
type ServerWork struct {
	task.Work
	ServerName string
}

// TestLesson03 Test Work long-running tasks
func TestLesson03(t *testing.T) {
	t.Log("=== Lesson 3: Work Long-running Tasks ===")
	t.Log("Course Objective: Learn difference between Work and Job, understand long-running task container characteristics")
	t.Log("Core Concept: Work's keepalive returns true, even if all subtasks complete, Work won't automatically stop")
	t.Log("Comparison with Job: Job automatically stops when all subtasks complete, while Work continues running")
	t.Log("Learning Content: Work container management, keepalive mechanism, manual Work task stopping")

	// Create server work task
	server := &ServerWork{ServerName: "Web Server"}

	// Add server task to root manager
	root.AddTask(server)

	// Create multiple worker threads (same as Lesson02_1, all have Run method and automatically complete)
	workers := make([]*WorkerTask03, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask03{WorkerID: i}
		server.AddTask(workers[i-1])
	}

	// Wait for all subtasks to complete
	for _, worker := range workers {
		worker.WaitStopped()
	}
	t.Log("All subtasks completed")

	// Wait for a while, observe Work's state
	time.Sleep(1 * time.Second)

	// Need to manually stop Work (this is key difference between Work and Job)
	// TODO: Uncomment to complete manual stop
	// server.Stop(task.ErrStopByUser)
	// server.WaitStopped()

	if server.GetState() == task.TASK_STATE_DISPOSED {
		t.Log("✓ Lesson 3 test passed: Work Long-running Tasks")
	} else {
		t.Errorf("Course not passed")
	}
}
