package task

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

// Use RootManager as root task manager
type TaskManager = RootManager[uint32, ManagerItem[uint32]]

// Create root task manager
var root TaskManager

func init() {
	root.Init()
}

func Test_AddTask_AddsTaskSuccessfully(t *testing.T) {
	var task Task
	root.AddTask(&task)
	_ = task.WaitStarted()
	root.RangeSubTask(func(t ITask) bool {
		if t.GetTaskID() == task.GetTaskID() {
			return false
		}
		return true
	})
}

type retryDemoTask struct {
	Task
}

func (task *retryDemoTask) Start() error {
	return io.ErrClosedPipe
}

func Test_RetryTask(t *testing.T) {
	var demoTask retryDemoTask
	var parent Job
	root.AddTask(&parent)
	demoTask.SetRetry(3, time.Second)
	parent.AddTask(&demoTask)
	_ = parent.WaitStopped()
	if demoTask.retry.RetryCount != 3 {
		t.Errorf("expected 3 retries, got %d", demoTask.retry.RetryCount)
	}
}

func Test_Call_ExecutesCallback(t *testing.T) {
	called := false
	root.Call(func() {
		called = true
		return
	})
	if !called {
		t.Errorf("expected callback to be called")
	}
}

func Test_StopByContext(t *testing.T) {
	var task Task
	ctx, cancel := context.WithCancel(context.Background())
	root.AddTask(&task, ctx)
	time.AfterFunc(time.Millisecond*100, cancel)
	if !errors.Is(task.WaitStopped(), context.Canceled) {
		t.Errorf("expected task to be stopped by context")
	}
}

func Test_ParentStop(t *testing.T) {
	var parent Job
	root.AddTask(&parent)
	var called atomic.Uint32
	var task Task
	checkCalled := func(expected uint32) {
		if count := called.Add(1); count != expected {
			t.Errorf("expected %d, got %d", expected, count)
		}
	}
	// 必须等两个 OnDispose 钩子都执行完再收尾：父任务的 dispose 排在子任务之后
	// （Job.dispose 先 waitChildrenDispose），只等子任务的 WaitStopped 会在父
	// 钩子执行前就返回，钩子里的 t.Errorf 随后落到已完成的测试上，直接 panic
	// （"Fail in goroutine after Test_ParentStop has completed"）。也不能改等
	// parent.WaitStopped()：父任务 start 走错误路径时 startup 被 Reject，
	// WaitStopped 开头的 WaitStarted 会带着停止原因提前返回，同样等不到 dispose。
	disposed := make(chan struct{}, 2)
	task.OnDispose(func() {
		checkCalled(1)
		disposed <- struct{}{}
	})
	parent.OnDispose(func() {
		checkCalled(2)
		disposed <- struct{}{}
	})
	parent.AddTask(&task)
	parent.Stop(ErrAutoStop)
	<-disposed
	<-disposed
	if !errors.Is(task.WaitStopped(), ErrAutoStop) {
		t.Errorf("expected task auto stop")
	}
}

func Test_ParentAutoStop(t *testing.T) {
	var parent Job
	root.AddTask(&parent)
	var called atomic.Uint32
	var task Task
	checkCalled := func(expected uint32) {
		if count := called.Add(1); count != expected {
			t.Errorf("expected %d, got %d", expected, count)
		}
	}
	task.OnDispose(func() {
		checkCalled(1)
	})
	parent.OnDispose(func() {
		checkCalled(2)
	})
	parent.AddTask(&task)
	time.AfterFunc(time.Second, func() {
		task.Stop(ErrTaskComplete)
	})
	if !errors.Is(parent.WaitStopped(), ErrAutoStop) {
		t.Errorf("expected task auto stop")
	}
}

// Test_Hooks 验证钩子的注册顺序语义：两对 OnStart/OnDispose 按注册顺序
// 执行，且 OnStart 全部先于 OnDispose。
//
// 历史：本测试原本写作"先 task.Stop 再 AddTask"，指望提前停止也被记住、
// 任务仍跑完整个生命周期。那个语义从未存在——旧实现把提前 Stop 整个吞掉
// （stopOnce 被消费时 context 尚未建立），任务带着永不关闭的新 context 正常
// 启动、跑完钩子后永久挂住，测试以 10 分钟超时收场。现已修复挂起（见
// Test_StopBeforeAddTask），但"已停止的任务不跑 OnStart 钩子"是框架自始至终
// 的 deliberate 语义（drainStartHooks 的 IsStopped 护栏，PR #6 明确保留），
// 因此这里改用正常启动后停止的场景来验证钩子顺序本身。
func Test_Hooks(t *testing.T) {
	var called atomic.Uint32
	var task Task
	checkCalled := func(expected uint32) {
		if count := called.Add(1); count != expected {
			t.Errorf("expected %d, got %d", expected, count)
		}
	}
	task.OnStart(func() {
		checkCalled(1)
	})
	task.OnDispose(func() {
		checkCalled(3)
	})
	task.OnStart(func() {
		checkCalled(2)
	})
	task.OnDispose(func() {
		checkCalled(4)
	})
	root.AddTask(&task)
	time.AfterFunc(time.Second, func() {
		task.Stop(ErrTaskComplete)
	})
	if err := task.WaitStopped(); !errors.Is(err, ErrTaskComplete) {
		t.Errorf("expected ErrTaskComplete, got %v", err)
	}
}

// Test_StopBeforeAddTask 覆盖"先 Stop 再 AddTask"的挂起缺陷。
//
// 旧实现用 stopOnce.Do 承载停止：AddTask 之前 CancelCauseFunc 还是 nil，
// 这一次 Stop 把 once 消费掉却没有取消任何 context；随后 InitContext 建一个
// 全新的、永不取消的 context，任务被当成正常任务跑完整个生命周期，信号通道
// 永不关闭——WaitStopped 与事件循环一起永久挂住。
//
// 修复后停止请求被记住，InitContext 建好 context 立即补上取消。锁定语义：
// OnStart 钩子不跑（已停止的任务不执行启动钩子，drainStartHooks 护栏），
// OnDispose 钩子照常执行，WaitStopped 返回停止原因。
func Test_StopBeforeAddTask(t *testing.T) {
	var task Task
	var onStartRan, onDisposeRan atomic.Bool
	task.OnStart(func() { onStartRan.Store(true) })
	task.OnDispose(func() { onDisposeRan.Store(true) })
	task.Stop(ErrTaskComplete)
	root.AddTask(&task)
	if err := task.WaitStopped(); !errors.Is(err, ErrTaskComplete) {
		t.Fatalf("expected ErrTaskComplete, got %v", err)
	}
	if onStartRan.Load() {
		t.Error("OnStart hook must not run for a task stopped before it starts")
	}
	if !onDisposeRan.Load() {
		t.Error("OnDispose hook must run so the task is fully torn down")
	}
}

type startFailTask struct {
	Task
}

func (task *startFailTask) Start() error {
	return errors.New("start failed")
}

func (task *startFailTask) Dispose() {
	task.Logger.Info("Dispose")
}

func Test_StartFail(t *testing.T) {
	var task startFailTask
	root.AddTask(&task)
	if err := task.WaitStarted(); err == nil {
		t.Errorf("expected start to fail")
	}
}

func Test_Block(t *testing.T) {
	var task Task
	block := make(chan struct{})
	var job Job
	task.OnStart(func() {
		task.OnStop(func() {
			close(block)
		})
		<-block
	})
	time.AfterFunc(time.Second*2, func() {
		job.Stop(ErrTaskComplete)
	})
	root.AddTask(&job)
	job.AddTask(&task)
	job.WaitStopped()
}

//
//type DemoTask struct {
//	Task
//	file     *os.File
//	filePath string
//}
//
//func (d *DemoTask) Start() (err error) {
//	d.file, err = os.Open(d.filePath)
//	return
//}
//
//func (d *DemoTask) Run() (err error) {
//	_, err = d.file.Write([]byte("hello"))
//	return
//}
//
//func (d *DemoTask) Dispose() {
//	d.file.Close()
//}
//
//type HelloWorld struct {
//	DemoTask
//}
//
//func (h *HelloWorld) Run() (err error) {
//	_, err = h.file.Write([]byte("world"))
//	return nil
//}

//type HelloWorld struct {
//	Task
//}
//
//func (h *HelloWorld) Start() (err error) {
//	fmt.Println("Hello World")
//	return nil
//}
