import './style.css';
import { pinixInvoke, pinixInvokeStream } from './bridge.js';

// ---------- 状态 ----------
let topics = [];
let currentTopicId = null;
let runs = [];
let isSending = false;

// ---------- DOM 渲染 ----------

function render() {
  const app = document.getElementById('app');
  app.innerHTML = `
    <header class="border-b border-border px-6 py-3 font-display">
      <h1 class="text-2xl font-bold tracking-tight">agents, assembled.</h1>
    </header>
    <div class="flex flex-1 min-h-0">
      <!-- 左侧 Topics 列表 -->
      <aside class="w-56 shrink-0 border-r border-border flex flex-col bg-sidebar text-sidebar-foreground">
        <div class="p-3 border-b border-sidebar-border">
          <button id="btn-new-topic"
            class="w-full px-3 py-1.5 text-sm font-medium bg-primary text-primary-foreground hover:opacity-90 transition-opacity duration-150">
            + New Topic
          </button>
        </div>
        <div id="topic-list" class="flex-1 overflow-y-auto">
          ${renderTopicList()}
        </div>
      </aside>

      <!-- 右侧对话区 -->
      <main class="flex-1 flex flex-col min-w-0">
        ${currentTopicId ? renderChat() : renderEmpty()}
      </main>
    </div>
  `;

  bindEvents();
}

function renderTopicList() {
  if (topics.length === 0) {
    return `<p class="p-3 text-sm text-muted-foreground">暂无话题</p>`;
  }
  return topics.map(t => `
    <button
      class="topic-item w-full text-left px-3 py-2 text-sm border-b border-sidebar-border hover:bg-sidebar-accent transition-colors duration-150 ${t.id === currentTopicId ? 'bg-sidebar-accent font-medium' : ''}"
      data-id="${t.id}">
      ${escapeHtml(t.name)}
    </button>
  `).join('');
}

function renderEmpty() {
  return `
    <div class="flex-1 flex items-center justify-center text-muted-foreground text-sm">
      选择一个话题或创建新话题
    </div>
  `;
}

function renderChat() {
  const topic = topics.find(t => t.id === currentTopicId);
  const topicName = topic ? topic.name : '';

  return `
    <div class="px-6 py-3 border-b border-border">
      <h2 class="text-lg font-display font-bold">${escapeHtml(topicName)}</h2>
    </div>
    <div id="messages" class="flex-1 overflow-y-auto px-6 py-4 space-y-4">
      ${renderMessages()}
    </div>
    <div class="border-t border-border px-6 py-3">
      <form id="send-form" class="flex gap-2">
        <input
          id="msg-input"
          type="text"
          placeholder="输入消息..."
          class="flex-1 px-3 py-2 border border-input bg-background text-foreground text-sm outline-none focus:border-foreground transition-colors duration-150"
          ${isSending ? 'disabled' : ''}
          autocomplete="off" />
        <button
          type="submit"
          class="px-4 py-2 text-sm font-medium bg-primary text-primary-foreground hover:opacity-90 transition-opacity duration-150 disabled:opacity-50"
          ${isSending ? 'disabled' : ''}>
          ${isSending ? '...' : '发送'}
        </button>
      </form>
    </div>
  `;
}

function renderMessages() {
  if (runs.length === 0) {
    return `<p class="text-sm text-muted-foreground">暂无消息，发一条开始对话</p>`;
  }

  return runs.map(run => {
    let parts = '';

    // 用户消息
    if (run.user_message) {
      parts += `
        <div class="border-b border-border pb-3">
          <div class="text-xs text-muted-foreground font-mono mb-1">user</div>
          <div class="text-sm whitespace-pre-wrap">${escapeHtml(run.user_message)}</div>
        </div>
      `;
    }

    // agent 回复（从 messages JSON 或 summary）
    const agentText = getAgentResponse(run);
    if (agentText) {
      parts += `
        <div class="border-b border-border pb-3">
          <div class="text-xs text-muted-foreground font-mono mb-1">agent</div>
          <div class="text-sm whitespace-pre-wrap">${escapeHtml(agentText)}</div>
        </div>
      `;
    }

    // 流式回复占位
    if (run.id === '__streaming__') {
      parts += `
        <div class="border-b border-border pb-3">
          <div class="text-xs text-muted-foreground font-mono mb-1">agent</div>
          <div id="streaming-text" class="text-sm whitespace-pre-wrap">${escapeHtml(run._streamText || '')}<span class="inline-block w-1.5 h-4 bg-foreground animate-pulse ml-0.5 align-text-bottom"></span></div>
        </div>
      `;
    }

    return parts;
  }).join('');
}

function getAgentResponse(run) {
  if (run.messages) {
    try {
      const msgs = JSON.parse(run.messages);
      const assistant = msgs.find(m => m.role === 'assistant');
      if (assistant) return assistant.content;
    } catch {}
  }
  if (run.summary && run.status === 'completed') {
    return run.summary;
  }
  return null;
}

// ---------- 事件绑定 ----------

function bindEvents() {
  // 新建话题
  const btnNew = document.getElementById('btn-new-topic');
  if (btnNew) {
    btnNew.addEventListener('click', async () => {
      const name = prompt('话题名称：');
      if (!name) return;
      await createTopic(name);
    });
  }

  // 点击话题
  document.querySelectorAll('.topic-item').forEach(el => {
    el.addEventListener('click', () => {
      currentTopicId = el.dataset.id;
      loadRuns();
    });
  });

  // 发送消息
  const form = document.getElementById('send-form');
  if (form) {
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const input = document.getElementById('msg-input');
      const message = input.value.trim();
      if (!message || isSending) return;
      input.value = '';
      await sendMessage(message);
    });
  }
}

// ---------- 数据操作 ----------

async function loadTopics() {
  try {
    const result = await pinixInvoke('list-topics');
    const stdout = result.stdout || result;
    topics = JSON.parse(typeof stdout === 'string' ? stdout : JSON.stringify(stdout));
  } catch (e) {
    console.error('loadTopics:', e);
    topics = [];
  }
  render();
}

async function createTopic(name) {
  try {
    const result = await pinixInvoke('create-topic', JSON.stringify({ name }));
    const stdout = result.stdout || result;
    const data = JSON.parse(typeof stdout === 'string' ? stdout : JSON.stringify(stdout));
    currentTopicId = data.topic_id;
    await loadTopics();
    await loadRuns();
  } catch (e) {
    console.error('createTopic:', e);
  }
}

async function loadRuns() {
  if (!currentTopicId) return;
  try {
    const result = await pinixInvoke('get-runs', JSON.stringify({ topic_id: currentTopicId }));
    const stdout = result.stdout || result;
    runs = JSON.parse(typeof stdout === 'string' ? stdout : JSON.stringify(stdout));
  } catch (e) {
    console.error('loadRuns:', e);
    runs = [];
  }
  render();
  scrollToBottom();
}

async function sendMessage(message) {
  if (!currentTopicId) return;
  isSending = true;

  // 添加用户消息 + 流式占位
  runs.push({
    id: '__streaming__',
    user_message: message,
    status: 'in_progress',
    _streamText: '',
  });
  render();
  scrollToBottom();

  let streamBuffer = '';

  pinixInvokeStream(
    'send-message',
    JSON.stringify({ topic_id: currentTopicId, message }),
    (chunk) => {
      streamBuffer += chunk;
      // 更新流式文本
      const streamEl = document.getElementById('streaming-text');
      if (streamEl) {
        streamEl.innerHTML = escapeHtml(streamBuffer) +
          '<span class="inline-block w-1.5 h-4 bg-foreground animate-pulse ml-0.5 align-text-bottom"></span>';
        scrollToBottom();
      }
    },
    (exitCode) => {
      isSending = false;
      // 重新加载完整的 runs
      loadRuns();
    }
  );
}

function scrollToBottom() {
  const el = document.getElementById('messages');
  if (el) {
    requestAnimationFrame(() => {
      el.scrollTop = el.scrollHeight;
    });
  }
}

// ---------- 工具 ----------

function escapeHtml(str) {
  if (!str) return '';
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

// ---------- 初始化 ----------
loadTopics();
