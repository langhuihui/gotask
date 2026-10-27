# 重构线 Phase 4 设计：外围渐进增强

> 输入：Phase 1–3 已收口（核心执行模型冻结：契约 C1–C20 全绿）。
> 本文件**只做设计，不含代码改动**，评审通过后按 §6 实施。
> Phase 4 是「外围」：不碰核心语义，每项独立可回滚，默认行为不变。

## 1. 目标与非目标

| # | 目标 | 对应病灶 |
|---|---|---|
| G1 | 让默认测试面回到绿色且可预期：`go test ./...` 不再被教程包的「练习式失败 + 永久挂起」拖累 | F37 / F38 |
| G2 | 修 Dashboard 侧两处既有缺陷（`StartTime` 裸字段竞争、`Blocked()` 语义漂移），并让示例模块在本机可构建 | F33 / F34 / F35 |
| G3 | 泛型编排与任务树的边界明确、退避公式去重，并提供「函数 → 任务」的桥 | F30 / F31 / F32 |
| G4 | 类型化配置与弃用标记（不 breaking） | F39 / F40 |

**非目标**

| # | 非目标 | 理由 |
|---|---|---|
| N1 | 改任何 import path、删任何导出符号、改 `Run`/`Go`/`RetryConfig` 形态 | 3.2「不在大版本前合并接口」 |
| N2 | 重写 Dashboard 前端（`dashboard/web`） | 与核心无关，收益不抵风险 |
| N3 | 引入新依赖 / 新持久化 | 外围增强不该加依赖 |
| N4 | 改核心执行语义 | Phase 1–3 的 20 条契约必须原样通过 |
| N5 | 把 history 从 dashboard 拆回主库 | 现在的「独立模块 + SQLite」已经是可选形态，主库零依赖 |
| N6 | 异步 dispose（3B.2 否决项） | 需要先有新的销毁顺序契约，单独议题 |

## 2. 现状精确走读（新增事实，续 F29）

| # | 事实 | 位置 |
|---|---|---|
| F30 | **泛型编排与任务树零耦合**：`generic.go` 在全仓库除自身与测试外**没有调用者**（唯一出现在 `task.go` 的注释里）。两套 API 并列，共享的只有 `Err*` 常量 | `generic.go` |
| F31 | **退避公式仍有两份**：`retryDecision` 与 `genericRetryDelay` 各写一遍 `interval * 2^(n-1)` + `MaxRetryInterval` 封顶（3A 只统一了「终止判定」） | `task.go` / `generic.go` |
| F32 | 能力不对称：generic 有 `WithTimeout`（`context.WithTimeoutCause` + `ErrTimeout`），任务树**至今没有「单轮 attempt 超时」**；generic 的 `ShouldRetry` 已与任务树对齐 | `generic.go` / `task.go` |
| F33 | Dashboard 是**独立 module**（`github.com/langhuihui/gotask-example`，`replace ../..`），钉 `go 1.24.0` + `toolchain go1.24.7`；本机 Go 1.23.4，toolchain 下载失败 → **示例模块在本机无法构建**，验收门「Dashboard 仍能运行」当前不可执行 | `dashboard/server/go.mod` |
| F34 | Dashboard 在 HTTP goroutine 里直接读 `task.GetTask().StartTime`（并用 `time.Since` 算时长），与事件循环上的写入并发（框架内部已改 `startNano` 快照，但**没有公开访问器**） | `dashboard/server/main.go:188-190`、`utils.go:19/64` |
| F35 | Dashboard 用 `Job.Blocked()` 判断「当前阻塞任务」；Phase 2/3 后 `Run` 与退避都不再占用循环，`Blocked()` 大多时刻为 `nil` → 展示退化 | `dashboard/server/utils.go:35/79` |
| F36 | Dashboard 用 `GetTaskPointer()`（uintptr）当任务标识——正是 3.1 要弃用的身份 API | `dashboard/server/utils.go:15/67` |
| F37 | **lessons 的「15/17 失败」不是框架语义漂移**：L1-1 的 `root.AddTask` 被注释（`//TODO: Uncomment`）——按教程设计应当失败；L2-2 用 `manager.WaitStopped()` 等一个「按本课设定永不自动停止」的 Job——按设计永久挂起。教程练习被当成单元测试跑了 | `lessons/lesson01_test.go`、`lesson02_test.go` |
| F38 | lessons 与主包同 module、同包路径（`.../lessons`、`.../lessons_CN`，各 17 个 `Test*`），所以 `go test ./...` 必然把它们算进验收面；一旦挂起就是整包超时（实测 60s 超时 + 剩余用例未执行） | `lessons/`、`lessons_CN/` |
| F39 | 类型化配置缺口：`AddTask(t, opt ...any)` 只认 `context.Context`/`Description`/`RetryConfig`/`*slog.Logger`/`int`；`int` 是调用栈深度偏移，靠 `runtime.Caller(callDepth)` 推断，错误只能运行时暴露 | `job.go` `initContext` |
| F40 | `Singleton[T]` 是导出类型，实际只服务事件循环通道懒初始化 | `event_loop.go` |

## 3. 方案设计

### 3.1 4A lessons 处置（G1）

**问题定性**：lessons 是**教程**，其「断言」包含两处刻意的失败与挂起。要让 `go test ./...` 有意义，必须先把它从默认验收面移出，再修掉真正的缺陷。

**方案（推荐）**

1. 给 `lessons/`、`lessons_CN/` 下全部 `*_test.go` 加文件头 `//go:build lesson`（各 17 个用例、共 20 个文件）。默认 `go test ./...` 不再编译它们；维护者/读者用 `go test -tags lesson ./lessons/...` 单独跑。
2. 修 L2-2 的**永久挂起**（真缺陷，不只是教学问题）：把 `manager.WaitStopped()` 改成「等 1.5s → 断言 3 个 worker 仍在 RUNNING（本课要讲的正是这点）→ 主动 `manager.Stop(...)` → 等停止并断言 DISPOSED」。教程价值不变，作为示例也不再挂死。
3. L1-1 标记为**练习**：保留注释掉的 `AddTask`，但在断言前 `t.Skip("exercise: uncomment AddTask to complete")`，让 `-tags lesson` 全量跑时得到「1 skipped + 其余通过」的可预期结果。
4. lessons_CN 同步（文案已经是中文，只改测试代码，教程正文不动）。

**备选**：把 lessons/lessons_CN 各自改成独立 module（与 dashboard 同款 `replace ../..`）。否决理由：为「不跑」付出两个 go.mod + 改变读者运行方式（要从仓库根 `cd`），而 build tag 一行即可且自文档化。

### 3.2 4B Dashboard 解耦与缺陷修复（G2）

1. **公开只读快照**（修 F34，主库唯一的代码改动）：
   ```go
   // GetStartTime 返回本轮启动时间的原子快照；未启动时为零值。
   func (task *Task) GetStartTime() time.Time
   // Elapsed 返回自本轮启动以来的时长；未启动时为 0。
   func (task *Task) Elapsed() time.Duration
   ```
   实现读既有 `startNano`（`atomic.Int64`），零额外开销。dashboard 的 `main.go`/`utils.go` 改用它们。
2. **`Blocked()` 展示退化**（修 F35）：dashboard 不再依赖 `Blocked()`，改为遍历 children 取 `GetState()` ∈ {RUNNING, GOING, DISPOSING} 的任务作为「正在执行」；`Blocked()` 的公开语义保持不变（它就是「正占用循环的那个」，Phase 2/3 后本来就该少）。
3. **指针身份**（修 F36）：`FromPointer`/`GetTaskPointer` 加 `Deprecated:` 注释（不删）；dashboard 的 `Pointer` 字段改用 `GetTaskID()`（前端只当字符串用）。
4. **可构建性**（修 F33）：把 `dashboard/server/go.mod` 的 `go` 指令降到 1.23、去掉 `toolchain` 钉版——示例代码没有用到 1.24 语言特性，这样「示例仍能运行」才可能被验收。若依赖下载受限，退化为「go vet + 人工 review」，并在文档记录。

### 3.3 4C 泛型编排边界（G3）

1. **退避公式去重**（修 F31）：抽
   ```go
   func retryDelayFor(attempt int, interval, max time.Duration) time.Duration
   ```
   供 `retryDecision`（attempt = `RetryCount`）与 `genericRetryDelay` 共用；两边各自的「是否重试」判定保持不动（已经共享 `isTerminalStopReason`）。
2. **桥接**（修 F30）：给泛型函数一个进入任务树的最小适配：
   ```go
   // AsTaskBlock 把泛型函数包装成 TaskBlock（返回值丢弃，错误即停止原因）。
   func AsTaskBlock[T any](fn TaskFunc[T]) TaskBlock
   ```
   这样「函数编排」可以 `job.AddTask(task.AsTaskBlock(fn))`，两套 API 不再是平行世界；不强推反向适配（Task → TaskFunc 会让生命周期变成闭包，收益低）。
3. **文档**：在 `generic.go` 头部与 README 写清边界——generic 是**纯函数编排**（无生命周期、无父子、无事件循环），任务树是**生命周期内核**；需要重试/超时策略时优先用任务树（它的重试已经是循环自由 + 可取消的）。
4. **能力不对称的记录**（F32）：任务树没有「单轮 attempt 超时」，这是有意的——`Run` 拿到的是 `context`，调用方自己 `context.WithTimeoutCause` 更灵活；文档给出一行示例（而不是新增框架 API）。

### 3.4 4D 类型化配置与 API 收尾（G4）

1. **类型化 option**（修 F39，与 `opt ...any` 共存）：
   ```go
   task.WithContext(ctx) / WithLogger(l) / WithDescription(d) / WithRetry(cfg) / WithCallDepth(n)
   ```
   每个返回私有 option 类型，`initContext` 的 switch 增加分支；旧写法（裸 `context.Context`/`RetryConfig`/`*slog.Logger`/`Description`/`int`）继续识别，`int` 在文档里标为内部用法。`AddTask`/`RunTask` 签名不变。
2. **弃用标记**（不删）：`FromPointer`/`GetTaskPointer` 加 `Deprecated:`（指向 `GetTaskID`）；`AdaptAnyTaskFunc`/`AsAnyTaskFunc` 已标，保持。
3. **`Singleton` 收尾**（修 F40）：保留导出类型（删是 breaking）但标 `Deprecated`，事件循环内部换成一个私有 `lazyChan` 实现，公开类型只作兼容壳。

## 4. 选型记录

| 方案 | 结论 |
|---|---|
| lessons 全部改写为「可运行示例」 | 否决：教程的练习属性会消失，读者体验倒退 |
| lessons 只修失败、不隔离 | 否决：`go test ./...` 仍要跑满 10 分钟并挂起，验收门形同虚设 |
| lessons 改独立 module | 备选，未采用（见 §3.1） |
| 删除 `FromPointer`/`Singleton` | 否决：3.2 禁止大版本前删导出符号 |
| generic 移入子包 `generic/` | 否决：breaking import path，收益只是「看着干净」 |
| 合并 `RetryPolicy` 与 `RetryConfig` | 否决：一个是纯函数配置、一个是生命周期配置，合并要么加泛型约束要么引入接口，收益低 |
| 给任务树加「attempt 超时」API | 否决（本期）：`Run` 已有 context，调用方一行 `WithTimeoutCause` 即可；加进框架反而与 Phase 3 的超时语义纠缠 |
| Dashboard 前端重写 | 否决：与核心无关 |

## 5. 兼容性影响与风险

| # | 风险 | 缓解 |
|---|---|---|
| R1 | lessons 加 build tag 后，依赖 `go test ./...` 覆盖教程的 CI 失去覆盖 | 文档写明 `-tags lesson`；`ai_rules` 里补一条运行方式 |
| R2 | dashboard `go` 指令降到 1.23 若与代码里隐式依赖 1.24 行为冲突 | 编译 + 启动冒烟验证；冲突则回退到「记录为环境限制」 |
| R3 | `GetStartTime()` 未启动时返回零值，调用方可能除以零算进度 | 注释写明；dashboard 侧判 `IsZero` |
| R4 | 抽 `retryDelayFor` 时改动 `retryDecision` 的公式细节 → 重试时序漂移 | 抽函数不改公式；C9/C10/C13 与 `Test_RetryTask` 锁定时序 |
| R5 | 修 L2-2 改变教程流程 | lessons_CN 同步，正文注释补一句「1.5s 后观察状态」 |
| R6 | dashboard 改 ID 后前端字段语义变化（pointer → id） | JSON 字段名保留 `pointer`，值改 ID，前端零改动 |
| R7 | 类型化 option 与裸 `int` 混用导致 callDepth 计算偏移 | `WithCallDepth` 明确替代；旧 `int` 路径保持原行为 |

## 6. 实施步骤

- **Step 0（可验证性先行）**：dashboard 模块可构建（§3.2.4）+ `go test ./...` 现状快照（lessons 失败清单）
- **Step 1（4A）**：build tag + 修 L2-2 + L1-1 skip + lessons_CN 同步 → 验收：`go test ./...` 绿、`go test -tags lesson ./lessons/... ./lessons_CN/...` 结果可预期
- **Step 2（4B）**：`GetStartTime`/`Elapsed` + dashboard 三处改法 + 弃用标记 → 验收：主库契约不动、dashboard 构建/冒烟、`go vet` 干净
- **Step 3（4C）**：`retryDelayFor` 抽取 + `AsTaskBlock` + 文档 → 验收：C9/C10/C13 与 generic 测试全绿、基准不退化
- **Step 4（4D）**：`WithXxx` + `Deprecated` + `lazyChan` → 验收：新契约（typed option 与旧写法等价）、全量 -race
- **Step 5**：文档（`docs/重构-执行模型v2.md` §7 Phase 4、README/README_CN 的 API 清单与教程运行方式）

## 7. 验收标准

1. `go test ./...`（默认 tag）**全绿且不超过 60s**；`go test -tags lesson ./lessons/... ./lessons_CN/...` 结果可预期（L1-1 skipped，其余通过，无挂起）。
2. 核心契约 20 条原样通过；全量 `-race` 通过；基准不退化（>10% 需说明）。
3. `dashboard/server` 可在本机构建并启动（或明确记录环境限制与替代验证）。
4. `go vet ./...` 无新增告警（存量 `FromPointer` 警告随 Deprecated 标记保留）。
5. 文档与真实语义一致：generic 与任务树边界、lessons 运行方式、新 option 与弃用项。

## 8. 待决问题（评审用）

- **Q1 范围与顺序**：先 4A+4B（修既有问题，让 `go test ./...` 变绿）再做 4C+4D（推荐）；还是 4C+4D 先行；还是一次全做？
- **Q2 lessons 处置**：`//go:build lesson` 隔离 + 修 L2-2 + L1-1 标练习（推荐）；改独立 module；只修失败不隔离。
- **Q3 泛型边界**：抽 `retryDelayFor` + `AsTaskBlock` 桥接 + 文档（推荐）；只抽公用不桥接；冻结不动。
- **Q4 类型化配置**：新增 `WithXxx` 与 `opt ...any` 共存 + 弃用标记（推荐）；直接以 `WithXxx` 取代 `opt ...any`（breaking）；本期不动。
