package main

import task "github.com/langhuihui/gotask"

// BuildTaskTree 构建任务树（参考 monibuca 的 TaskTree 方法）
func BuildTaskTree(root task.ITask) *TaskInfo {
	var fillData func(m task.ITask) *TaskInfo
	fillData = func(m task.ITask) (res *TaskInfo) {
		if m == nil {
			return
		}
		t := m.GetTask()
		res = &TaskInfo{
			ID:               m.GetTaskID(),
			Pointer:          uint64(t.GetTaskPointer()),
			State:            m.GetState(),
			Type:             m.GetTaskType(),
			OwnerType:        m.GetOwnerType(),
			StartTime:        t.StartTime,
			Descriptions:     m.GetDescriptions(),
			StartReason:      t.StartReason,
			Level:            uint32(m.GetLevel()),
			EventLoopRunning: false,
			RetryCount:       t.GetRetryCount(),
			MaxRetry:         t.GetMaxRetry(),
		}

		if m.IsStopped() {
			res.StopReason = m.StopReason().Error()
		}

		// 处理 Job 类型的任务
		if job, ok := m.(task.IJob); ok {
			// 处理阻塞任务
			if blockedTask := job.Blocked(); blockedTask != nil {
				res.Blocked = fillData(blockedTask)
			}
			res.EventLoopRunning = job.EventLoopRunning()

			// 递归处理子任务
			job.RangeSubTask(func(child task.ITask) bool {
				childInfo := fillData(child)
				if childInfo == nil {
					return true
				}
				res.Children = append(res.Children, childInfo)
				return true
			})
		}
		return
	}
	return fillData(root)
}

// GetTaskInfo 获取任务信息（简化版本）
func GetTaskInfo(taskItem task.ITask) *TaskInfo {
	t := taskItem.GetTask()
	info := &TaskInfo{
		ID:               taskItem.GetTaskID(),
		Type:             taskItem.GetTaskType(),
		OwnerType:        taskItem.GetOwnerType(),
		State:            taskItem.GetState(),
		Level:            uint32(taskItem.GetLevel()),
		StartTime:        t.StartTime,
		StartReason:      t.StartReason,
		Descriptions:     taskItem.GetDescriptions(),
		Pointer:          uint64(t.GetTaskPointer()),
		EventLoopRunning: false,
		RetryCount:       t.GetRetryCount(),
		MaxRetry:         t.GetMaxRetry(),
	}

	if taskItem.IsStopped() {
		info.StopReason = taskItem.StopReason().Error()
	}

	if job, ok := taskItem.(task.IJob); ok {
		info.EventLoopRunning = job.EventLoopRunning()
		if blockedTask := job.Blocked(); blockedTask != nil {
			info.Blocked = GetTaskInfo(blockedTask)
		}
	}

	return info
}

// FlattenTaskTree 展平任务树（参考 monibuca 的 flattenTree 算法）
func FlattenTaskTree(tree *TaskInfo) []*TaskInfo {
	if tree == nil {
		return []*TaskInfo{}
	}

	// 创建根节点的副本
	firstItem := *tree
	firstItem.Children = []*TaskInfo{}
	flattened := []*TaskInfo{&firstItem}

	var recurse func(nodes []*TaskInfo, parent *TaskInfo)
	recurse = func(nodes []*TaskInfo, parent *TaskInfo) {
		// 按 ID 排序
		sortedNodes := make([]*TaskInfo, len(nodes))
		copy(sortedNodes, nodes)

		// 简单的冒泡排序
		for i := 0; i < len(sortedNodes)-1; i++ {
			for j := 0; j < len(sortedNodes)-i-1; j++ {
				if sortedNodes[j].ID > sortedNodes[j+1].ID {
					sortedNodes[j], sortedNodes[j+1] = sortedNodes[j+1], sortedNodes[j]
				}
			}
		}

		for _, node := range sortedNodes {
			node.Parent = parent
			if parent != nil {
				node.ParentID = parent.ID
			} else {
				node.ParentID = 0
			}

			// 处理阻塞关系
			if parent != nil && parent.Blocked != nil && parent.Blocked.ID == node.ID {
				node.Blocking = true
			}

			flattened = append(flattened, node)
			if len(node.Children) > 0 {
				recurse(node.Children, node)
			}
		}
	}

	recurse(tree.Children, nil)
	return flattened
}

// TaskStateToString 任务状态转字符串
func TaskStateToString(state task.TaskState) string {
	switch state {
	case task.TASK_STATE_INIT:
		return "INIT"
	case task.TASK_STATE_STARTING:
		return "STARTING"
	case task.TASK_STATE_STARTED:
		return "STARTED"
	case task.TASK_STATE_RUNNING:
		return "RUNNING"
	case task.TASK_STATE_GOING:
		return "GOING"
	case task.TASK_STATE_DISPOSING:
		return "DISPOSING"
	case task.TASK_STATE_DISPOSED:
		return "DISPOSED"
	default:
		return "UNKNOWN"
	}
}

// TaskTypeToString 任务类型转字符串
func TaskTypeToString(taskType task.TaskType) string {
	switch taskType {
	case task.TASK_TYPE_TASK:
		return "TASK"
	case task.TASK_TYPE_JOB:
		return "JOB"
	case task.TASK_TYPE_Work:
		return "WORK"
	case task.TASK_TYPE_CHANNEL:
		return "CHANNEL"
	default:
		return "UNKNOWN"
	}
}
