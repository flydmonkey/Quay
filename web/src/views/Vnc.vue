<template>
  <div class="vnc-page">
    <div v-if="vncSaveSuccess" class="toast toast-success">{{ vncSaveSuccess }}</div>
    <div class="vnc-toolbar">
      <router-link to="/" class="back-link">← 返回首页</router-link>
      <label class="vnc-conn-label">
        连接
        <select v-model="selectedConnId" class="vnc-select">
          <option value="">手动输入</option>
          <option v-for="c in vncConnections" :key="c.id" :value="c.id">
            {{ c.name }} ({{ c.host }}:{{ c.port || 5900 }})
          </option>
        </select>
      </label>
      <div class="vnc-form">
        <label>
          主机
          <input v-model="host" type="text" placeholder="IP 或域名" class="vnc-input" />
        </label>
        <label>
          端口
          <input v-model.number="port" type="number" placeholder="5900" class="vnc-input vnc-input-num" />
        </label>
        <label class="vnc-pwd-label">
          密码
          <input v-model="password" type="password" placeholder="VNC 密码（可选）" class="vnc-input" />
        </label>
        <button v-if="!connected" type="button" class="btn btn-primary" :disabled="connecting" @click="connect">
          {{ connecting ? '连接中…' : '连接' }}
        </button>
        <button v-else type="button" class="btn" @click="disconnect">断开</button>
      </div>
      <button type="button" class="btn btn-sm" @click="openVncManage">管理 VNC 连接</button>
      <span v-if="statusText" class="vnc-status" :class="{ error: statusError }">{{ statusText }}</span>
    </div>
    <div v-if="connected || connecting" ref="screenRef" class="vnc-screen"></div>
    <div v-else class="vnc-placeholder">
      <p>选择已保存连接或输入主机、端口后点击「连接」</p>
      <p class="vnc-hint">默认端口 5900，多显示器时为 5901、5902…</p>
    </div>

    <!-- VNC 连接管理弹层 -->
    <div v-if="vncModalOpen" class="modal-overlay" @click.self="closeVncModal">
      <div class="modal-box modal-vnc">
        <h3>VNC 连接管理</h3>
        <div class="vnc-list">
          <div v-for="c in vncConnections" :key="c.id" class="vnc-list-item">
            <span class="vnc-list-name">{{ c.name }}</span>
            <span class="vnc-list-addr">{{ c.host }}:{{ c.port || 5900 }}</span>
            <div class="vnc-list-actions">
              <button type="button" class="btn btn-sm" @click="editVncConn(c)">编辑</button>
              <button type="button" class="btn btn-sm btn-danger" @click="deleteVncConn(c.id)">删除</button>
            </div>
          </div>
          <p v-if="!vncConnections.length" class="vnc-list-empty">暂无 VNC 连接，点击下方添加</p>
        </div>
        <form class="vnc-form-modal" @submit.prevent="saveVncConn">
          <h4>{{ editingVncId ? '编辑连接' : '添加连接' }}</h4>
          <label>名称</label>
          <input v-model="vncForm.name" type="text" required placeholder="如：办公室 KVM" />
          <label>主机</label>
          <input v-model="vncForm.host" type="text" required placeholder="IP 或域名" />
          <label>端口</label>
          <input v-model.number="vncForm.port" type="number" placeholder="5900" />
          <label>密码</label>
          <input v-model="vncForm.password" type="password" :placeholder="editingVncId ? '留空则不修改' : 'VNC 密码（可选）'" />
          <div class="modal-actions">
            <button type="button" class="btn btn-secondary" @click="closeVncModal">取消</button>
            <button type="submit" class="btn btn-primary">保存</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, onBeforeUnmount, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { redirectIfLoggedOut } from '../session';

interface VncConn {
  id: string;
  name: string;
  host: string;
  port: number;
  password?: string;
}

const router = useRouter();
const host = ref('');
const port = ref(5900);
const password = ref('');
const screenRef = ref<HTMLElement | null>(null);
const connecting = ref(false);
const connected = ref(false);
const statusText = ref('');
const statusError = ref(false);
const vncConnections = ref<VncConn[]>([]);
const selectedConnId = ref('');
const vncModalOpen = ref(false);
const vncSaveSuccess = ref('');
let vncSaveSuccessTimer: ReturnType<typeof setTimeout> | null = null;
const editingVncId = ref('');
const vncForm = ref({
  name: '',
  host: '',
  port: 5900,
  password: '',
});
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let rfb: any = null;

function setStatus(msg: string, error = false) {
  statusText.value = msg;
  statusError.value = error;
}

async function loadVncConnections() {
  try {
    const res = await fetch('/api/vnc/connections');
    const data = await res.json();
    vncConnections.value = (data.list || []).map((c: VncConn) => ({
      id: c.id,
      name: c.name,
      host: c.host,
      port: c.port || 5900,
      password: c.password,
    }));
  } catch {
    vncConnections.value = [];
  }
}

watch(selectedConnId, (id) => {
  if (!id) return;
  const c = vncConnections.value.find((x) => x.id === id);
  if (c) {
    host.value = c.host;
    port.value = c.port || 5900;
    password.value = ''; // 连接时再按 id 拉取密码
  }
});

function buildWsUrl(h: string, p: number): string {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const base = `${protocol}//${location.host}/api/vnc/ws`;
  const params = new URLSearchParams();
  params.set('host', h);
  params.set('port', String(p || 5900));
  return `${base}?${params.toString()}`;
}

async function connect() {
  let h = host.value.trim();
  let p = port.value || 5900;
  let pwd = password.value;

  if (selectedConnId.value) {
    try {
      const res = await fetch(`/api/vnc/connections/${selectedConnId.value}`);
      if (!res.ok) throw new Error('获取连接失败');
      const c = (await res.json()) as VncConn;
      h = c.host;
      p = c.port || 5900;
      if (c.password) pwd = c.password;
    } catch (e) {
      setStatus('获取连接失败: ' + (e instanceof Error ? e.message : String(e)), true);
      return;
    }
  }

  if (!h) {
    setStatus('请输入主机地址或选择已保存连接', true);
    return;
  }

  // 预检：把后端连接错误（如 connection refused）返回给前端展示
  try {
    const checkRes = await fetch('/api/vnc/check', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ host: h, port: p }),
    });
    const checkData = await checkRes.json();
    if (checkData.error || checkData.ok === false) {
      setStatus('无法连接 VNC 服务: ' + (checkData.error || '未知错误'), true);
      return;
    }
  } catch (e) {
    setStatus('预检请求失败: ' + (e instanceof Error ? e.message : String(e)), true);
    return;
  }

  setStatus('');
  connecting.value = true;
  statusError.value = false;
  await nextTick();
  if (!screenRef.value) {
    connecting.value = false;
    return;
  }
  try {
    const mod = await import('@novnc/novnc');
    const RFB = mod.default ?? mod;
    const wsUrl = buildWsUrl(h, p);
    const options: Record<string, unknown> = {};
    if (pwd) {
      options.credentials = { password: pwd };
    }
    rfb = new RFB(screenRef.value, wsUrl, options);
    rfb.addEventListener('connect', () => {
      connected.value = true;
      connecting.value = false;
      setStatus('已连接');
    });
    rfb.addEventListener('disconnect', () => {
      const wasConnected = connected.value;
      connected.value = false;
      connecting.value = false;
      rfb = null;
      setStatus('已断开');
      if (!wasConnected) void redirectIfLoggedOut(router);
    });
    rfb.addEventListener('credentialsrequired', () => {
      setStatus('需要 VNC 密码，请在密码框输入或保存连接时填写', true);
    });
    rfb.addEventListener('securityfailure', (e: { detail?: { status?: string } }) => {
      connecting.value = false;
      setStatus('安全/认证失败: ' + (e.detail?.status || '未知'), true);
    });
    rfb.scaleViewport = true;
    rfb.showDotCursor = true;
  } catch (e) {
    connecting.value = false;
    setStatus('连接失败: ' + (e instanceof Error ? e.message : String(e)), true);
  }
}

function disconnect() {
  if (rfb) {
    rfb.disconnect();
    rfb = null;
  }
  connected.value = false;
  connecting.value = false;
  setStatus('');
}

function openVncManage() {
  vncModalOpen.value = true;
  loadVncConnections();
}

function closeVncModal() {
  vncModalOpen.value = false;
  editingVncId.value = '';
  vncForm.value = { name: '', host: '', port: 5900, password: '' };
}

function editVncConn(c: VncConn) {
  editingVncId.value = c.id;
  vncForm.value = {
    name: c.name,
    host: c.host,
    port: c.port || 5900,
    password: '', // 编辑时不回显密码
  };
}

function deleteVncConn(id: string) {
  if (!confirm('确定删除该 VNC 连接？')) return;
  fetch(`/api/vnc/connections/${id}`, { method: 'DELETE' })
    .then((res) => {
      if (res.ok) {
        loadVncConnections();
        if (selectedConnId.value === id) selectedConnId.value = '';
      }
    })
    .catch(() => {});
}

function showSaveSuccess(msg: string) {
  vncSaveSuccess.value = msg;
  if (vncSaveSuccessTimer) clearTimeout(vncSaveSuccessTimer);
  vncSaveSuccessTimer = setTimeout(() => {
    vncSaveSuccess.value = '';
    vncSaveSuccessTimer = null;
  }, 2500);
}

async function saveVncConn() {
  const body = {
    id: editingVncId.value || undefined,
    name: vncForm.value.name.trim() || vncForm.value.host,
    host: vncForm.value.host.trim(),
    port: vncForm.value.port || 5900,
    password: vncForm.value.password || undefined,
  };
  if (!body.host) return;
  try {
    const res = await fetch('/api/vnc/connections', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (data.error) {
      setStatus(data.error, true);
      return;
    }
    await loadVncConnections();
    closeVncModal();
    showSaveSuccess('保存成功');
  } catch (e) {
    setStatus('保存失败: ' + (e instanceof Error ? e.message : String(e)), true);
  }
}

onMounted(() => {
  loadVncConnections();
});

onBeforeUnmount(() => {
  disconnect();
  if (vncSaveSuccessTimer) clearTimeout(vncSaveSuccessTimer);
});
</script>

<style scoped>
.vnc-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.vnc-toolbar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.25);
  background: rgba(15, 23, 42, 0.6);
  flex-wrap: wrap;
}
.back-link {
  color: #94a3b8;
  text-decoration: none;
  margin-right: 8px;
}
.back-link:hover {
  color: #e2e8f0;
}
.vnc-conn-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #94a3b8;
}
.vnc-select {
  padding: 6px 10px;
  border-radius: 6px;
  border: 1px solid rgba(148, 163, 184, 0.4);
  background: rgba(15, 23, 42, 0.8);
  color: #e2e8f0;
  font-size: 13px;
  min-width: 160px;
}
.vnc-form {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.vnc-form label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #94a3b8;
}
.vnc-input {
  padding: 6px 10px;
  border-radius: 6px;
  border: 1px solid rgba(148, 163, 184, 0.4);
  background: rgba(15, 23, 42, 0.8);
  color: #e2e8f0;
  font-size: 13px;
  min-width: 120px;
}
.vnc-input-num {
  min-width: 72px;
}
.vnc-pwd-label .vnc-input {
  min-width: 140px;
}
.vnc-status {
  font-size: 13px;
  color: #94a3b8;
  margin-left: auto;
}
.vnc-status.error {
  color: #f87171;
}
.vnc-screen {
  flex: 1;
  min-height: 400px;
  background: #1e293b;
}
.vnc-placeholder {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #64748b;
  font-size: 14px;
}
.vnc-hint {
  font-size: 12px;
  color: #475569;
}

/* Toast */
.toast {
  position: fixed;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  padding: 10px 20px;
  border-radius: 8px;
  font-size: 14px;
  z-index: 10000;
}
.toast-success {
  background: rgba(34, 197, 94, 0.95);
  color: #fff;
}

/* Modal */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}
.modal-box {
  background: #1e293b;
  border-radius: 12px;
  padding: 20px;
  max-width: 480px;
  width: 90%;
  max-height: 90vh;
  overflow-y: auto;
  border: 1px solid rgba(148, 163, 184, 0.3);
}
.modal-box h3 {
  margin: 0 0 16px;
  font-size: 16px;
}
.modal-box h4 {
  margin: 12px 0 8px;
  font-size: 14px;
  color: #94a3b8;
}
.vnc-list {
  margin-bottom: 16px;
  max-height: 200px;
  overflow-y: auto;
}
.vnc-list-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}
.vnc-list-name {
  font-weight: 500;
  min-width: 100px;
}
.vnc-list-addr {
  color: #94a3b8;
  font-size: 13px;
  flex: 1;
}
.vnc-list-actions {
  display: flex;
  gap: 8px;
}
.vnc-list-empty {
  color: #64748b;
  font-size: 13px;
  padding: 12px 0;
}
.vnc-form-modal label {
  display: block;
  margin-top: 8px;
  font-size: 13px;
  color: #94a3b8;
}
.vnc-form-modal input {
  width: 100%;
  padding: 8px 10px;
  margin-top: 4px;
  border-radius: 6px;
  border: 1px solid rgba(148, 163, 184, 0.4);
  background: rgba(15, 23, 42, 0.8);
  color: #e2e8f0;
  font-size: 13px;
  box-sizing: border-box;
}
.modal-actions {
  margin-top: 16px;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}
.btn {
  padding: 8px 14px;
  border-radius: 6px;
  font-size: 13px;
  cursor: pointer;
  border: 1px solid rgba(148, 163, 184, 0.4);
  background: rgba(51, 65, 85, 0.8);
  color: #e2e8f0;
}
.btn:hover {
  background: rgba(71, 85, 105, 0.9);
}
.btn-sm {
  padding: 6px 10px;
  font-size: 12px;
}
.btn-primary {
  background: rgba(59, 130, 246, 0.9);
  border-color: rgba(96, 165, 250, 0.5);
  color: #fff;
}
.btn-primary:hover {
  background: rgba(96, 165, 250, 0.95);
}
.btn-primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.btn-secondary {
  background: rgba(71, 85, 105, 0.8);
}
.btn-danger {
  background: rgba(185, 28, 28, 0.8);
  border-color: rgba(248, 113, 113, 0.3);
  color: #fff;
}
.btn-danger:hover {
  background: rgba(220, 38, 38, 0.9);
}
</style>
