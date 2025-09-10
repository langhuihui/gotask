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
├── manager.go              # 任务管理器
├── types.go                # 类型定义
├── utils.go                # 工具函数
├── go.mod                  # Go 模块文件
├── util/
│   └── promise.go          # Promise 实现
└── examples/
    ├── server/             # 示例后端程序
    │   ├── main.go         # 主程序
    │   └── go.mod          # Go 模块文件
    └── web/                # Vite React 前端项目
        ├── src/
        │   ├── components/ # React 组件
        │   ├── types/      # TypeScript 类型定义
        │   ├── services/   # API 服务
        │   └── App.tsx     # 主应用
        ├── public/
        └── package.json
```

## 功能特性

### 任务库特性
- **异步任务管理**: 支持任务的创建、调度、执行和监控
- **任务层次结构**: 支持父子任务关系，形成任务树
- **事件循环**: 基于反射的高效事件循环机制
- **重试机制**: 支持任务失败后的自动重试
- **生命周期管理**: 完整的任务生命周期控制
- **类型安全**: 使用泛型提供类型安全的任务管理
- **并发安全**: 线程安全的任务操作

### 前端界面特性
- **任务树视图**: 可视化展示任务层次结构
- **任务详情**: 显示任务的详细信息和状态
- **任务历史**: 记录任务的执行历史
- **实时更新**: 自动刷新任务状态
- **操作控制**: 支持任务的启动、停止等操作

## 快速开始

### 1. 启动后端服务

```bash
cd examples/server
go run main.go
```

服务将在 `http://localhost:8080` 启动，提供以下 API 端点：

- `GET /api/tasks/tree` - 获取任务树
- `GET /api/tasks` - 获取所有任务
- `POST /api/tasks` - 创建示例任务
- `GET /api/tasks/{id}` - 获取任务详情
- `POST /api/tasks/{id}/stop` - 停止任务
- `GET /api/tasks/history` - 获取任务历史
- `GET /api/tasks/stats` - 获取任务统计

### 2. 启动前端服务

```bash
cd examples/web
npm install
npm run dev
```

前端将在 `http://localhost:5173` 启动，自动连接到后端服务。

### 3. 在自己的项目中使用

在你的 Go 项目中添加依赖：

```bash
go get github.com/langhuihui/gotask
```

然后在代码中导入：

```go
import "github.com/langhuihui/gotask"
```

### 4. 构建生产版本

```bash
# 构建前端
cd examples/web
npm run build

# 构建后端
cd examples/server
go build -o server main.go
```

## 使用示例

### 创建任务

```go
// 创建任务容器
var job gotask.Job
root.AddTask(&job)

// 创建任务
var demoTask DemoTask
job.AddTask(&demoTask)
```

### 任务生命周期

```go
// 设置重试策略
task.SetRetry(3, time.Second)

// 添加生命周期钩子
task.OnStart(func() {
    fmt.Println("Task started")
})

task.OnDispose(func() {
    fmt.Println("Task disposed")
})

// 等待任务完成
task.WaitStopped()
```

### 自定义任务类型

```go
type MyTask struct {
    gotask.Task
    name string
}

func (mt *MyTask) GetOwnerType() string {
    return "MyTask"
}

func (mt *MyTask) Start() error {
    // 初始化逻辑
    return nil
}

func (mt *MyTask) Run() error {
    // 执行逻辑
    return gotask.ErrTaskComplete
}

func (mt *MyTask) Dispose() {
    // 清理逻辑
}
```

## API 文档

### 任务状态

- `INIT`: 初始化状态
- `STARTING`: 启动中
- `STARTED`: 已启动
- `RUNNING`: 运行中
- `GOING`: 异步运行中
- `DISPOSING`: 销毁中
- `DISPOSED`: 已销毁

### 任务类型

- `TASK`: 普通任务
- `JOB`: 任务容器
- `WORK`: 工作任务（长期运行）
- `CHANNEL`: 通道任务

## 贡献指南

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 许可证

本项目基于 MIT 许可证开源。

## 致谢

- 感谢 Monibuca 项目提供的优秀任务系统设计
- 感谢 React 和 Ant Design 提供的前端框架