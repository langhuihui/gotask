package task

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func Test_GenericTaskPipeline(t *testing.T) {
	pipeline := Chain(
		Then(
			Map(
				TaskFunc[int](func(context.Context) (int, error) {
					return 21, nil
				}),
				func(value int) (int, error) {
					return value * 2, nil
				},
			),
			func(_ context.Context, value int) (string, error) {
				return fmt.Sprintf("value:%d", value), nil
			},
		),
		func(value string) TaskFunc[int] {
			return func(context.Context) (int, error) {
				return len(value), nil
			}
		},
	)

	got, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}
	if got != len("value:42") {
		t.Fatalf("expected %d, got %d", len("value:42"), got)
	}
}

func Test_GenericParallelPreservesOrder(t *testing.T) {
	results, err := Parallel(
		context.Background(),
		TaskFunc[int](func(context.Context) (int, error) {
			time.Sleep(20 * time.Millisecond)
			return 1, nil
		}),
		TaskFunc[int](func(context.Context) (int, error) {
			time.Sleep(5 * time.Millisecond)
			return 2, nil
		}),
	)
	if err != nil {
		t.Fatalf("parallel failed: %v", err)
	}
	if len(results) != 2 || results[0] != 1 || results[1] != 2 {
		t.Fatalf("unexpected ordered results: %#v", results)
	}
}

func Test_GenericParallelMapCollectsTypedResults(t *testing.T) {
	results, err := ParallelMap(context.Background(), map[string]TaskFunc[int]{
		"alpha": func(context.Context) (int, error) { return 1, nil },
		"beta":  func(context.Context) (int, error) { return 2, nil },
	})
	if err != nil {
		t.Fatalf("parallel map failed: %v", err)
	}
	if len(results) != 2 || results["alpha"] != 1 || results["beta"] != 2 {
		t.Fatalf("unexpected map results: %#v", results)
	}
}

func Test_GenericParallelPropagatesErrors(t *testing.T) {
	targetErr := errors.New("boom")
	_, err := Parallel(
		context.Background(),
		TaskFunc[int](func(ctx context.Context) (int, error) {
			<-ctx.Done()
			return 0, context.Cause(ctx)
		}),
		TaskFunc[int](func(context.Context) (int, error) {
			return 0, targetErr
		}),
	)
	if !errors.Is(err, targetErr) {
		t.Fatalf("expected joined error to contain %v, got %v", targetErr, err)
	}
}

func Test_GenericWithRetryAndTimeout(t *testing.T) {
	var attempts atomic.Int32
	retried := WithRetry(func(context.Context) (int, error) {
		if attempts.Add(1) < 3 {
			return 0, errors.New("transient")
		}
		return 42, nil
	}, RetryPolicy{
		MaxRetry:      2,
		RetryInterval: time.Millisecond,
	})

	got, err := retried.Run(context.Background())
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}

	timedOut := WithTimeout(func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, context.Cause(ctx)
	}, 10*time.Millisecond)

	_, err = timedOut.Run(context.Background())
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func Test_GenericCompatibilityAdapters(t *testing.T) {
	typed := TaskFunc[int](func(context.Context) (int, error) { return 7, nil })
	legacy := AsAnyTaskFunc(typed)
	adapted := AdaptAnyTaskFunc(legacy)

	value, err := adapted.Run(context.Background())
	if err != nil {
		t.Fatalf("adapter failed: %v", err)
	}
	got, ok := value.(int)
	if !ok || got != 7 {
		t.Fatalf("unexpected adapted value: %#v", value)
	}
}
