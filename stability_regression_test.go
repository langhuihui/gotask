package task

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/langhuihui/gotask/util"
)

func waitForTaskStopped(t *testing.T, task ITask) error {
	t.Helper()
	result := make(chan error, 1)
	go func() {
		result <- task.WaitStopped()
	}()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for task %d to stop", task.GetTaskID())
		return nil
	}
}

func TestRunTaskStoppedParentRejectsChildOnly(t *testing.T) {
	var owner Job
	owner.handler = &owner
	owner.Context, owner.CancelCauseFunc = context.WithCancelCause(context.Background())
	owner.startup = util.NewPromise(context.Background())
	owner.CancelCauseFunc(ErrExit)

	var child Task
	if err := owner.RunTask(&child); !errors.Is(err, ErrExit) {
		t.Fatalf("expected ErrExit, got %v", err)
	}
	if err := child.WaitStarted(); !errors.Is(err, ErrExit) {
		t.Fatalf("expected child startup to be rejected with ErrExit, got %v", err)
	}
	select {
	case <-owner.startup.Done():
		t.Fatal("RunTask completed the receiver's startup promise")
	default:
	}
}

func TestCallReturnsWhenQueueIsFull(t *testing.T) {
	var job Job
	job.handler = &job
	job.Context, job.CancelCauseFunc = context.WithCancelCause(context.Background())
	job.startup = util.NewPromise(job.Context)
	job.Size.Store(1)
	job.eventLoop.bufferSize = 1
	job.eventLoop.getInput() <- struct{}{}

	var called atomic.Bool
	done := make(chan struct{})
	go func() {
		job.Call(func() { called.Store(true) })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		job.CancelCauseFunc(ErrExit)
		t.Fatal("Call did not return after the event loop rejected its callback")
	}
	if called.Load() {
		t.Fatal("callback ran despite not being enqueued")
	}
}

func TestRetryBackoffStopsWithParent(t *testing.T) {
	var parent Job
	parent.Context, parent.CancelCauseFunc = context.WithCancelCause(context.Background())
	var child Task
	child.parent = &parent
	child.StartTime = time.Now()
	child.SetRetry(1, 400*time.Millisecond)

	result := make(chan bool, 1)
	go func() {
		result <- child.checkRetry(errors.New("retry"))
	}()
	time.Sleep(20 * time.Millisecond)
	started := time.Now()
	parent.CancelCauseFunc(ErrExit)
	select {
	case retry := <-result:
		if retry {
			t.Fatal("retry continued after parent stopped")
		}
		if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
			t.Fatalf("retry backoff was not cancellable: %v", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("retry backoff did not finish")
	}
}

func TestRangeSubTaskStopsWhenCallbackReturnsFalse(t *testing.T) {
	var job Job
	job.children.Store(1, &Task{ID: 1})
	job.children.Store(2, &Task{ID: 2})
	var visits atomic.Int32
	job.RangeSubTask(func(ITask) bool {
		visits.Add(1)
		return false
	})
	if got := visits.Load(); got != 1 {
		t.Fatalf("expected one visit, got %d", got)
	}
}

type stabilityRunTask struct {
	Task
	attempts atomic.Int32
}

func (t *stabilityRunTask) Run() error {
	if t.attempts.Add(1) >= 5 {
		return ErrTaskComplete
	}
	return errors.New("run failed")
}

type stabilityGoTask struct {
	Task
	attempts atomic.Int32
}

func (t *stabilityGoTask) Go() error {
	if t.attempts.Add(1) >= 5 {
		return ErrTaskComplete
	}
	return errors.New("go failed")
}

func TestFiniteRetryLimitsRunAndGoFailures(t *testing.T) {
	t.Run("Run", func(t *testing.T) {
		task := &stabilityRunTask{}
		task.SetRetry(2, 0)
		root.AddTask(task)
		waitForAttempts(t, task.attempts.Load)
		if got := task.attempts.Load(); got != 3 {
			t.Fatalf("expected initial attempt and two retries, got %d attempts", got)
		}
	})
	t.Run("Go", func(t *testing.T) {
		task := &stabilityGoTask{}
		task.SetRetry(2, 0)
		root.AddTask(task)
		waitForAttempts(t, task.attempts.Load)
		if got := task.attempts.Load(); got != 3 {
			t.Fatalf("expected initial attempt and two retries, got %d attempts", got)
		}
	})
}

func TestEventLoopRecoversStringPanic(t *testing.T) {
	if ThrowPanic {
		t.Skip("panic propagation is enabled")
	}
	var job Work
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	var child Task
	job.AddTask(&child)
	if err := child.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	job.Call(func() { panic("string panic") })
	if err := waitForTaskStopped(t, &job); !errors.Is(err, ErrPanic) {
		t.Fatalf("expected ErrPanic, got %v", err)
	}
}

func waitForAttempts(t *testing.T, attempts func() int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for attempts() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
}
