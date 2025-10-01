package lessons

// To avoid type redeclaration, use different type names here

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

type WorkerTask02_1 struct {
	task.Task
	WorkerID int
}

func (t *WorkerTask02_1) Start() error {
	t.Info("Worker thread started", "workerID", t.WorkerID)
	return nil
}

func (t *WorkerTask02_1) Run() error {
	t.Info("Worker thread running", "workerID", t.WorkerID)
	return nil
}

// ManagerJob Management task container
type ManagerJob struct {
	task.Job
	JobName string
}

// TestLesson02 Test Job container management
func TestLesson02_1(t *testing.T) {
	t.Log("=== Lesson 2-1: Job Container Management ===")
	t.Log("Course Objective: Learn how to use Job to manage multiple subtasks, understand task hierarchy structure")
	t.Log("Core Concept: Job container can contain multiple subtasks, managing parent-child task lifecycle relationships")
	t.Log("Important Feature: When all subtasks complete, Job automatically stops and enters Disposed state")
	t.Log("Learning Content: AddTask for adding subtasks, task hierarchy management, WaitStarted/WaitStopped methods")

	// Create management task
	manager := &ManagerJob{JobName: "Work Manager"}

	// Add management task to root manager (important)
	root.AddTask(manager)

	// Create multiple worker threads
	workers := make([]*WorkerTask02_1, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask02_1{WorkerID: i}
		manager.AddTask(workers[i-1])
	}

	// Wait for all tasks to complete (Job automatically stops after subtasks complete), TODO: Uncomment to complete task management
	// manager.WaitStopped()

	if manager.GetState() == task.TASK_STATE_DISPOSED {
		t.Log("Lesson 2-1 test passed: Job Container Management")
		return
	}
	t.Errorf("Course not passed")
}

type WorkerTask02_2 struct {
	task.Task
	WorkerID int
}

func (t *WorkerTask02_2) Start() error {
	t.Info("Worker thread started", "workerID", t.WorkerID)
	return nil
}

// TODO: Uncomment to complete task execution
// func (t *WorkerTask02_2) Run() error {
// 	t.Info("Worker thread running", "workerID", t.WorkerID)
// 	return nil
// }

func TestLesson02_2(t *testing.T) {
	t.Log("=== Lesson 2-2: Task Lifecycle - Tasks Without Run Method ===")
	t.Log("Course Objective: Understand the impact of Run method on task lifecycle")
	t.Log("Core Concept: Tasks without Run method will remain running after Start, won't automatically end")
	t.Log("Learning Content: Task state management, long-running task characteristics, Job container stop conditions")

	// Create management task
	manager := &ManagerJob{JobName: "Work Manager"}

	// Add management task to root manager (important)
	root.AddTask(manager)

	// Create multiple worker threads
	workers := make([]*WorkerTask02_2, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask02_2{WorkerID: i}
		manager.AddTask(workers[i-1])
	}
	time.AfterFunc(1*time.Second, func() {
		if manager.GetState() == task.TASK_STATE_DISPOSED {
			t.Log("Lesson 2-2 test passed: Job Container Management")
			return
		}
		t.Errorf("Course not passed")
	})
	manager.WaitStopped()
}

type WorkerTask02_3 struct {
	task.Task
	WorkerID int
}

func (t *WorkerTask02_3) Start() error {
	t.Info("Worker thread started", "workerID", t.WorkerID)
	return nil
}

// TestLesson02_3 Test Job's Stop method causes all subtasks to be Stopped
func TestLesson02_3(t *testing.T) {
	t.Log("=== Lesson 2-3: Job Stop Propagation Mechanism ===")
	t.Log("Course Objective: Understand the impact of Job's Stop method on subtasks")
	t.Log("Core Concept: Calling Job's Stop method causes all subtasks to be Stopped")
	t.Log("Learning Content: Job stop propagation, subtask lifecycle management")

	// Create management task
	manager := &ManagerJob{JobName: "Work Manager"}

	// Add management task to root manager (important)
	root.AddTask(manager)

	// Create multiple worker threads (no Run method, will keep running)
	workers := make([]*WorkerTask02_3, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask02_3{WorkerID: i}
		manager.AddTask(workers[i-1])
		workers[i-1].WaitStarted()
	}

	// Actively stop management task, TODO: Uncomment to complete task stop
	t.Log("Actively stopping Job task...")
	// manager.Stop(task.ErrStopByUser)

	time.Sleep(1 * time.Second)

	// Verify all subtasks have been stopped
	allStopped := true
	for _, worker := range workers {
		if worker.GetState() != task.TASK_STATE_DISPOSED {
			t.Errorf("Worker thread %d not stopped, state: %d", worker.WorkerID, worker.GetState())
			allStopped = false
		}
	}

	// Verify management task itself has also stopped
	if manager.GetState() != task.TASK_STATE_DISPOSED {
		t.Errorf("Management task not stopped, state: %d", manager.GetState())
		allStopped = false
	}

	if allStopped {
		t.Log("✓ Lesson 2-3 test passed: Job's Stop method successfully stopped all subtasks")
	} else {
		t.Errorf("✗ Lesson 2-3 test failed: Some tasks not correctly stopped")
	}
}

type WorkerTask02_4 struct {
	task.Task
	WorkerID  int
	StartTime time.Time
}

func (t *WorkerTask02_4) Start() error {
	t.StartTime = time.Now()
	t.Info("Worker thread started", "workerID", t.WorkerID, "time", t.StartTime)
	return nil
}

// TODO: Second approach, use Go method instead of Run for asynchronous execution
func (t *WorkerTask02_4) Run() error {
	// First task blocks for 2 seconds, other tasks complete quickly
	if t.WorkerID == 1 {
		t.Info("Worker thread 1 starting blocking run", "workerID", t.WorkerID)
		time.Sleep(2 * time.Second) // TODO: Try commenting out this line, observe task startup time changes
		t.Info("Worker thread 1 completed running", "workerID", t.WorkerID)
	} else {
		t.Info("Worker thread running", "workerID", t.WorkerID)
	}
	return nil
}

// TestLesson02_4 Test subtask's Run will block other subtasks' execution
func TestLesson02_4(t *testing.T) {
	t.Log("=== Lesson 2-4: Subtask Run Blocking Characteristics ===")
	t.Log("Course Objective: Understand that subtask's Run method blocks event loop, affecting other subtasks' startup")
	t.Log("Core Concept: Job's event loop is single-threaded, subtask's Start and Run methods execute synchronously")
	t.Log("")
	t.Log("📝 Experiment Steps:")
	t.Log("   1. Run test, observe worker thread startup times")
	t.Log("   2. Comment out line 183's time.Sleep, run again")
	t.Log("   3. Compare time differences between two runs, understand Run method's blocking characteristics")

	// Create management task
	manager := &ManagerJob{JobName: "Work Manager"}
	root.AddTask(manager)

	// Create multiple worker threads
	workers := make([]*WorkerTask02_4, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask02_4{WorkerID: i}
		manager.AddTask(workers[i-1])
	}

	// Use Timer to check third subtask's state
	t.Log("")
	t.Log("🔍 Using Timer to check third subtask's state:")

	time.Sleep(1 * time.Second)
	// Check third task's state after 1 second
	worker3State := workers[2].GetState()
	t.Logf("  Worker thread 3 state after 1 second: %d", worker3State)

	if worker3State < task.TASK_STATE_STARTED {
		t.Log("  ✓ Verification passed: Worker thread 3 not started after 1 second")
		t.Log("    Explanation: Worker thread 1's Run method indeed blocked event loop")
		t.Log("    Conclusion: Run method executes synchronously, blocks subsequent tasks")
		t.Log("Course not passed")
	} else {
		t.Log("✓ Lesson 2-4 test passed: Job's Run method blocks subsequent tasks")
	}
}
