package task

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// v2 大版本的契约：把「断代」必须守住的行为钉死。序号续 Phase 4 的 C24，
// 设计见 docs/重构-执行模型v2-大版本v2设计.md。
//
// 这批契约的定位与 Phase 1–4 不同：Phase 1–4 锁的是**执行语义**（谁来跑、谁先停、
// 重试几次），v2 锁的是**API 形状收口后的安全网**——
//   - C25/C26：把两处裸类型断言的 panic 变成可诊断的启动失败（V-E）；
//   - C27：把「只实现 Go()」的 v1 任务从静默空转改为显式失败（V-A 迁移守卫）；
//   - C28/C29：证明删掉 uintptr 身份与 Task.GetKey() 不会抽掉脚下的地板
//     （ID 全生命周期稳定、集合 key 来自 handler 而非 Task），是 V-B 的前置条件。
//
// C25–C27 在实施前必须是失败的（契约先行）；C28/C29 是回归锁，实施前就应为绿。

// ---------- 辅助任务类型 ----------

// legacyGoOnlyTask 只实现 v1 的 TaskGo，不实现 TaskRun。
// v2 删除 TaskGo 后，它必须**立刻失败**而不是空转。
type legacyGoOnlyTask struct {
	Task
	executed atomic.Int32
}

func (t *legacyGoOnlyTask) Go() error {
	t.executed.Add(1)
	return nil
}

// retryIDTask 记录每一轮 attempt 看到的任务 ID。
type retryIDTask struct {
	Task
	mu       sync.Mutex
	ids      []uint32
	attempts int
}

func (t *retryIDTask) Run() error {
	t.mu.Lock()
	t.ids = append(t.ids, t.GetTaskID())
	t.attempts++
	n := t.attempts
	t.mu.Unlock()
	if n < 3 {
		return errors.New("fail on purpose")
	}
	return nil
}

func (t *retryIDTask) snapshot() []uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]uint32(nil), t.ids...)
}

// collectionKeyTask 的 GetKey 返回 string，遮蔽 Task.GetKey() 的 uint32。
type collectionKeyTask struct {
	Task
	name string
}

func (t *collectionKeyTask) GetKey() string { return t.name }

// ---------- 契约 25：TickTask 未绑定 handler 时返回错误而非 panic【V-E】 ----------

// v2 之前：TickTask.Start 直接 t.handler.(ITickTask)，handler 为 nil 时在调用方
// goroutine 上 panic（手动在 AddTask 之前调用是最常见的触发方式）。v2 起返回
// ErrNotTickTask，让错误走启动失败路径（可记录、可重试、可升级）。
func TestContract_TickTaskUnboundHandlerFails(t *testing.T) {
	tt := &TickTask{}

	var (
		err      error
		panicked any
	)
	func() {
		defer func() { panicked = recover() }()
		err = tt.Start()
	}()

	if panicked != nil {
		t.Fatalf("TickTask.Start panicked on unbound handler: %v (want ErrNotTickTask)", panicked)
	}
	if !errors.Is(err, ErrNotTickTask) {
		t.Fatalf("TickTask.Start error = %v, want ErrNotTickTask", err)
	}
}

// ---------- 契约 26：AsyncTickTask 未绑定 handler 时返回错误而非 panic【V-E】 ----------

func TestContract_AsyncTickTaskUnboundHandlerFails(t *testing.T) {
	at := &AsyncTickTask{}

	var (
		err      error
		panicked any
	)
	func() {
		defer func() { panicked = recover() }()
		err = at.Run()
	}()

	if panicked != nil {
		t.Fatalf("AsyncTickTask execution entry panicked on unbound handler: %v (want ErrNotTickTask)", panicked)
	}
	if !errors.Is(err, ErrNotTickTask) {
		t.Fatalf("AsyncTickTask execution entry error = %v, want ErrNotTickTask", err)
	}
}

// ---------- 契约 27：只实现 Go() 的 v1 任务必须显式失败【V-A 迁移守卫】 ----------

// 这是 v2 最重要的一道安全网。删掉 TaskGo 之后，`Go() error` 这个方法名仍然存在，
// 只是不再被识别——于是 v1 的 Go 任务会「编译通过、启动成功、永远不执行」，比编译
// 错误隐蔽得多。v2 起由 start() 的迁移守卫直接以 ErrLegacyTaskGo 失败。
func TestContract_LegacyGoOnlyTaskFailsFast(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}

	legacy := &legacyGoOnlyTask{}
	jt := job.AddTask(legacy)

	err := jt.WaitStopped()
	if !errors.Is(err, ErrLegacyTaskGo) {
		t.Fatalf("legacy Go-only task stop reason = %v, want ErrLegacyTaskGo", err)
	}
	if n := legacy.executed.Load(); n != 0 {
		t.Fatalf("legacy Go() executed %d times, want 0 (v2 must not schedule it)", n)
	}
	if s := jt.GetState(); s != TASK_STATE_DISPOSED {
		t.Fatalf("legacy task state = %v, want DISPOSED", s)
	}
	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
}

// ---------- 契约 28：任务 ID 在整个生命周期内稳定【V-B 前置条件】 ----------

// v2 删掉 FromPointer/GetTaskPointer 之后，GetTaskID() 是唯一的跨 goroutine、
// 跨重试、可序列化的任务身份。这条契约保证「删 uintptr 身份」不会抽掉地板：
// 重试的每一轮 attempt（reset 之后）看到的 ID 必须与最初 AddTask 返回的一致。
func TestContract_TaskIDStableAcrossRetries(t *testing.T) {
	var job Job
	root.AddTask(&job)
	if err := job.WaitStarted(); err != nil {
		t.Fatal(err)
	}

	task := &retryIDTask{}
	jt := job.AddTask(task, RetryConfig{MaxRetry: 5, RetryInterval: time.Millisecond})
	want := jt.GetTaskID()

	// 不能用 WaitStopped() 的返回值判断最终结果：重试任务的 stopErr 在第一次
	// 失败时就已落定（Phase 1 契约 C9 踩过的同一个坑），返回的是首轮原因。
	waitUntil(t, 5*time.Second, func() bool {
		return len(task.snapshot()) == 3 && jt.GetState() == TASK_STATE_DISPOSED
	}, "task did not finish after 3 attempts")

	ids := task.snapshot()
	if len(ids) != 3 {
		t.Fatalf("observed %d attempts, want 3", len(ids))
	}
	for i, id := range ids {
		if id != want {
			t.Fatalf("attempt %d saw task ID %d, want %d (ID must be stable across retries)", i+1, id, want)
		}
	}
	if got := jt.GetTaskID(); got != want {
		t.Fatalf("GetTaskID after dispose = %d, want %d", got, want)
	}
	job.Stop(ErrStopByUser)
	_ = job.WaitStopped()
}

// ---------- 契约 29：WorkCollection 的 key 来自 handler，与 Task.GetKey 无关【V-B 前置条件】 ----------

// v2 删掉 Task.GetKey() uint32（F49：同名不同义）时，集合语义必须原地不动。
// 集合的 key 是 handler 自己实现的 GetKey() K（经反射的 getKey()），而不是
// Task.GetKey() 那个返回 ID 的 uint32。
func TestContract_WorkCollectionKeyIndependentOfTaskGetKey(t *testing.T) {
	collection := &WorkCollection[string, *collectionKeyTask]{}
	root.AddTask(collection)

	a := &collectionKeyTask{name: "alpha"}
	b := &collectionKeyTask{name: "beta"}
	collection.AddTask(a)
	collection.AddTask(b)

	waitUntil(t, 2*time.Second, func() bool {
		return collection.Has("alpha") && collection.Has("beta")
	}, "collection did not index children by handler GetKey()")

	if got, ok := collection.Get("alpha"); !ok || got != a {
		t.Fatalf("collection.Get(\"alpha\") = %v, %v; want the alpha task", got, ok)
	}
	if _, ok := collection.Get("beta"); !ok {
		t.Fatal("collection.Get(\"beta\") missing")
	}
	if n := collection.Length(); n != 2 {
		t.Fatalf("collection length = %d, want 2", n)
	}

	collection.Stop(ErrStopByUser)
	_ = collection.WaitStopped()
}
