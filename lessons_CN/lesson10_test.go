package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// TestTaskFactory 测试任务工厂（基于 monibuca 模式）
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

// TestTaskConfig 测试任务配置（基于 monibuca 模式）
type TestTaskConfig struct {
	Action     string        `json:"action"`
	Delay      time.Duration `json:"delay"`
	ServerAddr string        `json:"serverAddr" default:"localhost"`
	StreamPath string        `json:"streamPath"`
}

// TestScenario 测试场景（基于 monibuca 的 TestCase 模式）
type TestScenario struct {
	task.Job         `json:"-"`
	Name      string           `json:"name"`
	Tasks     []TestTaskConfig `json:"tasks"`
}

func (ts *TestScenario) Start() error {
	ts.Info("测试场景启动", "name", ts.Name)

	// 创建并执行所有任务
	for _, taskConfig := range ts.Tasks {
		t, err := testTaskFactory.Create(taskConfig, ts)
		if err != nil {
			ts.Info("创建任务失败", "action", taskConfig.Action, "error", err)
			return err
		}
		ts.AddDependTask(t)
	}
	return nil
}

// WebServerTask 网络服务器任务（基于 monibuca 的 TestBaseTask 模式）
type WebServerTask struct {
	task.Task
	scenario *TestScenario
	TestTaskConfig
	ServerName string
	Port       int
}

func (w *WebServerTask) Start() error {
	w.Info("网络服务器启动", "serverName", w.ServerName, "port", w.Port)
	return nil
}

func (w *WebServerTask) Go() error {
	w.Info("网络服务器运行中", "serverName", w.ServerName)

	// 模拟服务器运行
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// 运行5次后自动停止
	count := 0
	for count < 5 {
		select {
		case <-ticker.C:
			w.Info("网络服务器状态检查正常", "serverName", w.ServerName)
			count++
		case <-w.Done():
			w.Info("网络服务器收到停止信号", "serverName", w.ServerName)
			return nil
		}
	}

	w.Info("网络服务器运行完成", "serverName", w.ServerName)
	w.Stop(task.ErrTaskComplete)
	return nil
}

func (w *WebServerTask) Dispose() {
	w.Info("网络服务器清理", "serverName", w.ServerName)
}

// DatabaseTask 数据库任务（基于 monibuca 的 TestBaseTask 模式）
type DatabaseTask struct {
	task.Task
	scenario *TestScenario
	TestTaskConfig
	ServiceName string
	Connected   bool
}

func (d *DatabaseTask) Start() error {
	d.Info("数据库服务启动", "serviceName", d.ServiceName)

	// 模拟数据库连接
	time.Sleep(500 * time.Millisecond)
	d.Connected = true
	d.Info("数据库服务连接成功", "serviceName", d.ServiceName)

	d.SetRetry(2, time.Second)
	return nil
}

func (d *DatabaseTask) Go() error {
	d.Info("数据库服务运行中", "serviceName", d.ServiceName)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 运行10次后自动停止
	count := 0
	for count < 10 {
		select {
		case <-ticker.C:
			d.Info("数据库服务连接池状态正常", "serviceName", d.ServiceName)
			count++
		case <-d.Done():
			d.Info("数据库服务收到停止信号", "serviceName", d.ServiceName)
			return nil
		}
	}

	d.Info("数据库服务运行完成", "serviceName", d.ServiceName)
	d.Stop(task.ErrTaskComplete)
	return nil
}

func (d *DatabaseTask) Dispose() {
	d.Connected = false
	d.Info("数据库服务清理", "serviceName", d.ServiceName)
}

// CacheTask 缓存任务（基于 monibuca 的 TestBaseTask 模式）
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
	c.Info("缓存服务启动", "serviceName", c.ServiceName)
	return c.TickTask.Start()
}

func (c *CacheTask) Tick(tick any) {
	c.CacheHit++
	c.Info("缓存服务执行", "serviceName", c.ServiceName, "cacheHit", c.CacheHit)

	if c.CacheHit >= 10 {
		c.Info("缓存服务执行完成，自动停止", "serviceName", c.ServiceName)
		c.Stop(task.ErrTaskComplete)
	}
}

func (c *CacheTask) Dispose() {
	c.Info("缓存服务清理", "serviceName", c.ServiceName, "totalCacheHit", c.CacheHit)
}

// 初始化任务工厂
func init() {
	testTaskFactory.Register("webserver", func(s *TestScenario, conf TestTaskConfig) task.ITask {
		return &WebServerTask{
			scenario:       s,
			TestTaskConfig: conf,
			ServerName:     "HTTP服务器",
			Port:           8080,
		}
	})

	testTaskFactory.Register("database", func(s *TestScenario, conf TestTaskConfig) task.ITask {
		return &DatabaseTask{
			scenario:       s,
			TestTaskConfig: conf,
			ServiceName:    "MySQL数据库",
		}
	})

	testTaskFactory.Register("cache", func(s *TestScenario, conf TestTaskConfig) task.ITask {
		return &CacheTask{
			scenario:       s,
			TestTaskConfig: conf,
			ServiceName:    "Redis缓存",
		}
	})
}

// TestLesson10 测试综合应用案例（基于 monibuca 测试插件模式）
func TestLesson10(t *testing.T) {
	// 创建测试场景
	scenario := &TestScenario{
		Name:    "综合应用测试场景",
		Tasks: []TestTaskConfig{
			{Action: "webserver", StreamPath: "test/webserver"},
			{Action: "database", StreamPath: "test/database"},
			{Action: "cache", StreamPath: "test/cache"},
		},
	}
	// 将测试场景添加到根任务管理器
	root.AddTask(scenario)
	scenario.WaitStopped()
	// 验证测试结果
	t.Logf("Lesson 10 测试通过：基于 monibuca 模式的综合应用案例")
}
