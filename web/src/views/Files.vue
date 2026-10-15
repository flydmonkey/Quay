<template>
  <div class="files-page">
    <div v-if="fsToast" class="toast toast-success">{{ fsToast }}</div>
    <div class="files-toolbar">
      <router-link to="/" class="back-link">← 返回首页</router-link>
      <label class="files-conn-label">
        连接
        <select v-model="selectedHostId" class="files-conn-select" @change="onConnChange">
          <option value="">本地</option>
          <option v-for="h in sshHosts" :key="h.id" :value="h.id">{{ h.name }} ({{ h.host }})</option>
        </select>
      </label>
      <button type="button" class="btn btn-sm" @click="openSshManage">管理 SSH 连接</button>
      <div class="path-bar">
        <span class="path-label">路径：</span>
        <input
          v-model="pathInput"
          type="text"
          class="path-input"
          placeholder="/"
          @keydown.enter="goToPath"
        />
        <button type="button" class="btn btn-sm" @click="goToPath">打开</button>
        <button type="button" class="btn btn-sm" @click="refreshList">刷新</button>
      </div>
    </div>

    <div class="upload-area">
      <input
        ref="fileInput"
        type="file"
        multiple
        class="file-input"
        @change="onFileSelect"
      />
      <button type="button" class="btn btn-upload" @click="$refs.fileInput?.click()">
        选择文件上传到当前目录
      </button>
      <button type="button" class="btn btn-mkdir" @click="createFolder">新建文件夹</button>
      <button type="button" class="btn btn-newfile" @click="createFile">新建文件</button>
      <span v-if="uploading" class="upload-status">上传中…</span>
      <span v-if="uploadError" class="upload-error">{{ uploadError }}</span>
    </div>

    <div v-if="loading" class="files-loading">加载中…</div>
    <div v-else-if="listError" class="files-error">{{ listError }}</div>
    <div v-else class="files-table-wrap">
      <table class="files-table">
        <thead>
          <tr>
            <th>名称</th>
            <th class="col-size">大小</th>
            <th class="col-time">修改时间</th>
            <th class="col-actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="currentPath !== '/'">
            <td colspan="4">
              <button type="button" class="link-btn" @click="goParent">.. 上一级</button>
            </td>
          </tr>
          <tr
            v-for="e in entries"
            :key="e.name"
            class="file-row"
            :class="{ dir: e.isDir }"
          >
            <td>
              <span v-if="e.isDir" class="dir-icon">📁</span>
              <span v-else class="file-icon">📄</span>
              <button
                v-if="e.isDir"
                type="button"
                class="link-btn"
                @click="enterDir(e.name)"
              >
                {{ e.name }}/
              </button>
              <span v-else class="file-name">{{ e.name }}</span>
            </td>
            <td class="col-size">{{ e.isDir ? "—" : formatSize(e.size) }}</td>
            <td class="col-time">{{ formatTime(e.mod) }}</td>
            <td class="col-actions">
              <div class="actions-cell">
                <button
                  v-if="!e.isDir && isPreviewable(e.name)"
                  type="button"
                  class="btn btn-sm"
                  @click="openPreview(e.name)"
                >
                  预览
                </button>
                <button
                  v-if="!e.isDir"
                  type="button"
                  class="btn btn-sm"
                  @click="editFile(e.name)"
                >
                  编辑
                </button>
                <a
                  v-if="!e.isDir"
                  :href="downloadUrl(e.name)"
                  class="btn btn-sm"
                  download
                >
                  下载
                </a>
                <button
                  type="button"
                  class="btn btn-sm btn-danger"
                  @click="removeEntry(e)"
                >
                  删除
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="entries.length === 0 && currentPath === '/'">
            <td colspan="4" class="empty">当前目录为空</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 图片/视频预览弹层 -->
    <div
      v-if="previewPath"
      class="preview-overlay"
      @click.self="closePreview"
    >
      <button type="button" class="preview-close" aria-label="关闭" @click="closePreview">×</button>
      <div class="preview-content">
        <img
          v-if="previewType === 'image'"
          :key="previewPath + previewTs"
          :src="previewUrl"
          class="preview-media"
          alt="预览"
          @error="previewError = true"
        />
        <video
          v-else-if="previewType === 'video'"
          :key="previewPath + previewTs"
          :src="previewUrl"
          class="preview-media"
          controls
          autoplay
        />
        <p v-if="previewError && previewType === 'image'" class="preview-error">图片加载失败</p>
      </div>
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
import { ref, computed, onMounted } from "vue";
import { useRouter } from "vue-router";

interface SshHost {
  id: string;
  name: string;
  host: string;
  port?: number;
  user?: string;
  auth_type?: string;
  password?: string;
  private_key_path?: string;
  private_key_content?: string;
}

interface FileEntry {
  name: string;
  size: number;
  isDir: boolean;
  mod: number;
}

const sshHosts = ref<SshHost[]>([]);
const selectedHostId = ref<string | null>(null);
const currentPath = ref("/");
const pathInput = ref("/");
const entries = ref<FileEntry[]>([]);
const loading = ref(false);
const listError = ref("");
const uploading = ref(false);
const uploadError = ref("");
const fileInput = ref<HTMLInputElement | null>(null);
const router = useRouter();
const previewPath = ref("");
const previewType = ref<"image" | "video">("image");
const previewTs = ref(0);
const previewError = ref(false);

const fsToast = ref("");
let fsToastTimer: ReturnType<typeof setTimeout> | null = null;
function showFsToast(msg: string) {
  if (fsToastTimer) clearTimeout(fsToastTimer);
  fsToast.value = msg;
  fsToastTimer = setTimeout(() => {
    fsToastTimer = null;
    fsToast.value = "";
  }, 2000);
}

const sshModalOpen = ref(false);
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

function openSshManage() {
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
function editSshHost(h: SshHost) {
  editingSshId.value = h.id;
  sshForm.value = {
    name: h.name,
    host: h.host,
    port: h.port ?? 22,
    user: h.user ?? "",
    auth_type: h.auth_type ?? "password",
    password: "",
    private_key_path: h.private_key_path ?? "",
    private_key_content: h.private_key_content ?? "",
  };
}
async function deleteSshHost(id: string) {
  if (!window.confirm("确定删除该 SSH 连接？")) return;
  try {
    const res = await fetch(`/api/ssh/hosts/${encodeURIComponent(id)}`, { method: "DELETE" });
    if (!res.ok) {
      const data = await res.json();
      throw new Error(data.error || "删除失败");
    }
    await loadSshHosts();
    if (selectedHostId.value === id) {
      selectedHostId.value = null;
      onConnChange();
    }
    cancelSshEdit();
  } catch (e) {
    listError.value = (e as Error).message;
  }
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
    showFsToast("保存成功");
  } catch (e) {
    listError.value = (e as Error).message;
  }
}

const isSsh = computed(() => !!selectedHostId.value);

async function loadSshHosts() {
  try {
    const res = await fetch("/api/ssh/hosts");
    const data = await res.json();
    if (data.list) sshHosts.value = data.list;
  } catch {
    sshHosts.value = [];
  }
}

function onConnChange() {
  currentPath.value = "/";
  pathInput.value = "/";
  fetchList();
}

const previewExts: Record<string, "image" | "video"> = {
  jpg: "image", jpeg: "image", png: "image", gif: "image", webp: "image",
  svg: "image", bmp: "image", ico: "image",
  mp4: "video", webm: "video", ogg: "video", mov: "video",
};

function isPreviewable(name: string): boolean {
  const ext = name.split(".").pop()?.toLowerCase() ?? "";
  return ext in previewExts;
}

const previewUrl = computed(() => {
  if (!previewPath.value) return "";
  const path = encodeURIComponent(previewPath.value);
  if (isSsh.value && selectedHostId.value) {
    return `/api/ssh/fs/preview?host=${encodeURIComponent(selectedHostId.value)}&path=${path}&_t=${previewTs.value}`;
  }
  return `/api/fs/preview?path=${path}&_t=${previewTs.value}`;
});

function openPreview(name: string) {
  if (!isPreviewable(name)) return;
  previewError.value = false;
  previewPath.value = fullPath(name);
  previewType.value = previewExts[name.split(".").pop()?.toLowerCase() ?? ""] ?? "image";
  previewTs.value = Date.now();
}

function closePreview() {
  previewPath.value = "";
  previewError.value = false;
}

function normalizePath(p: string): string {
  if (!p || p === ".") return "/";
  p = p.replace(/\\/g, "/").replace(/\/+/g, "/").trim();
  if (!p.startsWith("/")) p = "/" + p;
  if (p.length > 1 && p.endsWith("/")) p = p.slice(0, -1);
  return p || "/";
}

async function fetchList() {
  loading.value = true;
  listError.value = "";
  try {
    const path = normalizePath(currentPath.value);
    const url = isSsh.value && selectedHostId.value
      ? `/api/ssh/fs/list?host=${encodeURIComponent(selectedHostId.value)}&path=${encodeURIComponent(path)}`
      : `/api/fs/list?path=${encodeURIComponent(path)}`;
    const res = await fetch(url);
    const data = await res.json();
    if (!res.ok) {
      listError.value = data.error || "加载失败";
      entries.value = [];
      return;
    }
    entries.value = data.entries || [];
    currentPath.value = data.path || path;
    pathInput.value = currentPath.value;
  } catch (e) {
    listError.value = (e as Error).message || "请求失败";
    entries.value = [];
  } finally {
    loading.value = false;
  }
}

function goToPath() {
  currentPath.value = normalizePath(pathInput.value);
  fetchList();
}

function goParent() {
  const p = currentPath.value.replace(/\/+$/, "");
  const idx = p.lastIndexOf("/");
  currentPath.value = idx <= 0 ? "/" : p.slice(0, idx);
  pathInput.value = currentPath.value;
  fetchList();
}

function enterDir(name: string) {
  const base = currentPath.value === "/" ? "" : currentPath.value;
  currentPath.value = base + "/" + name;
  pathInput.value = currentPath.value;
  fetchList();
}

function refreshList() {
  fetchList();
}

function formatSize(n: number): string {
  if (n < 1024) return n + " B";
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
  if (n < 1024 * 1024 * 1024) return (n / (1024 * 1024)).toFixed(1) + " MB";
  return (n / (1024 * 1024 * 1024)).toFixed(1) + " GB";
}

function formatTime(unix: number): string {
  if (!unix) return "—";
  const d = new Date(unix * 1000);
  return d.toLocaleString("zh-CN");
}

function downloadUrl(name: string): string {
  const path = normalizePath(currentPath.value);
  const full = path === "/" ? "/" + name : path + "/" + name;
  if (isSsh.value && selectedHostId.value) {
    return `/api/ssh/fs/download?host=${encodeURIComponent(selectedHostId.value)}&path=${encodeURIComponent(full)}`;
  }
  return `/api/fs/download?path=${encodeURIComponent(full)}`;
}

function fullPath(name: string): string {
  const path = normalizePath(currentPath.value);
  return path === "/" ? "/" + name : path + "/" + name;
}

function editFile(name: string) {
  const query: Record<string, string> = { path: fullPath(name) };
  if (selectedHostId.value) query.host = selectedHostId.value;
  router.push({ path: "/files/editor", query });
}

async function createFolder() {
  const name = window.prompt("请输入文件夹名称");
  if (!name?.trim()) return;
  const path = normalizePath(currentPath.value);
  const body: Record<string, string> = { path, name: name.trim() };
  if (selectedHostId.value) body.host = selectedHostId.value;
  const url = isSsh.value ? "/api/ssh/fs/mkdir" : "/api/fs/mkdir";
  try {
    const res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) {
      listError.value = data.error || "创建失败";
      return;
    }
    listError.value = "";
    showFsToast("文件夹已创建");
    await fetchList();
  } catch (e) {
    listError.value = (e as Error).message || "创建失败";
  }
}

async function createFile() {
  const name = window.prompt("请输入文件名");
  if (!name?.trim()) return;
  const trimmed = name.trim();
  if (trimmed.includes("/") || trimmed.includes("..")) {
    listError.value = "文件名不能包含 / 或 ..";
    return;
  }
  const path = fullPath(trimmed);
  const body: Record<string, string> = { path, content: "" };
  if (selectedHostId.value) body.host = selectedHostId.value;
  const url = isSsh.value ? "/api/ssh/fs/write" : "/api/fs/write";
  listError.value = "";
  try {
    const res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) {
      listError.value = data.error || "创建失败";
      return;
    }
    listError.value = "";
    showFsToast("文件已创建");
    await fetchList();
    editFile(trimmed);
  } catch (e) {
    listError.value = (e as Error).message || "创建失败";
  }
}

async function removeEntry(e: FileEntry) {
  const path = fullPath(e.name);
  const label = e.isDir ? "文件夹" : "文件";
  if (!window.confirm(`确定要删除${label}「${e.name}」吗？`)) return;
  const body: Record<string, string> = { path };
  if (selectedHostId.value) body.host = selectedHostId.value;
  const url = isSsh.value ? "/api/ssh/fs/delete" : "/api/fs/delete";
  try {
    const res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) {
      listError.value = data.error || "删除失败";
      return;
    }
    listError.value = "";
    showFsToast("已删除");
    await fetchList();
  } catch (err) {
    listError.value = (err as Error).message || "删除失败";
  }
}

async function onFileSelect(ev: Event) {
  const input = ev.target as HTMLInputElement;
  const files = input.files;
  if (!files?.length) return;
  uploadError.value = "";
  uploading.value = true;
  const path = normalizePath(currentPath.value);
  const form = new FormData();
  if (selectedHostId.value) form.append("host", selectedHostId.value);
  form.append("path", path);
  for (let i = 0; i < files.length; i++) {
    form.append("file", files[i]);
  }
  const url = isSsh.value ? "/api/ssh/fs/upload" : "/api/fs/upload";
  try {
    const res = await fetch(url, {
      method: "POST",
      body: form,
    });
    const data = await res.json();
    if (!res.ok) {
      uploadError.value = data.error || "上传失败";
      return;
    }
    uploadError.value = "";
    showFsToast("上传成功");
    await fetchList();
  } catch (e) {
    uploadError.value = (e as Error).message || "上传失败";
  } finally {
    uploading.value = false;
    input.value = "";
  }
}

onMounted(() => {
  loadSshHosts();
  fetchList();
});
</script>

<style scoped>
.files-page {
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
.files-toolbar {
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
.files-conn-label {
  font-size: 12px;
  color: #94a3b8;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.files-conn-select {
  padding: 4px 8px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 13px;
  min-width: 140px;
}
.path-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 200px;
}
.path-label {
  font-size: 13px;
  color: #94a3b8;
}
.path-input {
  flex: 1;
  max-width: 400px;
  padding: 6px 10px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 14px;
}
.path-input:focus {
  outline: none;
  border-color: #60a5fa;
}
.btn {
  padding: 6px 12px;
  background: rgba(59, 130, 246, 0.6);
  border: 1px solid rgba(96, 165, 250, 0.5);
  border-radius: 6px;
  color: #e2e8f0;
  cursor: pointer;
  font-size: 13px;
}
.btn:hover {
  background: rgba(59, 130, 246, 0.8);
}
.btn-sm {
  padding: 4px 10px;
  font-size: 12px;
}
.file-input {
  display: none;
}
.upload-area {
  margin-bottom: 16px;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}
.upload-status {
  font-size: 13px;
  color: #facc15;
}
.upload-error {
  font-size: 13px;
  color: #f87171;
}
.files-loading,
.files-error {
  padding: 24px;
  text-align: center;
  color: #94a3b8;
}
.files-error {
  color: #f87171;
}
.files-table-wrap {
  flex: 1;
  overflow: auto;
  background: rgba(15, 23, 42, 0.5);
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.25);
}
.files-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}
.files-table th {
  text-align: left;
  padding: 6px 12px;
  color: #94a3b8;
  font-weight: 500;
  border-bottom: 1px solid rgba(148, 163, 184, 0.3);
}
.files-table td {
  padding: 8px 12px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.15);
}
.file-row:hover {
  background: rgba(148, 163, 184, 0.08);
}
.col-size {
  width: 100px;
}
.col-time {
  width: 180px;
}
.col-actions {
  width: 200px;
}
.actions-cell {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.actions-cell .btn {
  flex-shrink: 0;
}
.btn-danger {
  background: rgba(239, 68, 68, 0.5);
  border-color: rgba(248, 113, 113, 0.5);
}
.btn-danger:hover {
  background: rgba(239, 68, 68, 0.8);
}
.dir-icon,
.file-icon {
  margin-right: 6px;
}
.link-btn {
  background: none;
  border: none;
  color: #60a5fa;
  cursor: pointer;
  padding: 0;
  font-size: inherit;
  text-decoration: underline;
}
.link-btn:hover {
  color: #93c5fd;
}
.file-name {
  color: #e2e8f0;
}
.empty {
  color: #64748b;
  font-style: italic;
}
.files-table a.btn {
  text-decoration: none;
  display: inline-block;
}

/* SSH 连接管理弹层 */
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

/* 图片/视频预览弹层 */
.preview-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(0, 0, 0, 0.85);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
.preview-close {
  position: absolute;
  top: 16px;
  right: 16px;
  width: 40px;
  height: 40px;
  border: none;
  background: rgba(255, 255, 255, 0.15);
  color: #fff;
  font-size: 24px;
  line-height: 1;
  border-radius: 8px;
  cursor: pointer;
}
.preview-close:hover {
  background: rgba(255, 255, 255, 0.25);
}
.preview-content {
  max-width: 100%;
  max-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}
.preview-media {
  max-width: 100%;
  max-height: calc(100vh - 48px);
  object-fit: contain;
}
.preview-error {
  color: #f87171;
  margin: 16px 0 0;
}
</style>
