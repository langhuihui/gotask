package task

import (
	"runtime"
	"testing"
)

// Test_EventLoopRestartOverlap reproduces a data race between an event loop
// goroutine that is shutting down and the next one that replaces it.
//
// run() clears the running flag from *inside* the loop body:
//
//	if len(ch) == 0 && len(e.children) == 0 {
//	    if e.running.CompareAndSwap(true, false) { ... return }
//	}
//
// but it keeps writing shared loop state *after* that point — the deferred
// teardown ends with `mt.blocked = nil`. Meanwhile active() only needs to see
// running==false to spawn a replacement run(), and that replacement immediately
// writes `e.cases = []reflect.SelectCase{...}` and `mt.blocked = nil` too.
//
// So between the CompareAndSwap and the end of the defer, two goroutines own the
// same loop:
//
//	old: event_loop.go  mt.blocked = nil          (deferred teardown)
//	new: event_loop.go  e.cases = ...             (run() prologue)
//	new: event_loop.go  mt.blocked = nil          (loop head)
//
// The test drives a Work (keepalive, so the loop may idle out and be restarted
// without the job dying) through repeated add/stop cycles, and re-activates the
// loop the moment the flag goes false. It does not reach into internals to do
// so: EventLoopRunning() is exported API, and add() itself probes the very same
// flag via `!e.running.Load()`.
//
// Run with -race. Before the fix this reports a race within a few iterations.
func Test_EventLoopRestartOverlap(t *testing.T) {
	var w Work
	root.AddTask(&w)
	if err := w.WaitStarted(); err != nil {
		t.Fatalf("work failed to start: %v", err)
	}
	defer func() {
		w.Stop(ErrStopByUser)
		_ = w.WaitStopped()
	}()

	for i := 0; i < 300; i++ {
		child := &Task{}
		w.AddTask(child)
		if err := child.WaitStarted(); err != nil {
			t.Fatalf("iter %d: child failed to start: %v", i, err)
		}

		// Dropping the last child lets the loop fall through to its idle check.
		child.Stop(ErrStopByUser)
		_ = child.WaitStopped()

		// Spin until the loop has released the flag. At that instant the old
		// goroutine has returned from the for-loop but its deferred teardown is
		// still running, so the next AddTask below starts a second goroutine on
		// top of it.
		for w.EventLoopRunning() {
			runtime.Gosched()
		}
	}
}
