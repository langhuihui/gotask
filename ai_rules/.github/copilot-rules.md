# GitHub Copilot Rules for GoTask Project

## Project Context
GoTask is an asynchronous task management framework for Go that provides OS-like task manager capabilities. The core principle is "Everything is a Task" - all business logic should be abstracted as manageable task units.

## Key Architecture Rules

### 1. Single Goroutine Event Loop (CRITICAL)
- **NEVER** create goroutines directly in task implementations
- All child tasks must execute in the parent task's goroutine
- Use `parent.AddTask(child)` instead of `go func()`
- EventLoop handles sequential execution automatically

### 2. Task Lifecycle Management
- Always implement `Start()`, `Run()`, and `Dispose()` methods
- Use `RootManager` as the root task manager
- Never call `Start()` directly - it must be called by the parent task
- Implement proper resource cleanup in `Dispose()`

### 3. Resource Management Patterns
```go
// Correct resource management
func (t *MyTask) Start() error {
    t.Using(resource1, resource2)  // Add dependencies
    
    // OnStop: 用于关闭阻塞性资源（如端口监听、网络连接）
    t.OnStop(func() { 
        server.Close()  // 关闭阻塞性资源
    })
    
    // OnDispose: 用于清理非阻塞性资源
    t.OnDispose(func() {
        cache.Flush()  // 清理其他资源
    })
    return nil
}

func (t *MyTask) Dispose() {
    // 清理非阻塞性资源
    if t.conn != nil {
        t.conn.Close()
    }
}
```

### Task Execution Model
- **Sequential Execution**: 子任务在父任务协程中顺序执行
- **Blocking Behavior**: 子任务的Start或Run长时间运行会阻塞其他子任务
- **Use Cases**: 适合单个子任务重试、子任务排队执行
- **Not Suitable**: 不适合多个子任务并行处理
- **Stop Method**: Stop()不能传入nil，必须提供停止原因

## Task Type Selection Guide

### Use `task.Task` for:
- Simple, single-purpose tasks
- Basic task implementations

### Use `task.Job` for:
- Task containers that manage child tasks
- Coordinators that end when children complete

### Use `task.Work` for:
- Background workers that continue after children complete
- Long-running services

### Use `task.TickTask` for:
- Periodic tasks (timers, heartbeats, cleanup)
- Implement `GetTickInterval() time.Duration`

### Use `task.ChannelTask` for:
- Event-driven tasks
- Custom signal-based tasks
- Override `GetSignal()` method

## Code Generation Patterns

### When generating task implementations:
1. Always embed the appropriate task type
2. Include proper lifecycle methods
3. Add structured logging with task context
4. Implement error handling and retry logic
5. Use task's built-in methods for state management

### Example task template:
```go
type ExampleTask struct {
    task.Task  // Choose appropriate type
    // Add your fields
    Name string
    Config map[string]interface{}
}

func (t *ExampleTask) Start() error {
    t.Info("Task starting", "name", t.Name)
    // Initialize resources
    return nil
}

func (t *ExampleTask) Run() error {
    // Main task logic
    for !t.IsStopped() {
        // Do work
    }
    return nil
}

func (t *ExampleTask) Dispose() {
    t.Info("Task disposing", "name", t.Name)
    // Cleanup resources
}
```

## Error Handling Guidelines

### Panic vs Error Handling:
- Use `go build -tags taskpanic` for development (panics throw)
- Default build captures panics and converts to errors
- Always handle errors gracefully in `Run()` methods

### Retry Configuration:
```go
func (t *MyTask) Start() error {
    t.SetRetry(3, 5*time.Second)  // maxRetry, interval
    return nil
}
```

## Dashboard Integration

### Backend (dashboard/server):
- Go-based management service
- Uses GoTask for HTTP server lifecycle
- Provides RESTful APIs for task monitoring

### Frontend (dashboard/web):
- React + TypeScript interface
- Real-time task monitoring
- Task tree visualization

## Common Patterns to Generate

### 1. Network Service Tasks:
```go
type NetworkService struct {
    task.Job
    Port int
    Server *http.Server
}

func (n *NetworkService) Start() error {
    n.Server = &http.Server{Addr: fmt.Sprintf(":%d", n.Port)}
    n.OnStop(func() { n.Server.Close() })
    return nil
}
```

### 2. Periodic Cleanup Tasks:
```go
type CleanupTask struct {
    task.TickTask
    Interval time.Duration
}

func (c *CleanupTask) GetTickInterval() time.Duration {
    return c.Interval
}

func (c *CleanupTask) Run() error {
    // Cleanup logic
    return nil
}
```

### 3. Data Processing Tasks:
```go
type DataProcessor struct {
    task.Task
    Input  chan Data
    Output chan ProcessedData
}

func (d *DataProcessor) Run() error {
    for data := range d.Input {
        if d.IsStopped() {
            break
        }
        processed := d.process(data)
        select {
        case d.Output <- processed:
        case <-d.Context().Done():
            return d.Context().Err()
        }
    }
    return nil
}
```

## Anti-Patterns to Avoid

1. **Direct goroutine creation in tasks**
2. **Calling Start() directly**
3. **Ignoring resource cleanup**
4. **Using global variables for task state**
5. **Blocking operations without checking IsStopped()**

## Build and Dependencies

- Go 1.23+ required
- Use `go mod tidy` for dependency management
- Dashboard frontend uses pnpm
- Support conditional compilation with `taskpanic` tag

## Logging and Monitoring

- Use task's built-in logging: `Info()`, `Error()`, `Debug()`, `Warn()`
- Include task ID and context in log messages
- Framework automatically measures execution time
- Use `GetDuration()` for performance metrics

Remember: GoTask provides predictable execution, proper resource management, and comprehensive observability. Always follow the single-goroutine principle and implement proper lifecycle methods.
