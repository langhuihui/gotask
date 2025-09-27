package main

import (
	"time"

	task "github.com/langhuihui/gotask"
	"gorm.io/gorm"
)

type TaskInfo struct {
	ID               uint32            `json:"id"`
	Type             task.TaskType     `json:"type"`
	OwnerType        string            `json:"ownerType"`
	StartTime        time.Time         `json:"startTime"`
	Descriptions     map[string]string `json:"description"`
	State            task.TaskState    `json:"state"`
	Blocked          *TaskInfo         `json:"blocked,omitempty"`
	Blocking         bool              `json:"blocking,omitempty"`
	Pointer          uint64            `json:"pointer"`
	Children         []*TaskInfo       `json:"children,omitempty"`
	Parent           *TaskInfo         `json:"parent,omitempty"`
	ParentID         uint32            `json:"parentId,omitempty"`
	EventLoopRunning bool              `json:"eventLoopRunning"`
	Level            uint32            `json:"level"`
	StartReason      string            `json:"startReason"`
	StopReason       string            `json:"stopReason,omitempty"`
	RetryCount       int               `json:"retryCount"`
	MaxRetry         int               `json:"maxRetry"`
}

// TaskHistory 任务历史记录
type TaskHistory struct {
	gorm.Model
	TaskID       uint32         `json:"taskId" gorm:"column:task_id;not null"`
	Type         task.TaskType  `json:"type" gorm:"column:task_type;not null"`
	OwnerType    string         `json:"ownerType" gorm:"column:owner_type;not null"`
	StartTime    time.Time      `json:"startTime" gorm:"column:start_time;not null"`
	EndTime      time.Time      `json:"endTime" gorm:"column:end_time;not null"`
	Duration     int64          `json:"duration" gorm:"column:duration;not null"` // 存储纳秒
	State        task.TaskState `json:"state" gorm:"column:state;not null"`
	StopReason   string         `json:"stopReason" gorm:"column:stop_reason"`
	RetryCount   int            `json:"retryCount" gorm:"column:retry_count;not null"`
	Descriptions string         `json:"descriptions" gorm:"column:descriptions;type:text"` // JSON 格式存储
	MaxRetry     int            `json:"maxRetry" gorm:"column:max_retry;not null"`
	ParentID     *uint32        `json:"parentId,omitempty" gorm:"column:parent_id"`
	Level        uint32         `json:"level" gorm:"column:level;not null"`
	SessionID    string         `json:"sessionId" gorm:"column:session_id;not null"`
}

// TaskHistoryFilter 任务历史过滤条件
type TaskHistoryFilter struct {
	OwnerType string        `json:"ownerType,omitempty"`
	TaskType  task.TaskType `json:"taskType,omitempty"`
	StartTime *time.Time    `json:"startTime,omitempty"`
	EndTime   *time.Time    `json:"endTime,omitempty"`
	SessionID string        `json:"sessionId,omitempty"`
	ParentID  *uint32       `json:"parentId,omitempty"`
	Limit     int           `json:"limit,omitempty"`
	Offset    int           `json:"offset,omitempty"`
}

// TaskHistoryResponse 任务历史查询响应
type TaskHistoryResponse struct {
	Tasks      []TaskHistory `json:"tasks"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	TotalPages int           `json:"totalPages"`
}

// SessionInfo 会话信息
type SessionInfo struct {
	gorm.Model
	SessionID string     `json:"sessionId" gorm:"column:session_id;uniqueIndex;not null"`
	StartTime time.Time  `json:"startTime" gorm:"column:start_time;not null"`
	EndTime   *time.Time `json:"endTime,omitempty" gorm:"column:end_time"`
	PID       int        `json:"pid" gorm:"column:pid"`
	Args      string     `json:"args" gorm:"column:args"`
}

// TaskStats 任务统计信息
type TaskStats struct {
	TotalTasks     int `json:"totalTasks"`
	RunningTasks   int `json:"runningTasks"`
	CompletedTasks int `json:"completedTasks"`
	FailedTasks    int `json:"failedTasks"`
	RetryCount     int `json:"retryCount"`
}

// TaskTree 任务树结构
type TaskTree struct {
	Root *TaskInfo `json:"root"`
}
