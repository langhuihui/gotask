package task

import (
	"time"
)

// TaskInfo 任务信息结构（与 monibuca 保持一致）
type TaskInfo struct {
	ID               uint32            `json:"id"`
	Type             TaskType          `json:"type"`
	OwnerType        string            `json:"owner"`
	StartTime        time.Time         `json:"startTime"`
	Descriptions     map[string]string `json:"description"`
	State            TaskState         `json:"state"`
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
	Type         TaskType          `json:"type"`
	OwnerType    string            `json:"ownerType"`
	StartTime    time.Time         `json:"startTime"`
	EndTime      time.Time         `json:"endTime"`
	Duration     time.Duration     `json:"duration"`
	State        TaskState         `json:"state"`
	StopReason   string            `json:"stopReason"`
	RetryCount   int               `json:"retryCount"`
	Descriptions map[string]string `json:"descriptions"`
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