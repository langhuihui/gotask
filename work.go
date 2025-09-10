package task

// Work 工作任务，长期运行的任务容器
type Work struct {
	Job
}

func (m *Work) keepalive() bool {
	return true
}

func (*Work) GetTaskType() TaskType {
	return TASK_TYPE_Work
}

// WorkCollection 工作任务集合，支持泛型
type WorkCollection[K comparable, T interface {
	ITask
	GetKey() K
}] struct {
	Work
}

// Find 查找符合条件的任务
func (c *WorkCollection[K, T]) Find(f func(T) bool) (item T, ok bool) {
	c.RangeSubTask(func(task ITask) bool {
		if v, _ok := task.(T); _ok && f(v) {
			item = v
			ok = true
			return false
		}
		return true
	})
	return
}

// Get 根据键获取任务
func (c *WorkCollection[K, T]) Get(key K) (item T, ok bool) {
	var value any
	value, ok = c.children.Load(key)
	if ok {
		item, ok = value.(T)
	}
	return
}

// Range 遍历任务
func (c *WorkCollection[K, T]) Range(f func(T) bool) {
	c.RangeSubTask(func(task ITask) bool {
		if v, ok := task.(T); ok && !f(v) {
			return false
		}
		return true
	})
}

// Has 检查是否存在指定键的任务
func (c *WorkCollection[K, T]) Has(key K) (ok bool) {
	_, ok = c.children.Load(key)
	return
}

// ToList 转换为列表
func (c *WorkCollection[K, T]) ToList() (list []T) {
	c.Range(func(t T) bool {
		list = append(list, t)
		return true
	})
	return
}

// Length 获取任务数量
func (c *WorkCollection[K, T]) Length() int {
	return int(c.Size.Load())
}