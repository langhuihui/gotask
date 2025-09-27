package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	task "github.com/langhuihui/gotask"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

// Database 数据库操作结构
type Database struct {
	db *gorm.DB
}

// NewDatabase 创建数据库连接
func NewDatabase(dbPath string) (*Database, error) {
	db, err := gorm.Open(gormlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	database := &Database{db: db}

	// 自动迁移表结构
	if err := database.autoMigrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate tables: %w", err)
	}

	return database, nil
}

// autoMigrate 自动迁移数据库表结构
func (d *Database) autoMigrate() error {
	return d.db.AutoMigrate(
		&SessionInfo{},
		&TaskHistory{},
	)
}

// CreateSession 创建新会话
func (d *Database) CreateSession(pid int, args string) (string, error) {
	// 生成会话ID
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())

	session := &SessionInfo{
		SessionID: sessionID,
		StartTime: time.Now(),
		PID:       pid,
		Args:      args,
	}

	if err := d.db.Create(session).Error; err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return sessionID, nil
}

// GetSession 获取会话信息
func (d *Database) GetSession(sessionID string) (*SessionInfo, error) {
	var session SessionInfo
	err := d.db.Where("session_id = ?", sessionID).First(&session).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
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

	// 创建 GORM 模型
	taskHistory := &TaskHistory{
		TaskID:       history.TaskID,
		Type:         history.Type,
		OwnerType:    history.OwnerType,
		StartTime:    history.StartTime,
		EndTime:      history.EndTime,
		Duration:     history.Duration,
		State:        history.State,
		StopReason:   history.StopReason,
		RetryCount:   history.RetryCount,
		Descriptions: string(descriptionsJSON),
		MaxRetry:     history.MaxRetry,
		ParentID:     history.ParentID,
		Level:        history.Level,
		SessionID:    history.SessionID,
	}

	if err := d.db.Create(taskHistory).Error; err != nil {
		return fmt.Errorf("failed to save task history: %w", err)
	}

	return nil
}

// GetTaskHistory 获取任务历史记录
func (d *Database) GetTaskHistory(filter TaskHistoryFilter) (TaskHistoryResponse, error) {
	// 构建查询
	query := d.db.Model(&TaskHistory{})

	// 应用过滤条件
	if filter.OwnerType != "" {
		query = query.Where("owner_type = ?", filter.OwnerType)
	}

	if filter.TaskType != 0 {
		query = query.Where("task_type = ?", filter.TaskType)
	}

	if filter.SessionID != "" {
		query = query.Where("session_id = ?", filter.SessionID)
	}

	if filter.ParentID != nil {
		query = query.Where("parent_id = ?", *filter.ParentID)
	}

	if filter.StartTime != nil {
		query = query.Where("start_time >= ?", *filter.StartTime)
	}

	if filter.EndTime != nil {
		query = query.Where("start_time <= ?", *filter.EndTime)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
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

	// 查询数据
	var taskHistories []TaskHistory
	err := query.Order("start_time DESC").
		Limit(limit).
		Offset(offset).
		Find(&taskHistories).Error
	if err != nil {
		return TaskHistoryResponse{}, fmt.Errorf("failed to query task history: %w", err)
	}

	// 转换为响应格式
	var tasks []TaskHistory
	for _, th := range taskHistories {
		// 解析 descriptions JSON
		var descriptions map[string]string
		if err := json.Unmarshal([]byte(th.Descriptions), &descriptions); err != nil {
			log.Printf("Warning: failed to unmarshal descriptions for task %d: %v", th.TaskID, err)
			descriptions = make(map[string]string)
		}

		task := TaskHistory{
			TaskID:       th.TaskID,
			Type:         th.Type,
			OwnerType:    th.OwnerType,
			StartTime:    th.StartTime,
			EndTime:      th.EndTime,
			Duration:     th.Duration,
			State:        th.State,
			StopReason:   th.StopReason,
			RetryCount:   th.RetryCount,
			Descriptions: th.Descriptions, // 保持为字符串格式
			MaxRetry:     th.MaxRetry,
			ParentID:     th.ParentID,
			Level:        th.Level,
			SessionID:    th.SessionID,
		}
		tasks = append(tasks, task)
	}

	totalPages := (int(total) + limit - 1) / limit
	page := offset/limit + 1

	return TaskHistoryResponse{
		Tasks:      tasks,
		Total:      int(total),
		Page:       page,
		PageSize:   limit,
		TotalPages: totalPages,
	}, nil
}

// GetTaskHistoryStats 获取任务历史统计
func (d *Database) GetTaskHistoryStats(sessionID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 基本统计
	var totalTasks int64
	err := d.db.Model(&TaskHistory{}).Where("session_id = ?", sessionID).Count(&totalTasks).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count tasks: %w", err)
	}

	stats["totalTasks"] = int(totalTasks)
	stats["sessionId"] = sessionID

	// 获取会话开始时间
	var session SessionInfo
	err = d.db.Where("session_id = ?", sessionID).First(&session).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get session start time: %w", err)
	}
	stats["sessionStartTime"] = session.StartTime

	// 按OwnerType统计
	type OwnerTypeStat struct {
		OwnerType string
		Count     int64
	}
	var ownerTypeStats []OwnerTypeStat
	err = d.db.Model(&TaskHistory{}).
		Select("owner_type, COUNT(*) as count").
		Where("session_id = ?", sessionID).
		Group("owner_type").
		Scan(&ownerTypeStats).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query owner type stats: %w", err)
	}

	ownerTypeStatsMap := make(map[string]int)
	for _, stat := range ownerTypeStats {
		ownerTypeStatsMap[stat.OwnerType] = int(stat.Count)
	}
	stats["ownerTypeStats"] = ownerTypeStatsMap

	// 按TaskType统计
	type TaskTypeStat struct {
		TaskType int
		Count    int64
	}
	var taskTypeStats []TaskTypeStat
	err = d.db.Model(&TaskHistory{}).
		Select("task_type, COUNT(*) as count").
		Where("session_id = ?", sessionID).
		Group("task_type").
		Scan(&taskTypeStats).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query task type stats: %w", err)
	}

	taskTypeStatsMap := make(map[string]int)
	for _, stat := range taskTypeStats {
		taskTypeStatsMap[TaskTypeToString(task.TaskType(stat.TaskType))] = int(stat.Count)
	}
	stats["taskTypeStats"] = taskTypeStatsMap

	// 按状态统计
	type StateStat struct {
		State int
		Count int64
	}
	var stateStats []StateStat
	err = d.db.Model(&TaskHistory{}).
		Select("state, COUNT(*) as count").
		Where("session_id = ?", sessionID).
		Group("state").
		Scan(&stateStats).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query state stats: %w", err)
	}

	stateStatsMap := make(map[string]int)
	for _, stat := range stateStats {
		stateStatsMap[TaskStateToString(task.TaskState(stat.State))] = int(stat.Count)
	}
	stats["stateStats"] = stateStatsMap

	// 总执行时间
	var totalDurationNs int64
	err = d.db.Model(&TaskHistory{}).
		Select("COALESCE(SUM(duration), 0)").
		Where("session_id = ?", sessionID).
		Scan(&totalDurationNs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get total duration: %w", err)
	}

	if totalDurationNs > 0 {
		totalDuration := time.Duration(totalDurationNs)
		stats["totalDuration"] = totalDuration.String()

		if totalTasks > 0 {
			avgDuration := time.Duration(int64(totalDuration) / totalTasks)
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
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
