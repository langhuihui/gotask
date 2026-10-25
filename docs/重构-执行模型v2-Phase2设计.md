# 重构线 Phase 2 设计：控制与执行分离（Run 移出父事件循环）

> 分支 `refactor/execution-model-v2`，基线 `67c1012`（Phase 1 语义基线）。
> 本文是 Phase 2 的**设计文档**。按项目「完整设计文档后再开发」原则，
> 评审通过后才进入实施；实施步骤与验收标准见 §7、§8，待决问题见 §9。

---

## 1. 目标与范围

### 1.1 目标

把 `TaskBlock.Run()` 的执行从父 Job 的事件循环 goroutine 上移走，使控制平面
在业务执行期间保持响应：一个长 `Run` 不再堵塞同 Job 的 `Call`、兄弟任务的
启动/停止/销毁、ChannelTask 的 `Tick`。

对应 Phase 1 契约表中的 **C1**（`TestContract_RunOccupiesParentEventLoop`，
标注「可改变：Phase 2 的核心目标」）与走读事实 **F3**。

### 1.2 非目标（Phase 2 明确不做）

| 项 | 原因 | 去向 |
|---|---|---|
| C3：`TaskStarter.Start()` 移出循环 | 迁移成本见 §5.2；Start 的约定是快速初始化 | Phase 3 评估 |
| C5：重试退避移出循环 | 退避的驱动方是事件循环（§3.2），移动它需要重新设计监督协议 | Phase 3 |
| goroutine 池化 | 先度量「每 attempt 一个 goroutine」的真实成本（§6） | Phase 3 备选 |
| `Run`/`Go` 接口签名变化 | 3.2：不在大版本前合并接口，只改执行位置 | — |
| dashboard / lessons 适配 | 外围渐进增强 | Phase 4 |

---

## 2. 现状精确走读（设计依据）

### 2.1 子任务的两条驱动路径

事件循环 `run()`（`event_loop.go:168-224`）每轮迭代处理两类事件：

**路径 A——输入消息（ITask）**，`event_loop.go:177-197`：

```go
if mt.blocked = v; v.start() {                      // start() 内联在循环上
    e.cases = append(e.cases, ...v.GetSignal()...) // 启动成功：登记信号 case
    e.children = append(e.children, v)
    mt.onChildStart(v)
} else {
    mt.removeChild(v)                               // 启动失败：移除
}
```

**路径 B——子任务信号**，`event_loop.go:198-224`（子任务 context 被取消、
`Done()` 关闭时触发）：

```go
if mt.onChildDispose(child); child.checkRetry(child.StopReason()) {
    if child.reset(); child.start() {               // 重试：reset 后再次 start
        e.cases[chosen].Chan = reflect.ValueOf(child.GetSignal())
        mt.onChildStart(child)
        continue
    }
}
mt.removeChild(child)
e.removeChildCase(chosen)
```

### 2.2 `start()` 的返回协议（`task.go:590-665`）

返回值语义是「**子任务是否进入自主运行、此后由信号驱动**」：

- 返回 `true`：调用方登记信号 case，完成/失败稍后经信号回到路径 B。
  今天只有两类子任务走到：`TaskGo`（spawn 后即返回）与纯 `TaskStarter`。
- 返回 `false`：`start()` 内部已把子任务销毁完（错误路径依次执行
  `startup.Reject → Stop → parent.onChildDispose → checkRetry → reset 或返回`），
  调用方只需 `removeChild`。

**关键事实 F12（Phase 1 文档未记录，本次走读补充）**：
`TaskBlock` 子任务的 `Start → OnStart 钩子 → Run →（失败）dispose → 退避 →
reset → 再 Start` 整条生命周期**全部内联在路径 A 里**——`start()` 对
TaskBlock 只有最终失败才返回 `false`；Run 返回 `nil` 也会被转成
`ErrTaskComplete` 走错误路径（`task.go:635-638`、`649-663`）。运行期间该子任务
**不在** `e.children` 中（信号 case 从未登记）：循环既被占住，也看不到它的完成，
`initCases` 的 panic 重启重建（7a62329）也覆盖不到它。

**F13**：`util.Promise` 的 `Fulfill(nil)` 落 `ErrResolve` 原因，
`Await` 将其映射回 `nil`（`util/promise.go:37-53`）。因此 TaskBlock 子任务的
`startup` 在 Start 成功时即已落定（`WaitStarted()` 返回 `nil`），Run 之后的
`Reject` 是空操作——`WaitStarted` 的语义在 Phase 2 前后一致。

### 2.3 `RunTask` 的同步协议（`task.go:878-900`）

`RunTask` 不经过事件循环，在调用方 goroutine 上复刻路径 A：
`initContext → start()（内联跑完整个重试序列）→ <-Done() → dispose`。
由于 `start()` 对 TaskBlock 会内联跑完所有 attempt，RunTask 天然等到最终结局；
其返回值就是最终 `StopReason()`。

### 2.4 病灶的形式化

F3 + F12 的组合意味着：占住循环的不只是 Run 本身，还有**失败重试的整条序列**；
且运行中的 TaskBlock 子任务对循环不可见。任何依赖循环及时性的操作
（`Call`、兄弟任务、Tick、`waitChildrenDispose` 里对**其他**子任务的
Stop/等待）都会被一个长 Run 拖住。这正是 `ai_rules/Claude.md` 里
「单 goroutine 事件循环」约定要防的，而 Phase 2 要解开的正是这个结：
**循环只保留控制职责，执行全部外移。**

---

## 3. 方案设计

### 3.1 核心思路

**在 `start()` 的 TaskBlock 分支把执行 goroutine 化，让 TaskBlock 子任务从
「路径 A 内联」转入「路径 B 信号驱动」——与 `TaskGo` 完全同构。**

`task.run(handler)`（`task.go:824-840`）是现成的执行包装：panic 恢复 +
返回即 `Stop(err / ErrTaskComplete)`。TaskGo 正是用它
（`go task.run(goHandler.Go)`，`task.go:646`）。Run 复用同一机制：

```go
// task.go start()，TaskBlock 分支（现状 task.go:632-639）
if runHandler, ok := task.handler.(TaskBlock); ok {
    task.setState(TASK_STATE_RUNNING)
    task.Debug("task run", "taskId", task.ID, "taskType", task.GetTaskType(), "ownerType", task.GetOwnerType())
    err = runHandler.Run()        // ← 现状：内联占住父循环
    if err == nil {
        err = ErrTaskComplete
    }
}
```

改为：

```go
if runHandler, ok := task.handler.(TaskBlock); ok {
    task.setState(TASK_STATE_RUNNING)
    task.Debug("task run", "taskId", task.ID, "taskType", task.GetTaskType(), "ownerType", task.GetOwnerType())
    // Phase 2：执行移出父 Job 事件循环。与 TaskGo 同构——独立 goroutine、
    // panic 恢复、返回即 Stop；完成/失败经 context 取消汇入子任务信号，
    // 由事件循环的路径 B 驱动 dispose 与重试。
    go task.run(runHandler.Run)
    return true                   // ← 子任务进入「自主运行、信号驱动」
}
```

要点：

- `setState(TASK_STATE_RUNNING)` 与 Debug 日志保持在 `start()` 里（循环上），
  可观测状态序列不变；
- 执行 goroutine 与 TaskGo 共用 `task.run`，panic 恢复、`ErrTaskComplete`
  转换、`Stop` 落点全部同构，不引入第二种执行语义；
- `go task.run(...)` 是框架既有的受控 goroutine 模式（TaskGo 同款），
  符合 `ai_rules/Claude.md`「禁裸 `go func()`、只准生命周期受管的 goroutine」。

### 3.2 为什么事件循环零改动

路径 B 的现有代码就是目标协议，对 TaskBlock 逐条成立：

| 路径 B 步骤 | 对 TaskBlock 子任务的效果 |
|---|---|
| 信号触发（Run 返回 → `task.run` 的 defer `Stop` → context 取消） | 完成（`ErrTaskComplete`）/ 失败（原错误）/ 被停止（停止原因）汇入同一通道 |
| `mt.onChildDispose(child)` | 在**父循环**上跑子任务 dispose（今天在路径 A 内联跑，同一 goroutine） |
| `child.checkRetry(child.StopReason())` | 重试判定与今天一致（`ErrTaskComplete/ErrExit/ErrStopByUser` 不重试；耗尽不重试） |
| `child.reset(); child.start()` | reset 换新 context 后再次 spawn Run（`start()` 现在快速返回 `true`），更新 case 到新 `GetSignal()` |
| `removeChild` + `removeChildCase` | 与今天路径 A 的 `removeChild` 等價，时机在信号处理完 |

即：**TaskGo 子任务今天就走这条路，Phase 2 等于把 TaskBlock 并入既有协议**，
`event_loop.go` 不需要任何改动（包括 65535 门禁、`initCases` 重建、
teardown 的 `blocked` 逻辑）。

`start()` 的其余部分原样保留：`TaskStarter` 分支、错误路径、错误路径里的
内部重试循环——它们服务的是「Start 失败 / Run 前被停止」这两类**还没有执行
goroutine** 的场景，语义不变。

### 3.3 `RunTask` 的驱动协议化

`start()` 对 TaskBlock 不再内联跑完重试序列，`RunTask` 必须自己当驱动方
（在调用方 goroutine 上复刻路径 B 的协议）：

```go
func (task *Task) RunTask(t ITask, opt ...any) (err error) {
    tt := t.GetTask()
    mt := task.parent
    if job, ok := task.handler.(IJob); ok {
        mt = job.getJob()
    }
    mt.initContext(tt, t, opt...)
    if mt.IsStopped() {
        err = mt.StopReason()
        tt.startup.Reject(err)
        tt.CancelCauseFunc(err)
        return
    }
    task.OnStop(t)
    if !tt.start() {
        // start 内部已走完销毁（Start 失败或无重试余地），与今天一致。
        return tt.StopReason()
    }
    // Phase 2：start() 对 TaskBlock 只负责拉起执行 goroutine 并立即返回；
    // 完成、失败与重试的驱动职责移交给调用方——事件循环的路径 B 是同款
    // 协议，这里在调用方 goroutine 上复刻它。
    for {
        <-tt.Done() // 每轮读当前 context 的 Done：reset() 后自然等到新 attempt
        tt.dispose()
        if !tt.checkRetry(tt.StopReason()) {
            return tt.StopReason()
        }
        tt.reset()
        if !tt.start() {
            return tt.StopReason()
        }
    }
}
```

行为差异（其余序列与今天完全一致，见 §3.4 对照表）：

- **TaskBlock**：attempt 序列、每 attempt 一次 dispose、退避时长、
  最终 `StopReason` 与今天相同；退避从「start 内联」变为「RunTask 驱动」，
  但 RunTask 本来就阻塞调用方，墙钟行为不变。
- **TaskGo / 纯 TaskStarter**：今天 RunTask **不重试**（`start()` 返回 true 后
  只等一次 `Done` 再 dispose）；改造后统一走重试判定。这是与事件循环协议的
  有意对齐（同一子任务经 AddTask 会重试、经 RunTask 不重试，本身是既有不一致），
  列为待决问题 Q2。

### 3.4 可观测序列对照表

| 观测点 | 现状 | Phase 2 后 |
|---|---|---|
| Run 执行的 goroutine | 父 Job 事件循环 | 独立 goroutine（每 attempt 一个） |
| Run 期间父循环处理 `Call`/兄弟任务/Tick | 全部排队 | 正常及时 |
| 子任务状态序列 | STARTING→STARTED→RUNNING→DISPOSING→DISPOSED | 相同 |
| `startup` 落定时点 | Start 成功后、Run 前（`Fulfill(nil)`，F13） | 相同 |
| `WaitStopped()` 返回值 | dispose 末尾的 `StopReason`（完成=`ErrTaskComplete`） | 相同 |
| dispose 执行的 goroutine | 父循环（路径 A 内联） | 父循环（路径 B）——同一 goroutine |
| 重试序列 | dispose→退避→reset→Start→钩子→Run | 相同（退避在路径 B 的循环上，仍是 C5 现状） |
| `onDescendantsStart` 对 TaskBlock 子任务 | **从不触发**（`start()` 从不返回 true） | 首次 spawn + 每次重试各一次 |
| `onDescendantsDispose` 对 TaskBlock 子任务 | 每 attempt 一次（错误路径） | 每 attempt 一次（路径 B）——相同 |
| `Blocked()` 在 Run 期间 | 返回该子任务 | `nil`（仅 dashboard 展示差异，§5.1） |
| Run 期间对同 Job `AddTask`/`Call` | 入队缓冲延迟（Call 等不到回调，可能死锁） | 立即处理（F3 病理的直接修复） |
| 运行中的 TaskBlock 子任务在 `e.children` 中 | 不在 | 在（信号 case 已登记，`initCases` 重启可覆盖） |

### 3.5 停止、恐慌与重试的行为矩阵

| 场景 | 现状 | Phase 2 后 |
|---|---|---|
| Run 执行中父任务 Stop | 父循环被占；`waitChildrenDispose` 的 `child.Stop` 取消 context，Run 返回后在路径 A 内联 dispose | 父循环自由；同一传播链，dispose 改在路径 B，父循环等待期间可处理其他事件 |
| Run 执行中子任务自身 Stop | Run 返回停止原因 → 错误路径 dispose（不重试：`ErrStopByUser`） | 同一传播链，dispose 在路径 B |
| Run panic（`ThrowPanic` 关） | `start()` 的 defer 恢复 → 错误路径 Stop+dispose+可重试 | `task.run` 的 defer 恢复 → Stop → 路径 B dispose+可重试（与 TaskGo panic 完全同构） |
| Run panic（`ThrowPanic` 开） | panic 穿透 `start()` 与 `run()` 的 defer，进程崩溃（调试模式本意） | panic 在执行 goroutine 上穿诚，进程崩溃——同款结局 |
| Run 失败且可重试 | 错误路径：dispose→退避（占循环）→reset→再 Start/Run | 路径 B：dispose→退避（占循环，C5 现状）→reset→再 spawn。**序列相同，退避仍占循环** |
| Run 失败且重试耗尽 | 错误路径 dispose 后 `start()` 返回 false，`removeChild` | 路径 B dispose 后 `checkRetry` 返回 false，`removeChild` |
| 出生即停止（Stop 先于 AddTask） | initContext 补取消；`start()` 在 Run 前被 `IsStopped` 拦截，错误路径 | 完全相同（拦截点在 spawn 之前） |
| Stop 与 spawn 竞态 | 无（ spawn 不存在） | `IsStopped` 检查在 spawn 前同一临界区内；最坏情况 spawn 后立即被 Stop，Run 自行 context 返回——与 TaskGo 现有竞态同型 |

---

## 4. 选型记录（考虑过并否决的方案）

### 4.1 全异步 start（C3 一并做：Start 也移出循环）

做法：路径 A 收到 ITask 后立即登记信号 case 并 `go v.start()`，`start()`
的返回值对循环不再有意义，失败也经信号回到路径 B。

否决理由（本期不做，Phase 3 与监督 redesign 一并评估）：

1. 错误路径的内联 dispose 必须从 `start()` 里摘掉，否则双 dispose；
   摘掉后 `start()` 返回 false 的旧协议作废，`AddTask` 的拒绝路径、
   `RunTask`、路径 B 重试三处调用方都要改协议；
2. `onChildStart` 的触发时机相对 `AddTask` 返回变异步，兄弟代码若依赖
   「AddTask 返回后子任务已 Start 完」会破坏（现有测试即此类假设）；
3. C3 在契约表里本就标注「Phase 2 **评估**」，评估结论是迁移成本显著高于 C1
   （见 §5.2），不值得与 C1 挤在一个提交里。

### 4.2 Run 留循环 + 协作式让出（channel yield / runtime.Gosched 钩子）

否决：要求用户 Run 循环内主动让出，框架无法强制；语义不可组合；
`ai_rules` 的单循环约定正是要避免把正确性押在用户纪律上。

### 4.3 goroutine 池

否决（本期）：池引入池耗尽/饥饿失败模式（Run 相互等待即成对死锁），
而每 attempt 一个 goroutine 的成本只有 ~1-3μs（§6 先度量）。
若基准不可接受，Phase 3 与 C5 一起评估「受管执行器」时再上池化。

### 4.4 执行 goroutine 自带重试循环（E 驱动重试，顺带解决 C5）

否决：`e.cases`/`e.children` 是事件循环私有结构，重试要换 signal channel
（`GetSignal()` 随 reset 变），只有循环能改自己的 case；E 驱动重试会导致
「旧 Done 已关 + E 在重试」与循环的处理竞态（循环可能对已关闭的旧通道
重复 dispose）。**重试必须由循环驱动**——这也是 C5 留到 Phase 3 的原因：
Phase 3 要把退避移出循环，得先给「重试监督」设计独立于循环 case 管理的载体。

---

## 5. 兼容性影响与风险

### 5.1 使用方审计结果（已全量 grep 确认）

- `Blocked()`：仅 `dashboard/server/utils.go:35,79` 两处展示用途
  （Run 期间显示 nil）。无逻辑依赖。
- `RunTask`：包内测试仅 `stability_regression_test.go:36`
  （stopped-parent 拒绝路径，走不到 `start()`）；`lessons/` 无使用。
- `OnDescendantsStart/Dispose`：注册侧在 `job.go`；消费侧除框架自身外
  需在实施时对 dashboard/lessons 再 grep 一次 listeners（预期无）。
- `event_loop.go` 零改动 ⇒ 65535 门禁、`initCases`、teardown、
  ChannelTask 路径均不受影响。

### 5.2 C3 为何暂缓（评估结论）

`Start()` 的约定是「快速初始化后返回」；把它移出循环需要 §4.1 的协议重构，
收益（防慢 Start）低于成本。缓解：Start 里的重活应放 `Run`/`Go`——
Phase 2 后这不再是问题了（执行已外移）。文档中将此列为已知限制。

### 5.3 风险清单

| # | 风险 | 影响 | 缓解 |
|---|---|---|---|
| R1 | 每 attempt 一个 goroutine：`BenchmarkTaskStartStop`（benchRunTask 是 TaskBlock）从内联变 spawn+信号往返，预期 +2~6μs/op | 基准百分比可能 >10% | 该基准测的正是被改掉的语义；记录新基线+理由；新增「Run 阻塞期间 Call 延迟」能力基准（§6） |
| R2 | `onDescendantsStart` 对 TaskBlock 子任务从「不触发」变为每 attempt 触发 | 依赖「TaskBlock 子任务不触发 start 监听」的代码行为变化 | 语义上更正确（子任务确实启动了）；列为待决 Q1，如需严格保持可在 spawn 处特判 |
| R3 | `RunTask` 对 Go/Starter 子任务从不重试变为统一重试 | 仅当对这类子任务配了 `SetRetry` 且经 RunTask 运行时可见 | 与事件循环协议对齐的有意变更；列为待决 Q2 |
| R4 | `Blocked()` 在 Run 期间返回 nil | dashboard 展示差异 | 可接受；Phase 4 可改用 `GetState`/children 展示 |
| R5 | Run 期间对同 Job `AddTask`/`Call` 从「缓冲延迟/可能死锁」变「立即处理」 | 时序假设变化（属修复） | 有意行为；契约测试锁定新语义 |
| R6 | panic 恢复位置从循环栈变为执行 goroutine 栈 | 日志中 `debug.Stack` 的内容变化 | 行为等价（恢复→Stop→dispose→可重试）；ThrowPanic 模式同为进程崩溃 |
| R7 | 运行中的 TaskBlock 子任务占用信号 case（今天不占） | 65535 门禁对大并发 Job 更早触发 | 更正确（case 本就应为运行中的子任务保留） |
| R8 | dispose 从路径 A 内联挪到路径 B | 同一 goroutine（父循环），时机在信号处理后 | C6 契约（`TestContract_ChildrenDisposeBeforeParent`）锁定「子先于父」不回归 |

---

## 6. 基准影响预估与处理

- `BenchmarkTaskStartStop`（`benchRunTask`：TaskBlock，Run 立即返回）：
  新增 goroutine spawn/唤醒 + 一次 Select 往返 + case append/remove，
  预估 **+2~6μs/op**（绝对值小，百分比可能 >10%）。
- `BenchmarkJobStopWithLiveChildren`（Go 子任务）：不受影响（Go 路径不变）。
- `BenchmarkJobCall` / `BenchmarkStopIdempotent`：不受影响。
- 处理方式：实施后跑 stash 法前后对比；若 TaskStartStop 退化 >10%，
  在文档记录理由（基准语义被有意改变）并新增能力基准：
  `BenchmarkCallDuringBlockingRun`——Run 阻塞期间同 Job `Call` 的延迟
  （现状下该延迟 ≈ Run 时长，改造后应为微秒级），用「新能力」对冲
  「旧语义成本」。仍超门则评估 §4.3 池化。

---

## 7. 实施步骤（每步可验证，独立可回滚）

**Step 0 — 契约先行（先失败）**：改写 C1、新增 C8-C11，此时应在旧代码上失败：

- `TestContract_RunDoesNotOccupyParentEventLoop`（C1 改写）：
  Run 阻塞期间同 Job `Call` 在百毫秒内完成、兄弟 AddTask 及时启动、
  另一子任务的停止被及时处理；
- `TestContract_StopDuringBlockingRunIsPrompt`（C8）：Run 执行中 Stop 父 Job，
  父循环保持响应；Run 观察 context 返回后子任务 dispose、`WaitStopped` 收敛；
- `TestContract_RetryAfterRunFailureKeepsSequence`（C9）：Run 失败 N 次后成功，
  校验 attempt 数、每 attempt 一次 dispose、最终 `StopReason`、`Size` 归零；
- `TestContract_RunTaskDrivesRetryForBlockTask`（C10）：RunTask+TaskBlock+重试，
  序列与旧内联行为一致（今天的行为由现有测试旁证，改造后显式锁定）；
- `TestContract_RunPanicRecoveredAndRetried`（C11）：Run panic 被恢复、
  Stop、dispose、可重试（与 TaskGo panic 同构）。

**Step 1**：`task.go` `start()` TaskBlock 分支 goroutine 化（§3.1，约 5 行）。
单独提交前先跑 Step 0 中 C1/C8/C9/C11 + 全量 `-race`。

**Step 2**：`task.go` `RunTask` 驱动协议化（§3.3，约 10 行）。跑 C10 + 全量 `-race`。

**Step 3**：顺序敏感测试 `-count=30` 加压（`Test_ParentStop`/`Test_Hooks`/
`Test_Block`/`Test_StopBeforeAddTask`/C6/C7 等）；`go vet`。

**Step 4**：基准对比（stash 法）+ 新能力基准；更新基线表。

**Step 5**：既有测试逐个性效（失败=回归，必须修；预期内行为变化=R2/R3/R4/R5，
在文档与测试注释中记录理由）；更新 `docs/重构-执行模型v2.md`
（事实表补 F12/F13、契约表 C1/C3 命运与新基线、Phase 2 结论）。

---

## 8. 验收标准

1. Step 0 全部契约通过（新语义被显式锁定）；
2. C2/C4/C6/C7 与 main 同步来的全部测试无回归；
3. 全量 `go test -race .` 通过；顺序敏感测试 `-count=30` 加压通过；
4. 基准相对 §6 新基线无可感知退化，或退化有记录在案的理由；
5. `docs/重构-执行模型v2.md` 同步更新；
6. 不把执行语义反向同步回 main（5.3）。

---

## 9. 待决问题（评审时拍板）

- **Q1**：`onDescendantsStart` 对 TaskBlock 子任务由「从不触发」变为
  「首次 + 每 attempt 触发」——接受（推荐，语义更正确），还是严格保持旧行为
  （在 spawn 处特判，代价是协议出现特例）？
- **Q2**：`RunTask` 对 Go/Starter 子任务统一走重试——接受（推荐，与事件循环
  协议对齐），还是严格保持「仅 TaskBlock 重试」的旧行为？
- **Q3**：`BenchmarkTaskStartStop` 若退化 >10%（预期 +几 μs）——接受并记录
  理由，还是本轮就上 §4.3 池化？
- **Q4**：本期范围仅 Run（推荐），还是把 C3（Start 移出循环）也纳入
  （接受 §4.1 的协议重构成本）？
