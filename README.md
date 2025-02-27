# GoTask

English | [中文](README_CN.md)

GoTask is a powerful task management system for Go, providing flexible task orchestration, lifecycle management, and error handling mechanisms. It is particularly suitable for managing complex asynchronous workflows, scheduled tasks, and task groups with dependencies.

## Features

- Task lifecycle management (init, start, run, stop, dispose)
- Task dependency support
- Automatic retry mechanism
- Graceful shutdown support
- Scheduled task support
- Event-driven task processing
- Rich logging and debugging capabilities
- Support for nested task groups (Jobs)

## Installation

```bash
go get github.com/langhuihui/gotask
```

## Quick Start

### 1. Basic Task

Create a simple task:

```go
type MyTask struct {
	task.Task
	Name string
}

func (t *MyTask) Run() error {
	fmt.Printf("Task %s is running\n", t.Name)
	time.Sleep(time.Second)
	fmt.Printf("Task %s completed\n", t.Name)
	return nil
}
```

### 2. Scheduled Task

Create a task that executes periodically:

```go
type MyTickTask struct {
	task.TickTask
	Count int
}

func (t *MyTickTask) Tick(any) {
	t.Count--
	if t.Count <= 0 {
		t.Stop(task.ErrTaskComplete)
	}
}

func (t *MyTickTask) GetTickInterval() time.Duration {
	return time.Second / 2
}
```

### 3. Using Root Task Manager

```go
var root task.RootTask

func init() {
	root.Init()
}

func main() {
	// Add a basic task with retry
	taskWithRetry := &MyTask{Name: "RetryTask"}
	task1 := root.AddTask(taskWithRetry)
	task1.SetRetry(3, time.Second)

	// Add a scheduled task
	tickerTask := &MyTickTask{Count: 5}
	tickerTaskRef := root.AddTask(tickerTask)

	// Add a dependent task
	dependentTask := &MyTask{Name: "DependentTask"}
	task2 := root.AddTask(dependentTask)
	task2.Depend(tickerTaskRef)
}
```

## Advanced Features

### Task Lifecycle

Each task has the following lifecycle states:
- TASK_STATE_INIT
- TASK_STATE_STARTING
- TASK_STATE_STARTED
- TASK_STATE_RUNNING
- TASK_STATE_GOING
- TASK_STATE_DISPOSING
- TASK_STATE_DISPOSED

### Event Listening

Listen to various task events:

```go
task.OnStart(func() {
	logger.Info("Task started")
})

task.OnDispose(func() {
	logger.Info("Task disposed")
})
```

### Error Handling

The system defines several error types:
- ErrAutoStop
- ErrRetryRunOut
- ErrStopByUser
- ErrRestart
- ErrTaskComplete
- ErrExit
- ErrPanic

### Task Dependencies

Tasks can depend on other tasks:

```go
task2.Depend(task1) // task2 will start after task1 completes
```

### Graceful Shutdown

Support for graceful shutdown via system signals:

```go
go func() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-signalChan
	root.Stop(task.ErrExit)
}()
```

## Best Practices

1. Use `RootTask` as the top-level task manager
2. Implement graceful shutdown for long-running tasks
3. Set appropriate retry strategies
4. Use dependencies to manage task execution order
5. Implement proper error handling and logging

## Examples

Complete example code can be found in the `examples/task` directory.

## License

MIT License

## Contributing

Issues and Pull Requests are welcome! 