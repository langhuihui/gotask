# 重构线 Phase 3 设计：统一生命周期与监督策略

> 输入：Phase 2 已实施并验收（`0728930`）。本文件**只做设计，不含代码改动**，评审通过后按 §7 实施。
> 目标读者：维护者。文中所有代码位置均指 `refactor/execution-model-v2` 当前树。

## 1. 目标与非目标

开发计划 §5.2 给 Phase 3 的定义是「统一生命周期与监督策略：处理 `Run`/`Go`、停止超时、销毁超时、重试状态、故障升级」。据此拆成四个目标、六个非目标。

**目标**

| # | 目标 | 解决的病灶 |
|---|---|---|
| G1 | 重试与监督分离：退避不再占用事件循环 | C5 / F16 / F7 |
| G2 | 重试语义统一：任务树 `RetryConfig` 与 `generic.RetryPolicy` 对齐，`ErrRestart` 有真实生产者 | F19 / F20 / F21 |
| G3 | 生命周期超时可配置、可观测（停止、销毁） | F18 |
| G4 | 故障升级：子任务失败成为父层可决策事件（默认保持现状，opt-in 升级） | F19 / F24 |

**非目标**（明确不做，避免 Phase 3 失控）

| # | 非目标 | 理由 |
|---|---|---|
| N1 | **`Start` 移出事件循环（C3）** | 迁移成本高于收益，评估见 §6.1。本期只记录结论 |
| N2 | **`Run`/`Go` 接口合并** | 3.2 硬约束「不在大版本前合并接口」。迁移草案见 §6.2 |
| N3 | 监督策略族（one-for-one / one-for-all / rest-for-one） | API 面大、使用方零需求，收益不足 |
| N4 | 强杀、进程隔离、资源配额 | Go 硬边界（开发计划 §2），只能检测不能强制 |
| N5 | 状态机重构、快照一致性 | Phase 4 范畴 |
| N6 | 泛型编排与任务树边界 | Phase 4 范畴 |

**贯穿原则**：Go 无法强制杀死 goroutine，所以「超时」在本设计里一律等于**检测 + 记录 + 可选策略**，不是「中断执行」。文档与 API 注释都必须诚实表达这一点（`generic.WithTimeout` 的注释已是这个口径）。

## 2. 现状精确走读（Phase 3 相关新增事实）

事实编号接 Phase 1/2 的 F1–F15。

| # | 事实 | 代码位置 |
|---|---|---|
| F16 | 重试退避在**调用方 goroutine 上同步 sleep**：路径 B 就是事件循环，所以退避期间整个 Job 停摆；唯一逃逸口是 `parent.Done()` | `task.go` `checkRetry` |
| F17 | 退避起点是**本轮 attempt 的 `StartTime`**，不是失败时刻：`if delta := time.Since(task.StartTime); delta < retryDelay` → 实际等待 `retryDelay - delta` | `task.go` `checkRetry` |
| F18 | **停止与销毁都没有超时**。`Stop()` 只 cancel context + 排空 `closeOnStop`，不等待；`waitChildStopped` 每 30s 打一条 `wait child dispose stalled` 警告后**无限等**；`dispose()` 的 `PreDispose`/`waitChildrenDispose`/`Dispose()`/资源回收/`OnDispose` 钩子全部在循环上同步执行，任一段阻塞即拖死整个 Job | `job.go` `waitChildStopped`/`waitChildrenDispose`、`task.go` `dispose` |
| F19 | **故障不向上传播**。子任务重试耗尽 → `checkRetry` false → `removeChild`，父 Job 继续运行；唯一「升级」是循环自然退出时的 `mt.Stop(ErrAutoStop)`，**丢失子任务失败原因** | `event_loop.go` 信号分支 + teardown |
| F20 | `mt.blocked != nil` 的 AutoStop 原因分支**只在 panic 展开路径可达**：正常退出路径每轮循环顶部都会 `mt.blocked = nil`，随即 `return`，defer 里取到的永远是 nil | `event_loop.go` 循环顶部与 defer |
| F21 | `ErrRestart` **没有生产者**：全仓库只有 `checkRetry` 末尾 `return errors.Is(err, ErrRestart)` 在读它，没有任何代码产生该错误。「外部重启」能力实际不可用；且该分支**无视 `MaxRetry`**（MaxRetry=0 也会放行一次重试） | `task.go` `checkRetry` |
| F22 | **两套重试实现语义漂移**：`Task.retry`（`RetryConfig`）与 `generic.RetryPolicy`。后者多一个 `ShouldRetry func(error) bool` 谓词；`shouldRetryGeneric` 还额外把 `ErrTimeout` 排除在重试之外，`checkRetry` 没有这个排除 | `task.go` `checkRetry`、`generic.go` `shouldRetryGeneric` |
| F23 | `RetryCount` 是**终身累计**而非连续失败次数：main 线修复时移除了每轮 `ResetRetryCount`（否则 `MaxRetry` 永不生效），成功一次也不清零；`ResetRetryCount()` 公开可调 | `task.go` `checkRetry`/`ResetRetryCount` |
| F24 | `dispose()` 对 `state < TASK_STATE_STARTED` **提前返回**：只落定 `shutdown`，不跑 `PreDispose`/`Dispose`/资源回收/钩子。所以「Start 失败的任务」连清理都不做 | `task.go` `dispose` 首段 |
| F25 | `start()` 的 Start 失败路径在**循环上内联** dispose + 重试判定（`onChildDispose` → `checkRetry` → `reset` → 再 `Start`），这条链与 TaskBlock 的路径 B 是两套代码 | `task.go` `start` 错误路径 |
| F26 | `TickTask.Start` 用 `OnStop` 注册 `ticker.Reset`，`AsyncTickTask` 用 `Go()` 自己 select ticker 与 `Done`——两类定时任务的停止路径不同 | `channel.go` |
| F27 | `keepalive()` 是隐藏开关：`Work` 覆写返回 true → 永不自动停止；`RootManager.Init` 手工 `reset()` + `parentCtx=Background` + 手工 `setState(STARTED)`，完全绕过 `initContext`/`start()` | `work.go`、`root.go` |

**Phase 3 的核心病灶汇总**：F16（退避占循环）、F18（生命周期无超时）、F19+F20（故障原因丢失且无升级通道）、F22+F21（重试语义两套、重启能力是死代码）。

## 3. 方案设计

### 3.1 3A 重试与监督分离（G1，解 C5）

**思路**：把「等退避」从循环体内的一次阻塞 sleep，改成「循环可以同时等其它事件的一个信号」。与 Phase 2 同源——循环只做控制，不做等待。

**协议改造**（全部在 `task.go` + 循环的信号分支，`event_loop.go` 结构不变）：

1. `Task` 新增两个字段：`retryCh chan struct{}`（容量 1，`initContext`/`reset` 时重建）、`retryTimer *time.Timer`（受 `stopMu` 保护）。
2. `checkRetry` 拆成两个函数，判定逻辑**原样保留**（判定顺序、计数、日志、退避公式不变）：
   - `retryDecision(err) (delay time.Duration, ok bool)`：纯判定 + 算退避，**不 sleep**；把 F17 的「按 `StartTime` 扣减已耗时」折算成 `delay = max(0, retryDelay - since(StartTime))`；
   - `armRetry(delay)`：持 `stopMu` 存 `retryTimer = time.AfterFunc(delay, 非阻塞发送 retryCh)`；
   - `checkRetry(err) bool` 保留为薄包装（`retryDecision` + 阻塞等待），**`start()` 错误路径与 `RunTask` 继续用它**——这两处一个在循环上跑的是 TaskStarter 的 Start 失败链（短、罕见），一个在调用方专用 goroutine 上（sleep 不占别人），没必要改造，blast radius 最小。
3. `GetSignal()` 改为：`if task.retrying.Load() { return task.retryCh }; return task.Done()`。`retrying` 是 `atomic.Bool`，由 `armRetry` 置真、循环收到信号时置假。
4. 循环信号分支的 `default` 分支（非 `IChannelTask`）增加**第二个入口**——收到值时（`ok == true`）不再是无事可做：

   | 收到的现象 | 含义 | 动作 |
   |---|---|---|
   | `!ok`（channel 关闭） | 本轮 attempt 结束 | `onChildDispose` → `retryDecision`：可重试则 `armRetry` + 把该 case 重绑到 `child.GetSignal()`（现在返回 `retryCh`）+ `continue`；否则 `removeChild` + `removeChildCase` |
   | `ok`（收到值） | 退避到期（非 ChannelTask 在现状下**不可能**出现，见 F2/§2） | `retrying=false`；若 `child.IsStopped()` 或父已停止 → `removeChild` + `removeChildCase`；否则 `reset` + `start`，成功则把 case 重绑回 `GetSignal()`（Done）+ `onChildStart`，失败则移除 |

5. `stop()` 里取消在途退避：`retryTimer.Stop()` + 向 `retryCh` 非阻塞发一个令牌（`cancelRetry()`）。这样父任务停止时，正在等退避的子任务**立刻**被循环摘除，不用等完退避——这是 F18 的顺带改善，也是 C5 翻转后必须补的一环（否则循环会为一个已停止的任务挂着）。
6. 通道容量与竞态：`retryCh` 容量 1，timer 与 `stop()` 都只做非阻塞发送，谁先到谁生效；循环收到后统一以 `IsStopped()` 判定，不会重复 reset/start。

**可观测序列对照**

| 观测点 | 现状 | 3A 后 |
|---|---|---|
| 退避期间同 Job 的 `Call` | 全部排队（C5 锁定） | 立即执行（**C5 语义翻转**） |
| 退避期间兄弟任务启动/停止信号 | 排队 | 正常及时 |
| 重试序列（dispose → 退避 → reset → Start → 钩子 → Run） | 见 F7 | **完全不变** |
| 退避期间 `Blocked()` | 正在退避的子任务 | `nil`（无人阻塞循环；仅 dashboard 展示差异） |
| 退避期间收到父级 Stop | 靠 `<-parent.Done()` 逃逸，退避提前结束 | 靠 `cancelRetry` 令牌，循环立刻摘除该子任务 |
| 退避期间 Job 自动停止判定 | 循环被占，无从判定 | `len(e.children)` 仍含该子任务 → **不自动停止**（与现状一致） |
| `RunTask` 的退避 | 调用方 goroutine 阻塞 | 不变（专用 goroutine，没有共享循环可占） |

**为什么不改 `e.cases` 的结构**：给每个子任务再挂一个 timer case 会打破 `chosen - eventLoopBaseCases` 与 `e.children` 的下标对齐，`removeChildCase` 的 swap-delete 要整体重写，65535 门禁语义也跟着变。重绑既有 case 的 `Chan` 是循环里**已有**的做法（现状重试后就是 `e.cases[chosen].Chan = reflect.ValueOf(child.GetSignal())`），零结构风险。

### 3.2 3A 重试语义统一（G2）

1. **抽出共享判定**：把「哪些停止原因不重试」抽成一个函数（暂名 `isRetryable(err error) bool`），`checkRetry` 与 `shouldRetryGeneric` 共用。现状差异一次性对齐：
   - `ErrTaskComplete` / `ErrExit` / `ErrStopByUser` 不重试（两边一致）；
   - `ErrTimeout` 不重试（generic 有、任务树没有 → 统一为**不重试**，见 Q4）；
   - 父/自身已停止不重试（任务树有、generic 靠 `ctx.Err()`）。
2. **`RetryConfig` 增加 `ShouldRetry func(error) bool`**（可选，nil 表示按默认判定），与 `RetryPolicy` 对齐。`SetRetry` 签名不动，另加 `SetRetryPolicy(RetryConfig)` 或让谓词走 `SetShouldRetry(func(error) bool)`——具体形态见 Q3。
3. **`ErrRestart` 补生产者**：新增 `Task.Restart()`，语义是「正常停止本轮 attempt，然后按重试通道重新启动」（`Stop(ErrRestart)`），并让 `checkRetry` 的 `ErrRestart` 分支受 `MaxRetry` 约束（现状无视 MaxRetry 是缺陷）。它与 `ErrStopByUser` 的区别写进注释：`ErrStopByUser` 拒绝重试，`ErrRestart` 请求重试。
4. **`RetryCount` 语义文档化**：明确它是「任务终身累计的重试次数」而非「连续失败次数」，`GetRetryCount`/`ResetRetryCount` 行为不变（main 线已用有限重试测试锁定）。

### 3.3 3B 生命周期超时（G3）

**两个独立开关，默认值都保持现状（零语义变化）**，只在显式配置后生效。

**3B.1 停止超时**（`Task.SetStopTimeout(d)`，默认 0 = 无限等）

- 位置：`waitChildStopped` 的 30s ticker 循环改造成 `select { case err := <-done; case <-deadline }`；现有的 30s 心跳日志保留（降为 Debug，超时记 Error）。
- 超时后的动作按「保守 → 强制」分两档，由同一个配置的时间点决定：
  - **档 A（默认行为，仅记录）**：打 Error 级日志 + `SetDescription("stopTimeout", d)` + 继续等。零语义变化，纯可观测性提升；
  - **档 B（`SetStopTimeoutDetach(true)`，opt-in）**：放弃等待、父任务继续 dispose。**明确代价**：打破 C6「子先于父销毁」——父的 `OnDispose` 可能早于子的 `shutdown` 落定。只在调用方明确接受该代价时启用。
- 顺带简化：N 个子任务当前各起一个 `go child.shutdown.Await()` 等待 goroutine（串行 Range），改造后仍复用该 goroutine，但由父的 deadline 统一收口，不新增 goroutine 种类。

**3B.2 销毁超时**（`Task.SetDisposeTimeout(d)`，默认 0 = 无限）

- 本期只做**看门狗**：`dispose()` 进入各段（`PreDispose` / `wait children` / `Dispose()` / `resources` / `hooks`）时刷新 `disposeProcess` Description，另起一个 `time.AfterFunc` 在超时后打 Error 级日志（含 `GetDescriptions()` 快照与当前段），并 `SetDescription("disposeTimeout", true)`。**不能中断**，只能让人知道卡在哪一段。
- 不做异步 `dispose`：`dispose()` 现在由 `onChildDispose` 在循环上同步调用，`shutdown` 的落定是循环摘除子任务的依据（`waitChildStopped` 等它）。把用户代码段挪到 goroutine 会同时改动 C6 时序、F8（钩子在循环上）、以及 `Blocked()` 语义；没有新契约保护之前不做。列入 Phase 4 候选。

### 3.4 3B 故障升级（G4）

**现状**（F19/F20）：子任务失败耗尽重试只是 `removeChild`；父任务要么继续跑，要么在所有子任务消失后以 `ErrAutoStop` 停下——**失败原因彻底丢失**（AutoStop 的 `blocked` 分支只在 panic 展开时可达）。

**设计**：默认保持现状，提供 opt-in 升级策略。

```go
// job.go
type FailurePolicy struct {
    // Escalate 为真时：子任务以「失败」原因结束且无重试余地，
    // 父 Job 立刻以 errors.Join(childReason, ErrChildFailed) 停止。
    Escalate bool
}
func (mt *Job) SetFailurePolicy(p FailurePolicy)
```

- **「失败」判定**：`reason` 不属于 `{ErrTaskComplete, ErrAutoStop, ErrStopByUser, ErrExit}` 即为失败（`ErrTimeout` 是否算失败见 Q4）。
- **触发点**：循环的信号分支里 `retryDecision` 返回 false 时；`RunTask` 路径在 `checkRetry` 返回 false 时，同样判定（抽成 `maybeEscalate(child, reason)` 共用）。
- **级联**：父 `Stop` 走既有链路 → 父 dispose → `waitChildrenDispose` 让兄弟子任务收到同一个原因停止。**不引入新的停止路径**。
- **与 `keepalive` 的关系**：升级是「父被显式 Stop」，与自动停止无关，`Work` 的 keepalive 不豁免升级（否则常驻 Work 会静默吞掉失败）。这一点写进注释与文档。
- **可观测**：`SetDescription("failureEscalated", childID)`，日志 Warn。

### 3.5 可观测性汇总（三个子项共用）

新增/变更的 Description 键与日志级别：

| 键 | 出现时机 |
|---|---|
| `retryPending` / `retryDelay` | 退避等待中（现状已有 `retryDelay`，补 `retryPending` 布尔） |
| `stopTimeout` / `stopTimeoutDetach` | 停止超时触发时 |
| `disposeTimeout` | 销毁看门狗超时时（配合既有 `disposeProcess` 段名） |
| `failureEscalated` | 升级触发时（值为子任务 ID） |

`wait child dispose stalled` 从每 30s 一条 Warn 改为：前 30s 只 Debug 心跳，超过 `StopTimeout` 记 Error 一次（不刷屏）。

## 4. 选型记录（考虑过并否决的方案）

### 4.1 退避 timer 直接 `reset()`+`start()`（不经循环）

否决：`reset()` 改 `hookGen`/`loopGen`/promise、`start()` 改 `e.cases`/`e.children`/钩子落点，全是循环私有状态，在 timer goroutine 上跑就是数据竞争 + 顺序错乱。**重试必须由循环驱动**（这也是 C5 留到 Phase 3 的原始理由）。

### 4.2 每个子任务挂两个 select case（Done + timer）

否决：破坏 `chosen - eventLoopBaseCases` 与 `e.children` 下标对齐，`removeChildCase` 的 swap-delete、65535 门禁语义全部要改；收益只是省一次 case 重绑。

### 4.3 停止/销毁超时默认开启

否决：`StopTimeout` 默认生效等于给所有使用方引入新的「父先于子销毁」窗口，直接冲击 C6 与 `stability_regression_test.go` 锁定的行为；且「超时后放弃等待」是**语义决策**，不能替使用方做主。

### 4.4 异步 dispose（销毁移出循环）

否决（本期）：见 3B.2。收益真实但需要先有一套新的「销毁顺序」契约，属于 Phase 4。

### 4.5 `Start` 异步化（C3 一并做）

否决（本期）：见 §6.1。

### 4.6 强杀 / 子进程监督

否决：Go 运行时硬边界（开发计划 §2）。若真有强隔离需求，正确做法是把任务放进子进程由外部 supervisor 管，不在本库范围内。

### 4.7 把 `RetryCount` 改成「连续失败计数」

否决：会改变 `SetRetry(3, …)` 的实际重试次数语义（main 线 `Test_RetryTask` 锁定 `RetryCount == 3` 且明确每轮不清零），且「成功一次后清零」对长生命周期任务意味着无限重试风险。保持终身累计，只做文档澄清。

## 5. 兼容性影响与风险

**使用方审计**（仓库内）

| 位置 | 影响 |
|---|---|
| `dashboard/server` | 读 `RetryCount`/`StartTime`/`State`：字段与语义不变；`Blocked()` 在退避期间变 nil（展示差异） |
| `generic.go` | 共享 `isRetryable` 后 `ErrTimeout` 不再重试（已是现状，只是抽公用）；`RetryPolicy` 字段不变 |
| `lessons/`、`lessons_CN/` | 已知大面积失败（main 即如此），Phase 4 校准；本期不动 |
| `README.md` / `README_CN.md` | 重试/停止语义段落需按 §3.2、§3.3 更新 |
| 现有测试 | `TestContract_RetryBackoffOccupiesOwnerLoop` 必然翻转（改写为新语义 C5′）；`Test_RetryTask`/`Test_EventLoop`/`hook_retry_drainer_test.go`/`stability_regression_test.go` 的退避时序不变，应继续通过 |

**风险**

| # | 风险 | 缓解 |
|---|---|---|
| R1 | `retryCh` 与 `Done` 的重绑过程中漏收信号（退避到期但循环已把 case 换回 Done） | `armRetry` 与 case 重绑在**同一 goroutine（循环）内**串行完成；`GetSignal()` 读 `retrying` 原子标志，重绑后立即一致 |
| R2 | 父停止时正在等退避的子任务滞留 → 循环永不退出 | `stop()` 内 `retryTimer.Stop()` + 非阻塞令牌；循环收到后以 `IsStopped()` 判定并摘除（C13 契约锁定） |
| R3 | `retryDecision` 的判定逻辑与旧 `checkRetry` 漂移 | Step 0 契约先行：把现有序列（计数、顺序、`ResetRetryCount` 例外）落成测试，再动代码；两函数共用判定体 |
| R4 | `ErrRestart` 受 `MaxRetry` 约束后，若有隐藏使用方依赖「无视 MaxRetry」 | 全仓库无生产者（F21），无使用方；文档标注行为变化 |
| R5 | `StopTimeoutDetach` 打破 C6，使用方误用 | 默认关闭 + 注释写明代价 + 契约测试锁定「默认不 detach」 |
| R6 | `Escalate` 开启后，一个子任务失败导致整棵子树停止，比使用方预期更激进 | opt-in + 只对「无重试余地的失败」触发 + 停止原因携带子任务原始 reason |
| R7 | 退避期间 `Blocked()` 变 nil 影响 dashboard 判断 | 只在 dashboard 展示层；如需要可另加 `Task.RetryPending()` 查询 |
| R8 | `waitChildStopped` 的 deadline 与 `waitChildrenDispose` 串行 Range 叠加：N 个子任务各等一次超时 | 超时按**子任务维度**计（与现状串行等待的粒度一致）；如需总预算另议 |

## 6. C3 与 `Run`/`Go` 去留评估（结论：本期不做）

### 6.1 C3（`Start` 移出事件循环）迁移成本

| 成本项 | 说明 |
|---|---|
| 登记协议 | `start()` 的返回值是循环「是否登记为在册子任务」的唯一依据（`event_loop.go` ITask 分支）。Start 异步化后「登记」与「启动成败」解耦，需要新的 in-flight 状态与去向（`TASK_STATE_STARTING` 只有状态位，没有 case/归属） |
| 失败链搬迁 | F25：Start 失败 → 内联 dispose → `checkRetry` → `reset` → 再 Start 整条链在循环上。异步化必须整体搬到路径 B，否则出现两套重试逻辑 |
| 竞态面 | `TickTask.Start` 这类在 Start 里注册 `OnStop`、建 ticker 的实现，会与并发的 `Stop()` 竞争；`start()` 现有的 `IsStopped()` 检查窗口从「同 goroutine 相邻几行」变成「跨 goroutine」 |
| 收益 | Start 通常是短操作（建 ticker、连资源），阻塞循环的实测影响远小于 `Run`；Phase 2 基准里 TaskStartStop 的 11μs 已含 Start |
| 结论 | **不做**。触发条件：若将来出现「Start 长时间阻塞」的真实案例，再评估；届时与本设计的 3A/3B 契约一起做 |

### 6.2 `Run`/`Go` 接口合并草案（不做，仅记录）

Phase 2 后二者执行位置已同构（都是独立 goroutine + `task.run`），剩余差异只在接口形态：`TaskBlock` 先 `Start`/`OnStart` 再 `Run`，`TaskGo` 先 `Start`/`OnStart` 再 `Go`。长期方向（v2 大版本）：

```go
// 统一执行接口：生命周期钩子仍由 Task 提供，执行体只有一个形态
type TaskRun interface{ Run(ctx context.Context) error }
```

- `Run`/`Go` 同时实现时以 `Run` 优先（或直接判为用法错误并 Warn）；
- 迁移路径：`Go()` → `Run()`（`task.run` 的 panic 恢复/返回即 Stop 语义完全相同）；
- 不在 Phase 3 动，因为 3.2 禁止在大版本前合并接口，且合并会破坏现有实现的方法集检查（`TaskBlock`/`TaskGo` 类型断言是公开语义）。

## 7. 实施步骤

沿用 Phase 2 的「契约先行」节奏，三个可独立提交的小步。

**3A（重试与监督分离 + 语义统一）**

- **Step 0 契约先行**：新增契约并先在旧代码上跑（预期失败/通过定性）
  - C5′ `TestContract_RetryBackoffDoesNotOccupyOwnerLoop`（改写 C5：退避期间 `Call` 立即完成、序列不变）
  - C12 `TestContract_RetryCanceledByStopDuringBackoff`（退避中 Stop → 及时停止、无额外 attempt）
  - C13 `TestContract_ShouldRetryPredicateHonored`（谓词拒绝 → 不重试；谓词放行 → 重试）
  - C14 `TestContract_RestartRequestTriggersRetry`（`Restart()` 生产者）
  - C15 `TestContract_RetryCountIsLifetimeCounter`（终身累计，成功不清零）
- **Step 1**：`checkRetry` 拆 `retryDecision`/`armRetry`，`GetSignal()` 接 `retrying`，循环分支加 `ok==true` 入口，`stop()` 加 `cancelRetry`
- **Step 2**：`isRetryable` 抽公用；`RetryConfig.ShouldRetry`；`Task.Restart()`
- **Step 3**：全量 `-race` + 加压 + 基准（重点：`BenchmarkTaskStartStop`、新增 `BenchmarkCallLatencyDuringBackoff`）

**3B（生命周期超时 + 故障升级）**

- **Step 4 契约先行**：
  - C16 `TestContract_StopTimeoutRecordsAndKeepsWaiting`（默认档 A：只记录、不改变 C6）
  - C17 `TestContract_StopTimeoutDetachOptIn`（档 B：父先于子销毁，仅在 opt-in 时）
  - C18 `TestContract_DisposeTimeoutWatchdog`（卡住的 `Dispose()` 被标记，不中断）
  - C19 `TestContract_FailureEscalationOptIn`（opt-in 时父带 `ErrChildFailed` 停止兄弟）
  - C20 `TestContract_FailurePolicyDefaultKeepsRunning`（默认行为不变）
- **Step 5**：实现 + 文档（`docs/重构-执行模型v2.md` 契约表与事实表、`README`/`README_CN` 语义段落）

**提交切分**：3A 一个提交（含契约），3B 一个提交（含契约），文档各自同步。

## 8. 验收标准

1. 契约集全过（Phase 2 的 11 条 + 本期新增/改写，`-race`）；
2. 全量 `go test -race .` 通过；顺序敏感集 `-count=30` 加压通过（预算按 Phase 2 经验给足，慢路径单独 count=5）；
3. 基准：`BenchmarkTaskStartStop`/`JobCall`/`JobStopWithLiveChildren` 无感知退化（>10% 需说明理由）；新增「退避期间 Call 延迟」基准证明 C5 翻转生效；
4. 默认配置下**零语义变化**：C6/C7/C9/C10 等既有契约原样通过；唯一翻转是 C5；
5. 文档与真实语义一致（含新的「超时=检测不是中断」表述）。

## 9. 待决问题（评审结论已确认）

- **Q1 范围 → 3A 先行（推荐项）**：先完成「重试与监督分离 + 重试语义统一」并验收提交（已落地，见 `docs/重构-执行模型v2.md` §5），3B 紧随其后；C3 按 §6.1 结论本期不做。
- **Q2 停止超时的超时后动作 → 默认只记录 + opt-in detach（推荐项）**：默认零语义变化（保 C6），`SetStopTimeoutDetach(true)` 时才接受「父先于子销毁」。
- **Q3 `ShouldRetry` 接入方式 → `SetShouldRetry(func(error) bool)`（推荐项）**：最小改动，不动公开结构体形态（`RetryConfig` 只加一个可选字段）。
- **Q4 `ErrTimeout` 语义 → 不重试、算失败（推荐项）**：与 `generic.WithTimeout` 一致；3A 已实现「不重试」，3B 的故障升级按「失败」处理。

**3A 实施结果**：`docs/重构-执行模型v2.md` §5（契约 C5′/C12–C15、验收证据、基准对比、行为变化，含实施中发现并修复的两处竞态 F28/F29）。
