package task

import (
	"errors"
	"testing"
	"time"
)

type eventLoopTestTask struct {
	Task
}

func (t *eventLoopTestTask) Run() error {
	time.Sleep(200 * time.Millisecond)
	return errors.New("test error")
}

func Test_EventLoop(t *testing.T) {
	for i := 0; i < 10; i++ {
		var job Job
		root.AddTask(&job)
		var tt eventLoopTestTask
		tt.SetRetry(-1, 100*time.Millisecond)
		tt.OnStart(func() {
			t.Log("task started", "taskId", tt.GetTaskID())
		})
		tt.OnDispose(func() {
			t.Log("task disposed", "taskId", tt.GetTaskID())
		})
		job.OnStart(func() {
			t.Log("job started", "jobId", job.GetTaskID())
		})
		job.OnDispose(func() {
			t.Log("job disposed", "jobId", job.GetTaskID())
		})
		job.AddTask(&tt)
	}
	time.Sleep(10 * time.Second)
	root.RangeSubTask(func(tt ITask) bool {
		if j, ok := tt.(IJob); ok {
			j.Stop(ErrTaskComplete)
			j.WaitStopped()
			t.Log("job stopped", "jobId", j.GetTaskID())
		}
		return true
	})
}
