package main

import (
	"time"

	task "github.com/langhuihui/gotask"
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
	ID           uint32            `json:"id"`
	Type         task.TaskType     `json:"type"`
	OwnerType    string            `json:"ownerType"`
	StartTime    time.Time         `json:"startTime"`
	EndTime      time.Time         `json:"endTime"`
	Duration     time.Duration     `json:"duration"`
	State        task.TaskState    `json:"state"`
	StopReason   string            `json:"stopReason"`
	RetryCount   int               `json:"retryCount"`
	Descriptions map[string]string `json:"descriptions"`
	MaxRetry     int               `json:"maxRetry"`
	ParentID     uint32            `json:"parentId,omitempty"`
	Level        uint32            `json:"level"`
	SessionID    string            `json:"sessionId,omitempty"`
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
	ID        string    `json:"id"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime,omitempty"`
	PID       int       `json:"pid"`
	Args      string    `json:"args"`
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
