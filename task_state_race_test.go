package task

import (
	"testing"
)

// Test_TaskStateRace reproduces a data race on the unsynchronized Task.state
// field.
//
// Before this fix state was a plain byte field, written without any
// synchronization (start(), dispose(), RootManager.Init) and read through
// GetState(), which was a plain field read. The two ends live on different
// goroutines:
//
//	write: task.go  task.setState(TASK_STATE_DISPOSING)
//	         <- child.dispose(), called by the *job's own* event loop goroutine
//	            via onChildDispose()
//	read:  job.go   "childState", child.GetState()
//	         <- job.waitChildrenDispose(), called by the *parent's* event loop
//	            goroutine while it disposes the job
//
// Note the read sites are Debug() arguments. Go evaluates call arguments eagerly
// and Debug is an ordinary variadic method, so GetState() executes on every
// teardown regardless of the configured log level — turning logging down does
// not avoid the race.
//
// The same unsynchronized read also feeds real decisions, not just logging:
//
//	job.go   AddTask: existingTask.GetState() >= TASK_STATE_DISPOSING
//	work.go  WorkCollection.active(), reached from Get/Has/Range/ToList
//
// This test stops a job that still owns children, which is the ordinary
// shutdown path, so no artificial construction is needed.
//
// Run with -race. Before the fix this reports a race within a few iterations.
func Test_TaskStateRace(t *testing.T) {
	for i := 0; i < 50; i++ {
		var job Job
		root.AddTask(&job)
		if err := job.WaitStarted(); err != nil {
			t.Fatalf("iter %d: job failed to start: %v", i, err)
		}

		children := make([]*Task, 8)
		for j := range children {
			children[j] = &Task{}
			job.AddTask(children[j])
		}
		for j, c := range children {
			if err := c.WaitStarted(); err != nil {
				t.Fatalf("iter %d: child %d failed to start: %v", i, j, err)
			}
		}

		// The parent event loop disposes the job and walks its children
		// (reading state) while the job's own event loop disposes those same
		// children (writing state).
		job.Stop(ErrStopByUser)
		_ = job.WaitStopped()
	}
}
