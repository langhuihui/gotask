```mermaid
sequenceDiagram
    autonumber
    participant User
    participant Job
    participant EventLoop
    participant InputChan as Input Channel
    participant ChildTask as Child Task

    %% 1. 添加任务流程
    Note over User, EventLoop: 阶段 1: 添加任务 (Lazy Start)
    User->>Job: AddTask(ChildTask)
    activate Job
    Job->>EventLoop: add(ChildTask)
    activate EventLoop
    
    EventLoop->>InputChan: Send ChildTask
    
    %% 检查是否需要启动 Loop
    EventLoop->>EventLoop: active()
    alt Not Running
        EventLoop->>EventLoop: Set running = true
        EventLoop->>EventLoop: go run()
    end
    deactivate EventLoop
    deactivate Job

    %% 2. Event Loop 主循环
    Note right of EventLoop: 阶段 2: Event Loop 运行中 (Goroutine)
    loop reflect.Select (监听所有通道)
        
        alt Case 0: Input Channel (收到新任务)
            InputChan->>EventLoop: Receive ChildTask
            EventLoop->>ChildTask: start()
            activate ChildTask
            ChildTask-->>EventLoop: Return Signal Channel
            EventLoop->>EventLoop: Append Signal Channel to cases
            EventLoop->>Job: onChildStart()
            
        else Case 0: Input Channel (收到 Job.Call 指令)
            InputChan->>EventLoop: Receive func()
            EventLoop->>EventLoop: Execute func() (线程安全执行)
            
        else Case N: Child Task Signal (子任务有动静)
            ChildTask->>EventLoop: Signal (Done/Panic/Data)
            deactivate ChildTask
            
            alt Task Finished (非 ChannelTask)
                EventLoop->>Job: onChildDispose()
                EventLoop->>ChildTask: checkRetry()
                
                alt Retry Needed (需要重试)
                    ChildTask->>ChildTask: reset()
                    ChildTask->>ChildTask: start()
                    activate ChildTask
                    EventLoop->>EventLoop: Update Signal Channel in cases
                    EventLoop->>Job: onChildStart()
                else No Retry (无需重试)
                    EventLoop->>Job: removeChild()
                    EventLoop->>EventLoop: Remove from cases & children
                end
            end
        end
        
        %% 3. 退出条件
        opt No Children AND Input Empty
            EventLoop->>EventLoop: Set running = false
            Note right of EventLoop: Loop Exit (自动休眠)
        end
    end

```

### 图解说明

1.  **Lazy Start (惰性启动)**:
    *   `EventLoop` 并不是一开始就运行的。只有当 `AddTask` 或 `Call` 被调用时，`active()` 方法才会检查并启动 `run()` Goroutine。
    *   如果 Loop 已经在运行，`active()` 只是确保状态正确，不会重复启动。

2.  **Input Channel (Case 0)**:
    *   这是 `reflect.Select` 的第 0 个 case，优先级最高。
    *   它接收两种类型的数据：
        *   `ITask`: 新的子任务，会被启动并加入监听列表。
        *   `func()`: 闭包函数（来自 `Job.Call`），会在 Loop 的 Goroutine 中直接执行，保证了对 Job 内部状态修改的线程安全性。

3.  **Child Task Signal (Case N)**:
    *   当子任务完成或报错时，对应的 Channel 会被触发。
    *   **重试机制**: `EventLoop` 会询问任务 `checkRetry()`。如果返回 `true`，任务会被重置并重新启动，保持在 Loop 中；否则，任务会被彻底移除。

4.  **自动退出**:
    *   当没有子任务在运行 (`len(children) == 0`) 且 Input Channel 为空时，`EventLoop` 会自动退出，释放 Goroutine 资源。下次有任务时再重新启动。
