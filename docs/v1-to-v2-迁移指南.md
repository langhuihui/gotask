# v1 → v2 迁移指南

> 面向 `github.com/langhuihui/gotask` 的 v1 用户。设计依据见
> [重构-执行模型v2-大版本v2设计.md](./重构-执行模型v2-大版本v2设计.md)。
>
> **v1 不会停止维护。** 不想改的用户停在 v1 即可，`main` 分支继续收 bug 修复。
> v2 是一次显式断代：模块路径带 `/v2`，两代可以在同一个项目里共存。

## 1. 一分钟迁移

绝大多数项目只需要两步：

```diff
-import "github.com/langhuihui/gotask"
+import "github.com/langhuihui/gotask/v2"
```

```diff
-func (t *MyTask) Go() error {
+func (t *MyTask) Run() error {
     ...
 }
```

如果你的任务本来就用 `Run() error`，那只改 import 就够了。

```bash
go get github.com/langhuihui/gotask/v2@v2.0.0
```

> **注意**：如果只改了 import 却忘了把 `Go()` 改成 `Run()`，编译**会通过**，
> 但任务会以 `ErrLegacyTaskGo` 立刻停止。这是刻意的设计——见 §4。

## 2. 为什么有 v2

v1 修完执行模型（Phase 1–4）之后，仓库里留下了三类「只能在大版本清掉」的东西：

1. **两套执行入口**：`TaskBlock{ Run() error }` 与 `TaskGo{ Go() error }`。把 `Run`
   移出父事件循环之后，两者的执行路径完全同构，剩下的唯一差异是「同时实现时
   `Run` 优先」这条从未写进文档的隐式规则。
2. **一批 `Deprecated` 导出符号**：为了「大版本前不破坏兼容」而保留，但只有
   历史价值。
3. **靠约定维持的脆弱点**：裸 `int` 当调用栈深度、`TickTask` 里的裸类型断言、
   `Task.GetKey()` 与内部 `getKey()` 同名不同义。

## 3. 必须改（否则编译失败）

| # | v1 | v2 | 迁移动作 |
|---|---|---|---|
| B1 | `import "github.com/langhuihui/gotask"` | `.../gotask/v2` | 全局替换 import |
| B2 | `func (t *T) Go() error` | `func (t *T) Run() error` | 方法改名。若同时存在 `Run`，把两份逻辑合并（v1 本来也只跑 `Run`） |
| B3 | `task.FromPointer(p)` | — | 改用 `GetTaskID()`；需要对象就在调用方保存 `ITask` |
| B4 | `t.GetTaskPointer()` | — | 同上 |
| B5 | `task.Singleton[T]` | — | 自己用 `sync.Once`（或 `lazyChan` 式的私有结构）包一层 |
| B6 | `task.AnyTaskFunc` / `AdaptAnyTaskFunc` / `AsAnyTaskFunc` | — | 直接用 `TaskFunc[T]`（`TaskFunc[any]` 可自由转换） |
| B7 | `t.GetKey()`（取 uint32 ID） | — | 改 `t.GetTaskID()` |
| B8 | `AddTask(t, 2)`（裸 int 当 callDepth） | — | 改 `AddTask(t, WithCallDepth(2))` |

## 4. 不会编译失败，但行为会变（必须知道）

这类比编译错误更危险，因为它们是静默的。v2 已经为最要紧的那条加了守卫。

### 4.1 只实现 `Go()` 的任务会立刻失败

```go
// v1：正常执行
type Worker struct{ task.Task }
func (w *Worker) Go() error { /* 主循环 */ return nil }
```

v2 删除 `TaskGo` 之后，`Go()` 方法名仍然存在，只是**不再被识别**。如果不加
处理，这个任务会「启动成功、永远不执行」，一直空转到被外部停止——没有报错、
没有日志、没有 panic。

所以 v2 加了迁移守卫：检测到「实现了 `Go()` 但没有 `Run()`」时，直接以
`ErrLegacyTaskGo` 走失败路径。

```go
task := (&Worker{} /* 忘了改名 */)
parent.AddTask(task)
// 任务以 ErrLegacyTaskGo 停止，可诊断、可重试，也能被 FailurePolicy 感知
```

看到这个错误，把方法名改成 `Run` 即可。

### 4.2 `TASK_STATE_GOING` 消失

状态机少了一个中间态，执行中的任务统一是 `TASK_STATE_RUNNING`。

```diff
-if t.GetState() == task.TASK_STATE_GOING {
+if t.GetState() == task.TASK_STATE_RUNNING {
```

### 4.3 `TickTask` 装错 handler 从 panic 变错误

v1 里 `TickTask.Start()` / `AsyncTickTask.Go()` 直接写 `t.handler.(ITickTask)`：
handler 还没绑定（在 `AddTask` 之前手动调用 `Start` 是最常见的触发方式）就是
一次裸断言 panic，而且 panic 落在调用方 goroutine 上，进程直接崩。

v2 返回 `ErrNotTickTask`，走正常的启动失败路径。

### 4.4 集合键的兜底显式化了

`WorkCollection` 的字典键来自 handler 自己实现的 `GetKey() K`。v1 里没有自定义
`GetKey` 的任务能靠继承 `Task.GetKey()` 拿到 ID 当兜底；v2 删了那个导出方法，
改成 `getKey()` 内部显式兜底。**可观测行为完全一致**，只是不再依赖那个同名
不同义的公开方法。

## 5. 前后对照：一个最小完整例子

**v1**

```go
package main

import "github.com/langhuihui/gotask"

type Puller struct {
    task.Job
    url string
}

func (p *Puller) Go() error {
    return p.Pull(p.url)
}

func main() {
    var root task.RootManager[string, *Puller]
    root.Init()
    p := &Puller{url: "rtsp://example/stream"}
    root.AddTask(p)
    root.Shutdown()
}
```

**v2**

```go
package main

import (
    task "github.com/langhuihui/gotask/v2"
)

type Puller struct {
    task.Job
    url string
}

func (p *Puller) Run() error { // ← Go 改名 Run
    return p.Pull(p.url)
}

func main() {
    var root task.RootManager[string, *Puller]
    root.Init()
    p := &Puller{url: "rtsp://example/stream"}
    root.AddTask(p)
    root.Shutdown()
}
```

## 6. v1 与 v2 能共存吗

能。两者是不同的 import path，可以同时出现在一个 `go.mod` 里：

```go
import (
    v1 "github.com/langhuihui/gotask"
    v2 "github.com/langhuihui/gotask/v2"
)
```

用途主要是渐进迁移：先把新代码写成 v2，老代码留在 v1，逐包替换。注意两代的
`ITask` / `Task` 是**完全不同的类型**，不能互相传递。

## 7. 迁移自检清单

- [ ] import 全部替换成 `/v2`
- [ ] 搜索 `) Go() error`，全部改成 `) Run() error`
- [ ] 搜索 `GetTaskPointer`、`FromPointer`、`Singleton`、`AnyTaskFunc`、`AdaptAnyTaskFunc`、`AsAnyTaskFunc`
- [ ] 搜索 `.GetKey()`，确认不是想拿任务 ID（要 ID 用 `GetTaskID`）
- [ ] 搜索 `AddTask(` 的第二个参数，裸 int 改成 `WithCallDepth(n)`
- [ ] 搜索 `TASK_STATE_GOING`
- [ ] 跑一遍测试，确认没有任务以 `ErrLegacyTaskGo` 停止

最后一条最容易被忽略：编译全过、测试也全绿，但某个任务悄悄从 `Run` 路径漏到了
`Go` 路径——v2 的守卫会把它变成一次可见的失败，而不是一段不执行的代码。

## 8. 迁移后能拿到什么

除了一次性的清理，v2 还带来几处以前没有的东西（都是 opt-in，默认行为不变）：

| 能力 | API | 说明 |
|---|---|---|
| 停止超时 | `Job.SetStopTimeout` / `SetStopTimeoutDetach` | 子任务停不下来时不再无限等 |
| 销毁看门狗 | `Task.SetDisposeTimeout` | 慢 `Dispose` 会带 `disposeProcess` 快照报警（只记录不中断） |
| 故障升级 | `FailurePolicy{Escalate: true}` | 子任务失败且无重试余地 → 父任务级联停止 |
| 重试谓词 | `Task.SetShouldRetry` | 按错误值决定值不值得重试 |
| 类型化 option | `WithContext` / `WithLogger` / `WithDescription` / `WithRetryConfig` / `WithCallDepth` | 替代裸 `any` 参数 |
| 运行时长 | `Task.GetStartTime()` / `Elapsed()` | 跨 goroutine 安全的原子快照 |
| 泛型桥接 | `AsTaskBlock(TaskFunc[T])` | 把泛型编排函数挂进任务树 |
