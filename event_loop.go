package task

import (
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

// Singleton 单例模式
type Singleton[T comparable] struct {
	instance atomic.Value
	mux      sync.Mutex
}

func (s *Singleton[T]) Load() T {
	return s.instance.Load().(T)
}

func (s *Singleton[T]) Get(newF func() T) T {
	ch := s.instance.Load() //fast
	if ch == nil {          // slow
		s.mux.Lock()
		defer s.mux.Unlock()
		if ch = s.instance.Load(); ch == nil {
			ch = newF()
			s.instance.Store(ch)
		}
	}
	return ch.(T)
}

// EventLoop 事件循环
type EventLoop struct {
	cases    []reflect.SelectCase
	children []ITask
	addSub   Singleton[chan any]
	running  atomic.Bool
	// bufferSize allows configuring the channel capacity; zero uses the default.
	bufferSize int
}

const eventLoopBaseCases = 1

func (e *EventLoop) removeChildCase(chosen int) {
	// chosen: index in e.cases; child index = chosen - eventLoopBaseCases
	childIndex := chosen - eventLoopBaseCases
	lastCaseIndex := len(e.cases) - 1
	lastChildIndex := len(e.children) - 1
	if childIndex < 0 || lastChildIndex < 0 {
		return
	}
	// swap-delete (order not preserved)
	if chosen != lastCaseIndex {
		e.cases[chosen] = e.cases[lastCaseIndex]
		e.children[childIndex] = e.children[lastChildIndex]
	}
	e.cases = e.cases[:lastCaseIndex]
	e.children = e.children[:lastChildIndex]
}

func (e *EventLoop) getInput() chan any {
	size := e.bufferSize
	if size <= 0 {
		size = DefaultEventLoopBufferSize
	}
	return e.addSub.Get(func() chan any {
		return make(chan any, size)
	})
}

// SetBufferSize customizes the event loop channel capacity; call before use.
func (e *EventLoop) SetBufferSize(size int) {
	if size > 0 {
		e.bufferSize = size
	}
}

func (e *EventLoop) active(mt *Job) {
	if mt.parent != nil {
		mt.parent.eventLoop.active(mt.parent)
	}
	if e.running.CompareAndSwap(false, true) {
		go e.run(mt)
	}
}

func (e *EventLoop) add(mt *Job, sub any) (err error) {
	shouldActive := true
	switch sub.(type) {
	case TaskStarter, TaskBlock, TaskGo:
	case IJob:
		shouldActive = false
	}
	select {
	case e.getInput() <- sub:
		if shouldActive || !e.running.Load() || mt.IsStopped() {
			e.active(mt)
		}
		return nil
	default:
		return ErrTooManyChildren
	}
}

// initCases 初始化（或重建）本轮的 select case 集合：第 0 个固定为输入通道，
// 其余为每个在册子任务的信号通道。
//
// 重建不是可选优化：上一轮 run() 可能异常退出（用户代码 panic 被 recover 后
// 整个 run() 被 unwind），此时 e.children 里仍有子任务。若不按 e.children
// 重建，重启的循环只剩输入通道一个 case，那些子任务的完成/停止信号永远没有
// case 接收——子任务得不到 dispose，等待它们的 waitChildStopped 永久挂起，
// 父循环被拖死，后续一切级联超时（TestEventLoopRecoversStringPanic 触发过
// 这条路径：Call 回调里 panic，job 的事件循环死亡，重启后丢了子任务 case）。
func (e *EventLoop) initCases(ch chan any) {
	cases := make([]reflect.SelectCase, 0, len(e.children)+1)
	cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)})
	for _, child := range e.children {
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(child.GetSignal())})
	}
	e.cases = cases
}

func (e *EventLoop) run(mt *Job) {
	mt.Debug("event loop start", "jobId", mt.GetTaskID(), "type", mt.GetOwnerType())
	myGen := mt.loopGen.Load()
	ch := e.getInput()
	e.initCases(ch)
	defer func() {
		err := recover()
		if err != nil {
			mt.Error("job panic", "err", err, "stack", string(debug.Stack()))
			if !ThrowPanic {
				mt.Stop(errors.Join(fmt.Errorf("%v", err), ErrPanic))
			} else {
				panic(err)
			}
		}
		mt.Debug("event loop exit", "jobId", mt.GetTaskID(), "type", mt.GetOwnerType())
		if !mt.handler.keepalive() && mt.loopGen.Load() == myGen {
			if mt.blocked != nil {
				mt.Stop(errors.Join(mt.blocked.StopReason(), ErrAutoStop))
			} else {
				mt.Stop(ErrAutoStop)
			}
		}
		mt.blocked = nil
		// Release the loop as the very last action, after every write to shared
		// loop state. active() starts a replacement run() as soon as it observes
		// running==false, and that replacement immediately writes e.cases and
		// mt.blocked; clearing the flag from inside the loop body (as before) let
		// it overlap this teardown, which writes mt.blocked above.
		//
		// Re-check the input channel AFTER the store so an add() that enqueued
		// while we were tearing down is not lost: if that add() observed
		// running==true it skipped active(), and the store below is what makes it
		// visible to us. len() on a channel is safe for concurrent use.
		e.running.Store(false)
		if len(ch) > 0 {
			e.active(mt)
		}
	}()

	// Main event loop - only exit when no more events AND no children. The
	// running flag is released by the deferred teardown above, not here.
	for {
		mt.blocked = nil
		if len(ch) == 0 && len(e.children) == 0 {
			return
		}
		if chosen, rev, ok := reflect.Select(e.cases); chosen == 0 {
			switch v := rev.Interface().(type) {
			case func():
				v()
			case ITask:
				if len(e.cases) >= 65535 {
					mt.Warn("task children too many, may cause performance issue", "count", len(e.cases), "taskId", mt.GetTaskID(), "taskType", mt.GetTaskType(), "ownerType", mt.GetOwnerType())
					// 停止之外还必须走一遍销毁：仅 Stop 的话 shutdown 永不落定
					// （父任务销毁时 waitChildStopped 等它，永久挂起），任务也会
					// 一直留在 children 里把 Size 撑住，循环永远等不到 children
					// 清空。dispose 对未启动任务走的是提前返回分支，只落定
					// shutdown、不跑钩子，代价可以忽略。
					v.Stop(ErrTooManyChildren)
					mt.onChildDispose(v)
					mt.removeChild(v)
					continue
				}
				if mt.blocked = v; v.start() {
					e.cases = append(e.cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(v.GetSignal())})
					e.children = append(e.children, v)
					mt.onChildStart(v)
				} else {
					mt.removeChild(v)
				}
			}
		} else {
			taskIndex := chosen - eventLoopBaseCases
			child := e.children[taskIndex]
			mt.blocked = child
			switch tt := mt.blocked.(type) {
			case IChannelTask:
				if tt.IsStopped() {
					mt.onChildDispose(child)
					mt.removeChild(child)
					e.removeChildCase(chosen)
				} else {
					tt.Tick(rev.Interface())
				}
			default:
				if !ok {
					if mt.onChildDispose(child); child.checkRetry(child.StopReason()) {
						if child.reset(); child.start() {
							e.cases[chosen].Chan = reflect.ValueOf(child.GetSignal())
							mt.onChildStart(child)
							continue
						}
					}
					mt.removeChild(child)
					e.removeChildCase(chosen)
				}
			}
		}
	}
}
