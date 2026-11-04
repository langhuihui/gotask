/* GoTask 官网 · 原理动画实验室
   每组动画由「步骤状态快照」驱动：可自动播放、单步、跳转，并与源码位置对应。 */
(() => {
  'use strict';
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

  const mk = (tag, cls, text) => {
    const node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text !== undefined) node.textContent = text;
    return node;
  };
  const pad = n => String(n).padStart(2, '0');

  function paintLog(el, lines) {
    const last = lines.slice(-3);
    el.replaceChildren();
    last.forEach((text, i) => {
      el.appendChild(mk('div', 'log-line' + (i === last.length - 1 ? ' is-latest' : ''), text));
    });
  }

  /* ============================================================
     场景 1 · 单协程事件循环
     ============================================================ */
  const loopEl = {
    scene: $('#scene-loop'), producer: $('#producerBox'), queue: $$('#loopQueue .queue-slot'),
    core: $('#loopCore'), status: $('#loopStatus'), select: $('#loopSelect'),
    cases: $('#loopCases'), op: $('#loopOp'), children: $('#loopChildren'),
    packet: $('#loopPacket'), log: $('#loopLog')
  };

  function loopSteps() {
    const steps = [
      {
        t: '空闲：事件循环并不常驻',
        d: 'EventLoop 采用惰性启动。没有子任务且输入通道为空时，run() 退出并把 running 置回 false，不占用 goroutine。',
        src: 'event_loop.go · active() / run()',
        log: 'EventLoop: len(children)==0 → idle',
        s: { producer: false, queue: 0, core: false, status: 'idle', cases: 1, hotCase: null, child: null, op: '等待事件…', packet: null }
      },
      {
        t: '① 任意 goroutine 提交任务',
        d: 'AddTask(t) 分配全局递增 ID、context.WithCancelCause、startup/shutdown 双 Promise，并按业务 Key 去重。',
        src: 'job.go · AddTask() / initContext()',
        log: 'AddTask(*Worker) → taskId=1 · context.WithCancelCause',
        s: { producer: true, op: 'job.AddTask(t) 被调用', packet: '#producerTask' }
      },
      {
        t: '② 投入 Input Channel',
        d: 'EventLoop.add 把任务写入缓冲通道（默认 256）。通道满时返回 ErrTooManyChildren，形成天然背压。',
        src: 'event_loop.go · add()',
        log: 'EventLoop.add(t) → input chan (buffer 256)',
        s: { queue: 1, op: '等待 select 唤醒…', packet: '#loopQueue .queue-slot:nth-child(1)', packetAmber: true }
      },
      {
        t: '③ 惰性唤醒：active() 原子 CAS',
        d: '只有 running 从 false 变为 true 的那次 CAS 成功，才会 go run(mt)；已在运行则直接返回，不会重复启动。',
        src: 'event_loop.go · active()',
        log: 'active(): CAS running=false→true → go run(mt)',
        s: { core: true, status: 'running', op: 'go EventLoop.run(mt)', packet: '#loopCore', packetAmber: false }
      },
      {
        t: '④ reflect.Select 命中 case 0',
        d: 'cases[0] 恒为输入通道，优先级最高。initCases 每轮按 children 重建，即使 run() 被 panic unwind 后重启也不丢子任务 case。',
        src: 'event_loop.go · initCases() / run()',
        log: 'reflect.Select(cases): chosen = 0 (input chan)',
        s: { hotCase: 0, queue: 0, op: 'chosen = 0 → v = ITask', packet: '#loopSelect' }
      },
      {
        t: '⑤ start() 并动态追加 case',
        d: 'start() 依次执行 Start() → STARTED → startup.Fulfill → OnStart → RUNNING；返回的 signal chan 被追加进 cases 与 children。',
        src: 'event_loop.go:173 · task.go · start()',
        log: 'v.start() → STARTING → STARTED → RUNNING',
        s: { cases: 2, hotCase: null, child: { state: 'STARTING', retry: 0 }, op: 'append(v.GetSignal()) → cases[1]', packet: '#loopChildren' }
      },
      {
        t: '⑥ 子任务在父协程中运行',
        d: '实现 Run() 的任务阻塞在父循环内；实现 Go() 的任务另起 goroutine；Job.Call 的闭包也在此串行执行，无需加锁。',
        src: 'task.go · start() / job.go · Call()',
        log: 'onChildStart() → 后代事件广播 · 子任务 RUNNING',
        s: { child: { state: 'RUNNING', retry: 0 }, op: 'select 继续监听 cases[0..1]', packet: '#loopCore' }
      },
      {
        t: '⑦ 信号到达：chosen = 1',
        d: '子任务结束或 panic 时 signal 被触发，父循环调用 onChildDispose()，随后询问 checkRetry() 决定重试还是移除。',
        src: 'event_loop.go:198 · job.go:190',
        log: 'chosen = 1 → onChildDispose() → checkRetry()',
        s: { hotCase: 1, child: { state: '错误退出', retry: 0 }, op: 'checkRetry() ?', packet: '#loopSelect' }
      },
      {
        t: '⑧a 需要重试：原地替换 case',
        d: 'reset() 递增 hookGen 排空旧代钩子，start() 重新执行，并原地更新该 case 的 Chan——不新增 case，也不重建整个循环。',
        src: 'task.go · reset() / checkRetry()',
        log: 'checkRetry=true → reset() + start() → 更新 cases[1].Chan',
        s: { caseReplaced: true, hotCase: null, child: { state: 'RETRY', retry: 1 }, op: 'retryCount=1 → 原地替换 Chan', packet: '#loopChildren' },
        onlyWhen: 'retry'
      },
      {
        t: '⑧b 无需重试：swap-delete 移除',
        d: 'removeChildCase 用 O(1) 的 swap-delete 把该子任务从 cases 与 children 中摘除，监听列表保持紧凑。',
        src: 'event_loop.go · removeChildCase()',
        log: 'checkRetry=false → removeChild() → swap-delete case 1',
        s: { cases: 1, caseReplaced: false, hotCase: null, child: null, op: 'children: 0 · cases: 1', packet: '#loopCore' }
      },
      {
        t: '⑨ 分阶段回收并回调等待者',
        d: 'PreDispose → waitChildrenDispose → 自身 Dispose → resources → OnDispose → DISPOSED；shutdown.Fulfill(reason) 唤醒 WaitStopped。',
        src: 'task.go · dispose() / job.go · waitChildrenDispose()',
        log: 'dispose(): PreDispose → children → Dispose → OnDispose → DISPOSED',
        s: { op: 'shutdown.Fulfill(reason) → WaitStopped() 返回', packet: null }
      },
      {
        t: '⑩ 自动休眠，等待下一次唤醒',
        d: '输入通道为空且没有子任务时循环退出，running 置回 false。下一次 AddTask 会再次 active()，整个过程重新开始。',
        src: 'event_loop.go:170 · root.go · OSSignal',
        log: 'len(ch)==0 && len(children)==0 → return · running=false',
        s: { core: false, status: 'idle', producer: false, hotCase: null, op: '等待事件…', packet: null }
      }
    ];
    return steps.filter(step => step.onlyWhen !== 'retry' || loopRetry);
  }

  function paintLoop(state, logs) {
    loopEl.producer.classList.toggle('is-hot', !!state.producer);
    loopEl.queue.forEach((slot, i) => slot.classList.toggle('is-filled', i < (state.queue || 0)));
    loopEl.core.classList.toggle('is-running', !!state.core);
    loopEl.status.textContent = state.status || 'idle';
    loopEl.status.classList.toggle('is-on', state.status === 'running');
    loopEl.select.classList.toggle('is-hot', state.hotCase !== null && state.hotCase !== undefined);
    loopEl.op.textContent = state.op || '';

    const base = mk('div', 'case-chip case-chip--base' + (state.hotCase === 0 ? ' is-hot' : ''), 'case 0 · input chan');
    loopEl.cases.replaceChildren(base);
    if ((state.cases || 1) > 1) {
      const extra = mk('div', 'case-chip' +
        (state.hotCase === 1 ? ' is-hot' : '') +
        (state.caseReplaced ? ' is-replaced' : ''),
      state.caseReplaced ? 'case 1 · signal chan ↻ replaced' : 'case 1 · signal chan');
      loopEl.cases.appendChild(extra);
    }

    loopEl.children.replaceChildren();
    if (state.child) {
      const node = mk('div', 'child-node');
      node.appendChild(mk('b', '', '*Worker · taskId=1'));
      node.appendChild(mk('small', '', 'parent: background-worker'));
      const badge = mk('span', 'child-state', state.child.state + (state.child.retry ? ` · retry ${state.child.retry}` : ''));
      badge.dataset.state = state.child.state;
      node.appendChild(badge);
      loopEl.children.appendChild(node);
    } else {
      loopEl.children.appendChild(mk('div', 'child-empty', '暂无子任务'));
    }

    if (state.packet) {
      const target = document.querySelector(state.packet);
      if (target) {
        const sceneRect = loopEl.scene.getBoundingClientRect();
        const r = target.getBoundingClientRect();
        const x = r.left - sceneRect.left + r.width / 2 - 6.5;
        const y = r.top - sceneRect.top + r.height / 2 - 6.5;
        loopEl.packet.style.transform = `translate(${Math.round(x)}px, ${Math.round(y)}px)`;
        loopEl.packet.classList.add('is-visible');
        loopEl.packet.classList.toggle('is-amber', !!state.packetAmber);
      }
    } else {
      loopEl.packet.classList.remove('is-visible');
    }
    paintLog(loopEl.log, logs);
  }

  /* ============================================================
     场景 2 · 七态生命周期
     ============================================================ */
  const fsmEl = {
    svg: $('#scene-fsm .fsm-svg'), current: $('#fsmCurrent'), method: $('#fsmMethod'),
    flags: $$('#fsmFlags span'), log: $('#fsmLog')
  };

  function fsmSteps() {
    const exec = fsmMode === 'run'
      ? { node: 'running', edge: 'e3', out: 'e4', state: 'RUNNING', method: 'Run() error · 阻塞父循环', desc: '实现 Run() 的任务在父任务事件循环内阻塞执行；父循环同时仍在 select 其它 case。' }
      : { node: 'going', edge: 'e3b', out: 'e4b', state: 'GOING', method: 'Go() error · 独立 goroutine', desc: '实现 Go() 的任务由独立 goroutine 执行，父循环立即回到 select，适合长连接与网络服务。' };
    return [
      { t: 'INIT · 任务被创建', d: 'AddTask 为任务建立 ID、Context、启动原因与双 Promise，此时状态仍是 INIT。', src: 'job.go · initContext()', log: 'state=INIT · 分配 taskId / StartReason(runtime.Caller)', s: { active: 'init', done: [], hot: null, current: 'INIT', method: 'job.AddTask(t)', flags: [false, false] } },
      { t: 'STARTING · 调用 Start()', d: 'start() 带 recover 兜底，按鸭子类型检测接口；此处调用可选的 Start() error 做资源初始化。', src: 'task.go:590 · start()', log: 'STARTING → Start() → recover 兜底', s: { active: 'starting', done: ['init'], hot: 'e1', current: 'STARTING', method: 'start() → Start()' } },
      { t: 'STARTED · 启动闸门打开', d: 'startup.Fulfill 让 WaitStarted() 返回；OnStart 钩子按代际排空，晚注册的钩子会被补跑。', src: 'task.go · drainStartHooks()', log: 'STARTED → startup.Fulfill() → OnStart hooks', s: { active: 'started', done: ['init', 'starting'], hot: 'e2', current: 'STARTED', method: 'startup.Fulfill() / OnStart', flags: [true, false] } },
      { t: `${exec.state} · 任务真正开始工作`, d: exec.desc, src: 'task.go:632 · Run() / Go() / Tick()', log: `${exec.state} → 任务执行中`, s: { active: exec.node, done: ['init', 'starting', 'started'], hot: exec.edge, current: exec.state, method: exec.method } },
      { t: 'STOP · 停止原因沿树传播', d: 'Stop(err) 幂等：CancelCauseFunc 取消 Context 并携带原因，OnStop 关联资源被排空，子任务收到信号。', src: 'task.go:311 · Stop()', log: 'Stop(reason) → CancelCauseFunc(reason) → OnStop', s: { hot: null, current: exec.state, method: 'Stop(err) → CancelCauseFunc(err)' } },
      { t: 'DISPOSING · 分阶段清理', d: 'PreDispose（等子任务前释放）→ waitChildrenDispose（级联等待）→ Dispose → resources → OnDispose。', src: 'task.go:730 · dispose()', log: 'DISPOSING → PreDispose → children → Dispose → OnDispose', s: { active: 'disposing', done: ['init', 'starting', 'started', exec.node], hot: exec.out, current: 'DISPOSING', method: 'PreDispose → waitChildrenDispose → Dispose' } },
      { t: 'DISPOSED · 销毁闸门打开', d: 'shutdown.Fulfill(reason) 让 WaitStopped() 返回；父任务等的是这一道闸门，避免启动失败路径的竞态。', src: 'util/promise.go · Fulfill()', log: 'DISPOSED → shutdown.Fulfill(reason) → WaitStopped()', s: { active: 'disposed', done: ['init', 'starting', 'started', exec.node, 'disposing'], hot: 'e5', current: 'DISPOSED', method: 'shutdown.Fulfill(reason)', flags: [true, true] } },
      { t: 'RETRY · 回到 STARTING 的新一代', d: '若启用了 SetRetry，checkRetry() 按指数退避判定后 reset() 递增 hookGen、清理旧代资源，再走一次 start()。', src: 'task.go:533 · checkRetry() / reset()', log: 'checkRetry=true → reset() (hookGen+1) → start()', s: { active: 'starting', done: [], hot: 'e6', current: 'STARTING ↻', method: 'checkRetry() → reset() → start()', flags: [false, false] } }
    ];
  }

  function paintFsm(state, logs) {
    const dimNodes = fsmMode === 'run' ? ['going'] : ['running'];
    const dimEdges = fsmMode === 'run' ? ['e3b', 'e4b'] : ['e3', 'e4'];
    $$('.fsm-node', fsmEl.svg).forEach(node => {
      const key = node.dataset.node;
      node.classList.toggle('is-active', key === state.active);
      node.classList.toggle('is-done', (state.done || []).includes(key));
      node.classList.toggle('is-dim', dimNodes.includes(key) && key !== state.active);
    });
    $$('.fsm-edge', fsmEl.svg).forEach(edge => {
      edge.classList.toggle('is-hot', edge.dataset.edge === state.hot);
      edge.classList.toggle('is-dim', dimEdges.includes(edge.dataset.edge) && edge.dataset.edge !== state.hot);
    });
    fsmEl.current.textContent = state.current || 'INIT';
    fsmEl.method.textContent = state.method || '';
    const flags = state.flags || [false, false];
    fsmEl.flags[0].textContent = flags[0] ? 'startup ✓ fulfilled' : 'startup ⧗ pending';
    fsmEl.flags[1].textContent = flags[1] ? 'shutdown ✓ fulfilled' : 'shutdown ⧗ pending';
    fsmEl.flags.forEach((f, i) => f.classList.toggle('is-fulfilled', !!flags[i]));
    paintLog(fsmEl.log, logs);
  }

  /* ============================================================
     场景 3 · 级联销毁
     ============================================================ */
  const cascadeEl = {
    tree: $('#cascadeTree'), order: $('#cascadeOrder'), log: $('#cascadeLog')
  };
  const NODE_ORDER = { ossignal: 1, task4: 2, task5: 3, task6: 4, long: 5, job: 6, work: 7, root: 8 };
  const ORDER_LINES = [
    'OSSignal · Stop(ErrExit)',
    'demo-task-4 · DISPOSED',
    'demo-task-5 · DISPOSED',
    'demo-task-6 · DISPOSED',
    'LongRunningTask · DISPOSED',
    'background-worker · DISPOSED',
    'KeepaliveTask · DISPOSED',
    'RootManager · shutdown.Fulfill'
  ];

  const liveMap = () => ({
    root: 'live', ossignal: 'live', job: 'live', task4: 'live',
    task5: 'live', task6: 'live', work: 'live', long: 'live'
  });

  function cascadeSteps() {
    return [
      { t: '任务树就绪', d: 'RootManager.Init() 建立根并注册 OSSignal 任务；Job、Work 与 TASK 形成三级树。', src: 'root.go · Init()', log: 'RootManager.Init() → 任务树就绪 (8 个节点)', s: { nodes: liveMap(), orderCount: 0 } },
      { t: '① OS 信号触发优雅关闭', d: 'OSSignal 捕获 SIGHUP / SIGINT / SIGTERM / SIGQUIT，Tick 中调用 RootManager.Shutdown()。', src: 'root.go · OSSignal.Tick() / Shutdown()', log: 'SIGTERM → OSSignal.Tick → root.Shutdown() → Stop(ErrExit)', s: { nodes: { ...liveMap(), root: 'signal' }, orderCount: 0 } },
      { t: '② 停止原因沿树向下传播', d: 'CancelCauseFunc 把 Context 与停止原因传给所有子任务；父任务进入等待状态，不抢先释放资源。', src: 'task.go · Stop() / job.go · AddDependTask()', log: 'CancelCauseFunc(ErrExit) · 停止原因逐层下传', s: { nodes: { root: 'wait', ossignal: 'signal', job: 'signal', work: 'signal', task4: 'wait', task5: 'wait', task6: 'wait', long: 'wait' }, orderCount: 0 } },
      { t: '③ 叶子节点先完成分阶段清理', d: '每个子任务依次 PreDispose → 等待自己的子任务 → Dispose → resources（io.Closer / Recyclable）→ OnDispose。', src: 'task.go:730 · dispose()', log: '叶子节点：PreDispose → Dispose → resources → OnDispose → DISPOSED', s: { nodes: { root: 'wait', ossignal: 'done', job: 'wait', work: 'wait', task4: 'done', task5: 'done', task6: 'done', long: 'done' }, orderCount: 5 } },
      { t: '④ 父容器等待全部子任务', d: 'waitChildrenDispose 逐个 Stop 并等待子任务的 shutdown Promise；停滞超过 30s 会打印告警。', src: 'job.go:107 · waitChildrenDispose()', log: 'waitChildrenDispose() → 逐个等待 shutdown · Job/Work 完成销毁', s: { nodes: { root: 'wait', ossignal: 'done', job: 'done', work: 'done', task4: 'done', task5: 'done', task6: 'done', long: 'done' }, orderCount: 7 } },
      { t: '⑤ 根节点收尾，进程优雅退出', d: 'shutdown.Fulfill(ErrExit) 唤醒所有等待者；任务树自底向上完成，没有资源被遗漏。', src: 'root.go:62 · Shutdown()', log: 'shutdown.Fulfill(ErrExit) → RootManager DISPOSED → 优雅退出', s: { nodes: { root: 'done', ossignal: 'done', job: 'done', work: 'done', task4: 'done', task5: 'done', task6: 'done', long: 'done' }, orderCount: 8 } }
    ];
  }

  function paintCascade(state, logs) {
    Object.entries(state.nodes || {}).forEach(([key, value]) => {
      const node = cascadeEl.tree.querySelector(`[data-node="${key}"]`);
      if (!node) return;
      node.dataset.state = value;
      const badge = node.querySelector('.tree-badge');
      badge.textContent = value === 'done' ? String(NODE_ORDER[key]) : '';
      node.style.transitionDelay = `${(2 - Number(node.dataset.depth)) * 110}ms`;
    });
    cascadeEl.order.replaceChildren();
    ORDER_LINES.slice(0, state.orderCount || 0).forEach((text, i) => {
      const li = mk('li');
      li.appendChild(mk('b', '', `#${i + 1}`));
      li.appendChild(document.createTextNode(text));
      cascadeEl.order.appendChild(li);
    });
    paintLog(cascadeEl.log, logs);
  }

  /* ============================================================
     场景 4 · 指数退避重试
     ============================================================ */
  const retryEl = {
    count: $('#retryCount'), label: $('.retry-counter span:last-child'),
    attempts: $$('.retry-attempt'), result: $('#retryResult'), log: $('#retryLog')
  };
  const FILLS = [6, 14, 28, 52];
  const STATE_TEXT = { idle: '待执行', fail: '失败', wait: '退避中', run: '执行中', success: '成功', exhaust: '终止' };

  function retrySnapshot(count, states) {
    const attempts = {};
    const fills = {};
    states.forEach((value, i) => {
      attempts[i + 1] = value;
      fills[i + 1] = value === 'idle' ? 0 : FILLS[i];
    });
    return { count, attempts, fills, result: '', resultClass: '' };
  }

  function retrySteps() {
    const success = retryOutcome === 'success';
    const step4 = {
      t: success ? '④ 第 4 次启动成功' : '④ 第 4 次仍失败',
      d: success
        ? '退避 4s 后 reset() + start() 成功，任务回到 RUNNING；retryCount 作为可观测字段保留，可随时 ResetRetryCount。'
        : '若 MaxRetry 设为 3，第 4 次仍失败时 checkRetry() 返回 false，不再安排下一轮退避。',
      src: 'task.go · checkRetry() / ResetRetryCount()',
      log: success ? '#4 start() → 成功 · state=RUNNING' : '#4 失败 · checkRetry()=false（MaxRetry 用尽）',
      s: retrySnapshot(3, ['fail', 'fail', 'fail', success ? 'success' : 'exhaust'])
    };
    const step5 = {
      t: success ? '结论：故障被自动吸收' : '结论：以明确原因收敛',
      d: success
        ? '重试策略让瞬时故障无需人工介入；父任务未停止之前，循环会继续守护这个子任务。'
        : '任务以 ErrRetryRunOut 进入 DISPOSING，走完分阶段清理后 DISPOSED，停止原因可被面板与日志追溯。',
      src: success ? 'task.go · SetRetry()' : 'task.go · ErrRetryRunOut / dispose()',
      log: success ? '✓ 任务恢复运行，重试计数可用于监控告警' : '× ErrRetryRunOut → DISPOSING → DISPOSED',
      s: {
        ...retrySnapshot(3, ['fail', 'fail', 'fail', success ? 'success' : 'exhaust']),
        result: success ? '✓ 第 4 次启动成功 · 任务保持 RUNNING，retryCount 保留供观测' : '× MaxRetry 用尽 → ErrRetryRunOut → DISPOSING → DISPOSED',
        resultClass: success ? 'is-success' : 'is-fail'
      }
    };
    return [
      { t: '① 首次执行失败', d: 'Run() 返回错误。父循环捕获信号后调用 checkRetry()，先排除 ErrTaskComplete / ErrExit / ErrStopByUser 等终止原因。', src: 'task.go:533 · checkRetry()', log: '#1 Run() → error · 进入 checkRetry()', s: retrySnapshot(0, ['fail', 'idle', 'idle', 'idle']) },
      { t: '② 判定可以重试', d: 'interval = base × 2^(retryCount-1)，位运算实现指数，MaxRetryInterval 封顶；MaxRetry < 0 表示无限重试。', src: 'task.go:533 · 指数退避', log: 'checkRetry()=true · interval = 1s × 2⁰ = 1s', s: retrySnapshot(0, ['fail', 'wait', 'idle', 'idle']) },
      { t: '③ 退避 1s 后重启，再次失败', d: 'reset() 递增 hookGen、清理上一代钩子与资源，再走一次完整 start()；新一轮退避变为 2s。', src: 'task.go:667 · reset()', log: '#2 reset()+start() → 失败 · 下一轮退避 2s', s: retrySnapshot(1, ['fail', 'fail', 'wait', 'idle']) },
      { t: '③ 退避 2s 后重启，第三次失败', d: '每一代重试都重新经过 STARTING / STARTED，钩子代际护栏确保监听器不丢失、不串台。', src: 'task.go · hookGen', log: '#3 失败 · 下一轮退避 4s · hookGen+1', s: retrySnapshot(2, ['fail', 'fail', 'fail', 'wait']) },
      step4,
      step5
    ];
  }

  function paintRetry(state, logs) {
    retryEl.count.textContent = String(state.count || 0);
    retryEl.label.textContent = retryOutcome === 'success' ? '/ MaxRetry 5' : '/ MaxRetry 3';
    retryEl.attempts.forEach(card => {
      const key = Number(card.dataset.attempt);
      const value = (state.attempts || {})[key] || 'idle';
      card.dataset.state = value;
      card.querySelector('.attempt-state').textContent = STATE_TEXT[value] || value;
      card.querySelector('.attempt-fill').style.width = `${(state.fills || {})[key] || 0}%`;
    });
    retryEl.result.textContent = state.result || '点击播放，观察退避过程 →';
    retryEl.result.classList.toggle('is-success', state.resultClass === 'is-success');
    retryEl.result.classList.toggle('is-fail', state.resultClass === 'is-fail');
    paintLog(retryEl.log, logs);
  }

  /* ============================================================
     通用播放器
     ============================================================ */
  class Player {
    constructor(root, config) {
      this.root = root;
      this.config = config;
      this.baseInterval = config.interval || 2500;
      this.speed = 1;
      this.index = 0;
      this.playing = false;
      this.wanted = !reduced;
      this.timer = null;
      this.build();
      this.refresh(config.getSteps(), 0);
    }

    build() {
      const controls = mk('div', 'player-controls');
      this.toggleBtn = mk('button', 'pbtn pbtn--play', '▶');
      this.toggleBtn.type = 'button';
      this.toggleBtn.setAttribute('aria-label', '播放');
      this.toggleBtn.addEventListener('click', () => {
        this.wanted = !this.playing;
        if (this.playing) this.pause(); else this.play();
      });
      const prev = mk('button', 'pbtn', '‹');
      const next = mk('button', 'pbtn', '›');
      const reset = mk('button', 'pbtn', '↺');
      [prev, next, reset].forEach(b => { b.type = 'button'; });
      prev.setAttribute('aria-label', '上一步');
      next.setAttribute('aria-label', '下一步');
      reset.setAttribute('aria-label', '重置');
      prev.addEventListener('click', () => this.stepTo(this.index - 1));
      next.addEventListener('click', () => this.stepTo(this.index + 1));
      reset.addEventListener('click', () => this.stepTo(0));

      const speed = mk('div', 'player-speed');
      this.speedBtns = [1, 2].map(value => {
        const b = mk('button', value === 1 ? 'is-active' : '', `${value}×`);
        b.type = 'button';
        b.addEventListener('click', () => {
          this.speed = value;
          this.speedBtns.forEach((btn, i) => btn.classList.toggle('is-active', [1, 2][i] === value));
          if (this.playing) this.schedule();
        });
        return b;
      });
      speed.append(...this.speedBtns);
      const hint = mk('span', 'player-hint', '点击圆点跳转 · ← → 单步');
      controls.append(this.toggleBtn, prev, next, reset, speed, hint);

      const track = mk('div', 'player-track');
      this.fill = mk('div', 'player-track-fill');
      this.dots = mk('div', 'player-dots');
      track.append(this.fill, this.dots);

      const caption = mk('div', 'player-caption');
      const meta = mk('div', 'caption-meta');
      this.indexEl = mk('span', 'caption-index');
      this.srcEl = mk('span', 'caption-src');
      meta.append(this.indexEl, this.srcEl);
      this.titleEl = mk('h4', 'caption-title');
      this.descEl = mk('p', 'caption-desc');
      caption.append(meta, this.titleEl, this.descEl);

      this.root.replaceChildren(controls, track, caption);
      this.root.tabIndex = 0;
      this.root.setAttribute('aria-label', '动画播放器，可用左右方向键单步查看');
      this.root.addEventListener('keydown', e => {
        if (e.key === 'ArrowLeft') { e.preventDefault(); this.stepTo(this.index - 1); }
        if (e.key === 'ArrowRight') { e.preventDefault(); this.stepTo(this.index + 1); }
      });
    }

    refresh(steps, index = 0) {
      this.pause();
      this.steps = steps;
      this.dots.replaceChildren();
      this.stepBtns = steps.map((_, i) => {
        const b = mk('button', 'player-dot');
        b.type = 'button';
        b.setAttribute('aria-label', `跳转到第 ${i + 1} 步`);
        b.addEventListener('click', () => this.stepTo(i));
        return b;
      });
      this.dots.append(...this.stepBtns);
      this.go(Math.max(0, Math.min(index, steps.length - 1)));
    }

    render(index) {
      const state = {};
      const logs = [];
      for (let i = 0; i <= index; i += 1) {
        Object.assign(state, this.steps[i].s || {});
        if (this.steps[i].log) logs.push(this.steps[i].log);
      }
      this.config.paint(state, logs);
      const step = this.steps[index];
      this.indexEl.textContent = `${pad(index + 1)} / ${pad(this.steps.length)}`;
      this.srcEl.textContent = step.src || '';
      this.titleEl.textContent = step.t;
      this.descEl.textContent = step.d;
      this.fill.style.width = this.steps.length > 1 ? `${(index / (this.steps.length - 1)) * 100}%` : '100%';
      this.stepBtns.forEach((b, i) => {
        b.classList.toggle('is-done', i < index);
        b.classList.toggle('is-active', i === index);
      });
    }

    go(index) {
      this.index = Math.max(0, Math.min(index, this.steps.length - 1));
      this.render(this.index);
    }

    stepTo(index) {
      this.wanted = false;
      this.pause();
      this.go(index);
    }

    play() {
      if (this.playing) return;
      if (this.index >= this.steps.length - 1) this.go(0);
      this.playing = true;
      this.toggleBtn.textContent = '❚❚';
      this.toggleBtn.setAttribute('aria-label', '暂停');
      this.schedule();
    }

    pause() {
      this.playing = false;
      if (this.timer) { window.clearTimeout(this.timer); this.timer = null; }
      if (this.toggleBtn) {
        this.toggleBtn.textContent = '▶';
        this.toggleBtn.setAttribute('aria-label', '播放');
      }
    }

    schedule() {
      if (this.timer) window.clearTimeout(this.timer);
      if (!this.playing) return;
      const atEnd = this.index >= this.steps.length - 1;
      const delay = atEnd ? 2000 : this.baseInterval / this.speed;
      this.timer = window.setTimeout(() => {
        if (!this.playing) return;
        this.go(atEnd ? 0 : this.index + 1);
        this.schedule();
      }, delay);
    }
  }

  /* ============================================================
     装配与联动
     ============================================================ */
  let loopRetry = $('#loopRetry').checked;
  let fsmMode = 'run';
  let retryOutcome = 'success';

  const configs = {
    loop: { getSteps: loopSteps, paint: paintLoop, interval: 2600 },
    fsm: { getSteps: fsmSteps, paint: paintFsm, interval: 2500 },
    cascade: { getSteps: cascadeSteps, paint: paintCascade, interval: 2700 },
    retry: { getSteps: retrySteps, paint: paintRetry, interval: 2400 }
  };

  const players = {};
  Object.keys(configs).forEach(id => {
    const root = $(`[data-player="${id}"]`);
    if (root) players[id] = new Player(root, configs[id]);
  });

  let activeId = 'loop';
  let labVisible = false;

  const sync = () => {
    Object.entries(players).forEach(([id, player]) => {
      if (id === activeId && labVisible && player.wanted) player.play();
      else player.pause();
    });
  };

  const refreshPlayer = id => {
    const player = players[id];
    if (!player) return;
    const wasWanted = player.wanted;
    player.refresh(configs[id].getSteps(), 0);
    player.wanted = wasWanted;
    sync();
  };

  const activate = id => {
    activeId = id;
    $$('.lab-tab').forEach(tab => {
      const active = tab.dataset.tab === id;
      tab.classList.toggle('is-active', active);
      tab.setAttribute('aria-selected', String(active));
    });
    $$('[data-panel]').forEach(panel => {
      const active = panel.dataset.panel === id;
      panel.classList.toggle('is-active', active);
      panel.hidden = !active;
    });
    if (players[id]) {
      players[id].wanted = !reduced;
      players[id].render(players[id].index);
    }
    sync();
  };
  $$('.lab-tab').forEach(tab => tab.addEventListener('click', () => activate(tab.dataset.tab)));

  $('#loopRetry').addEventListener('change', e => {
    loopRetry = e.target.checked;
    refreshPlayer('loop');
  });
  $$('#fsmMode button').forEach(btn => btn.addEventListener('click', () => {
    fsmMode = btn.dataset.mode;
    $$('#fsmMode button').forEach(b => b.classList.toggle('is-active', b === btn));
    refreshPlayer('fsm');
  }));
  $$('#retryOutcome button').forEach(btn => btn.addEventListener('click', () => {
    retryOutcome = btn.dataset.outcome;
    $$('#retryOutcome button').forEach(b => b.classList.toggle('is-active', b === btn));
    refreshPlayer('retry');
  }));
  $('#cascadeTrigger').addEventListener('click', () => {
    const player = players.cascade;
    if (!player) return;
    player.wanted = true;
    player.go(0);
    player.play();
  });

  const labShell = $('.lab-shell');
  if ('IntersectionObserver' in window) {
    const labObserver = new IntersectionObserver(entries => {
      labVisible = entries.some(e => e.isIntersecting);
      sync();
    }, { threshold: 0.18 });
    labObserver.observe(labShell);
  } else {
    labVisible = true;
    sync();
  }
})();
