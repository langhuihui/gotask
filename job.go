package task

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/langhuihui/gotask/util"
)

var idG sync.Mutex
var taskIDCounter uint32
var sourceFilePathPrefix string

type ExistTaskError struct {
	Task ITask
}

func (e ExistTaskError) Error() string {
	return fmt.Sprintf("%v exist", e.Task.getKey())
}

func init() {
	if _, file, _, ok := runtime.Caller(0); ok {
		sourceFilePathPrefix = strings.TrimSuffix(file, "job.go")
	}
}

func GetNextTaskID() uint32 {
	idG.Lock()
	defer idG.Unlock()
	taskIDCounter++
	return taskIDCounter
}

// Job 任务容器，可以包含和管理多个子任务
type Job struct {
	Task
	children                    sync.Map
	descendantsDisposeListeners []func(ITask)
	descendantsStartListeners   []func(ITask)
	blocked                     ITask
	eventLoop                   EventLoop
	Size                        atomic.Int32
}

func (*Job) GetTaskType() TaskType {
	return TASK_TYPE_JOB
}

func (mt *Job) getJob() *Job {
	return mt
}

func (mt *Job) Blocked() ITask {
	return mt.blocked
}

func (mt *Job) EventLoopRunning() bool {
	return mt.eventLoop.running.Load()
}

// SetEventLoopBufferSize sets the capacity for the job's event loop channel.
// Call before the job starts to ensure the new buffer size is used.
func (mt *Job) SetEventLoopBufferSize(size int) {
	mt.eventLoop.SetBufferSize(size)
}

func (mt *Job) waitChildStopped(child ITask) error {
	done := make(chan error, 1)
	go func() {
		done <- child.WaitStopped()
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return err
		case <-ticker.C:
			mt.Warn(
				"wait child dispose stalled",
				"childId", child.GetTaskID(),
				"childType", child.GetTaskType(),
				"childOwner", child.GetOwnerType(),
				"childState", child.GetState(),
				"childDescriptions", child.GetDescriptions(),
			)
		}
	}
}

func (mt *Job) waitChildrenDispose(stopReason error) {
	mt.Debug("wait children dispose begin", "reason", stopReason, "childCount", mt.Size.Load())
	defer mt.Debug("wait children dispose end", "reason", stopReason, "childCount", mt.Size.Load())
	mt.eventLoop.active(mt)
	mt.children.Range(func(key, value any) bool {
		child := value.(ITask)
		mt.Debug(
			"wait child dispose begin",
			"childId", child.GetTaskID(),
			"childType", child.GetTaskType(),
			"childOwner", child.GetOwnerType(),
			"childState", child.GetState(),
			"childDescriptions", child.GetDescriptions(),
		)
		child.Stop(stopReason)
		mt.Debug(
			"wait child stop signaled",
			"childId", child.GetTaskID(),
			"childType", child.GetTaskType(),
			"childOwner", child.GetOwnerType(),
			"childState", child.GetState(),
			"childStopReason", child.StopReason(),
		)
		mt.SetDescription("waitChildDispose", child.GetTaskID())
		err := mt.waitChildStopped(child)
		mt.Debug(
			"wait child dispose end",
			"childId", child.GetTaskID(),
			"childType", child.GetTaskType(),
			"childOwner", child.GetOwnerType(),
			"childState", child.GetState(),
			"childStopReason", child.StopReason(),
			"err", err,
			"childDescriptions", child.GetDescriptions(),
		)
		mt.RemoveDescription("waitChildDispose")
		return true
	})
}

// OnDescendantsDispose 注册后代任务销毁时的回调。
//
// 与 Task 上那四个钩子同理，用内嵌 Task 的 hookMu 保护——注册端在业务
// goroutine，消费端在事件循环 goroutine。这里的波及面还更大一些：
// onDescendantsDispose 会一路向上传播到祖先，也就是**祖先的切片会被后代的事件
// 循环 goroutine 读**。向上传播时取的是各自 Job 的锁，只上行不回环，不会死锁。
func (mt *Job) OnDescendantsDispose(listener func(ITask)) {
	mt.hookMu.Lock()
	defer mt.hookMu.Unlock()
	mt.descendantsDisposeListeners = append(mt.descendantsDisposeListeners, listener)
}

func (mt *Job) onDescendantsDispose(descendants ITask) {
	mt.hookMu.Lock()
	listeners := mt.descendantsDisposeListeners
	mt.hookMu.Unlock()
	for _, listener := range listeners {
		listener(descendants)
	}
	if mt.parent != nil {
		mt.parent.onDescendantsDispose(descendants)
	}
}

func (mt *Job) onChildDispose(child ITask) {
	mt.onDescendantsDispose(child)
	child.dispose()
}

func (mt *Job) removeChild(child ITask) {
	if mt.children.CompareAndDelete(child.getKey(), child) {
		remains := mt.Size.Add(-1)
		mt.Debug("remove child", "id", child.GetTaskID(), "remains", remains)
	}
}

// OnDescendantsStart 注册后代任务启动时的回调，同步方式见 OnDescendantsDispose。
func (mt *Job) OnDescendantsStart(listener func(ITask)) {
	mt.hookMu.Lock()
	defer mt.hookMu.Unlock()
	mt.descendantsStartListeners = append(mt.descendantsStartListeners, listener)
}

func (mt *Job) onDescendantsStart(descendants ITask) {
	mt.hookMu.Lock()
	listeners := mt.descendantsStartListeners
	mt.hookMu.Unlock()
	for _, listener := range listeners {
		listener(descendants)
	}
	if mt.parent != nil {
		mt.parent.onDescendantsStart(descendants)
	}
}

func (mt *Job) onChildStart(child ITask) {
	mt.onDescendantsStart(child)
}

func (mt *Job) RangeSubTask(callback func(task ITask) bool) {
	mt.children.Range(func(key, value any) bool {
		callback(value.(ITask))
		return true
	})
}

func (mt *Job) AddDependTask(t ITask, opt ...any) (task *Task) {
	t.Using(mt)
	opt = append(opt, 1)
	return mt.AddTask(t, opt...)
}

func (mt *Job) initContext(task *Task, opt ...any) {
	callDepth := 2
	for _, o := range opt {
		switch v := o.(type) {
		case context.Context:
			task.parentCtx = v
		case Description:
			task.SetDescriptions(v)
		case RetryConfig:
			task.retry = v
		case *slog.Logger:
			task.Logger = v
		case int:
			callDepth += v
		}
	}
	_, file, line, ok := runtime.Caller(callDepth)
	if ok {
		task.StartReason = fmt.Sprintf("%s:%d", strings.TrimPrefix(file, sourceFilePathPrefix), line)
	}
	task.parent = mt
	if task.parentCtx == nil {
		task.parentCtx = mt.Context
	}
	task.level = mt.level + 1
	if task.ID == 0 {
		task.ID = GetNextTaskID()
	}
	task.Context, task.CancelCauseFunc = context.WithCancelCause(task.parentCtx)
	task.startup = util.NewPromise(task.Context)
	task.shutdown = util.NewPromise(context.Background())
	if task.Logger == nil {
		task.Logger = mt.Logger
	}
}

func (mt *Job) AddTask(t ITask, opt ...any) (task *Task) {
	task = t.GetTask()
	task.handler = t
	mt.initContext(task, opt...)
	if mt.IsStopped() {
		mt.Warn("[fix1] AddTask rejected: parent already stopped", "key", t.getKey(), "newId", task.ID, "reason", mt.StopReason())
		task.startup.Reject(mt.StopReason())
		task.CancelCauseFunc(mt.StopReason()) // also cancel task.ctx so IsStopped() returns true
		return
	}
	actual, loaded := mt.children.LoadOrStore(t.getKey(), t)
	if loaded {
		existingTask := actual.(ITask)
		if existingTask.GetState() >= TASK_STATE_DISPOSING || existingTask.IsStopped() {
			// Existing task is stopped/disposing/disposed, replace it with the new one
			if !mt.children.CompareAndSwap(t.getKey(), actual, t) {
				// CAS failed: old task was already removed by removeChild, retry LoadOrStore
				if actual, loaded = mt.children.LoadOrStore(t.getKey(), t); loaded {
					mt.Warn("[fix1] AddTask rejected: CAS failed, key still occupied", "key", t.getKey(), "newId", task.ID, "existingId", actual.(ITask).GetTask().ID)
					task.startup.Reject(ExistTaskError{
						Task: actual.(ITask),
					})
					task.CancelCauseFunc(ExistTaskError{Task: actual.(ITask)}) // also cancel task.ctx
					return
				}
			} else {
				// CAS succeeded: compensate Size for the replaced task whose removeChild
				// CompareAndDelete will fail (map value is now the new task)
				mt.Size.Add(-1)
			}
		} else {
			mt.Warn("[fix1] AddTask rejected: key conflict, existing task still running", "key", t.getKey(), "newId", task.ID, "existingId", existingTask.GetTask().ID)
			task.startup.Reject(ExistTaskError{
				Task: existingTask,
			})
			task.CancelCauseFunc(ExistTaskError{Task: existingTask}) // also cancel task.ctx so IsStopped() returns true
			return
		}
	}
	var err error
	defer func() {
		if err != nil {
			mt.children.Delete(t.getKey())
			mt.Warn("[fix1] AddTask rejected: eventLoop/stopped after insert", "key", t.getKey(), "newId", task.ID, "reason", err)
			task.startup.Reject(err)
			task.CancelCauseFunc(err) // also cancel task.ctx so IsStopped() returns true
		}
	}()
	if err = mt.eventLoop.add(mt, t); err != nil {
		return
	}
	if mt.IsStopped() {
		err = mt.StopReason()
		return
	}
	remains := mt.Size.Add(1)
	mt.Debug("child added", "id", task.ID, "remains", remains)
	return
}

func (mt *Job) Call(callback func()) {
	_, file, line, _ := runtime.Caller(1)
	caller := fmt.Sprintf("%s:%d", strings.TrimPrefix(file, sourceFilePathPrefix), line)
	if mt.Size.Load() <= 0 {
		mt.Debug("call immediately", "caller", caller)
		startTime := time.Now()
		callback()
		mt.Debug("call immediately done", "caller", caller, "elapsed", time.Since(startTime))
		return
	}
	ctx, cancel := context.WithCancel(mt)
	_ = mt.eventLoop.add(mt, func() {
		startTime := time.Now()
		mt.Debug("call", "caller", caller)
		callback()
		mt.Debug("call done", "caller", caller, "elapsed", time.Since(startTime))
		cancel()
	})
	<-ctx.Done()
}
