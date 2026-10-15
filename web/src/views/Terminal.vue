<template>
  <div class="terminal-page">
    <div v-if="sshSaveSuccess" class="toast toast-success">{{ sshSaveSuccess }}</div>
    <div class="terminal-toolbar">
      <router-link to="/" class="back-link">← 返回首页</router-link>
      <div class="terminal-tabs">
        <div
          v-for="tab in tabs"
          :key="tab.id"
          role="button"
          tabindex="0"
          class="terminal-tab"
          :class="{ 'terminal-tab-active': tab.id === activeId }"
          @click="switchTab(tab.id)"
          @keydown.enter.space.prevent="switchTab(tab.id)"
        >
          <span class="terminal-tab-name">{{ tab.name }}</span>
          <button
            type="button"
            class="terminal-tab-close"
            aria-label="关闭"
            @click.stop="closeTab(tab.id)"
          >
            ×
          </button>
        </div>
        <button type="button" class="terminal-tab-add" aria-label="新建终端" @click="addTab">
          +
        </button>
      </div>
      <label class="terminal-host-label">
        连接
        <select v-model="selectedHostId" class="terminal-host-select">
          <option value="">本地</option>
          <option v-for="h in sshHosts" :key="h.id" :value="h.id">{{ h.name }} ({{ h.host }})</option>
        </select>
      </label>
      <button type="button" class="btn btn-sm" @click="openSshManage">管理 SSH 连接</button>
      <span v-if="activeTab && activeTab.status" class="status" :class="activeTab.status">
        {{ activeTab.statusText }}
      </span>
    </div>
    <div v-if="tabs.length" ref="terminalContainer" class="terminal-container"></div>
    <div v-else class="terminal-empty">
      <p>暂无终端，选择连接后点击「+」新建</p>
    </div>

    <!-- SSH 连接管理弹层 -->
    <div v-if="sshModalOpen" class="modal-overlay" @click.self="closeSshModal">
      <div class="modal-box modal-ssh">
        <h3>SSH 连接管理</h3>
        <div class="ssh-list">
          <div v-for="h in sshHosts" :key="h.id" class="ssh-list-item">
            <span class="ssh-list-name">{{ h.name }}</span>
            <span class="ssh-list-addr">{{ h.user }}@{{ h.host }}:{{ h.port || 22 }}</span>
            <div class="ssh-list-actions">
              <button type="button" class="btn btn-sm" @click="editSshHost(h)">编辑</button>
              <button type="button" class="btn btn-sm btn-danger" @click="deleteSshHost(h.id)">删除</button>
            </div>
          </div>
          <p v-if="!sshHosts.length" class="ssh-list-empty">暂无 SSH 连接，点击下方添加</p>
        </div>
        <form class="ssh-form" @submit.prevent="saveSshHost">
          <h4>{{ editingSshId ? '编辑连接' : '添加连接' }}</h4>
          <label>名称</label>
          <input v-model="sshForm.name" type="text" required placeholder="如：生产服务器" />
          <label>主机</label>
          <input v-model="sshForm.host" type="text" required placeholder="IP 或域名" />
          <label>端口</label>
          <input v-model.number="sshForm.port" type="number" placeholder="22" />
          <label>用户名</label>
          <input v-model="sshForm.user" type="text" required placeholder="root" />
          <label>认证方式</label>
          <select v-model="sshForm.auth_type">
            <option value="password">密码</option>
            <option value="key">证书（私钥）</option>
          </select>
          <template v-if="sshForm.auth_type === 'password'">
            <label>密码</label>
            <input v-model="sshForm.password" type="password" :placeholder="editingSshId ? '留空则不修改' : ''" />
          </template>
          <template v-else>
            <label>私钥路径（服务器上的路径）</label>
            <input v-model="sshForm.private_key_path" type="text" placeholder="/home/user/.ssh/id_rsa" />
            <label>或粘贴私钥内容</label>
            <textarea v-model="sshForm.private_key_content" class="ssh-key-input" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;...&#10;-----END OPENSSH PRIVATE KEY-----" rows="4"></textarea>
          </template>
          <div class="modal-actions">
            <button type="button" class="btn btn-secondary" @click="closeSshModal">取消</button>
            <button type="submit" class="btn btn-primary">保存</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted, onUnmounted, watch, nextTick } from "vue";
import { useRouter } from "vue-router";
import { redirectIfLoggedOut } from "../session";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

const router = useRouter();
const terminalContainer = ref<HTMLElement | null>(null);

interface SshHost {
  id: string;
  name: string;
  host: string;
  port: number;
  user: string;
  auth_type: string;
}

interface TabState {
  id: string;
  name: string;
  hostId: string;
  hostLabel: string;
  term: Terminal;
  fitAddon: FitAddon;
  ws: WebSocket;
  status: "connecting" | "connected" | "closed" | "error";
  statusText: string;
}

const tabs = ref<TabState[]>([]);
const activeId = ref<string | null>(null);
const sshHosts = ref<SshHost[]>([]);
const selectedHostId = ref<string | null>(null);
const sshModalOpen = ref(false);
const sshSaveSuccess = ref("");
let sshSaveSuccessTimer: ReturnType<typeof setTimeout> | null = null;
const editingSshId = ref("");
const sshForm = ref({
  name: "",
  host: "",
  port: 22,
  user: "",
  auth_type: "password",
  password: "",
  private_key_path: "",
  private_key_content: "",
});

let tabIdCounter = 0;
let resizeObserver: ResizeObserver | null = null;
let fitTimer: ReturnType<typeof setTimeout> | null = null;
const lastFitCols = ref<Record<string, number>>({});
const lastFitRows = ref<Record<string, number>>({});

const activeTab = computed(() => {
  if (!activeId.value) return null;
  return tabs.value.find((t) => t.id === activeId.value) || null;
});

function getWsUrl(hostId?: string | null): string {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  let url = `${proto}//${location.host}/api/terminal`;
  if (hostId) {
    url += `?host=${encodeURIComponent(hostId)}`;
  }
  return url;
}

async function loadSshHosts() {
  try {
    const res = await fetch("/api/ssh/hosts");
    const data = await res.json();
    if (data.list) sshHosts.value = data.list;
  } catch {
    sshHosts.value = [];
  }
}

function editSshHost(h: SshHost) {
  editingSshId.value = h.id;
  sshForm.value = {
    name: h.name,
    host: h.host,
    port: h.port || 22,
    user: h.user,
    auth_type: h.auth_type || "password",
    password: "",
    private_key_path: "",
    private_key_content: "",
  };
}

function openSshManage() {
  loadSshHosts();
  cancelSshEdit();
  sshModalOpen.value = true;
}

function closeSshModal() {
  sshModalOpen.value = false;
  cancelSshEdit();
}

function cancelSshEdit() {
  editingSshId.value = "";
  sshForm.value = {
    name: "",
    host: "",
    port: 22,
    user: "",
    auth_type: "password",
    password: "",
    private_key_path: "",
    private_key_content: "",
  };
}

async function saveSshHost() {
  const body: Record<string, unknown> = {
    name: sshForm.value.name,
    host: sshForm.value.host,
    port: sshForm.value.port || 22,
    user: sshForm.value.user,
    auth_type: sshForm.value.auth_type,
  };
  if (editingSshId.value) body.id = editingSshId.value;
  if (sshForm.value.auth_type === "password" && sshForm.value.password) {
    body.password = sshForm.value.password;
  }
  if (sshForm.value.auth_type === "key") {
    if (sshForm.value.private_key_path) body.private_key_path = sshForm.value.private_key_path;
    if (sshForm.value.private_key_content) body.private_key_content = sshForm.value.private_key_content;
  }
  try {
    const res = await fetch("/api/ssh/hosts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "保存失败");
    await loadSshHosts();
    cancelSshEdit();
    if (sshSaveSuccessTimer) clearTimeout(sshSaveSuccessTimer);
    sshSaveSuccess.value = "保存成功";
    sshSaveSuccessTimer = setTimeout(() => {
      sshSaveSuccessTimer = null;
      sshSaveSuccess.value = "";
    }, 2000);
  } catch (e) {
    alert((e as Error).message);
  }
}

async function deleteSshHost(id: string) {
  if (!confirm("确定删除该 SSH 主机？")) return;
  try {
    const res = await fetch(`/api/ssh/hosts/${id}`, { method: "DELETE" });
    if (!res.ok) throw new Error("删除失败");
    await loadSshHosts();
    if (selectedHostId.value === id) selectedHostId.value = null;
  } catch (e) {
    alert((e as Error).message);
  }
}

function fitTab(tab: TabState) {
  tab.fitAddon.fit();
  if (tab.ws.readyState === WebSocket.OPEN && tab.term) {
    let cols = tab.term.cols;
    let rows = tab.term.rows;
    if (cols < 2) cols = 80;
    if (rows < 2) rows = 24;
    const lastC = lastFitCols.value[tab.id] ?? 0;
    const lastR = lastFitRows.value[tab.id] ?? 0;
    if (cols !== lastC || rows !== lastR) {
      lastFitCols.value[tab.id] = cols;
      lastFitRows.value[tab.id] = rows;
      tab.ws.send(JSON.stringify({ cols, rows }));
    }
  }
}

function debouncedFit() {
  if (fitTimer) clearTimeout(fitTimer);
  fitTimer = setTimeout(() => {
    fitTimer = null;
    const tab = activeTab.value;
    if (tab) fitTab(tab);
  }, 150);
}

function setTabVisible(tab: TabState, visible: boolean) {
  const el = tab.term.element;
  if (el) el.style.display = visible ? "" : "none";
}

function ensureActiveInContainer() {
  const active = activeTab.value;
  tabs.value.forEach((tab) => setTabVisible(tab, tab.id === activeId.value));
  if (active) nextTick(() => fitTab(active));
}

function addTab() {
  tabIdCounter += 1;
  const id = `tab-${tabIdCounter}`;
  const hostId = selectedHostId.value || "";
  const hostLabel = hostId
    ? sshHosts.value.find((h) => h.id === hostId)?.name || "SSH"
    : "本地";
  const name = hostLabel === "本地" ? `终端 ${tabIdCounter}` : `${hostLabel} ${tabIdCounter}`;

  const term = new Terminal({
    cursorBlink: true,
    theme: {
      background: "#0f172a",
      foreground: "#e2e8f0",
      cursor: "#94a3b8",
      black: "#1e293b",
      red: "#f87171",
      green: "#4ade80",
      yellow: "#facc15",
      blue: "#60a5fa",
      magenta: "#c084fc",
      cyan: "#22d3ee",
      white: "#f1f5f9",
    },
  });

  const fitAddon = new FitAddon();
  term.loadAddon(fitAddon);

  const ws = new WebSocket(getWsUrl(hostId || undefined));
  let opened = false;
  const tab = reactive<TabState>({
    id,
    name,
    hostId,
    hostLabel,
    term,
    fitAddon,
    ws,
    status: "connecting",
    statusText: "正在连接…",
  });

  ws.onopen = () => {
    opened = true;
    tab.status = "connected";
    tab.statusText = "已连接";
    if (tab.id === activeId.value) nextTick(() => fitTab(tab));
  };

  ws.onclose = () => {
    tab.status = "closed";
    tab.statusText = "连接已关闭";
    if (!opened) void redirectIfLoggedOut(router);
  };

  ws.onerror = () => {
    tab.status = "error";
    tab.statusText = "连接错误";
  };

  ws.onmessage = (ev: MessageEvent) => {
    if (ev.data instanceof Blob) {
      ev.data.arrayBuffer().then((buf) => {
        term.write(new Uint8Array(buf));
      });
    } else {
      term.write(ev.data as string);
    }
  };

  term.onData((data) => {
    if (ws.readyState === WebSocket.OPEN) ws.send(data);
  });

  tabs.value.push(tab);
  activeId.value = id;

  nextTick(() => {
    const cont = terminalContainer.value;
    if (!cont) return;
    term.open(cont);
    if (tabs.value.length > 1) {
      tabs.value.forEach((t) => setTabVisible(t, t.id === id));
    }
    fitTab(tab);
  });
}

function switchTab(id: string) {
  if (activeId.value === id) return;
  const prev = tabs.value.find((t) => t.id === activeId.value);
  const next = tabs.value.find((t) => t.id === id);
  if (prev) setTabVisible(prev, false);
  if (next) {
    setTabVisible(next, true);
    activeId.value = id;
    nextTick(() => fitTab(next));
  }
}

function closeTab(id: string) {
  const idx = tabs.value.findIndex((t) => t.id === id);
  if (idx === -1) return;
  const tab = tabs.value[idx];
  tab.ws.close();
  tab.term.dispose();
  tabs.value.splice(idx, 1);
  if (tabs.value.length === 0) {
    activeId.value = null;
    return;
  }
  if (activeId.value === id) {
    const next = tabs.value[Math.min(idx, tabs.value.length - 1)];
    activeId.value = next.id;
    setTabVisible(next, true);
    nextTick(() => fitTab(next));
  }
  delete lastFitCols.value[id];
  delete lastFitRows.value[id];
}

watch(activeId, () => {
  ensureActiveInContainer();
});

onMounted(() => {
  loadSshHosts();
  addTab();
  nextTick(() => {
    const container = terminalContainer.value;
    if (container) {
      resizeObserver = new ResizeObserver(() => debouncedFit());
      resizeObserver.observe(container);
    }
  });
});

onUnmounted(() => {
  if (fitTimer) clearTimeout(fitTimer);
  resizeObserver?.disconnect();
  tabs.value.forEach((tab) => {
    tab.ws.close();
    tab.term.dispose();
  });
  tabs.value.length = 0;
  activeId.value = null;
});
</script>

<style scoped>
.terminal-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 16px;
  position: relative;
  overflow: hidden;
}
.toast {
  position: fixed;
  top: 80px;
  left: 50%;
  transform: translateX(-50%);
  padding: 10px 20px;
  border-radius: 8px;
  font-size: 14px;
  z-index: 1100;
  animation: toast-in 0.2s ease;
}
.toast-success {
  background: rgba(34, 197, 94, 0.95);
  color: #fff;
  border: 1px solid rgba(74, 222, 128, 0.6);
}
@keyframes toast-in {
  from {
    opacity: 0;
    transform: translateX(-50%) translateY(-8px);
  }
  to {
    opacity: 1;
    transform: translateX(-50%) translateY(0);
  }
}
.terminal-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.back-link {
  color: #94a3b8;
  text-decoration: none;
  font-size: 14px;
  flex-shrink: 0;
}
.back-link:hover {
  color: #e2e8f0;
}
.terminal-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  flex-shrink: 0;
}
.terminal-tab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  background: rgba(30, 41, 59, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.3);
  border-radius: 6px;
  color: #94a3b8;
  font-size: 13px;
  cursor: pointer;
  flex-shrink: 0;
}
.terminal-tab:hover {
  background: rgba(51, 65, 85, 0.9);
  color: #e2e8f0;
}
.terminal-tab-active {
  background: rgba(59, 130, 246, 0.25);
  border-color: rgba(96, 165, 250, 0.5);
  color: #e2e8f0;
}
.terminal-tab-name {
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.terminal-tab-close {
  padding: 0;
  margin: 0;
  background: none;
  border: none;
  color: #94a3b8;
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  border-radius: 2px;
}
.terminal-tab-close:hover {
  color: #f87171;
  background: rgba(248, 113, 113, 0.15);
}
.terminal-tab-add {
  padding: 4px 10px;
  background: rgba(30, 41, 59, 0.8);
  border: 1px dashed rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #94a3b8;
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
}
.terminal-tab-add:hover {
  border-color: rgba(96, 165, 250, 0.6);
  color: #60a5fa;
}
.status {
  font-size: 12px;
  margin-left: auto;
}
.status.connecting {
  color: #facc15;
}
.status.connected {
  color: #4ade80;
}
.status.closed,
.status.error {
  color: #f87171;
}
.terminal-container {
  flex: 1;
  min-height: 320px;
  max-height: calc(100vh - 140px);
  padding: 8px;
  background: #0f172a;
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.3);
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.terminal-container :deep(.xterm) {
  height: 100% !important;
}
.terminal-container :deep(.xterm-viewport) {
  overflow-y: auto !important;
}
.terminal-empty {
  flex: 1;
  min-height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.5);
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.2);
  color: #64748b;
  font-size: 14px;
}
.terminal-host-label {
  font-size: 12px;
  color: #94a3b8;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.terminal-host-select {
  padding: 4px 8px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 13px;
  min-width: 140px;
}
.terminal-toolbar .btn {
  flex-shrink: 0;
}
.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
.modal-box {
  background: #1e293b;
  border-radius: 12px;
  padding: 24px;
  width: 100%;
  max-width: 420px;
  border: 1px solid rgba(148, 163, 184, 0.3);
  max-height: 90vh;
  overflow-y: auto;
}
.modal-box h3 {
  margin: 0 0 12px;
  font-size: 18px;
  color: #e2e8f0;
}
.modal-box h4 {
  margin: 16px 0 8px;
  font-size: 14px;
  color: #94a3b8;
}
.modal-ssh {
  max-width: 520px;
}
.ssh-list {
  margin-bottom: 16px;
  max-height: 180px;
  overflow-y: auto;
}
.ssh-list-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}
.ssh-list-name {
  font-weight: 500;
  color: #e2e8f0;
  min-width: 80px;
}
.ssh-list-addr {
  font-size: 12px;
  color: #64748b;
  flex: 1;
}
.ssh-list-actions {
  display: flex;
  gap: 6px;
}
.ssh-list-empty {
  color: #64748b;
  font-size: 13px;
  padding: 12px 0;
}
.ssh-form label {
  display: block;
  margin: 10px 0 4px;
  font-size: 13px;
  color: #94a3b8;
}
.ssh-form input,
.ssh-form select {
  width: 100%;
  padding: 8px 10px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 13px;
}
.ssh-key-input {
  width: 100%;
  padding: 8px 10px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 12px;
  font-family: ui-monospace, monospace;
  resize: vertical;
}
.modal-actions {
  margin-top: 16px;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}
.btn {
  padding: 6px 12px;
  border-radius: 6px;
  border: 1px solid rgba(148, 163, 184, 0.4);
  background: rgba(59, 130, 246, 0.5);
  color: #e2e8f0;
  font-size: 13px;
  cursor: pointer;
}
.btn:hover {
  background: rgba(59, 130, 246, 0.7);
}
.btn-sm {
  padding: 4px 10px;
  font-size: 12px;
}
.btn-secondary {
  background: rgba(100, 116, 139, 0.4);
  border-color: rgba(148, 163, 184, 0.4);
}
.btn-secondary:hover {
  background: rgba(100, 116, 139, 0.6);
}
.btn-primary {
  background: rgba(34, 197, 94, 0.6);
  border-color: rgba(74, 222, 128, 0.5);
}
.btn-primary:hover {
  background: rgba(34, 197, 94, 0.8);
}
.btn-danger {
  background: rgba(239, 68, 68, 0.5);
  border-color: rgba(248, 113, 113, 0.5);
}
.btn-danger:hover {
  background: rgba(239, 68, 68, 0.8);
}
</style>
