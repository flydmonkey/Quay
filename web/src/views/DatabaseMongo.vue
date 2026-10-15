<template>
  <div class="db-page">
    <div v-if="toastMsg" class="toast toast-success">{{ toastMsg }}</div>
    <div class="db-toolbar">
      <router-link to="/" class="back-link">← 返回首页</router-link>
      <div class="db-tabs">
        <div
          v-for="tab in tabs"
          :key="tab.id"
          role="button"
          tabindex="0"
          class="db-tab"
          :class="{ 'db-tab-active': tab.id === activeId }"
          @click="switchTab(tab.id)"
          @keydown.enter.space.prevent="switchTab(tab.id)"
        >
          <span class="db-tab-name">{{ tab.name }}</span>
          <button type="button" class="db-tab-close" aria-label="关闭" @click.stop="closeTab(tab.id)">×</button>
        </div>
        <button type="button" class="db-tab-add" aria-label="新建" @click="addTab">+</button>
      </div>
    </div>

    <div v-if="!currentState" class="db-empty">
      <p>暂无工作区，点击「+」新建</p>
    </div>
    <div v-else class="db-layout">
      <aside class="db-side">
        <div class="db-conn-bar">
          <select v-model="currentState.selectedConnId" class="db-select" @change="onConnChange">
            <option value="">选择连接</option>
            <option v-for="c in connections" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
          <div class="db-conn-actions">
            <button type="button" class="btn btn-sm" @click="showConnModal()">添加连接</button>
            <button type="button" class="btn btn-sm" @click="openConnManage">管理连接</button>
            <button
              type="button"
              class="btn btn-sm"
              :disabled="!currentState.selectedConnId"
              @click="showConnModal(currentState.selectedConnId)"
            >
              编辑
            </button>
            <button
              type="button"
              class="btn btn-sm btn-danger"
              :disabled="!currentState.selectedConnId"
              @click="deleteConn"
            >
              删除
            </button>
          </div>
        </div>
        <div v-if="currentState.selectedConnId" class="db-tree-wrap">
          <div v-if="currentState.treeLoading" class="db-tree-loading">加载中…</div>
          <div v-else class="db-tree">
            <template v-for="db in currentState.treeDatabases" :key="db.id">
              <div class="tree-node">
                <button type="button" class="tree-toggle" @click="toggleDb(db.id)">
                  {{ currentState.expandedDb[db.id] ? '▼' : '▶' }}
                </button>
                <span class="tree-label tree-db" @click="selectDatabase(db.label)">{{ db.label }}</span>
              </div>
              <div v-if="currentState.expandedDb[db.id]" class="tree-children">
                <div
                  v-for="col in db.children || []"
                  :key="col.id"
                  class="tree-node tree-leaf tree-table"
                  @click="selectCollection(db.label, col.label)"
                >
                  <span class="tree-dot">·</span>
                  <span class="tree-label">{{ col.label }}</span>
                </div>
              </div>
            </template>
          </div>
        </div>
      </aside>

      <main class="db-main">
        <div class="db-sql-bar">
          <span v-if="currentState.selectedDatabase && currentState.selectedCollection" class="db-current-db">
            {{ currentState.selectedDatabase }} / {{ currentState.selectedCollection }}
          </span>
          <div class="db-sql-bar-actions">
            <button type="button" class="btn btn-sm" @click="formatFilterJson">格式化</button>
            <button
              type="button"
              class="btn btn-run"
              :disabled="!currentState.selectedConnId || !currentState.selectedDatabase || !currentState.selectedCollection || currentState.querying"
              @click="runQuery"
            >
              {{ currentState.querying ? '执行中…' : '执行' }}
            </button>
          </div>
        </div>
        <div class="db-mongo-query">
          <div class="db-mongo-row">
            <label>过滤条件 (JSON)，按回车自动格式化</label>
            <textarea
              v-model="currentState.filterJson"
              class="db-mongo-filter"
              placeholder='{}'
              spellcheck="false"
              @keydown.enter.exact.prevent="formatFilterJson"
            ></textarea>
          </div>
          <div class="db-mongo-opts">
            <label>
              每页
              <input v-model.number="currentState.limit" type="number" min="1" max="5000" class="db-mongo-num" />
              条
            </label>
            <label>
              跳过
              <input v-model.number="currentState.skip" type="number" min="0" class="db-mongo-num" />
              条
            </label>
          </div>
        </div>
        <div v-if="currentState.queryError" class="db-query-error">{{ currentState.queryError }}</div>
        <div v-if="currentState.queryColumns.length" class="db-result-wrap">
          <div class="db-result-scroll">
            <table class="db-result-table">
              <thead>
                <tr>
                  <th v-for="col in currentState.queryColumns" :key="col">{{ col }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, i) in displayedRows" :key="pageStart + i">
                  <td v-for="(colName, j) in currentState.queryColumns" :key="j">{{ formatCell(row[j]) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="db-result-footer">
            <div class="db-result-meta">
              共 {{ currentState.queryRows.length }} 条
              <template v-if="currentState.queryRows.length > 0">
                ，第 {{ pageStart + 1 }}-{{ Math.min(pageEnd, currentState.queryRows.length) }} 条
              </template>
            </div>
            <div class="db-pagination">
              <label class="db-page-size">
                每页
                <select v-model.number="currentState.pageSize">
                  <option :value="20">20</option>
                  <option :value="25">25</option>
                  <option :value="50">50</option>
                  <option :value="100">100</option>
                  <option :value="200">200</option>
                </select>
                条
              </label>
              <button
                type="button"
                class="btn btn-sm"
                :disabled="currentState.currentPage <= 1"
                @click="currentState.currentPage = Math.max(1, currentState.currentPage - 1)"
              >
                上一页
              </button>
              <span class="db-page-info">第 {{ currentState.currentPage }} / {{ totalPages || 1 }} 页</span>
              <button
                type="button"
                class="btn btn-sm"
                :disabled="currentState.currentPage >= totalPages"
                @click="currentState.currentPage = Math.min(totalPages, currentState.currentPage + 1)"
              >
                下一页
              </button>
              <span class="db-export-sep">|</span>
              <button type="button" class="btn btn-sm btn-export" @click="exportCSV">导出 CSV</button>
            </div>
          </div>
        </div>
      </main>
    </div>

    <!-- 添加/编辑连接弹层 -->
    <div v-if="connModal" class="modal-overlay" @click.self="closeConnModal">
      <div class="modal-box">
        <h3>{{ editingConnId ? '编辑连接' : '添加连接' }}</h3>
        <form class="conn-form" @submit.prevent="saveConn">
          <label>名称</label>
          <input v-model="form.name" type="text" required placeholder="如：本地 MongoDB" />
          <label>连接字符串 (URI)</label>
          <input
            v-model="form.uri"
            type="text"
            required
            placeholder="mongodb://localhost:27017 或 mongodb://user:pass@host:27017/authSource"
          />
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
        <h3>MongoDB 连接管理</h3>
        <div class="conn-list">
          <div v-for="c in connections" :key="c.id" class="conn-list-item">
            <span class="conn-list-name">{{ c.name }}</span>
            <span class="conn-list-addr">{{ c.uri || '' }}</span>
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
          <input v-model="form.name" type="text" required placeholder="如：本地 MongoDB" />
          <label>连接字符串 (URI)</label>
          <input
            v-model="form.uri"
            type="text"
            :placeholder="editingConnId ? '留空则不修改' : 'mongodb://localhost:27017'"
          />
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
import { ref, computed, reactive, onMounted, watch } from "vue";

interface Conn {
  id: string;
  name: string;
  uri: string;
}

interface TreeItem {
  id: string;
  label: string;
  type: string;
  children?: TreeItem[];
}

interface MongoTabState {
  selectedConnId: string;
  treeDatabases: TreeItem[];
  treeLoading: boolean;
  expandedDb: Record<string, boolean>;
  selectedDatabase: string;
  selectedCollection: string;
  filterJson: string;
  limit: number;
  skip: number;
  queryColumns: string[];
  queryRows: unknown[][];
  queryError: string;
  querying: boolean;
  pageSize: number;
  currentPage: number;
}

function createDefaultTabState(): MongoTabState {
  return reactive({
    selectedConnId: "",
    treeDatabases: [],
    treeLoading: false,
    expandedDb: {},
    selectedDatabase: "",
    selectedCollection: "",
    filterJson: "{}",
    limit: 100,
    skip: 0,
    queryColumns: [],
    queryRows: [],
    queryError: "",
    querying: false,
    pageSize: 25,
    currentPage: 1,
  });
}

const tabs = ref<{ id: string; name: string }[]>([]);
const activeId = ref<string | null>(null);
const stateByTabId = reactive<Record<string, MongoTabState>>({});
const currentState = computed(() => (activeId.value ? stateByTabId[activeId.value] : null));

const connections = ref<Conn[]>([]);
const connModal = ref(false);
const connManageModal = ref(false);
const toastMsg = ref("");
let toastTimer: ReturnType<typeof setTimeout> | null = null;
const editingConnId = ref("");
const form = ref({ name: "", uri: "" });

const totalPages = computed(() =>
  currentState.value
    ? Math.max(1, Math.ceil(currentState.value.queryRows.length / currentState.value.pageSize))
    : 1
);
const pageStart = computed(() =>
  currentState.value ? (currentState.value.currentPage - 1) * currentState.value.pageSize : 0
);
const pageEnd = computed(() =>
  currentState.value ? currentState.value.currentPage * currentState.value.pageSize : 0
);
const displayedRows = computed(() =>
  currentState.value
    ? currentState.value.queryRows.slice(pageStart.value, pageEnd.value)
    : []
);

function addTab() {
  const id = `mongo-${Date.now()}`;
  const name = `Mongo ${tabs.value.length + 1}`;
  tabs.value.push({ id, name });
  stateByTabId[id] = createDefaultTabState();
  activeId.value = id;
}

function switchTab(id: string) {
  activeId.value = id;
}

function closeTab(id: string) {
  const idx = tabs.value.findIndex((t) => t.id === id);
  if (idx === -1) return;
  tabs.value.splice(idx, 1);
  delete stateByTabId[id];
  if (activeId.value === id) {
    activeId.value = tabs.value[idx]?.id ?? tabs.value[idx - 1]?.id ?? null;
  }
}

function showToast(msg: string) {
  if (toastTimer) clearTimeout(toastTimer);
  toastMsg.value = msg;
  toastTimer = setTimeout(() => {
    toastTimer = null;
    toastMsg.value = "";
  }, 2000);
}

async function loadConnections() {
  try {
    const res = await fetch("/api/mongo/connections");
    const data = await res.json();
    if (data.list) connections.value = data.list;
  } catch {
    connections.value = [];
  }
}

function onConnChange() {
  const s = currentState.value;
  if (!s) return;
  s.treeDatabases = [];
  s.selectedDatabase = "";
  s.selectedCollection = "";
  s.expandedDb = {};
  if (!s.selectedConnId) return;
  s.treeLoading = true;
  fetch(`/api/mongo/tree?connectionId=${encodeURIComponent(s.selectedConnId)}`)
    .then((r) => r.json())
    .then((data) => {
      if (data.databases) s.treeDatabases = data.databases;
      s.treeLoading = false;
    })
    .catch(() => {
      s.treeLoading = false;
    });
}

function toggleDb(id: string) {
  const s = currentState.value;
  if (!s) return;
  s.expandedDb[id] = !s.expandedDb[id];
}

function selectDatabase(name: string) {
  const s = currentState.value;
  if (s) s.selectedDatabase = name;
}

function selectCollection(dbName: string, colName: string) {
  const s = currentState.value;
  if (!s) return;
  s.selectedDatabase = dbName;
  s.selectedCollection = colName;
  s.filterJson = "{}";
  s.skip = 0;
  s.limit = 100;
  runQuery();
}

function showConnModal(id?: string) {
  editingConnId.value = id || "";
  if (id) {
    const c = connections.value.find((x) => x.id === id);
    if (c) form.value = { name: c.name, uri: "" };
  } else {
    form.value = { name: "", uri: "mongodb://localhost:27017" };
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
  form.value = { name: "", uri: "" };
}

function closeConnManageModal() {
  connManageModal.value = false;
  editingConnId.value = "";
}

function editConnFromList(c: Conn) {
  editingConnId.value = c.id;
  form.value = { name: c.name, uri: "" };
}

function cancelConnEdit() {
  editingConnId.value = "";
  form.value = { name: "", uri: "" };
}

async function deleteConnFromList(id: string) {
  if (!confirm("确定删除该连接？")) return;
  try {
    const res = await fetch(`/api/mongo/connections/${id}`, { method: "DELETE" });
    if (!res.ok) throw new Error("删除失败");
    await loadConnections();
    for (const tid of Object.keys(stateByTabId)) {
      const s = stateByTabId[tid];
      if (s.selectedConnId === id) {
        s.selectedConnId = "";
        s.treeDatabases = [];
        s.selectedDatabase = "";
        s.selectedCollection = "";
      }
    }
    cancelConnEdit();
    showToast("已删除");
  } catch (e) {
    const s = currentState.value;
    if (s) s.queryError = (e as Error).message;
  }
}

async function saveConnFromManage() {
  const body: Record<string, string> = {
    name: form.value.name,
    uri: form.value.uri,
  };
  if (editingConnId.value) body.id = editingConnId.value;
  try {
    const res = await fetch("/api/mongo/connections", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "保存失败");
    await loadConnections();
    cancelConnEdit();
    showToast("保存成功");
  } catch (e) {
    const s = currentState.value;
    if (s) s.queryError = (e as Error).message;
  }
}

async function saveConn() {
  const body: Record<string, string> = {
    name: form.value.name,
    uri: form.value.uri,
  };
  if (editingConnId.value) body.id = editingConnId.value;
  try {
    const res = await fetch("/api/mongo/connections", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "保存失败");
    connModal.value = false;
    await loadConnections();
    const s = currentState.value;
    if (data.id && s) {
      s.selectedConnId = data.id;
      onConnChange();
    }
    showToast("保存成功");
  } catch (e) {
    const s = currentState.value;
    if (s) s.queryError = (e as Error).message;
  }
}

async function deleteConn() {
  const s = currentState.value;
  if (!s?.selectedConnId || !confirm("确定删除该连接？")) return;
  try {
    const res = await fetch(`/api/mongo/connections/${s.selectedConnId}`, { method: "DELETE" });
    if (!res.ok) throw new Error("删除失败");
    s.selectedConnId = "";
    s.treeDatabases = [];
    s.selectedDatabase = "";
    s.selectedCollection = "";
    await loadConnections();
    showToast("已删除");
  } catch (e) {
    if (s) s.queryError = (e as Error).message;
  }
}

function formatFilterJson() {
  const s = currentState.value;
  if (!s) return;
  const raw = s.filterJson.trim() || "{}";
  try {
    const parsed = JSON.parse(raw);
    s.filterJson = JSON.stringify(parsed, null, 2);
  } catch {
    s.queryError = "当前内容不是合法 JSON，无法格式化";
    setTimeout(() => { s.queryError = ""; }, 2000);
  }
}

async function runQuery() {
  const s = currentState.value;
  if (!s || !s.selectedConnId || !s.selectedDatabase || !s.selectedCollection) return;
  let filter: Record<string, unknown>;
  try {
    filter = JSON.parse(s.filterJson.trim() || "{}");
  } catch {
    s.queryError = "过滤条件不是合法 JSON";
    return;
  }
  s.queryError = "";
  s.querying = true;
  try {
    const res = await fetch("/api/mongo/query", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        connectionId: s.selectedConnId,
        database: s.selectedDatabase,
        collection: s.selectedCollection,
        filter,
        limit: s.limit || 100,
        skip: s.skip || 0,
      }),
    });
    const data = await res.json();
    if (data.error) {
      s.queryError = data.error;
      s.queryColumns = [];
      s.queryRows = [];
      return;
    }
    s.queryColumns = data.columns || [];
    s.queryRows = data.rows || [];
    s.currentPage = 1;
  } finally {
    s.querying = false;
  }
}

watch(
  () => [currentState.value?.pageSize, currentState.value?.queryRows.length],
  () => {
    const s = currentState.value;
    if (s) s.currentPage = 1;
  }
);

function formatCell(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "object") return JSON.stringify(v);
  return String(v);
}

function exportCSV() {
  const s = currentState.value;
  if (!s || !s.queryColumns.length || !s.queryRows.length) return;
  const cols = s.queryColumns;
  const escape = (x: unknown) => {
    const str = x == null ? "" : String(x);
    if (/[",\n\r]/.test(str)) return '"' + str.replace(/"/g, '""') + '"';
    return str;
  };
  const line = (row: unknown[]) => cols.map((_, i) => escape(row[i])).join(",");
  const bom = "\uFEFF";
  const csv = bom + cols.join(",") + "\n" + s.queryRows.map(line).join("\n");
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `mongo_${s.selectedDatabase}_${s.selectedCollection}.csv`;
  a.click();
  URL.revokeObjectURL(url);
  showToast("已导出 CSV");
}

onMounted(() => {
  loadConnections();
  if (tabs.value.length === 0) addTab();
});
</script>

<style scoped>
.db-page {
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
.db-toolbar {
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
.db-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  flex-shrink: 0;
}
.db-tab {
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
.db-tab:hover {
  background: rgba(51, 65, 85, 0.9);
  color: #e2e8f0;
}
.db-tab-active {
  background: rgba(59, 130, 246, 0.25);
  border-color: rgba(96, 165, 250, 0.5);
  color: #e2e8f0;
}
.db-tab-name {
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.db-tab-close {
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
.db-tab-close:hover {
  color: #f87171;
  background: rgba(248, 113, 113, 0.15);
}
.db-tab-add {
  padding: 4px 10px;
  background: rgba(30, 41, 59, 0.8);
  border: 1px dashed rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #94a3b8;
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
}
.db-tab-add:hover {
  border-color: rgba(96, 165, 250, 0.6);
  color: #60a5fa;
}
.db-empty {
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
.db-layout {
  flex: 1;
  display: flex;
  gap: 20px;
  min-height: 0;
}
.db-side {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: rgba(15, 23, 42, 0.5);
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  overflow: hidden;
}
.db-conn-bar {
  padding: 10px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.db-select {
  padding: 6px 8px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 13px;
}
.db-conn-actions {
  display: flex;
  gap: 8px;
}
.db-tree-wrap {
  flex: 1;
  overflow: auto;
  padding: 8px;
}
.db-tree-loading {
  color: #94a3b8;
  font-size: 13px;
}
.db-tree {
  font-size: 13px;
}
.tree-node {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 0;
}
.tree-toggle {
  background: none;
  border: none;
  color: #94a3b8;
  cursor: pointer;
  padding: 0;
  width: 18px;
  font-size: 10px;
}
.tree-label {
  color: #e2e8f0;
}
.tree-db {
  cursor: pointer;
  font-weight: 500;
}
.tree-db:hover {
  color: #60a5fa;
}
.tree-children {
  padding-left: 18px;
}
.tree-leaf .tree-dot {
  color: #64748b;
  margin-right: 4px;
}
.tree-table {
  cursor: pointer;
}
.tree-table:hover .tree-label {
  color: #60a5fa;
}
.db-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: rgba(15, 23, 42, 0.5);
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  overflow: hidden;
}
.db-sql-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}
.db-current-db {
  font-size: 13px;
  color: #94a3b8;
}
.db-sql-bar-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
}
.btn-run {
  padding: 6px 16px;
  background: rgba(34, 197, 94, 0.6);
  border-color: rgba(74, 222, 128, 0.5);
}
.btn-run:hover:not(:disabled) {
  background: rgba(34, 197, 94, 0.8);
}
.db-mongo-query {
  padding: 12px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  flex-shrink: 0;
}
.db-mongo-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.db-mongo-row label {
  font-size: 13px;
  color: #94a3b8;
}
.db-mongo-filter {
  width: 100%;
  padding: 10px 12px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 13px;
  font-family: ui-monospace, monospace;
  line-height: 1.5;
  resize: vertical;
  min-height: 200px;
  max-height: 400px;
}
.db-mongo-opts {
  display: flex;
  gap: 16px;
  margin-top: 8px;
  font-size: 13px;
  color: #94a3b8;
}
.db-mongo-opts label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.db-mongo-num {
  width: 70px;
  padding: 4px 8px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 4px;
  color: #e2e8f0;
}
.db-query-error {
  padding: 8px 12px;
  color: #f87171;
  font-size: 13px;
}
.db-result-wrap {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}
.db-result-scroll {
  flex: 1;
  overflow: auto;
}
.db-result-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}
.db-result-table th,
.db-result-table td {
  padding: 6px 12px;
  text-align: left;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  min-width: 3em;
}
.db-result-table th {
  position: sticky;
  top: 0;
  background: rgba(30, 41, 59, 0.95);
  color: #94a3b8;
  font-weight: 500;
}
.db-result-table td {
  color: #e2e8f0;
}
.db-result-footer {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 8px 12px;
  border-top: 1px solid rgba(148, 163, 184, 0.2);
  font-size: 13px;
  color: #94a3b8;
}
.db-result-meta {
  flex: 1;
}
.db-pagination {
  display: flex;
  align-items: center;
  gap: 8px;
}
.db-page-size select {
  padding: 2px 6px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 4px;
  color: #e2e8f0;
  font-size: 12px;
}
.db-page-info {
  min-width: 80px;
}
.db-export-sep {
  color: rgba(148, 163, 184, 0.5);
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
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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
.btn-export {
  background: rgba(148, 163, 184, 0.3);
  border-color: rgba(148, 163, 184, 0.4);
}
.btn-export:hover {
  background: rgba(148, 163, 184, 0.5);
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
  word-break: break-all;
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
.conn-form input {
  width: 100%;
  padding: 8px 10px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 13px;
}
.modal-actions {
  margin-top: 16px;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}
</style>
