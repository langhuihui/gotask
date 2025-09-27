package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	task "github.com/langhuihui/gotask"
)

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

// 使用gotask项目的根任务管理器
type TaskManager = task.RootManager[uint32, task.ManagerItem[uint32]]

// 服务器结构
type Server struct {
	taskManager   *TaskManager
	database      *Database
	sessionID     string
	sessionStart  time.Time
	enableHistory bool
}

func NewServer() *Server {
	tm := &TaskManager{}
	tm.Init()

	// 创建一个keepalive任务来防止任务管理器退出
	keepaliveTask := &KeepaliveTask{}
	tm.AddTask(keepaliveTask)

	// 初始化数据库
	db, err := NewDatabase("gotask.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 创建会话
	sessionID, err := db.CreateSession(os.Getpid(), strings.Join(os.Args, " "))
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	return &Server{
		taskManager:   tm,
		database:      db,
		sessionID:     sessionID,
		sessionStart:  time.Now(),
		enableHistory: true,
	}
}

func (s *Server) createDemoTasks() {
	// 创建长期运行的任务
	longTask := NewLongRunningTask("background-worker", time.Second*3)
	s.taskManager.AddTask(longTask)

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
		s.taskManager.AddTask(demoTask)
	}

	// 创建一个会产生子任务的任务
	parentTask := NewDemoTask("parent-task", time.Second*10, func() {
		for j := 1; j <= 2; j++ {
			time.Sleep(time.Second * 2)
		}
	})
	s.taskManager.AddTask(parentTask)

	// 设置任务完成事件监听（参考monibuca实现）
	s.taskManager.OnDescendantsDispose(s.saveTask)
}

// saveTask 保存任务信息到历史记录（参考monibuca实现）
func (s *Server) saveTask(task task.ITask) {
	if !s.enableHistory {
		return
	}

	// 将 descriptions 转换为 JSON 字符串
	descriptionsJSON, _ := json.Marshal(task.GetDescriptions())

	history := TaskHistory{
		TaskID:       task.GetTaskID(),
		Type:         task.GetTaskType(),
		OwnerType:    task.GetOwnerType(),
		StartTime:    task.GetTask().StartTime,
		EndTime:      time.Now(),
		Duration:     time.Since(task.GetTask().StartTime).Nanoseconds(),
		State:        task.GetState(),
		RetryCount:   task.GetTask().GetRetryCount(),
		Descriptions: string(descriptionsJSON),
		MaxRetry:     task.GetTask().GetMaxRetry(),
		Level:        uint32(task.GetLevel()),
		SessionID:    s.sessionID,
	}

	// 设置父任务ID
	if parent := task.GetParent(); parent != nil {
		parentID := parent.GetTaskID()
		history.ParentID = &parentID
	}

	if task.StopReason() != nil {
		history.StopReason = task.StopReason().Error()
	}

	// 保存到数据库
	if err := s.database.SaveTaskHistory(history); err != nil {
		log.Printf("Failed to save task history: %v", err)
	} else {
		log.Printf("Task saved to history: ID=%d, Type=%s, Duration=%v, Session=%s",
			history.TaskID, history.OwnerType, time.Duration(history.Duration), history.SessionID)
	}
}

func (s *Server) getTaskTree() *TaskInfo {
	return BuildTaskTree(s.taskManager)
}

func (s *Server) getTasks() []TaskInfo {
	var tasks []TaskInfo
	s.taskManager.Range(func(t task.ManagerItem[uint32]) bool {
		info := GetTaskInfo(t)
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
	response, err := s.database.GetTaskHistory(TaskHistoryFilter{})
	if err != nil {
		log.Printf("Failed to get task history: %v", err)
		return []TaskHistory{}
	}
	return response.Tasks
}

func (s *Server) getTaskHistoryWithFilter(filter TaskHistoryFilter) TaskHistoryResponse {
	response, err := s.database.GetTaskHistory(filter)
	if err != nil {
		log.Printf("Failed to get task history with filter: %v", err)
		return TaskHistoryResponse{}
	}
	return response
}

func (s *Server) getSessionInfo() SessionInfo {
	session, err := s.database.GetSession(s.sessionID)
	if err != nil {
		log.Printf("Failed to get session info: %v", err)
		endTime := time.Now()
		return SessionInfo{
			SessionID: s.sessionID,
			StartTime: s.sessionStart,
			EndTime:   &endTime,
			PID:       os.Getpid(),
			Args:      strings.Join(os.Args, " "),
		}
	}

	// 更新结束时间
	endTime := time.Now()
	session.EndTime = &endTime
	return *session
}

func (s *Server) getTaskStats() TaskStats {
	stats := TaskStats{}

	s.taskManager.Range(func(t task.ManagerItem[uint32]) bool {
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

	info := GetTaskInfo(t)
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
	// 解析查询参数
	filter := TaskHistoryFilter{}

	if ownerType := r.URL.Query().Get("ownerType"); ownerType != "" {
		filter.OwnerType = ownerType
	}

	if taskTypeStr := r.URL.Query().Get("taskType"); taskTypeStr != "" {
		if taskType, err := strconv.Atoi(taskTypeStr); err == nil {
			filter.TaskType = task.TaskType(taskType)
		}
	}

	if sessionID := r.URL.Query().Get("sessionId"); sessionID != "" {
		filter.SessionID = sessionID
	}

	if parentIDStr := r.URL.Query().Get("parentId"); parentIDStr != "" {
		if parentID, err := strconv.ParseUint(parentIDStr, 10, 32); err == nil {
			parentIDUint32 := uint32(parentID)
			filter.ParentID = &parentIDUint32
		}
	}

	if startTimeStr := r.URL.Query().Get("startTime"); startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			filter.StartTime = &startTime
		}
	}

	if endTimeStr := r.URL.Query().Get("endTime"); endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			filter.EndTime = &endTime
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	response := s.getTaskHistoryWithFilter(filter)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

func (s *Server) getSessionInfoHandler(w http.ResponseWriter, r *http.Request) {
	sessionInfo := s.getSessionInfo()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessionInfo)
}

func (s *Server) getTaskHistoryStatsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := s.database.GetTaskHistoryStats(s.sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get task history stats: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
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
	api.HandleFunc("/tasks/history/stats", server.getTaskHistoryStatsHandler).Methods("GET")
	api.HandleFunc("/session", server.getSessionInfoHandler).Methods("GET")
	api.HandleFunc("/tasks", server.getTasksHandler).Methods("GET")
	api.HandleFunc("/tasks", server.createDemoTaskHandler).Methods("POST")
	api.HandleFunc("/tasks/{id:[0-9]+}/stop", server.stopTaskHandler).Methods("POST")
	api.HandleFunc("/tasks/{id:[0-9]+}", server.getTaskHandler).Methods("GET")

	// 静态文件服务
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/dist")))

	fmt.Println("GoTask Server starting on :8082...")
	fmt.Println("API endpoints:")
	fmt.Println("  GET  /api/tasks/tree           - Get task tree")
	fmt.Println("  GET  /api/tasks                - Get all tasks")
	fmt.Println("  POST /api/tasks                - Create demo tasks")
	fmt.Println("  GET  /api/tasks/{id}           - Get task details")
	fmt.Println("  POST /api/tasks/{id}/stop      - Stop a task")
	fmt.Println("  GET  /api/tasks/history        - Get task history (with filtering)")
	fmt.Println("  GET  /api/tasks/history/stats  - Get task history statistics")
	fmt.Println("  GET  /api/session              - Get session information")
	fmt.Println("  GET  /api/tasks/stats          - Get task statistics")
	fmt.Println("")
	fmt.Println("Task History Query Parameters:")
	fmt.Println("  ownerType  - Filter by owner type")
	fmt.Println("  taskType   - Filter by task type (0=TASK, 1=JOB, 2=WORK, 3=CHANNEL)")
	fmt.Println("  sessionId  - Filter by session ID")
	fmt.Println("  parentId   - Filter by parent task ID")
	fmt.Println("  startTime  - Filter by start time (RFC3339 format)")
	fmt.Println("  endTime    - Filter by end time (RFC3339 format)")
	fmt.Println("  limit      - Number of results per page (default: 50)")
	fmt.Println("  offset     - Number of results to skip (default: 0)")
	s := http.Server{
		Addr:    ":8082",
		Handler: r,
	}
	server.taskManager.OnStop(func() {
		s.Close()
		server.database.Close()
	})
	s.ListenAndServe()
	log.Fatal(s.ListenAndServe())
}
