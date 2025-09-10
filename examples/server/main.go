package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	task "github.com/langhuihui/gotask"
)

// 使用gotask项目定义的任务信息结构
type TaskInfo = task.TaskInfo
type TaskHistory = task.TaskHistory
type TaskTree = task.TaskTree
type TaskStats = task.TaskStats

// DemoTask 演示任务
type DemoTask struct {
	task.Task
	name     string
	duration time.Duration
	workFunc func()
}

func NewDemoTask(name string, duration time.Duration, workFunc func()) *DemoTask {
	dt := &DemoTask{
		name:     name,
		duration: duration,
		workFunc: workFunc,
	}
	dt.SetDescription("name", name)
	dt.SetDescription("duration", duration.String())
	dt.SetDescription("ownerType", "DemoTask")
	return dt
}

func (dt *DemoTask) GetOwnerType() string {
	return "DemoTask"
}

func (dt *DemoTask) Run() error {
	dt.SetDescription("status", "running")
	if dt.workFunc != nil {
		dt.workFunc()
	} else {
		// 默认工作：等待指定时间后完成
		time.Sleep(dt.duration)
	}
	return nil
}

// LongRunningTask 长期运行任务
type LongRunningTask struct {
	task.Job
	name    string
	counter int
}

func NewLongRunningTask(name string, interval time.Duration) *LongRunningTask {
	lrt := &LongRunningTask{
		name: name,
	}
	lrt.SetDescription("name", name)
	lrt.SetDescription("type", "long-running")
	lrt.SetDescription("ownerType", "LongRunningTask")
	return lrt
}

func (lrt *LongRunningTask) GetOwnerType() string {
	return "LongRunningTask"
}

func (lrt *LongRunningTask) GetTickInterval() time.Duration {
	return time.Second * 3
}

func (lrt *LongRunningTask) Tick(any) {
	lrt.counter++
	lrt.SetDescription("counter", fmt.Sprintf("%d", lrt.counter))
	lrt.SetDescription("last_tick", time.Now().Format("15:04:05"))

	// 模拟偶尔创建子任务
	if lrt.counter%5 == 0 {
		childTask := NewDemoTask(
			fmt.Sprintf("child-%d", lrt.counter),
			time.Second*2,
			func() {
				time.Sleep(time.Second * 2)
			},
		)
		lrt.AddTask(childTask)
	}
}

// KeepaliveTask 保持任务管理器运行的任务
type KeepaliveTask struct {
	task.Work
}

func (kt *KeepaliveTask) GetOwnerType() string {
	return "KeepaliveTask"
}

// TaskItem 实现ManagerItem接口的任务项
type TaskItem struct {
	task.ITask
}

func (ti *TaskItem) GetKey() uint32 {
	return ti.GetTaskID()
}

// 使用gotask项目的根任务管理器
type TaskManager = task.RootManager[uint32, *TaskItem]

// 服务器结构
type Server struct {
	taskManager  *TaskManager
	taskHistory  []TaskHistory
	historyMutex sync.Mutex
}

func NewServer() *Server {
	tm := &TaskManager{}
	tm.Init()

	// 创建一个keepalive任务来防止任务管理器退出
	keepaliveTask := &KeepaliveTask{}
	tm.AddTask(keepaliveTask)

	return &Server{
		taskManager: tm,
		taskHistory: make([]TaskHistory, 0),
	}
}

func (s *Server) createDemoTasks() {
	// 创建长期运行的任务
	longTask := NewLongRunningTask("background-worker", time.Second*3)
	s.taskManager.AddTask(&TaskItem{longTask})

	// 创建一些短期任务
	for i := 1; i <= 3; i++ {
		demoTask := NewDemoTask(
			fmt.Sprintf("demo-task-%d", i),
			time.Duration(i*2)*time.Second,
			func() {
				// 模拟一些工作
				time.Sleep(time.Millisecond * 500)
			},
		)
		s.taskManager.AddTask(&TaskItem{demoTask})
	}

	// 创建一个会产生子任务的任务
	parentTask := NewDemoTask("parent-task", time.Second*10, func() {
		for j := 1; j <= 2; j++ {
			time.Sleep(time.Second * 2)
		}
	})
	s.taskManager.AddTask(&TaskItem{parentTask})

	// 启动任务监控
	go s.monitorTasks()
}

func (s *Server) monitorTasks() {
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	for range ticker.C {
		// 检查已完成的任务并记录到历史
		s.taskManager.Range(func(t *TaskItem) bool {
			if t.GetState() == task.TASK_STATE_DISPOSED {
				s.addToHistory(t.ITask)
			}
			return true
		})
	}
}

func (s *Server) addToHistory(t task.ITask) {
	history := TaskHistory{
		ID:           t.GetTaskID(),
		Type:         t.GetTaskType(),
		OwnerType:    t.GetOwnerType(),
		StartTime:    t.GetTask().StartTime,
		EndTime:      time.Now(),
		Duration:     time.Since(t.GetTask().StartTime),
		State:        t.GetState(),
		RetryCount:   t.GetTask().GetRetryCount(),
		Descriptions: t.GetDescriptions(),
	}

	if t.StopReason() != nil {
		history.StopReason = t.StopReason().Error()
	}

	s.historyMutex.Lock()
	// 避免重复添加
	exists := false
	for _, h := range s.taskHistory {
		if h.ID == history.ID {
			exists = true
			break
		}
	}
	if !exists {
		s.taskHistory = append(s.taskHistory, history)
		log.Printf("Task added to history: ID=%d, Type=%s, Duration=%v",
			history.ID, history.OwnerType, history.Duration)
	}
	s.historyMutex.Unlock()
}

func (s *Server) getTaskTree() *TaskTree {
	// 使用gotask项目提供的BuildTaskTree函数
	rootInfo := task.BuildTaskTree(s.taskManager)
	if rootInfo == nil {
		return &TaskTree{}
	}

	return &TaskTree{
		Root: rootInfo,
	}
}

func (s *Server) getTasks() []TaskInfo {
	var tasks []TaskInfo
	s.taskManager.Range(func(t *TaskItem) bool {
		info := task.GetTaskInfo(t.ITask)
		if info != nil {
			tasks = append(tasks, *info)
		}
		return true
	})
	return tasks
}

func (s *Server) stopTask(id uint32, reason string) error {
	t, ok := s.taskManager.Get(id)
	if !ok {
		return fmt.Errorf("task not found")
	}

	if t.GetState() == task.TASK_STATE_DISPOSED {
		return fmt.Errorf("task already disposed")
	}

	t.Stop(task.ErrStopByUser)
	log.Printf("Task stopped: ID=%d, Reason=%s", id, reason)
	return nil
}

func (s *Server) getTaskHistory() []TaskHistory {
	s.historyMutex.Lock()
	defer s.historyMutex.Unlock()
	return s.taskHistory
}

func (s *Server) getTaskStats() TaskStats {
	stats := TaskStats{}

	s.taskManager.Range(func(t *TaskItem) bool {
		stats.TotalTasks++
		if t.GetState() == task.TASK_STATE_DISPOSED {
			if t.StopReason() == task.ErrTaskComplete {
				stats.CompletedTasks++
			} else {
				stats.FailedTasks++
			}
		} else {
			stats.RunningTasks++
		}
		stats.RetryCount += t.GetTask().GetRetryCount()
		return true
	})

	return stats
}

// HTTP 处理器
func (s *Server) getTaskTreeHandler(w http.ResponseWriter, r *http.Request) {
	tree := s.getTaskTree()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func (s *Server) getTasksHandler(w http.ResponseWriter, r *http.Request) {
	tasks := s.getTasks()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (s *Server) getTaskHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var id uint32
	_, err := fmt.Sscanf(taskID, "%d", &id)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	t, ok := s.taskManager.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	info := task.GetTaskInfo(t)
	if info == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (s *Server) stopTaskHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var id uint32
	_, err := fmt.Sscanf(taskID, "%d", &id)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Reason == "" {
		req.Reason = "User stop"
	}

	if err := s.stopTask(id, req.Reason); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Task stopped"})
}

func (s *Server) getTaskHistoryHandler(w http.ResponseWriter, r *http.Request) {
	history := s.getTaskHistory()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

func (s *Server) getTaskStatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := s.getTaskStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) createDemoTaskHandler(w http.ResponseWriter, r *http.Request) {
	s.createDemoTasks()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Demo tasks created"})
}

func main() {
	server := NewServer()

	// 创建初始示例任务
	server.createDemoTasks()

	// 设置路由
	r := mux.NewRouter()

	// CORS 中间件
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// API 路由
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/tasks/tree", server.getTaskTreeHandler).Methods("GET")
	api.HandleFunc("/tasks/stats", server.getTaskStatsHandler).Methods("GET")
	api.HandleFunc("/tasks/history", server.getTaskHistoryHandler).Methods("GET")
	api.HandleFunc("/tasks", server.getTasksHandler).Methods("GET")
	api.HandleFunc("/tasks", server.createDemoTaskHandler).Methods("POST")
	api.HandleFunc("/tasks/{id:[0-9]+}/stop", server.stopTaskHandler).Methods("POST")
	api.HandleFunc("/tasks/{id:[0-9]+}", server.getTaskHandler).Methods("GET")

	// 静态文件服务
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/dist")))

	fmt.Println("GoTask Server starting on :8080...")
	fmt.Println("API endpoints:")
	fmt.Println("  GET  /api/tasks/tree     - Get task tree")
	fmt.Println("  GET  /api/tasks          - Get all tasks")
	fmt.Println("  POST /api/tasks          - Create demo tasks")
	fmt.Println("  GET  /api/tasks/{id}     - Get task details")
	fmt.Println("  POST /api/tasks/{id}/stop - Stop a task")
	fmt.Println("  GET  /api/tasks/history  - Get task history")
	fmt.Println("  GET  /api/tasks/stats    - Get task statistics")

	log.Fatal(http.ListenAndServe(":8082", r))
}
