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
	if errors.Is(err, ErrTaskComplete) || errors.Is(err, ErrExit) || errors.Is(err, ErrStopByUser) || errors.Is(err, ErrTimeout) {
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
	delay := policy.RetryInterval
	if retries > 1 && policy.RetryInterval > 0 {
		exponent := retries - 1
		if exponent < 30 {
			delay = policy.RetryInterval * time.Duration(1<<exponent)
		} else {
			delay = policy.RetryInterval * time.Duration(1<<30)
		}
	}
	if policy.MaxRetryInterval > 0 && delay > policy.MaxRetryInterval {
		return policy.MaxRetryInterval
	}
	return delay
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
