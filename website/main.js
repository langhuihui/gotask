/* GoTask 官网 · 全局交互 */
(() => {
  'use strict';
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

  /* ---------- 滚动进度 / Header / 回到顶部 ---------- */
  const progress = $('#scrollProgress');
  const header = $('#siteHeader');
  const toTop = $('#toTop');
  const onScroll = () => {
    const max = document.documentElement.scrollHeight - window.innerHeight;
    const ratio = max > 0 ? window.scrollY / max : 0;
    progress.style.width = `${(ratio * 100).toFixed(2)}%`;
    header.classList.toggle('is-scrolled', window.scrollY > 12);
    toTop.classList.toggle('is-visible', window.scrollY > 700);
  };
  window.addEventListener('scroll', onScroll, { passive: true });
  onScroll();
  toTop.addEventListener('click', () => window.scrollTo({ top: 0, behavior: reduced ? 'auto' : 'smooth' }));

  /* ---------- 移动端菜单 ---------- */
  const navToggle = $('#navToggle');
  const nav = $('#mainNav');
  navToggle.addEventListener('click', () => {
    const open = nav.classList.toggle('is-open');
    navToggle.setAttribute('aria-expanded', String(open));
    navToggle.setAttribute('aria-label', open ? '关闭菜单' : '打开菜单');
  });
  $$('.nav-link', nav).forEach(link => link.addEventListener('click', () => {
    nav.classList.remove('is-open');
    navToggle.setAttribute('aria-expanded', 'false');
  }));

  /* ---------- 导航高亮 ---------- */
  const navLinks = $$('.nav-link');
  const spy = new IntersectionObserver(entries => {
    entries.forEach(entry => {
      if (!entry.isIntersecting) return;
      navLinks.forEach(link => link.classList.toggle('is-active', link.getAttribute('href') === `#${entry.target.id}`));
    });
  }, { rootMargin: '-38% 0px -55% 0px' });
  ['why', 'lab', 'features', 'start', 'observability'].forEach(id => {
    const section = document.getElementById(id);
    if (section) spy.observe(section);
  });

  /* ---------- 滚动入场 + 错峰 ---------- */
  const revealGroups = new Map();
  $$('.reveal').forEach(el => {
    const parent = el.parentElement;
    const list = revealGroups.get(parent) || [];
    list.push(el);
    revealGroups.set(parent, list);
  });
  revealGroups.forEach(list => list.forEach((el, i) => { el.style.transitionDelay = `${Math.min(i, 6) * 70}ms`; }));

  if (reduced || !('IntersectionObserver' in window)) {
    $$('.reveal').forEach(el => el.classList.add('is-visible'));
  } else {
    const revealObserver = new IntersectionObserver(entries => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          entry.target.classList.add('is-visible');
          revealObserver.unobserve(entry.target);
        }
      });
    }, { threshold: 0.12, rootMargin: '0px 0px -40px 0px' });
    $$('.reveal').forEach(el => revealObserver.observe(el));
  }

  /* ---------- 数字滚动 ---------- */
  const counters = $$('[data-count]');
  const animateCount = el => {
    const target = Number(el.dataset.count);
    if (reduced || target === 0) { el.textContent = String(target); return; }
    const duration = 1100;
    const start = performance.now();
    const tick = now => {
      const t = Math.min((now - start) / duration, 1);
      const eased = 1 - Math.pow(1 - t, 3);
      el.textContent = String(Math.round(target * eased));
      if (t < 1) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  };
  if ('IntersectionObserver' in window) {
    const countObserver = new IntersectionObserver(entries => {
      entries.forEach(entry => {
        if (entry.isIntersecting) { animateCount(entry.target); countObserver.unobserve(entry.target); }
      });
    }, { threshold: 0.6 });
    counters.forEach(el => countObserver.observe(el));
  } else {
    counters.forEach(animateCount);
  }

  /* ---------- 复制 ---------- */
  const copyText = async text => {
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        return true;
      }
    } catch (_) { /* 回退到 execCommand */ }
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    let ok = false;
    try { ok = document.execCommand('copy'); } catch (_) { ok = false; }
    document.body.removeChild(ta);
    return ok;
  };
  $$('.copy-btn').forEach(btn => btn.addEventListener('click', async () => {
    let text = btn.dataset.copy;
    if (btn.dataset.copyTarget) {
      const target = document.getElementById(btn.dataset.copyTarget);
      if (target) text = target.textContent;
    }
    if (!text) return;
    const ok = await copyText(text);
    const original = btn.textContent;
    btn.textContent = ok ? '已复制' : '复制失败';
    btn.classList.toggle('is-copied', ok);
    window.setTimeout(() => { btn.textContent = original; btn.classList.remove('is-copied'); }, 1600);
  }));

  /* ---------- 代码 Tab ---------- */
  const codeCopyMap = { basic: 'codeBasic', generic: 'codeGeneric', hooks: 'codeHooks' };
  const windowCopy = $('.copy-btn--window');
  $$('.code-tab').forEach(tab => tab.addEventListener('click', () => {
    const key = tab.dataset.code;
    $$('.code-tab').forEach(t => {
      const active = t === tab;
      t.classList.toggle('is-active', active);
      t.setAttribute('aria-selected', String(active));
    });
    $$('[data-code-panel]').forEach(panel => {
      const active = panel.dataset.codePanel === key;
      panel.classList.toggle('is-active', active);
      panel.hidden = !active;
    });
    if (windowCopy) windowCopy.dataset.copyTarget = codeCopyMap[key];
  }));

  /* ---------- 终端逐行输出 ---------- */
  const terminal = $('#terminal');
  const terminalLines = [
    [['$ go run ./cmd/demo', 't-dim']],
    [['INFO ', 't-blue'], ['taskId=1 owner=*main.Worker state=STARTING', '']],
    [['INFO ', 't-blue'], ['taskId=1 state=STARTED  startup.Fulfill', '']],
    [['INFO ', 't-blue'], ['taskId=1 state=RUNNING  worker ready', 't-teal']],
    [['^C  ', 't-amber'], ['SIGINT → OSSignal.Tick → Shutdown()', '']],
    [['INFO ', 't-blue'], ['taskId=1 state=DISPOSING  preDispose', '']],
    [['INFO ', 't-blue'], ['taskId=1 state=DISPOSED  reason=ErrExit', 't-teal']],
    [['✓ ', 't-teal'], ['资源全部释放，进程优雅退出', 't-dim']]
  ];
  const appendLine = parts => {
    const line = document.createElement('div');
    line.className = 'terminal-line';
    parts.forEach(([text, cls]) => {
      if (!cls) { line.appendChild(document.createTextNode(text)); return; }
      const span = document.createElement('span');
      span.className = cls;
      span.textContent = text;
      line.appendChild(span);
    });
    terminal.appendChild(line);
  };
  const cursor = () => {
    const c = document.createElement('span');
    c.className = 'terminal-cursor';
    terminal.appendChild(c);
    return c;
  };
  let terminalStarted = false;
  const runTerminal = () => {
    if (terminalStarted) return;
    terminalStarted = true;
    if (reduced) { terminalLines.forEach(appendLine); return; }
    let c = cursor();
    let i = 0;
    const next = () => {
      if (i >= terminalLines.length) { c.remove(); return; }
      c.remove();
      appendLine(terminalLines[i]);
      c = cursor();
      i += 1;
      window.setTimeout(next, i === terminalLines.length ? 900 : 480);
    };
    window.setTimeout(next, 400);
  };
  if ('IntersectionObserver' in window) {
    const termObserver = new IntersectionObserver(entries => {
      if (entries.some(e => e.isIntersecting)) { runTerminal(); termObserver.disconnect(); }
    }, { threshold: 0.25 });
    termObserver.observe(terminal);
  } else {
    runTerminal();
  }

  /* ---------- Hero 状态轮播 ---------- */
  const heroState = $('#heroState');
  if (heroState && !reduced) {
    const states = ['RUNNING', 'GOING', 'STARTING', 'DISPOSING', 'RUNNING'];
    let idx = 0;
    window.setInterval(() => {
      idx = (idx + 1) % states.length;
      heroState.textContent = states[idx];
    }, 2600);
  }

  /* ---------- 特性卡片光标光晕 ---------- */
  $$('.feature-card').forEach(card => card.addEventListener('pointermove', e => {
    const rect = card.getBoundingClientRect();
    card.style.setProperty('--mx', `${e.clientX - rect.left}px`);
    card.style.setProperty('--my', `${e.clientY - rect.top}px`);
  }));
})();
