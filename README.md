# GoTask 项目

GoTask 是一个基于 Go 语言的异步任务管理框架，提取自 Monibuca 项目的任务系统。本项目包含了一个独立的任务库（`github.com/langhuihui/gotask`）、一个基于 React 的管理界面和一个示例后端程序。

## 项目结构

```
gotask/
├── task.go                 # 核心任务实现
├── job.go                  # 任务容器
├── event_loop.go           # 事件循环
├── work.go                 # 工作任务
├── channel.go              # 通道任务
├── root.go                 # 根任务管理器
├── panic.go                # 非panic模式配置
├── panic_true.go           # panic模式配置
├── task_test.go            # 任务测试文件
├── go.mod                  # Go 模块文件
├── util/
│   └── promise.go          # Promise 实现
└── dashboard/
    ├── server/             # 后端管理服务
    │   ├── main.go         # 主程序入口
    │   ├── database.go     # 数据库操作
    │   ├── types.go        # 数据类型定义
    │   ├── utils.go        # 工具函数
    │   ├── go.mod          # Go 模块文件
    │   └── go.sum          # 依赖校验文件
    ├── start.sh            # 启动脚本
    └── web/                # Vite React 前端项目
        ├── src/
        │   ├── components/ # React 组件
        │   │   ├── LanguageSwitcher.tsx  # 语言切换组件
        │   │   ├── Logo.tsx             # Logo组件
        │   │   ├── TaskDetail.tsx       # 任务详情组件
        │   │   ├── TaskHistoryView.tsx  # 任务历史视图
        │   │   └── TaskTree.tsx         # 任务树组件
        │   ├── hooks/      # React Hooks
        │   │   └── useLanguage.ts       # 语言Hook
        │   ├── i18n/       # 国际化
        │   │   └── index.ts             # 国际化配置
        │   ├── locales/    # 语言包
        │   │   ├── en.json             # 英文语言包
        │   │   └── zh.json             # 中文语言包
        │   ├── services/   # API 服务
        │   │   └── api.ts              # API接口
        │   ├── types/      # TypeScript 类型定义
        │   │   └── task.ts             # 任务类型定义
        │   ├── assets/     # 静态资源
        │   │   ├── logo-go.svg         # Go Logo
        │   │   ├── logo-simple.svg     # 简化Logo
        │   │   └── react.svg           # React Logo
        │   ├── App.tsx     # 主应用组件
        │   ├── App.css     # 主应用样式
        │   ├── main.tsx    # 应用入口
        │   └── index.css   # 全局样式
        ├── public/         # 公共资源
        │   └── vite.svg    # Vite Logo
        ├── dist/           # 构建输出目录
        ├── package.json    # 项目配置
        ├── pnpm-lock.yaml  # pnpm 锁定文件
        ├── vite.config.ts  # Vite 配置
        ├── tsconfig.json   # TypeScript 配置
        └── eslint.config.js # ESLint 配置
```

## 功能特性

### 九大核心特性

1. **将在父任务协程中被调用** - 所有子任务在父任务协程中顺序执行，避免并发问题
2. **优雅关闭** - 支持任务的优雅停止和资源清理，确保系统稳定性
3. **拥有唯一的ID** - 每个任务都有唯一标识符，便于追踪和管理
4. **可衡量调用耗时** - 内置性能监控，精确测量任务执行时间
5. **可扩展** - 支持钩子机制和方法重写，提供灵活的扩展能力
6. **可追溯** - 提供广义调用栈，支持任务执行路径追踪
7. **兜底机制** - 错误可以被拦截和处理，提供完善的异常处理
8. **可选的重试机制** - 支持任务失败后的自动重试，可配置重试策略
9. **可存入历史记录** - 任务执行历史可以被记录和查询

### 九大核心特性详解

#### 1. 将在父任务协程中被调用
**业务痛点**: 在复杂的异步系统中，子任务的并发执行往往导致资源竞争、状态不一致等问题。传统的goroutine管理方式难以控制执行顺序，容易出现数据竞态条件。

**实现原理**: GoTask采用单协程事件循环模式，所有子任务的Start()和Dispose()方法都在父任务的专用协程中顺序执行。通过EventLoop机制，确保同一父任务下的子任务永远不会并发执行。

**核心概念**:
- **宏任务（父任务）**: 可以包含多个子任务的执行，本身也是一个任务
- **子任务协程**: 每一个宏任务都会启动一个协程，用来执行子任务的 Start、Run、Dispose 方法
- **懒加载**: 协程可能不会一开始就创建，只有在有子任务时才会创建

**代码示例**:
```go
// 连接管理器（父任务）
type Connection struct {
    task.Job  // Job可包含子任务，子任务全部结束后，Job会结束
    Plugin     *Plugin
    StreamPath string
    RemoteURL  string
    HTTPClient *http.Client
}

// 心跳检测任务（子任务）
type HeartbeatTask struct {
    task.TickTask
    connection *Connection
    interval   time.Duration
}

func (h *HeartbeatTask) GetTickInterval() time.Duration {
    return h.interval
}

func (h *HeartbeatTask) Start() error {
    // 在父任务协程中启动
    h.Info("心跳检测任务启动", "url", h.connection.RemoteURL)
    return nil
}

func (h *HeartbeatTask) Dispose() {
    // 在父任务协程中清理
    h.Info("心跳检测任务清理", "url", h.connection.RemoteURL)
}

// 使用示例
conn := &Connection{RemoteURL: "rtmp://example.com/live/stream"}
heartbeat := &HeartbeatTask{connection: conn, interval: 30*time.Second}
conn.AddTask(heartbeat)
// 心跳任务会在连接管理器的协程中顺序执行
```

**解决价值**: 
- 彻底避免并发访问共享资源的问题
- 简化了复杂异步逻辑的调试和维护
- 保证了任务执行的可预测性和一致性
- 减少了锁的使用，提高了系统性能

#### 2. 优雅关闭
**业务痛点**: 系统关闭时，正在执行的任务可能被强制中断，导致资源泄露、数据不一致等问题。特别是在处理网络连接、文件操作等需要清理资源的场景中，粗暴的进程终止会造成严重后果。

**实现原理**: 通过context.Context机制实现优雅关闭。当父任务收到停止信号时，会依次调用所有子任务的Stop()方法，等待所有子任务完成资源清理后再退出。EventLoop会检测context取消信号，确保所有任务都能正确执行Dispose()方法。

**等待机制**: 通过WaitStarted()和WaitStopped()方法来实现等待任务开始和结束。这种方式会阻塞当前协程，确保任务状态的同步。

**代码示例**:
```go
// SRT接收器任务
type Receiver struct {
    task.Task
    mpegts.MpegTsStream
    srt.Conn
}

func (r *Receiver) Start() error {
    // 建立SRT连接和内存分配器
    r.Allocator = util.NewScalableMemoryAllocator(1 << util.MinPowerOf2)
    r.Using(r.Allocator, r.Publisher)
    // 设置连接关闭钩子
    r.OnStop(r.Conn.Close)
    return nil
}

func (r *Receiver) Dispose() {
    // 优雅关闭SRT连接
    if r.Conn != nil {
        r.Conn.Close()
    }
}

// 服务器优雅关闭
func (s *Server) OnStop() {
    // 设置停止钩子，3秒后退出
    s.Servers.OnStop(func() {
        time.AfterFunc(3*time.Second, exit)
    })
    // 设置清理钩子
    s.Servers.OnDispose(exit)
}

// 等待任务完成
func waitForReceiver() {
    receiver := &Receiver{}
    receiver.Start()
    
    // 等待任务开始
    receiver.WaitStarted()
    
    // 等待任务结束
    receiver.WaitStopped()
}
```

**解决价值**:
- 保证系统关闭时所有资源都能正确释放
- 避免数据丢失和状态不一致
- 支持热重启和滚动更新
- 提高了系统的可靠性和稳定性

#### 3. 拥有唯一的ID
**业务痛点**: 在微服务架构中，任务追踪和问题定位非常困难。当系统出现问题时，很难快速定位到具体的任务实例，特别是在分布式环境中，缺乏统一的标识体系。

**实现原理**: 每个任务在创建时都会分配一个全局唯一的ID，这个ID在整个任务生命周期中保持不变。通过ID可以追踪任务的创建、执行、停止等各个阶段的状态变化。

**代码示例**:
```go
// 发布者任务
type Publisher struct {
    task.Task
    StreamPath string
    RemoteAddr string
    Type       string
}

func (p *Publisher) Start() error {
    // 每个发布者都有唯一的ID
    p.Info("发布者启动", "streamPath", p.StreamPath, "taskId", p.GetID())
    return nil
}

// 获取流路径作为唯一标识
func (p *Publisher) GetKey() string {
    return p.StreamPath
}

// 任务ID可以用于日志追踪
func (p *Publisher) Run() error {
    p.Info("发布者运行中", "streamPath", p.StreamPath, "taskId", p.GetID())
    return nil
}

// 使用示例
publisher := &Publisher{
    StreamPath: "/live/stream1",
    RemoteAddr: "192.168.1.100:8080",
    Type:       "rtmp",
}
// 框架自动为任务分配唯一ID，可用于追踪
```

**解决价值**:
- 实现了完整的任务生命周期追踪
- 支持分布式系统的任务关联分析
- 便于性能监控和问题诊断
- 为审计和合规提供了基础数据

#### 4. 可衡量调用耗时
**业务痛点**: 性能问题往往难以定位，特别是在复杂的异步系统中，很难准确测量每个组件的执行时间。传统的性能分析工具在异步场景下效果有限。

**实现原理**: 框架在任务的关键节点自动记录时间戳，包括任务开始、结束、重试等时间点。通过内置的时间测量机制，可以精确统计每个任务的执行耗时，支持毫秒级精度。

**代码示例**:
```go
// SRT发送器任务
type Sender struct {
    task.Task
    hls.TsInMemory
    srt.Conn
    Subscriber *m7s.Subscriber
}

func (s *Sender) Start() error {
    // 框架自动记录开始时间
    s.SetAllocator(util.NewScalableMemoryAllocator(1 << util.MinPowerOf2))
    s.Using(s.GetAllocator(), s.Subscriber)
    s.OnStop(s.Conn.Close)
    return nil
}

func (s *Sender) Run() error {
    // 执行媒体流处理，可能耗时较长
    pesAudio, pesVideo := mpegts.CreatePESWriters()
    return m7s.PlayBlock(s.Subscriber, func(audio *format.Mpeg2Audio) error {
        // 处理音频数据
        return nil
    }, func(video *mpegts.VideoFrame) error {
        // 处理视频数据
        return nil
    })
}

func (s *Sender) Dispose() {
    // 框架自动记录结束时间，可以获取总耗时
    duration := s.GetDuration()
    s.Info("SRT发送器执行完成", "duration", duration)
}
```

**解决价值**:
- 快速识别性能瓶颈和热点代码
- 支持实时性能监控和告警
- 为系统优化提供数据支撑
- 帮助制定合理的超时和重试策略

#### 5. 可扩展
**业务痛点**: 不同的业务场景需要不同的任务处理逻辑，但传统的任务框架往往缺乏足够的扩展性。开发者需要在框架限制和业务需求之间做出妥协。

**实现原理**: 通过Go的接口和嵌入机制，支持多种任务类型的继承和扩展。提供丰富的钩子方法（OnStart、OnBeforeDispose、OnDispose），允许开发者在任务的关键节点插入自定义逻辑。

**任务类型体系**:
- `task.Task` - 所有任务的基类，定义了任务的基本属性和方法
- `task.Job` - 可包含子任务，子任务全部结束后，Job会结束
- `task.Work` - 同Job，但子任务结束后，Work会继续执行
- `task.ChannelTask` - 自定义信号的任务，通过覆盖GetSignal方法来实现
- `task.TickTask` - 定时任务，继承自ChannelTask，通过覆盖GetTickInterval方法来控制定时器间隔

**任务生命周期方法**:
- `Start() error` - 任务启动方法，用于资源创建（可选）
- `Run() error` - 任务执行过程，阻塞式执行（可选）
- `Go() error` - 非阻塞式执行（可选）
- `Dispose()` - 任务销毁过程，用于资源清理（可选）

**代码示例**:
```go
// 流管理器（Job类型）
type StreamManager struct {
    task.Job  // 可包含子任务，子任务全部结束后，Job会结束
    StreamPath string
    Publishers util.Collection[string, *Publisher]
    Subscribers util.Collection[string, *Subscriber]
}

// 统计任务（定时任务）
type StatsTask struct {
    task.TickTask
    manager  *StreamManager
    interval time.Duration
}

func (s *StatsTask) GetTickInterval() time.Duration {
    return s.interval
}

// 清理任务（定时任务）
type CleanupTask struct {
    task.TickTask
    manager *StreamManager
}

func (c *CleanupTask) GetTickInterval() time.Duration {
    return 5 * time.Minute
}

// 钩子方法
func (s *StreamManager) OnStart() {
    s.Info("流管理器启动前的钩子")
}

func (s *StreamManager) OnDispose() {
    s.Info("流管理器销毁后的钩子")
}

// 任务启动方法
func (s *StreamManager) Start() error {
    // 初始化流管理器资源
    s.Publishers = util.NewCollection[string, *Publisher]()
    s.Subscribers = util.NewCollection[string, *Subscriber]()
    return nil
}

// 任务执行过程
func (s *StreamManager) Run() error {
    // 执行流管理逻辑，会阻塞父任务的子任务协程
    return nil
}

// 任务销毁过程
func (s *StreamManager) Dispose() {
    // 清理所有发布者和订阅者
    s.Publishers.Range(func(key string, pub *Publisher) {
        pub.Stop()
    })
    s.Subscribers.Range(func(key string, sub *Subscriber) {
        sub.Stop()
    })
}
```

**解决价值**:
- 支持各种复杂的业务场景
- 提供了灵活的定制化能力
- 减少了重复代码的编写
- 保持了框架的简洁性和易用性

#### 6. 可追溯
**业务痛点**: 在复杂的异步系统中，当出现问题时很难追踪到具体的执行路径。传统的日志记录方式在异步场景下往往信息不完整，难以重现问题场景。

**实现原理**: 通过维护任务调用栈和状态变化历史，记录每个任务的完整执行路径。包括任务的创建、启动、执行、停止等各个阶段，以及任务之间的父子关系。

**代码示例**:
```go
// RTMP客户端任务
type RTMPClient struct {
    task.Task
    URL      string
    StreamPath string
    direction int
    pullCtx  *PullJob
}

func (c *RTMPClient) Start() error {
    // 框架自动记录调用栈和任务关系
    c.Info("RTMP客户端启动", "url", c.URL, "taskId", c.GetID())
    return nil
}

func (c *RTMPClient) Run() error {
    // 可以获取完整的执行路径和任务层次关系
    c.Info("RTMP客户端执行中", "path", c.GetExecutionPath())
    
    // 执行RTMP连接逻辑
    if err := c.connect(); err != nil {
        c.Error("RTMP连接失败", "err", err)
        return err
    }
    
    return nil
}

// 使用步骤追踪
func (c *RTMPClient) connect() error {
    // 记录执行步骤
    c.Step("URLParsing", "解析RTMP URL")
    c.Step("Connection", "连接到RTMP服务器")
    c.Step("Handshake", "执行RTMP握手")
    c.Step("Streaming", "接收媒体流")
    return nil
}
```

**解决价值**:
- 提供了完整的执行路径追踪
- 支持问题场景的精确重现
- 便于系统行为的分析和优化
- 为故障排查提供了强有力的工具

#### 7. 兜底机制
**业务痛点**: 异步系统中的panic和异常往往难以处理，一个任务的崩溃可能导致整个系统不稳定。传统的异常处理机制在异步场景下效果有限。

**实现原理**: 通过recover机制捕获panic，将异常转换为错误信息向上传播。同时提供多种错误处理策略，包括重试、降级、熔断等，确保系统的稳定性。

**代码示例**:
```go
// SRT接收器任务（带异常处理）
type Receiver struct {
    task.Task
    mpegts.MpegTsStream
    srt.Conn
}

func (r *Receiver) Run() error {
    defer func() {
        if r := recover(); r != nil {
            // 框架自动捕获panic并转换为错误
            r.Error("SRT接收器发生panic", "panic", r)
        }
    }()
    
    // 可能发生panic的媒体流处理
    for !r.IsStopped() {
        packet, err := r.ReadPacket()
        if err != nil {
            return err
        }
        
        // 处理媒体包，可能发生panic
        err = r.Feed(bytes.NewReader(packet.Data()))
        if err != nil {
            return err
        }
    }
    
    return r.StopReason()
}

// 服务器级别的异常处理
func (s *Server) Start() error {
    defer func() {
        if r := recover(); r != nil {
            s.Error("服务器启动异常", "panic", r)
            // 执行恢复逻辑
        }
    }()
    
    // 服务器启动逻辑
    return nil
}
```

**解决价值**:
- 防止单个任务的异常影响整个系统
- 提供了完善的错误恢复机制
- 支持优雅降级和熔断保护
- 大大提高了系统的健壮性

#### 8. 可选的重试机制
**业务痛点**: 网络请求、数据库操作等外部依赖经常出现临时性失败，但缺乏统一的重试策略。手动实现重试逻辑既复杂又容易出错。

**实现原理**: 提供可配置的重试策略，支持设置最大重试次数、重试间隔、退避算法等。支持不同错误类型的差异化重试策略，以及特定错误类型的重试终止。

**重试机制详解**:
- **触发条件**: 
  - 当Start失败时，会重试调用Start直到成功
  - 当Run或者Go失败时，则会先调用Dispose释放资源后再调用Start开启重试流程
- **终止条件**:
  - 当重试次数满了之后就不再重试了
  - 当Start或者Run、Go返回ErrStopByUser、ErrExit、ErrTaskComplete时，则终止重试
- **配置方法**: 通过SetRetry(maxRetry int, retryInterval time.Duration)设置重试策略

**代码示例**:
```go
// HTTP文件拉流任务（带重试机制）
type HTTPFilePuller struct {
    task.Task
    PullJob PullJob
    URL     string
    MaxRetry int
    RetryInterval time.Duration
}

func (h *HTTPFilePuller) Start() error {
    // 配置重试策略：最多重试3次，间隔5秒
    h.SetRetry(h.MaxRetry, h.RetryInterval)
    return nil
}

func (h *HTTPFilePuller) Run() error {
    // 可能失败的HTTP拉流操作
    if err := h.pullHTTPStream(); err != nil {
        h.Error("HTTP拉流失败", "url", h.URL, "err", err)
        return err
    }
    return nil
}

// WebSocket连接任务（无限重试）
type WebSocketClient struct {
    task.Task
    URL string
}

func (w *WebSocketClient) Start() error {
    // 配置无限重试，间隔1秒
    w.SetRetry(-1, time.Second)
    return nil
}

// 重试过程中的资源管理
func (h *HTTPFilePuller) Dispose() {
    // 每次重试前会调用此方法清理资源
    if h.PullJob.Connection != nil {
        h.PullJob.Connection.Close()
    }
    h.Info("清理HTTP连接，准备重试")
}
```

**解决价值**:
- 自动处理临时性故障
- 提高了系统的可用性和稳定性
- 减少了手动重试逻辑的复杂性
- 支持智能化的重试策略

#### 9. 可存入历史记录
**业务痛点**: 任务执行历史对于系统监控、问题诊断、性能分析等非常重要，但传统的任务框架往往缺乏历史记录功能。

**实现原理**: 自动记录任务执行的关键信息，包括任务ID、执行时间、状态变化、错误信息等。支持历史数据的查询和分析，为系统监控提供数据支撑。

**代码示例**:
```go
// 推流任务（带历史记录）
type Pusher struct {
    task.Task
    StreamPath string
    URL        string
    MaxRetry   int
}

func (p *Pusher) Start() error {
    // 设置任务描述，用于历史记录
    p.SetDescriptions(task.Description{
        "plugin":     "rtmp",
        "streamPath": p.StreamPath,
        "url":        p.URL,
        "maxRetry":   p.MaxRetry,
    })
    return nil
}

func (p *Pusher) Run() error {
    // 框架自动记录执行历史
    p.Info("开始推流", "url", p.URL)
    return nil
}

// 服务器任务历史记录
func (s *Server) Start() error {
    // 设置服务器描述
    s.SetDescriptions(task.Description{
        "version": Version,
        "port":    s.HTTP.Port,
    })
    
    // 设置钩子记录关键事件
    s.OnStart(func() {
        s.Info("服务器启动完成")
    })
    
    s.OnDispose(func() {
        s.Info("服务器关闭")
    })
    
    return nil
}

// 查询任务历史记录
func queryTaskHistory() {
    history := task.GetTaskHistory()
    for _, record := range history {
        fmt.Printf("任务ID: %s, 执行时间: %v, 状态: %s, 描述: %v\n", 
            record.ID, record.Duration, record.Status, record.Description)
    }
}
```

**解决价值**:
- 支持完整的任务执行历史追踪
- 为系统监控和告警提供数据基础
- 支持性能分析和容量规划
- 满足了审计和合规的要求



## 使用指南

### 任务启动
任务通过调用父任务的 AddTask 来启动，此时会进入队列中等待启动，父任务的 EventLoop 会接受到子任务，然后调用子任务的 Start 方法进行启动操作。

**重要原则**: 不可以直接主动调用任务的 Start 方法。Start 方法必须是被父任务调用。

### EventLoop 机制
**懒加载设计**: 为了节省资源，EventLoop 在没有子任务时不会创建协程，一直等到有子任务时才会创建，并且如果这个子任务也是一个空的 Job（即没有 Start、Run、Go）则仍然不会创建协程。

**自动停止**: 当 EventLoop 中没有待执行的子任务时，会在以下情况退出：
1. 没有待处理的任务且没有活跃的子任务，且父任务的 keepalive() 返回 false
2. EventLoop 的状态被设置为停止状态（-1）

### 任务停止
**主动停止**: 调用任务的 Stop 方法即可停止某个任务，此时该任务会由其父任务的 eventLoop 检测到 context 取消信号然后开始执行任务的 dispose 来进行销毁。

**停止原因**: 通过调用 StopReason() 方法可以检查任务的停止原因。

**Call 方法**: 调用 Job 的 Call 会创建一个临时任务，用来在子任务协程中执行一个函数，通常用来访问 map 等需要防止并发读写的资源。

### 竞态条件处理
为了确保任务系统的线程安全，我们采取了以下措施：

**状态管理**:
- 使用 `sync.RWMutex` 保护 EventLoop 的状态转换
- `add()` 方法使用读锁检查状态，防止在停止后添加新任务
- `stop()` 方法使用写锁设置状态，确保原子性

**EventLoop 生命周期**:
- EventLoop 只有在状态从 0（ready）转换到 1（running）时才启动新的 goroutine
- 即使状态为 -1（stopped），`active()` 方法仍可被调用以处理剩余任务
- 使用 `hasPending` 标志和互斥锁跟踪待处理任务，避免频繁检查 channel 长度

**任务添加**:
- 添加任务时会检查 EventLoop 状态，如果已停止则返回 `ErrDisposed`
- 使用 `pendingMux` 保护 `hasPending` 标志，避免竞态条件

## 管理面板

### 后端服务 (dashboard/server)

这是一个基于GoTask的管理服务，提供任务系统的可视化管理功能。

**项目特点**:
- 使用GoTask管理HTTP服务器生命周期
- 实现了任务监控和管理API
- 支持任务历史记录查询
- 提供RESTful API接口

**启动方式**:
```bash
cd dashboard/server
go mod tidy
go run main.go
```

**API接口**:
- `GET /api/tasks` - 获取所有任务列表
- `GET /api/tasks/{id}` - 获取特定任务详情
- `GET /api/tasks/{id}/history` - 获取任务执行历史
- `POST /api/tasks/{id}/stop` - 停止指定任务

### 前端界面 (dashboard/web)

这是一个基于React + TypeScript的Web管理界面，提供了可视化的任务管理功能。

**项目特点**:
- 现代化的React + TypeScript技术栈
- 支持中英文国际化
- 实时任务状态监控
- 任务历史记录可视化
- 响应式设计，支持移动端

**技术栈**:
- React 18 + TypeScript
- Vite 构建工具
- Ant Design UI组件库
- i18next 国际化
- Axios HTTP客户端

**启动方式**:
```bash
cd dashboard/web
pnpm install
pnpm run dev
```

**功能特性**:
- **任务树视图**: 以树形结构展示任务层次关系
- **实时监控**: 实时显示任务状态和执行进度
- **历史记录**: 查看任务执行历史和性能数据
- **多语言支持**: 支持中文和英文界面
- **响应式设计**: 适配桌面和移动设备

**开发命令**:
```bash
pnpm run dev          # 启动开发服务器
pnpm run build        # 构建生产版本
pnpm run preview      # 预览构建结果
pnpm run lint         # 代码检查
```

### 快速开始

1. **启动后端服务**:
   ```bash
   cd dashboard/server
   go run main.go
   ```

2. **启动前端界面**:
   ```bash
   cd dashboard/web
   pnpm install
   pnpm run dev
   ```

3. **访问管理界面**:
   打开浏览器访问 `http://localhost:5173`

4. **查看API文档**:
   访问 `http://localhost:8080/api/tasks` 查看任务列表

### 项目结构说明

```
dashboard/
├── server/                 # Go后端管理服务
│   ├── main.go            # 主程序入口
│   ├── database.go        # 数据库操作
│   ├── types.go           # 数据类型定义
│   ├── utils.go           # 工具函数
│   └── go.mod             # Go模块依赖
├── web/                   # React前端管理界面
│   ├── src/
│   │   ├── components/    # React组件
│   │   ├── services/      # API服务
│   │   ├── types/         # TypeScript类型
│   │   └── locales/       # 国际化文件
│   ├── package.json       # 项目配置
│   └── vite.config.ts     # Vite配置
└── start.sh              # 一键启动脚本
```

### 一键启动

项目提供了便捷的启动脚本：

```bash
# 给脚本执行权限
chmod +x dashboard/start.sh

# 一键启动前后端服务
./dashboard/start.sh
```

该脚本会自动启动后端服务和前端开发服务器，并打开浏览器访问管理界面。