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

func (c *WorkCollection[K, T]) active(value any) (t T, ok bool) {
	t, ok = value.(T)
	if !ok {
		return
	}
	s := t.GetState()
	ok = s >= TASK_STATE_STARTED && s < TASK_STATE_DISPOSING
	if ok {
		return t, true
	}
	var zero T
	return zero, false
}

// Find 查找符合条件的任务
func (c *WorkCollection[K, T]) Find(f func(T) bool) (item T, ok bool) {
	c.Range(func(v T) bool {
		if f(v) {
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
		return c.active(value)
	}
	return
}

// Range 遍历任务
func (c *WorkCollection[K, T]) Range(f func(T) bool) {
	c.children.Range(func(key, value any) bool {
		if v, ok := c.active(value); ok {
			return f(v)
		}
		return true
	})
}

// Has 检查是否存在指定键的任务
func (c *WorkCollection[K, T]) Has(key K) (ok bool) {
	var value any
	value, ok = c.children.Load(key)
	if ok {
		_, ok = c.active(value)
	}
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
