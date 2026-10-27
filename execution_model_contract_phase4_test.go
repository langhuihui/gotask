package task

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Phase 4（外围渐进增强）的契约：把 4C「泛型与任务树的桥接/退避一致」与 4D
//「类型化 option 与裸值等价」的语义钉死。既有 21 条契约在
// execution_model_contract_test.go。

// ---------- 契约 22：泛型函数可挂进任务树【4C】 ----------
//
// 泛型编排层此前与任务树零耦合（全仓库无调用者），两套 API 是两个平行世界。
// AsTaskBlock 提供单向桥接；桥接后的函数必须遵守任务树的语义——Run 不占用父
// 循环（C1 / Phase 2）、拿到本轮 attempt 的 context、返回 nil 记为完成
// （ErrTaskComplete）、返回错误即停止原因且可重试（C9/C11）。
func TestContract_AsTaskBlockRunsGenericFunction(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}

	var (
		received  context.Context
		receivedC = make(chan struct{})
		release   = make(chan struct{})
	)
	block := AsTaskBlock(TaskFunc[int](func(ctx context.Context) (int, error) {
		received = ctx
		close(receivedC)
		<-release
		return 42, nil
	}))
	job.AddTask(block)
	<-receivedC

	// 函数运行期间父循环仍响应（桥接后它就是普通的 TaskRun）。
	select {
	case <-callWithin(&job):
	case <-time.After(2 * time.Second):
		t.Fatal("job.Call blocked while a generic function was running")
	}
	// 函数拿到的是任务本轮的 context，不是 context.Background()。
	if received == nil || received != block.Context {
		t.Fatalf("generic function did not receive the attempt context: %v", received)
	}

	close(release)
	waitUntil(t, 3*time.Second, func() bool {
		return block.GetState() == TASK_STATE_DISPOSED
	}, "generic task did not complete")
	if err := block.StopReason(); !errors.Is(err, ErrTaskComplete) {
		t.Fatalf("expected ErrTaskComplete after a nil-returning function, got %v", err)
	}

	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
}

// 桥接后的失败必须进入任务树的重试链路（而不是把错误吞掉）。
func TestContract_AsTaskBlockFailureIsRetried(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}

	var attempts atomic.Int32
	block := AsTaskBlock(TaskFunc[int](func(ctx context.Context) (int, error) {
		if attempts.Add(1) == 1 {
			return 0, errors.New("generic boom")
		}
		return 7, nil
	}))
	block.SetRetry(1, 20*time.Millisecond)
	job.AddTask(block)

	// 不能等 state==DISPOSED：attempt 1 失败时 dispose 已经把它置为 DISPOSED，
	// 而重试还在 20ms 的退避里（C9 踩过同一个坑）。等「第二次 attempt 跑完且
	// 最终原因是完成」。
	waitUntil(t, 3*time.Second, func() bool {
		return attempts.Load() == 2 && errors.Is(block.StopReason(), ErrTaskComplete)
	}, "generic task did not settle after retry")
	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts (failure + retry), got %d (stop reason %v)", got, block.StopReason())
	}
	if err := block.StopReason(); !errors.Is(err, ErrTaskComplete) {
		t.Fatalf("expected ErrTaskComplete after the retry succeeded, got %v", err)
	}

	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
}

// ---------- 契约 23：任务树与泛型的退避一致【4C】 ----------
//
// 两套重试实现曾各写一遍指数退避，上限的边界还不一致：泛型侧始终封顶，任务树只在
// retryCount > 1 时封顶。于是「RetryInterval=2s + MaxRetryInterval=100ms」在任务
// 树上首轮会白等 2s（F31）。本契约把两侧钉在同一份公式上。
func TestContract_RetryDelayCapAppliesToFirstRetry(t *testing.T) {
	const (
		interval    = 2 * time.Second
		maxInterval = 100 * time.Millisecond
	)

	// 公式层：首轮也封顶，且两侧同源。
	if got := retryDelayFor(1, interval, maxInterval); got != maxInterval {
		t.Fatalf("retryDelayFor(first, capped) = %v, want %v", got, maxInterval)
	}
	if got := retryDelayFor(3, 10*time.Millisecond, 0); got != 40*time.Millisecond {
		t.Fatalf("retryDelayFor(3, 10ms, uncapped) = %v, want 40ms", got)
	}
	policy := RetryPolicy{MaxRetry: 1, RetryInterval: interval, MaxRetryInterval: maxInterval}
	if got := genericRetryDelay(1, policy); got != maxInterval {
		t.Fatalf("generic first-retry delay = %v, want %v", got, maxInterval)
	}

	// 行为层：任务树的首轮退避必须被封顶（旧实现会等满 2s）。
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}
	failing := &retryFailTask{}
	failing.SetRetry(1, interval)
	failing.SetMaxRetryInterval(maxInterval)
	start := time.Now()
	job.AddTask(failing)

	waitUntil(t, time.Second, func() bool {
		return failing.attempts.Load() >= 2
	}, "second attempt did not start within the capped backoff (first retry ignored MaxRetryInterval?)")
	if elapsed := time.Since(start); elapsed < maxInterval/2 {
		t.Fatalf("second attempt started after %v, expected about %v", elapsed, maxInterval)
	}
	_ = job.WaitStopped()
}

// ---------- 契约 24：类型化 option 与裸值等价【4D】 ----------
//
// AddTask 的 opt ...any 历史上直接吃裸值（context.Context / *slog.Logger /
// Description / RetryConfig / int）。WithXxx 是它们的具名形式，两者必须产生
// 完全相同的初始化结果；旧写法不得因新 API 出现而改变语义。
func TestContract_TypedOptionsMatchRawValues(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	parentCtx, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	retry := RetryConfig{MaxRetry: 4, RetryInterval: 30 * time.Millisecond}

	raw := &blockingCtxRunTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(raw, parentCtx, logger, Description{"tag": "raw"}, retry)
	typed := &blockingCtxRunTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(typed,
		WithContext(parentCtx),
		WithLogger(logger),
		WithDescription(Description{"tag": "typed"}),
		WithRetryConfig(retry),
	)
	<-raw.entered
	<-typed.entered

	for name, tc := range map[string]*blockingCtxRunTask{"raw": raw, "typed": typed} {
		got := tc.GetTask()
		if got.parentCtx != parentCtx {
			t.Fatalf("%s: parent context not applied", name)
		}
		if got.Logger != logger {
			t.Fatalf("%s: logger not applied", name)
		}
		if got.retry.MaxRetry != retry.MaxRetry || got.retry.RetryInterval != retry.RetryInterval {
			t.Fatalf("%s: retry config not applied: %+v", name, got.retry)
		}
		if tag, ok := got.GetDescription("tag"); !ok || tag == nil {
			t.Fatalf("%s: description not applied", name)
		}
	}

	// WithCallDepth：StartReason 必须指向封装函数的调用方，而不是封装函数自己。
	reason, file := addTaskWithDepth(t, &job)
	if want := strings.TrimPrefix(file, sourceFilePathPrefix); !strings.HasPrefix(reason, want+":") {
		t.Fatalf("WithCallDepth recorded the wrong frame: StartReason=%q, want prefix %q", reason, want+":")
	}

	close(raw.release)
	close(typed.release)
	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
}

// addTaskWithDepth 用 WithCallDepth(1) 越过本函数登记 StartReason，并把调用方
// 的源码位置一并返回（由 runtime.Caller(1) 取）。
func addTaskWithDepth(t *testing.T, job *Job) (startReason, callerFile string) {
	t.Helper()
	_, file, _, _ := runtime.Caller(1)
	created := &blockingCtxRunTask{entered: make(chan struct{}), release: make(chan struct{})}
	job.AddTask(created, WithCallDepth(1))
	<-created.entered
	close(created.release)
	return created.StartReason, file
}
