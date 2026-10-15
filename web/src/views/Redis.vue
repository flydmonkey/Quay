<template>
  <div class="redis-page">
    <div v-if="toastMsg" class="toast toast-success">{{ toastMsg }}</div>
    <div class="redis-toolbar">
      <router-link to="/" class="back-link">← 返回首页</router-link>
      <label class="redis-conn-label">
        连接
        <select v-model="selectedConnId" class="redis-select" @change="onConnChange">
          <option value="">选择连接</option>
          <option v-for="c in connections" :key="c.id" :value="c.id">{{ c.name }} ({{ c.host }}:{{ c.port || 6379 }})</option>
        </select>
      </label>
      <button type="button" class="btn btn-sm" @click="showConnModal()">添加连接</button>
      <button type="button" class="btn btn-sm" @click="openConnManage">管理连接</button>
      <button
        type="button"
        class="btn btn-sm"
        :disabled="!selectedConnId"
        @click="showConnModal(selectedConnId)"
      >
        编辑
      </button>
      <button
        type="button"
        class="btn btn-sm btn-danger"
        :disabled="!selectedConnId"
        @click="deleteConn"
      >
        删除
      </button>
    </div>

    <div v-if="!selectedConnId" class="redis-empty">
      <p>请选择或添加 Redis 连接</p>
    </div>
    <div v-else class="redis-layout">
      <aside class="redis-side">
        <div class="redis-db-bar">
          <label>DB 索引</label>
          <input v-model.number="dbIndex" type="number" min="0" max="15" class="redis-db-input" />
        </div>
        <div class="redis-keys-section">
          <label>Key 列表（SCAN）</label>
          <div class="redis-pattern-row">
            <input v-model="keyPattern" type="text" class="redis-pattern-input" placeholder="*" />
            <button type="button" class="btn btn-sm" :disabled="loadingKeys" @click="loadKeys">
              {{ loadingKeys ? '…' : '列出' }}
            </button>
          </div>
          <div class="redis-keys-list">
            <div
              v-for="k in keysList"
              :key="k"
              class="redis-key-item"
              @click="runGetKey(k)"
            >
              {{ k }}
            </div>
            <p v-if="keysList.length === 0 && keysLoaded" class="redis-keys-empty">无匹配 key</p>
          </div>
        </div>
      </aside>

      <main class="redis-main">
        <div class="redis-command-bar">
          <label>执行命令（如 GET key、SET key value）</label>
          <div class="redis-command-row">
            <input
              v-model="commandInput"
              type="text"
              class="redis-command-input"
              placeholder="PING 或 GET mykey"
              @keydown.enter.prevent="runCommand"
            />
            <button
              type="button"
              class="btn btn-run"
              :disabled="!commandInput.trim() || executing"
              @click="runCommand"
            >
              {{ executing ? '执行中…' : '执行' }}
            </button>
          </div>
        </div>
        <div v-if="execError" class="redis-error">{{ execError }}</div>
        <div v-if="execResult !== null" class="redis-result-wrap">
          <pre class="redis-result">{{ formatResult(execResult) }}</pre>
        </div>
      </main>
    </div>

    <!-- 添加/编辑连接弹层 -->
    <div v-if="connModal" class="modal-overlay" @click.self="closeConnModal">
      <div class="modal-box">
        <h3>{{ editingConnId ? '编辑连接' : '添加连接' }}</h3>
        <form class="conn-form" @submit.prevent="saveConn">
          <label>名称</label>
          <input v-model="form.name" type="text" required placeholder="如：本地 Redis" />
          <label>主机</label>
          <input v-model="form.host" type="text" required placeholder="127.0.0.1" />
          <label>端口</label>
          <input v-model.number="form.port" type="number" placeholder="6379" />
          <label>密码（可选）</label>
          <input v-model="form.password" type="password" :placeholder="editingConnId ? '留空则不修改' : ''" />
          <label>DB 索引（0-15，集群模式固定为 0）</label>
          <input v-model.number="form.db" type="number" min="0" max="15" placeholder="0" />
          <div class="redis-form-options">
            <label class="redis-checkbox">
              <input v-model="form.cluster" type="checkbox" />
              集群模式 (Cluster)
            </label>
            <label class="redis-checkbox">
              <input v-model="form.use_tls" type="checkbox" />
              使用 SSL/TLS
            </label>
            <label v-if="form.use_tls" class="redis-checkbox">
              <input v-model="form.insecure_skip_verify" type="checkbox" />
              跳过证书验证（仅测试环境）
            </label>
          </div>
          <div class="modal-actions">
            <button type="button" class="btn btn-secondary" @click="closeConnModal">取消</button>
            <button type="submit" class="btn btn-primary">保存</button>
          </div>
        </form>
      </div>
    </div>

    <!-- 管理连接弹层 -->
    <div v-if="connManageModal" class="modal-overlay" @click.self="closeConnManageModal">
      <div class="modal-box modal-conn-manage">
        <h3>Redis 连接管理</h3>
        <div class="conn-list">
          <div v-for="c in connections" :key="c.id" class="conn-list-item">
            <span class="conn-list-name">{{ c.name }}</span>
            <span class="conn-list-addr">
              {{ c.host }}:{{ c.port || 6379 }} DB{{ c.db ?? 0 }}
              <span v-if="c.cluster" class="conn-tag">Cluster</span>
              <span v-if="c.use_tls" class="conn-tag">SSL</span>
            </span>
            <div class="conn-list-actions">
              <button type="button" class="btn btn-sm" @click="editConnFromList(c)">编辑</button>
              <button type="button" class="btn btn-sm btn-danger" @click="deleteConnFromList(c.id)">删除</button>
            </div>
          </div>
          <p v-if="!connections.length" class="conn-list-empty">暂无连接，点击下方添加</p>
        </div>
        <form class="conn-form" @submit.prevent="saveConnFromManage">
          <h4>{{ editingConnId ? '编辑连接' : '添加连接' }}</h4>
          <label>名称</label>
          <input v-model="form.name" type="text" required placeholder="如：本地 Redis" />
          <label>主机</label>
          <input v-model="form.host" type="text" required placeholder="127.0.0.1" />
          <label>端口</label>
          <input v-model.number="form.port" type="number" placeholder="6379" />
          <label>密码（可选）</label>
          <input v-model="form.password" type="password" :placeholder="editingConnId ? '留空则不修改' : ''" />
          <label>DB 索引</label>
          <input v-model.number="form.db" type="number" min="0" max="15" placeholder="0" />
          <div class="redis-form-options">
            <label class="redis-checkbox">
              <input v-model="form.cluster" type="checkbox" />
              集群模式 (Cluster)
            </label>
            <label class="redis-checkbox">
              <input v-model="form.use_tls" type="checkbox" />
              使用 SSL/TLS
            </label>
            <label v-if="form.use_tls" class="redis-checkbox">
              <input v-model="form.insecure_skip_verify" type="checkbox" />
              跳过证书验证
            </label>
          </div>
          <div class="modal-actions">
            <button type="button" class="btn btn-secondary" @click="closeConnManageModal">取消</button>
            <button type="submit" class="btn btn-primary">保存</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";

interface Conn {
  id: string;
  name: string;
  host: string;
  port: number;
  password?: string;
  db?: number;
  cluster?: boolean;
  use_tls?: boolean;
  insecure_skip_verify?: boolean;
}

const connections = ref<Conn[]>([]);
const selectedConnId = ref("");
const dbIndex = ref(0);
const commandInput = ref("");
const executing = ref(false);
const execResult = ref<unknown>(null);
const execError = ref("");
const keyPattern = ref("*");
const keysList = ref<string[]>([]);
const keysLoaded = ref(false);
const loadingKeys = ref(false);
const connModal = ref(false);
const connManageModal = ref(false);
const toastMsg = ref("");
let toastTimer: ReturnType<typeof setTimeout> | null = null;
const editingConnId = ref("");
const form = ref({
  name: "",
  host: "127.0.0.1",
  port: 6379,
  password: "",
  db: 0,
  cluster: false,
  use_tls: false,
  insecure_skip_verify: false,
});

function showToast(msg: string) {
  if (toastTimer) clearTimeout(toastTimer);
  toastMsg.value = msg;
  toastTimer = setTimeout(() => {
    toastTimer = null;
    toastMsg.value = "";
  }, 2000);
}

async function safeJson(res: Response): Promise<Record<string, unknown>> {
  const text = await res.text();
  const t = text?.trim();
  if (!t) return {};
  try {
    return JSON.parse(t) as Record<string, unknown>;
  } catch {
    return { error: t || "响应不是合法 JSON" };
  }
}

async function loadConnections() {
  try {
    const res = await fetch("/api/redis/connections");
    const data = await safeJson(res);
    if (data.list) connections.value = data.list as Conn[];
  } catch {
    connections.value = [];
  }
}

function onConnChange() {
  keysList.value = [];
  keysLoaded.value = false;
  execResult.value = null;
  execError.value = "";
}

function showConnModal(id?: string) {
  editingConnId.value = id || "";
  if (id) {
    const c = connections.value.find((x) => x.id === id);
    if (c) form.value = { name: c.name, host: c.host, port: c.port || 6379, password: "", db: c.db ?? 0, cluster: c.cluster ?? false, use_tls: c.use_tls ?? false, insecure_skip_verify: c.insecure_skip_verify ?? false };
  } else {
    form.value = { name: "", host: "127.0.0.1", port: 6379, password: "", db: 0, cluster: false, use_tls: false, insecure_skip_verify: false };
  }
  connModal.value = true;
}

function closeConnModal() {
  connModal.value = false;
  editingConnId.value = "";
}

function openConnManage() {
  connManageModal.value = true;
  editingConnId.value = "";
  form.value = { name: "", host: "127.0.0.1", port: 6379, password: "", db: 0, cluster: false, use_tls: false, insecure_skip_verify: false };
}

function closeConnManageModal() {
  connManageModal.value = false;
  editingConnId.value = "";
}

function editConnFromList(c: Conn) {
  editingConnId.value = c.id;
  form.value = { name: c.name, host: c.host, port: c.port || 6379, password: "", db: c.db ?? 0, cluster: c.cluster ?? false, use_tls: c.use_tls ?? false, insecure_skip_verify: c.insecure_skip_verify ?? false };
}

async function deleteConnFromList(id: string) {
  if (!confirm("确定删除该连接？")) return;
  try {
    const res = await fetch(`/api/redis/connections/${id}`, { method: "DELETE" });
    if (!res.ok) throw new Error("删除失败");
    await loadConnections();
    if (selectedConnId.value === id) selectedConnId.value = "";
    showToast("已删除");
  } catch (e) {
    execError.value = (e as Error).message;
  }
}

async function saveConnFromManage() {
  const body: Record<string, unknown> = {
    name: form.value.name,
    host: form.value.host,
    port: form.value.port || 6379,
    db: form.value.db ?? 0,
    cluster: form.value.cluster,
    use_tls: form.value.use_tls,
    insecure_skip_verify: form.value.insecure_skip_verify,
  };
  if (editingConnId.value) body.id = editingConnId.value;
  if (form.value.password) body.password = form.value.password;
  try {
    const res = await fetch("/api/redis/connections", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await safeJson(res);
    if (!res.ok) throw new Error((data.error as string) || "保存失败");
    await loadConnections();
    editingConnId.value = "";
    showToast("保存成功");
  } catch (e) {
    execError.value = (e as Error).message;
  }
}

async function saveConn() {
  const body: Record<string, unknown> = {
    name: form.value.name,
    host: form.value.host,
    port: form.value.port || 6379,
    db: form.value.db ?? 0,
    cluster: form.value.cluster,
    use_tls: form.value.use_tls,
    insecure_skip_verify: form.value.insecure_skip_verify,
  };
  if (editingConnId.value) body.id = editingConnId.value;
  if (form.value.password) body.password = form.value.password;
  try {
    const res = await fetch("/api/redis/connections", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await safeJson(res);
    if (!res.ok) throw new Error((data.error as string) || "保存失败");
    connModal.value = false;
    await loadConnections();
    if (data.id) {
      selectedConnId.value = data.id as string;
      onConnChange();
    }
    showToast("保存成功");
  } catch (e) {
    execError.value = (e as Error).message;
  }
}

async function deleteConn() {
  if (!selectedConnId.value || !confirm("确定删除该连接？")) return;
  try {
    const res = await fetch(`/api/redis/connections/${selectedConnId.value}`, { method: "DELETE" });
    if (!res.ok) throw new Error("删除失败");
    selectedConnId.value = "";
    await loadConnections();
    showToast("已删除");
  } catch (e) {
    execError.value = (e as Error).message;
  }
}

function parseCommand(line: string): string[] {
  const trimmed = line.trim();
  if (!trimmed) return [];
  const parts: string[] = [];
  let i = 0;
  while (i < trimmed.length) {
    if (trimmed[i] === '"' || trimmed[i] === "'") {
      const quote = trimmed[i];
      i++;
      let s = "";
      while (i < trimmed.length && trimmed[i] !== quote) {
        if (trimmed[i] === "\\") {
          i++;
          if (i < trimmed.length) s += trimmed[i++];
        } else {
          s += trimmed[i++];
        }
      }
      if (i < trimmed.length) i++;
      parts.push(s);
      continue;
    }
    if (/\s/.test(trimmed[i])) {
      i++;
      continue;
    }
    let s = "";
    while (i < trimmed.length && !/[\s"']/.test(trimmed[i])) {
      s += trimmed[i++];
    }
    parts.push(s);
  }
  return parts;
}

async function runCommand() {
  const cmd = parseCommand(commandInput.value);
  if (!cmd.length) return;
  await execCommand(cmd);
}

async function runGetKey(key: string) {
  if (!selectedConnId.value) return;
  execError.value = "";
  execResult.value = null;
  executing.value = true;
  try {
    const typeRes = await fetch("/api/redis/type", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        connectionId: selectedConnId.value,
        db: dbIndex.value,
        key,
      }),
    });
    const typeData = await safeJson(typeRes);
    if (typeData.error) {
      execError.value = typeData.error as string;
      return;
    }
    const keyType = (typeData.type as string) || "";
    let cmd: string[];
    if (keyType === "string") {
      cmd = ["GET", key];
    } else if (keyType === "list") {
      cmd = ["LRANGE", key, "0", "-1"];
    } else if (keyType === "set") {
      cmd = ["SMEMBERS", key];
    } else if (keyType === "zset") {
      cmd = ["ZRANGE", key, "0", "-1", "WITHSCORES"];
    } else if (keyType === "hash") {
      cmd = ["HGETALL", key];
    } else if (keyType === "stream") {
      cmd = ["XRANGE", key, "-", "+", "COUNT", "100"];
    } else {
      execResult.value = `类型: ${keyType}`;
      return;
    }
    commandInput.value = cmd.join(" ");
    await execCommand(cmd);
  } catch (e) {
    execError.value = (e as Error).message;
  } finally {
    executing.value = false;
  }
}

async function execCommand(cmd: string[]) {
  if (!selectedConnId.value || !cmd.length) return;
  execError.value = "";
  execResult.value = null;
  executing.value = true;
  try {
    const res = await fetch("/api/redis/exec", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        connectionId: selectedConnId.value,
        db: dbIndex.value,
        command: cmd,
      }),
    });
    const data = await safeJson(res);
    if (data.error) {
      execError.value = data.error as string;
      return;
    }
    execResult.value = data.result;
  } catch (e) {
    execError.value = (e as Error).message;
  } finally {
    executing.value = false;
  }
}

async function loadKeys() {
  if (!selectedConnId.value) return;
  loadingKeys.value = true;
  keysList.value = [];
  keysLoaded.value = true;
  try {
    const pattern = keyPattern.value.trim() || "*";
    let cursor: number | string = 0;
    const all: string[] = [];
    for (let i = 0; i < 10; i++) {
      const res = await fetch(
        `/api/redis/keys?connectionId=${encodeURIComponent(selectedConnId.value)}&db=${dbIndex.value}&pattern=${encodeURIComponent(pattern)}&cursor=${cursor}&count=100`
      );
      const data = await safeJson(res);
      if (data.error) {
        execError.value = data.error as string;
        break;
      }
      const keys = (data.keys || []) as string[];
      all.push(...keys);
      cursor = data.cursor ?? 0;
      if (cursor === 0 || cursor === "0") break;
    }
    keysList.value = all;
  } catch (e) {
    execError.value = (e as Error).message;
  } finally {
    loadingKeys.value = false;
  }
}

function formatResult(v: unknown): string {
  if (v === null || v === undefined) return "nil";
  if (Array.isArray(v)) return JSON.stringify(v, null, 2);
  if (typeof v === "object") return JSON.stringify(v, null, 2);
  return String(v);
}

onMounted(() => {
  loadConnections();
});
</script>

<style scoped>
.redis-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 16px;
  position: relative;
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
.redis-toolbar {
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
}
.back-link:hover {
  color: #e2e8f0;
}
.redis-conn-label {
  font-size: 13px;
  color: #94a3b8;
}
.redis-select {
  padding: 4px 8px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 13px;
  min-width: 180px;
}
.redis-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.5);
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.2);
  color: #64748b;
  font-size: 14px;
}
.redis-layout {
  flex: 1;
  display: flex;
  gap: 20px;
  min-height: 0;
}
.redis-side {
  width: 260px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
  background: rgba(15, 23, 42, 0.5);
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  padding: 12px;
}
.redis-db-bar label,
.redis-keys-section label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: #94a3b8;
}
.redis-db-input {
  width: 80px;
  padding: 6px 8px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 13px;
}
.redis-pattern-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}
.redis-pattern-input {
  flex: 1;
  padding: 6px 8px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 13px;
}
.redis-keys-list {
  max-height: 300px;
  overflow-y: auto;
  font-size: 13px;
}
.redis-key-item {
  padding: 4px 8px;
  cursor: pointer;
  color: #e2e8f0;
  border-radius: 4px;
  word-break: break-all;
}
.redis-key-item:hover {
  background: rgba(59, 130, 246, 0.2);
  color: #93c5fd;
}
.redis-keys-empty {
  color: #64748b;
  font-size: 12px;
  padding: 8px 0;
}
.redis-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: rgba(15, 23, 42, 0.5);
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  padding: 12px;
}
.redis-command-bar label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: #94a3b8;
}
.redis-command-row {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}
.redis-command-input {
  flex: 1;
  padding: 8px 12px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 14px;
}
.redis-command-input:focus {
  outline: none;
  border-color: #60a5fa;
}
.btn-run {
  padding: 8px 16px;
  background: rgba(34, 197, 94, 0.6);
  border: 1px solid rgba(74, 222, 128, 0.5);
  border-radius: 6px;
  color: #e2e8f0;
  cursor: pointer;
  flex-shrink: 0;
}
.btn-run:hover:not(:disabled) {
  background: rgba(34, 197, 94, 0.8);
}
.btn-run:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.redis-error {
  color: #f87171;
  font-size: 13px;
  margin-bottom: 8px;
}
.redis-result-wrap {
  flex: 1;
  overflow: auto;
  min-height: 100px;
}
.redis-result {
  margin: 0;
  font-size: 13px;
  color: #e2e8f0;
  white-space: pre-wrap;
  word-break: break-all;
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
.btn:hover:not(:disabled) {
  background: rgba(59, 130, 246, 0.7);
}
.btn-sm {
  padding: 4px 10px;
  font-size: 12px;
}
.btn-danger {
  background: rgba(239, 68, 68, 0.5);
  border-color: rgba(248, 113, 113, 0.5);
}
.btn-danger:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.8);
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
.modal-conn-manage {
  max-width: 520px;
}
.conn-list {
  margin-bottom: 16px;
  max-height: 180px;
  overflow-y: auto;
}
.conn-list-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}
.conn-list-name {
  font-weight: 500;
  color: #e2e8f0;
  min-width: 80px;
}
.conn-list-addr {
  font-size: 12px;
  color: #64748b;
  flex: 1;
}
.conn-list-actions {
  display: flex;
  gap: 6px;
}
.conn-list-empty {
  color: #64748b;
  font-size: 13px;
  padding: 12px 0;
}
.conn-form label {
  display: block;
  margin: 10px 0 4px;
  font-size: 13px;
  color: #94a3b8;
}
.conn-form input[type="text"],
.conn-form input[type="number"],
.conn-form input[type="password"] {
  width: 100%;
  padding: 8px 10px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 13px;
}
.redis-form-options {
  margin: 12px 0 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.redis-checkbox {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 13px;
  color: #94a3b8;
}
.redis-checkbox input[type="checkbox"] {
  width: auto;
}
.conn-tag {
  margin-left: 6px;
  padding: 1px 6px;
  font-size: 11px;
  background: rgba(59, 130, 246, 0.3);
  border-radius: 4px;
  color: #93c5fd;
}
.modal-actions {
  margin-top: 16px;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}
</style>
