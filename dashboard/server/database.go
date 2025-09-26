package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	task "github.com/langhuihui/gotask"
	_ "github.com/mattn/go-sqlite3"
)

// Database 数据库操作结构
type Database struct {
	db *sql.DB
}

// NewDatabase 创建数据库连接
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	database := &Database{db: db}

	// 创建表结构
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return database, nil
}

// createTables 创建数据库表
func (d *Database) createTables() error {
	// 创建会话表
	sessionTable := `
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT UNIQUE NOT NULL,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		pid INTEGER,
		args TEXT
	);`

	// 创建任务历史表
	taskHistoryTable := `
	CREATE TABLE IF NOT EXISTS task_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL,
		task_type INTEGER NOT NULL,
		owner_type TEXT NOT NULL,
		start_time DATETIME NOT NULL,
		end_time DATETIME NOT NULL,
		duration INTEGER NOT NULL, -- 存储纳秒
		state INTEGER NOT NULL,
		stop_reason TEXT,
		retry_count INTEGER NOT NULL,
		descriptions TEXT, -- JSON 格式存储
		max_retry INTEGER NOT NULL,
		parent_id INTEGER,
		level INTEGER NOT NULL,
		session_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(session_id)
	);`

	// 创建索引
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_task_history_session_id ON task_history(session_id);",
		"CREATE INDEX IF NOT EXISTS idx_task_history_task_id ON task_history(task_id);",
		"CREATE INDEX IF NOT EXISTS idx_task_history_owner_type ON task_history(owner_type);",
		"CREATE INDEX IF NOT EXISTS idx_task_history_start_time ON task_history(start_time);",
		"CREATE INDEX IF NOT EXISTS idx_sessions_session_id ON sessions(session_id);",
	}

	// 执行表创建
	if _, err := d.db.Exec(sessionTable); err != nil {
		return fmt.Errorf("failed to create sessions table: %w", err)
	}

	if _, err := d.db.Exec(taskHistoryTable); err != nil {
		return fmt.Errorf("failed to create task_history table: %w", err)
	}

	// 创建索引
	for _, index := range indexes {
		if _, err := d.db.Exec(index); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// CreateSession 创建新会话
func (d *Database) CreateSession(pid int, args string) (string, error) {
	// 生成会话ID
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())

	query := `
		INSERT INTO sessions (session_id, start_time, pid, args)
		VALUES (?, ?, ?, ?)`

	_, err := d.db.Exec(query, sessionID, time.Now(), pid, args)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return sessionID, nil
}

// GetSession 获取会话信息
func (d *Database) GetSession(sessionID string) (*SessionInfo, error) {
	query := `
		SELECT session_id, start_time, end_time, pid, args
		FROM sessions
		WHERE session_id = ?`

	var session SessionInfo
	var endTime sql.NullTime

	err := d.db.QueryRow(query, sessionID).Scan(
		&session.ID,
		&session.StartTime,
		&endTime,
		&session.PID,
		&session.Args,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if endTime.Valid {
		session.EndTime = endTime.Time
	}

	return &session, nil
}

// SaveTaskHistory 保存任务历史记录
func (d *Database) SaveTaskHistory(history TaskHistory) error {
	// 将 descriptions 转换为 JSON
	descriptionsJSON, err := json.Marshal(history.Descriptions)
	if err != nil {
		return fmt.Errorf("failed to marshal descriptions: %w", err)
	}

	query := `
		INSERT INTO task_history (
			task_id, task_type, owner_type, start_time, end_time, duration,
			state, stop_reason, retry_count, descriptions, max_retry,
			parent_id, level, session_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = d.db.Exec(query,
		history.ID,
		history.Type,
		history.OwnerType,
		history.StartTime,
		history.EndTime,
		history.Duration.Nanoseconds(),
		history.State,
		history.StopReason,
		history.RetryCount,
		string(descriptionsJSON),
		history.MaxRetry,
		history.ParentID,
		history.Level,
		history.SessionID,
	)

	if err != nil {
		return fmt.Errorf("failed to save task history: %w", err)
	}

	return nil
}

// GetTaskHistory 获取任务历史记录
func (d *Database) GetTaskHistory(filter TaskHistoryFilter) (TaskHistoryResponse, error) {
	// 构建查询条件
	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if filter.OwnerType != "" {
		whereClause += " AND owner_type = ?"
		args = append(args, filter.OwnerType)
	}

	if filter.TaskType != 0 {
		whereClause += " AND task_type = ?"
		args = append(args, filter.TaskType)
	}

	if filter.SessionID != "" {
		whereClause += " AND session_id = ?"
		args = append(args, filter.SessionID)
	}

	if filter.ParentID != nil {
		whereClause += " AND parent_id = ?"
		args = append(args, *filter.ParentID)
	}

	if filter.StartTime != nil {
		whereClause += " AND start_time >= ?"
		args = append(args, *filter.StartTime)
	}

	if filter.EndTime != nil {
		whereClause += " AND start_time <= ?"
		args = append(args, *filter.EndTime)
	}

	// 获取总数
	countQuery := "SELECT COUNT(*) FROM task_history " + whereClause
	var total int
	err := d.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return TaskHistoryResponse{}, fmt.Errorf("failed to count task history: %w", err)
	}

	// 设置分页参数
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// 构建查询
	query := `
		SELECT task_id, task_type, owner_type, start_time, end_time, duration,
			   state, stop_reason, retry_count, descriptions, max_retry,
			   parent_id, level, session_id
		FROM task_history ` + whereClause + `
		ORDER BY start_time DESC
		LIMIT ? OFFSET ?`

	args = append(args, limit, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return TaskHistoryResponse{}, fmt.Errorf("failed to query task history: %w", err)
	}
	defer rows.Close()

	var tasks []TaskHistory
	for rows.Next() {
		var history TaskHistory
		var descriptionsJSON string
		var parentID sql.NullInt32

		err := rows.Scan(
			&history.ID,
			&history.Type,
			&history.OwnerType,
			&history.StartTime,
			&history.EndTime,
			&history.Duration,
			&history.State,
			&history.StopReason,
			&history.RetryCount,
			&descriptionsJSON,
			&history.MaxRetry,
			&parentID,
			&history.Level,
			&history.SessionID,
		)
		if err != nil {
			return TaskHistoryResponse{}, fmt.Errorf("failed to scan task history: %w", err)
		}

		// 转换 duration 从纳秒到 time.Duration
		history.Duration = time.Duration(history.Duration)

		// 解析 descriptions JSON
		if err := json.Unmarshal([]byte(descriptionsJSON), &history.Descriptions); err != nil {
			log.Printf("Warning: failed to unmarshal descriptions for task %d: %v", history.ID, err)
			history.Descriptions = make(map[string]string)
		}

		// 设置 ParentID
		if parentID.Valid {
			history.ParentID = uint32(parentID.Int32)
		}

		tasks = append(tasks, history)
	}

	if err := rows.Err(); err != nil {
		return TaskHistoryResponse{}, fmt.Errorf("error iterating task history: %w", err)
	}

	totalPages := (total + limit - 1) / limit
	page := offset/limit + 1

	return TaskHistoryResponse{
		Tasks:      tasks,
		Total:      total,
		Page:       page,
		PageSize:   limit,
		TotalPages: totalPages,
	}, nil
}

// GetTaskHistoryStats 获取任务历史统计
func (d *Database) GetTaskHistoryStats(sessionID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 基本统计
	countQuery := "SELECT COUNT(*) FROM task_history WHERE session_id = ?"
	var totalTasks int
	err := d.db.QueryRow(countQuery, sessionID).Scan(&totalTasks)
	if err != nil {
		return nil, fmt.Errorf("failed to count tasks: %w", err)
	}

	stats["totalTasks"] = totalTasks
	stats["sessionId"] = sessionID

	// 获取会话开始时间
	sessionQuery := "SELECT start_time FROM sessions WHERE session_id = ?"
	var sessionStart time.Time
	err = d.db.QueryRow(sessionQuery, sessionID).Scan(&sessionStart)
	if err != nil {
		return nil, fmt.Errorf("failed to get session start time: %w", err)
	}
	stats["sessionStartTime"] = sessionStart

	// 按OwnerType统计
	ownerTypeQuery := `
		SELECT owner_type, COUNT(*) 
		FROM task_history 
		WHERE session_id = ? 
		GROUP BY owner_type`

	ownerTypeStats := make(map[string]int)
	rows, err := d.db.Query(ownerTypeQuery, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query owner type stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ownerType string
		var count int
		if err := rows.Scan(&ownerType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan owner type stats: %w", err)
		}
		ownerTypeStats[ownerType] = count
	}
	stats["ownerTypeStats"] = ownerTypeStats

	// 按TaskType统计
	taskTypeQuery := `
		SELECT task_type, COUNT(*) 
		FROM task_history 
		WHERE session_id = ? 
		GROUP BY task_type`

	taskTypeStats := make(map[string]int)
	rows, err = d.db.Query(taskTypeQuery, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query task type stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var taskType int
		var count int
		if err := rows.Scan(&taskType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan task type stats: %w", err)
		}
		taskTypeStats[TaskTypeToString(task.TaskType(taskType))] = count
	}
	stats["taskTypeStats"] = taskTypeStats

	// 按状态统计
	stateQuery := `
		SELECT state, COUNT(*) 
		FROM task_history 
		WHERE session_id = ? 
		GROUP BY state`

	stateStats := make(map[string]int)
	rows, err = d.db.Query(stateQuery, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query state stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var state int
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			return nil, fmt.Errorf("failed to scan state stats: %w", err)
		}
		stateStats[TaskStateToString(task.TaskState(state))] = count
	}
	stats["stateStats"] = stateStats

	// 总执行时间
	durationQuery := "SELECT SUM(duration) FROM task_history WHERE session_id = ?"
	var totalDurationNs sql.NullInt64
	err = d.db.QueryRow(durationQuery, sessionID).Scan(&totalDurationNs)
	if err != nil {
		return nil, fmt.Errorf("failed to get total duration: %w", err)
	}

	if totalDurationNs.Valid {
		totalDuration := time.Duration(totalDurationNs.Int64)
		stats["totalDuration"] = totalDuration.String()

		if totalTasks > 0 {
			avgDuration := time.Duration(int64(totalDuration) / int64(totalTasks))
			stats["averageDuration"] = avgDuration.String()
		} else {
			stats["averageDuration"] = "0s"
		}
	} else {
		stats["totalDuration"] = "0s"
		stats["averageDuration"] = "0s"
	}

	return stats, nil
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	return d.db.Close()
}
