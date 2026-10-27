// 泛型编排层：纯函数组合（Then/Map/Chain/Parallel/ParallelMap/WithRetry/
// WithTimeout）。它**不涉及生命周期**——没有父子、没有事件循环、没有
// Start/Stop/Dispose 钩子、没有任务树里的可观测状态；所有中断都靠传入的
// context，所有返回值都是 (T, error)。
//
// 与任务树的分工（Phase 4 设计 §3.3）：
//
//   - 需要「谁在跑、跑了几轮、失败后怎么退避、停止时谁先谁后」用任务树
//     （Task/Job）：重试退避在循环外、可取消、有计数与描述可观测；
//   - 需要在一次调用内组合若干函数并汇聚结果，用泛型编排；
//   - 两者用 AsTaskBlock 单向桥接：函数进任务树；反向适配刻意不提供。
//
// 共享的重试语义：终态原因判定（isTerminalStopReason）与指数退避公式
// （retryDelayFor）由两层共用，避免同一个配置算出两种退避。
package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// TaskFunc is the core generic task function type for typed orchestration.
type TaskFunc[T any] func(context.Context) (T, error)

// Run executes the task function.
func (fn TaskFunc[T]) Run(ctx context.Context) (T, error) {
	return runGeneric(ctx, fn)
}

// TaskRunner is the generic equivalent of a runnable task abstraction.
type TaskRunner[T any] interface {
	Run(context.Context) (T, error)
}

// Result stores a typed task result and its error.
type Result[T any] struct {
	Value T
	Err   error
}

// RetryPolicy configures the generic retry decorator. MaxRetry matches the
// existing task retry semantics: 0 disables retries, positive values allow that
// many retries after the initial attempt, and negative values retry without a
// fixed limit. The first retry waits for RetryInterval, and each later retry
// doubles the delay up to MaxRetryInterval.
type RetryPolicy struct {
	MaxRetry         int
	RetryInterval    time.Duration
	MaxRetryInterval time.Duration
	ShouldRetry      func(error) bool
}

// AnyTaskFunc is a compatibility adapter for legacy any-based task functions.
//
// Deprecated: use TaskFunc[T] instead.
type AnyTaskFunc func(context.Context) (any, error)

// Run executes the compatibility task function.
func (fn AnyTaskFunc) Run(ctx context.Context) (any, error) {
	return runGeneric(ctx, TaskFunc[any](fn))
}

// Run executes a typed task runner.
func Run[T any](ctx context.Context, task TaskRunner[T]) (T, error) {
	return runGeneric(ctx, task.Run)
}

// Then chains a downstream task that consumes the previous typed result.
func Then[A, B any](task TaskFunc[A], next func(context.Context, A) (B, error)) TaskFunc[B] {
	return func(ctx context.Context) (B, error) {
		value, err := task.Run(ctx)
		if err != nil {
			var zero B
			return zero, err
		}
		return runGeneric(ctx, TaskFunc[B](func(ctx context.Context) (B, error) {
			return next(ctx, value)
		}))
	}
}

// Map transforms a typed task result without exposing any.
func Map[A, B any](task TaskFunc[A], mapper func(A) (B, error)) TaskFunc[B] {
	return Then(task, func(_ context.Context, value A) (B, error) {
		return mapper(value)
	})
}

// Chain selects the next typed task from the previous typed result.
func Chain[A, B any](task TaskFunc[A], next func(A) TaskFunc[B]) TaskFunc[B] {
	return func(ctx context.Context) (B, error) {
		value, err := task.Run(ctx)
		if err != nil {
			var zero B
			return zero, err
		}
		return next(value).Run(ctx)
	}
}

// Parallel runs typed tasks concurrently, preserves input order, and returns
// joined errors when one or more tasks fail. The derived context is canceled on
// the first error to help sibling tasks stop early.
func Parallel[T any](ctx context.Context, tasks ...TaskFunc[T]) ([]T, error) {
	results := make([]T, len(tasks))
	if len(tasks) == 0 {
		return results, nil
	}
	derivedCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	wg.Add(len(tasks))
	for i, task := range tasks {
		go func(index int, task TaskFunc[T]) {
			defer wg.Done()
			value, err := task.Run(derivedCtx)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				if len(errs) == 1 {
					cancel(err)
				}
				mu.Unlock()
				return
			}
			results[index] = value
		}(i, task)
	}
	wg.Wait()
	return results, errors.Join(errs...)
}

// ParallelMap runs typed tasks concurrently and collects results in a typed map.
func ParallelMap[K comparable, V any](ctx context.Context, tasks map[K]TaskFunc[V]) (map[K]V, error) {
	results := make(map[K]V, len(tasks))
	if len(tasks) == 0 {
		return results, nil
	}
	derivedCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	wg.Add(len(tasks))
	for key, task := range tasks {
		go func(key K, task TaskFunc[V]) {
			defer wg.Done()
			value, err := task.Run(derivedCtx)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				if len(errs) == 1 {
					cancel(err)
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			results[key] = value
			mu.Unlock()
		}(key, task)
	}
	wg.Wait()
	return results, errors.Join(errs...)
}

// WithRetry wraps a typed task with the generic retry policy.
func WithRetry[T any](task TaskFunc[T], policy RetryPolicy) TaskFunc[T] {
	return func(ctx context.Context) (T, error) {
		var retries int
		for {
			value, err := task.Run(ctx)
			if err == nil {
				return value, nil
			}
			if !shouldRetryGeneric(ctx, err, policy, retries) {
				var zero T
				return zero, err
			}
			retries++
			if err := sleepContext(ctx, genericRetryDelay(retries, policy)); err != nil {
				var zero T
				cause := context.Cause(ctx)
				if cause != nil {
					return zero, cause
				}
				return zero, err
			}
		}
	}
}

// WithTimeout wraps a typed task with a timeout. The wrapped task receives a
// child context carrying ErrTimeout as its cancellation cause. Like other
// context-based timeouts in Go, this stops waiting when the deadline expires
// but cannot forcibly terminate a task that ignores context cancellation.
func WithTimeout[T any](task TaskFunc[T], timeout time.Duration) TaskFunc[T] {
	return func(ctx context.Context) (T, error) {
		if timeout <= 0 {
			return task.Run(ctx)
		}
		derivedCtx, cancel := context.WithTimeoutCause(ctx, timeout, ErrTimeout)
		defer cancel()

		resultCh := make(chan Result[T], 1)
		go func() {
			value, err := task.Run(derivedCtx)
			resultCh <- Result[T]{Value: value, Err: err}
		}()

		select {
		case result := <-resultCh:
			return result.Value, result.Err
		case <-derivedCtx.Done():
			select {
			case result := <-resultCh:
				return result.Value, result.Err
			default:
			}
			var zero T
			if cause := context.Cause(derivedCtx); cause != nil {
				return zero, cause
			}
			return zero, derivedCtx.Err()
		}
	}
}

// AsTaskFunc adapts a generic runner to TaskFunc.
func AsTaskFunc[T any](task TaskRunner[T]) TaskFunc[T] {
	return task.Run
}

// AdaptAnyTaskFunc adapts a legacy any-based function to TaskFunc[any].
//
// Deprecated: use TaskFunc[T] directly.
func AdaptAnyTaskFunc(task AnyTaskFunc) TaskFunc[any] {
	return TaskFunc[any](task)
}

// AsAnyTaskFunc adapts a typed task to a legacy any-based function.
//
// Deprecated: use TaskFunc[T] directly.
func AsAnyTaskFunc[T any](task TaskFunc[T]) AnyTaskFunc {
	return func(ctx context.Context) (any, error) {
		return task.Run(ctx)
	}
}

func runGeneric[T any](ctx context.Context, fn TaskFunc[T]) (value T, err error) {
	if ThrowPanic {
		return fn(ctx)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.Join(fmt.Errorf("%v", recovered), ErrPanic)
		}
	}()
	return fn(ctx)
}

func shouldRetryGeneric(ctx context.Context, err error, policy RetryPolicy, retries int) bool {
	if err == nil {
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	if isTerminalStopReason(err) {
		return false
	}
	if policy.MaxRetry >= 0 && retries >= policy.MaxRetry {
		return false
	}
	if policy.ShouldRetry != nil {
		return policy.ShouldRetry(err)
	}
	return true
}

func genericRetryDelay(retries int, policy RetryPolicy) time.Duration {
	return retryDelayFor(retries, policy.RetryInterval, policy.MaxRetryInterval)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ---------- 与任务树的桥接（Phase 4 / F30） ----------

// GenericTask 是一个把泛型函数挂进任务树的任务：它内嵌 Task（于是满足 ITask
// 的全部要求），同时实现 TaskBlock，`Job.AddTask`/`Job.RunTask` 可以直接接收它。
//
// 生命周期语义与手写的 TaskBlock 完全一致：Start 成功后 Run 在独立 goroutine
// 上执行（Phase 2），函数返回 nil 视为本轮完成（ErrTaskComplete），返回错误即
// 本轮停止原因；父任务停止时 context 被取消，函数应当尊重它——这也是泛型
// 函数唯一的「被中断」通道（Go 无法强杀 goroutine）。
type GenericTask struct {
	Task
	fn func(context.Context) error
}

// Run 执行被包装的泛型函数，context 取本轮 attempt 的 Task.Context。
func (g *GenericTask) Run() error {
	return g.fn(g.Context)
}

// AsTaskBlock 把泛型函数包装成可挂进任务树的 TaskBlock。
//
// 返回值是 *GenericTask（而不只是 TaskBlock 接口）：调用方拿到的对象既能在
// 任务树里用 SetRetry/SetDisposeTimeout 等生命周期配置，也能在外部观察状态。
//
// 典型用法：
//
//	blk := task.AsTaskBlock(task.TaskFunc[int](func(ctx context.Context) (int, error) {
//		return fetch(ctx) // 尊重 ctx，父任务停止时才停得下来
//	}))
//	blk.SetRetry(3, time.Second)
//	job.AddTask(blk)
//
// 反向适配（Task → TaskFunc）刻意不提供：那会把整个生命周期塞进一个闭包，
// 让「函数编排」看起来像能管理子任务，收益远低于代价。
func AsTaskBlock[T any](fn TaskFunc[T]) *GenericTask {
	if fn == nil {
		return &GenericTask{fn: func(context.Context) error { return ErrTaskComplete }}
	}
	return &GenericTask{fn: func(ctx context.Context) error {
		// runGeneric 负责 panic 包装（ThrowPanic=false 时并上 ErrPanic）。
		_, err := runGeneric(ctx, fn)
		return err
	}}
}
