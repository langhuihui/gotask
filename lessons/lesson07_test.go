package lessons

import (
	"sync"
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// FileHandle File handle resource
type FileHandle struct {
	FileName string
	Opened   bool
}

func (fh *FileHandle) Close() error {
	fh.Opened = false
	return nil
}

// LogService Log service
type LogService struct {
	task.Task
	ServiceName string
}

type DataService07_1 struct {
	task.Task
	ServiceName string
}

func (s *DataService07_1) Start() error {
	s.Info("Data service started", "serviceName", s.ServiceName)

	// Using example: file resource management
	configFile := &FileHandle{FileName: "config.json", Opened: true}
	s.Using(configFile)
	s.Info("Reading configuration file", "fileName", configFile.FileName)

	return nil
}

func (s *DataService07_1) Run() error {
	s.Info("Data service running")
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (s *DataService07_1) Dispose() {
	s.Info("Dispose: Using has automatically cleaned up all resources")
}

// TestLesson07_1 Test Using method - file resources and associated cleanup
func TestLesson07_1(t *testing.T) {
	t.Log("=== Lesson 7-1: Using Method - File Resources and Associated Cleanup ===")
	t.Log("Course Objective: Learn the two main uses of the Using method")
	t.Log("")
	t.Log("📝 Using Two Uses:")
	t.Log("  1. File resource management: configuration files, log files, etc.")
	t.Log("  2. Associated task cleanup: when one task stops, associated tasks also automatically stop")
	t.Log("")
	t.Log("📝 Advantages:")
	t.Log("  - Simplified resource cleanup: no need to manually manage file closing")
	t.Log("  - Automatic associated cleanup: avoid forgetting to close associated services")

	service := &DataService07_1{ServiceName: "Data Service"}
	root.AddTask(service)

	// Using example: associated task cleanup
	logService := &LogService{ServiceName: service.ServiceName + "-logger"}
	root.AddTask(logService)
	// TODO: Uncomment to complete the course
	// service.Using(logService) // Associated cleanup: when data service stops, log service also automatically stops
	service.Stop(task.ErrTaskComplete)
	time.Sleep(time.Second)
	if logService.GetState() != task.TASK_STATE_DISPOSED {
		t.Fatal("Course not passed")
		return
	}
	t.Log("\n✓ Lesson 7-1 test passed: Using method file resources and associated cleanup")
}

type NetworkService07_2_OnStop struct {
	task.Task
	ServiceName string
	wg          sync.WaitGroup
}

func (n *NetworkService07_2_OnStop) Start() error {
	n.wg.Add(1)
	// TODO: Uncomment to complete the course
	// n.OnStop(n.wg.Done)
	return nil
}

func (n *NetworkService07_2_OnStop) Run() error {
	n.Info("Network service running")
	n.wg.Wait() // Wait for blocking resource release, simulate blocking resource
	return nil
}

func (n *NetworkService07_2_OnStop) Dispose() {
	n.Info("Dispose: OnStop has immediately released blocking resources")
}

// TestLesson07_2 Test OnStop method
func TestLesson07_2(t *testing.T) {
	t.Log("=== Lesson 7-2: OnStop Method ===")
	t.Log("Course Objective: Learn how to use the OnStop method")
	t.Log("")
	t.Log("📝 OnStop Usage Scenarios:")
	t.Log("  - Handle blocking resources (network connections, port listening)")
	t.Log("  - Immediately release resources when task stops")
	t.Log("")
	t.Log("📝 Practical Scenarios:")
	t.Log("  Server service: OnStop handles network connections and port listening")

	service2 := &NetworkService07_2_OnStop{ServiceName: "Server Service"}
	root.AddTask(service2)
	service2.WaitStarted()
	service2.Stop(task.ErrTaskComplete)
	time.Sleep(time.Second)
	if service2.GetState() != task.TASK_STATE_DISPOSED {
		t.Fatal("Course not passed")
		return
	}
	t.Log("\n✓ Lesson 7-2 test passed: OnStop method")
}
