//go:build lesson

package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask/v2"
)

// TestTaskFactory Test task factory (based on monibuca pattern)
type TestTaskFactory struct {
	tasks map[string]func(*TestScenario, TestTaskConfig) task.ITask
}

func (f *TestTaskFactory) Register(action string, taskCreator func(*TestScenario, TestTaskConfig) task.ITask) {
	f.tasks[action] = taskCreator
}

func (f *TestTaskFactory) Create(taskConfig TestTaskConfig, scenario *TestScenario) (task.ITask, error) {
	if taskCreator, exists := f.tasks[taskConfig.Action]; exists {
		return taskCreator(scenario, taskConfig), nil
	}
	return nil, nil
}

var testTaskFactory = TestTaskFactory{
	tasks: make(map[string]func(*TestScenario, TestTaskConfig) task.ITask),
}

// TestTaskConfig Test task configuration (based on monibuca pattern)
type TestTaskConfig struct {
	Action     string        `json:"action"`
	Delay      time.Duration `json:"delay"`
	ServerAddr string        `json:"serverAddr" default:"localhost"`
	StreamPath string        `json:"streamPath"`
}

// TestScenario Test scenario (based on monibuca's TestCase pattern)
type TestScenario struct {
	task.Job `json:"-"`
	Name     string           `json:"name"`
	Tasks    []TestTaskConfig `json:"tasks"`
}

func (ts *TestScenario) Start() error {
	ts.Info("Test scenario started", "name", ts.Name)

	// Create and execute all tasks
	for _, taskConfig := range ts.Tasks {
		t, err := testTaskFactory.Create(taskConfig, ts)
		if err != nil {
			ts.Info("Task creation failed", "action", taskConfig.Action, "error", err)
			return err
		}
		ts.AddDependTask(t)
	}
	return nil
}

// WebServerTask Web server task (based on monibuca's TestBaseTask pattern)
type WebServerTask struct {
	task.Task
	scenario *TestScenario
	TestTaskConfig
	ServerName string
	Port       int
}

func (w *WebServerTask) Start() error {
	w.Info("Web server started", "serverName", w.ServerName, "port", w.Port)
	return nil
}

func (w *WebServerTask) Go() error {
	w.Info("Web server running", "serverName", w.ServerName)

	// Simulate server running
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Automatically stop after 5 runs
	count := 0
	for count < 5 {
		select {
		case <-ticker.C:
			w.Info("Web server status check normal", "serverName", w.ServerName)
			count++
		case <-w.Done():
			w.Info("Web server received stop signal", "serverName", w.ServerName)
			return nil
		}
	}

	w.Info("Web server run completed", "serverName", w.ServerName)
	w.Stop(task.ErrTaskComplete)
	return nil
}

func (w *WebServerTask) Dispose() {
	w.Info("Web server cleaned up", "serverName", w.ServerName)
}

// DatabaseTask Database task (based on monibuca's TestBaseTask pattern)
type DatabaseTask struct {
	task.Task
	scenario *TestScenario
	TestTaskConfig
	ServiceName string
	Connected   bool
}

func (d *DatabaseTask) Start() error {
	d.Info("Database service started", "serviceName", d.ServiceName)

	// Simulate database connection
	time.Sleep(500 * time.Millisecond)
	d.Connected = true
	d.Info("Database service connected successfully", "serviceName", d.ServiceName)

	d.SetRetry(2, time.Second)
	return nil
}

func (d *DatabaseTask) Go() error {
	d.Info("Database service running", "serviceName", d.ServiceName)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Automatically stop after 10 runs
	count := 0
	for count < 10 {
		select {
		case <-ticker.C:
			d.Info("Database service connection pool status normal", "serviceName", d.ServiceName)
			count++
		case <-d.Done():
			d.Info("Database service received stop signal", "serviceName", d.ServiceName)
			return nil
		}
	}

	d.Info("Database service run completed", "serviceName", d.ServiceName)
	d.Stop(task.ErrTaskComplete)
	return nil
}

func (d *DatabaseTask) Dispose() {
	d.Connected = false
	d.Info("Database service cleaned up", "serviceName", d.ServiceName)
}

// CacheTask Cache task (based on monibuca's TestBaseTask pattern)
type CacheTask struct {
	task.TickTask
	scenario *TestScenario
	TestTaskConfig
	ServiceName string
	CacheHit    int
}

func (c *CacheTask) GetTickInterval() time.Duration {
	return 500 * time.Millisecond
}

func (c *CacheTask) Start() error {
	c.Info("Cache service started", "serviceName", c.ServiceName)
	return c.TickTask.Start()
}

func (c *CacheTask) Tick(tick any) {
	c.CacheHit++
	c.Info("Cache service executed", "serviceName", c.ServiceName, "cacheHit", c.CacheHit)

	if c.CacheHit >= 10 {
		c.Info("Cache service execution completed, automatically stopping", "serviceName", c.ServiceName)
		c.Stop(task.ErrTaskComplete)
	}
}

func (c *CacheTask) Dispose() {
	c.Info("Cache service cleaned up", "serviceName", c.ServiceName, "totalCacheHit", c.CacheHit)
}

// Initialize task factory
func init() {
	testTaskFactory.Register("webserver", func(s *TestScenario, conf TestTaskConfig) task.ITask {
		return &WebServerTask{
			scenario:       s,
			TestTaskConfig: conf,
			ServerName:     "HTTP Server",
			Port:           8080,
		}
	})

	testTaskFactory.Register("database", func(s *TestScenario, conf TestTaskConfig) task.ITask {
		return &DatabaseTask{
			scenario:       s,
			TestTaskConfig: conf,
			ServiceName:    "MySQL Database",
		}
	})

	testTaskFactory.Register("cache", func(s *TestScenario, conf TestTaskConfig) task.ITask {
		return &CacheTask{
			scenario:       s,
			TestTaskConfig: conf,
			ServiceName:    "Redis Cache",
		}
	})
}

// TestLesson10 Test comprehensive application case (based on monibuca test plugin pattern)
func TestLesson10(t *testing.T) {
	// Create test scenario
	scenario := &TestScenario{
		Name: "Comprehensive Application Test Scenario",
		Tasks: []TestTaskConfig{
			{Action: "webserver", StreamPath: "test/webserver"},
			{Action: "database", StreamPath: "test/database"},
			{Action: "cache", StreamPath: "test/cache"},
		},
	}
	// Add test scenario to root task manager
	root.AddTask(scenario)
	scenario.WaitStopped()
	// Verify test results
	t.Logf("Lesson 10 test passed: Comprehensive application case based on monibuca pattern")
}
