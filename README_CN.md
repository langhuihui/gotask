# GoTask

[English](README.md) | 中文

GoTask 是一个强大的 Go 语言任务管理系统，提供了灵活的任务编排、生命周期管理和错误处理机制。它特别适合用于管理复杂的异步任务流程、定时任务和具有依赖关系的任务组。

## 特性

- 任务生命周期管理（初始化、启动、运行、停止、销毁）
- 支持任务依赖关系
- 自动重试机制
- 优雅关闭支持
- 定时任务支持
- 事件驱动的任务处理
- 丰富的日志和调试功能
- 支持任务组（Job）嵌套

## 安装

```bash
go get github.com/langhuihui/gotask
```

## 快速开始

### 1. 基本任务

创建一个简单的任务：

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

### 2. 定时任务

创建一个定时执行的任务：

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

### 3. 使用根任务管理器

```go
var root task.RootTask

func init() {
    root.Init()
}

func main() {
    // 添加一个普通任务
    taskWithRetry := &MyTask{Name: "RetryTask"}
    task1 := root.AddTask(taskWithRetry)
    task1.SetRetry(3, time.Second) // 设置重试

    // 添加一个定时任务
    tickerTask := &MyTickTask{Count: 5}
    tickerTaskRef := root.AddTask(tickerTask)

    // 添加一个依赖任务
    dependentTask := &MyTask{Name: "DependentTask"}
    task2 := root.AddTask(dependentTask)
    task2.Depend(tickerTaskRef) // 设置依赖关系
}
```

## 高级特性

### 任务生命周期

每个任务都有以下生命周期状态：
- TASK_STATE_INIT: 初始化状态
- TASK_STATE_STARTING: 正在启动
- TASK_STATE_STARTED: 已启动
- TASK_STATE_RUNNING: 运行中
- TASK_STATE_GOING: 正在进行
- TASK_STATE_DISPOSING: 正在销毁
- TASK_STATE_DISPOSED: 已销毁

### 事件监听

可以监听任务的各种事件：

```go
task.OnStart(func() {
    logger.Info("Task started")
})

task.OnDispose(func() {
    logger.Info("Task disposed")
})
```

### 错误处理

系统定义了多种错误类型：
- ErrAutoStop: 自动停止
- ErrRetryRunOut: 重试次数用尽
- ErrStopByUser: 用户停止
- ErrRestart: 重启
- ErrTaskComplete: 任务完成
- ErrExit: 退出
- ErrPanic: 发生panic

### 任务依赖

任务可以依赖其他任务，被依赖的任务完成后才会启动依赖它的任务：

```go
task2.Depend(task1) // task2 将在 task1 完成后启动
```

### 优雅关闭

系统支持优雅关闭，可以通过系统信号触发：

```go
go func() {
    signalChan := make(chan os.Signal, 1)
    signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
    <-signalChan
    root.Stop(task.ErrExit)
}()
```

## 最佳实践

1. 使用 `RootTask` 作为顶层任务管理器
2. 为长时间运行的任务实现优雅关闭
3. 合理设置重试策略
4. 使用依赖关系管理任务执行顺序
5. 实现适当的错误处理和日志记录

## 示例

完整的示例代码可以在 `examples/task` 目录中找到。

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！ 