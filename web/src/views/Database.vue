<template>
  <div class="db-page">
    <div v-if="connSaveSuccess" class="toast toast-success">{{ connSaveSuccess }}</div>
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
      <!-- 左侧：连接 + 树 -->
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
                  v-for="t in db.children"
                  :key="t.id"
                  class="tree-node tree-leaf tree-table"
                  @click="selectTable(db.label, t.label)"
                >
                  <span class="tree-dot">·</span>
                  <span class="tree-label">{{ t.label }}</span>
                </div>
              </div>
            </template>
          </div>
        </div>
      </aside>

      <!-- 右侧：SQL + 结果 -->
      <main class="db-main">
        <div class="db-sql-bar">
          <span v-if="currentState.selectedDatabase" class="db-current-db">当前库：{{ currentState.selectedDatabase }}</span>
          <button
            type="button"
            class="btn btn-run"
            :disabled="!currentState.selectedConnId || !currentState.sql.trim() || currentState.querying"
            @click="runQuery"
          >
            {{ currentState.querying ? '执行中…' : '执行' }}
          </button>
        </div>
        <div class="db-sql-input-wrap">
          <textarea
            ref="sqlInputRef"
            v-model="currentState.sql"
            class="db-sql-input"
            placeholder="输入 SQL，如 SELECT * FROM table LIMIT 10；选中部分文字可仅执行选中 SQL，支持提示补全"
            spellcheck="false"
            @input="onSqlInput"
            @keydown="onSqlKeydown"
            @blur="onSqlBlur"
          />
          <div
            v-if="showHintDropdown && hintList.length"
            class="db-sql-hint"
            role="listbox"
            :style="hintPosition"
          >
            <div
              v-for="(item, i) in hintList"
              :key="item"
              class="db-sql-hint-item"
              :class="{ 'db-sql-hint-item-active': i === hintSelectedIndex }"
              role="option"
              @mousedown.prevent="applyHint(item)"
            >
              {{ item }}
            </div>
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
              共 {{ currentState.queryRows.length }} 行
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
              <button type="button" class="btn btn-sm btn-export" @click="exportSQL">导出 SQL</button>
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
          <input v-model="form.name" type="text" required placeholder="如：本地 MySQL" />
          <label>类型</label>
          <select v-model="form.driver">
            <option value="mysql">MySQL</option>
            <option value="postgres">PostgreSQL</option>
          </select>
          <label>主机</label>
          <input v-model="form.host" type="text" required placeholder="127.0.0.1" />
          <label>端口</label>
          <input v-model.number="form.port" type="number" placeholder="3306 / 5432" />
          <label>用户名</label>
          <input v-model="form.user" type="text" required />
          <label>密码</label>
          <input v-model="form.password" type="password" :placeholder="editingConnId ? '留空则不修改' : ''" />
          <label>默认数据库（可选）</label>
          <input v-model="form.database" type="text" placeholder="连接后使用的库" />
          <div class="modal-actions">
            <button type="button" class="btn btn-secondary" @click="closeConnModal">取消</button>
            <button type="submit" class="btn btn-primary">保存</button>
          </div>
        </form>
      </div>
    </div>

    <!-- 管理连接弹层：列表 + 编辑/删除 -->
    <div v-if="connManageModal" class="modal-overlay" @click.self="closeConnManageModal">
      <div class="modal-box modal-conn-manage">
        <h3>数据库连接管理</h3>
        <div class="conn-list">
          <div v-for="c in connections" :key="c.id" class="conn-list-item">
            <span class="conn-list-name">{{ c.name }}</span>
            <span class="conn-list-addr">{{ c.driver }} · {{ c.user }}@{{ c.host }}:{{ c.port || (c.driver === 'mysql' ? 3306 : 5432) }}</span>
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
          <input v-model="form.name" type="text" required placeholder="如：本地 MySQL" />
          <label>类型</label>
          <select v-model="form.driver">
            <option value="mysql">MySQL</option>
            <option value="postgres">PostgreSQL</option>
          </select>
          <label>主机</label>
          <input v-model="form.host" type="text" required placeholder="127.0.0.1" />
          <label>端口</label>
          <input v-model.number="form.port" type="number" placeholder="3306 / 5432" />
          <label>用户名</label>
          <input v-model="form.user" type="text" required />
          <label>密码</label>
          <input v-model="form.password" type="password" :placeholder="editingConnId ? '留空则不修改' : ''" />
          <label>默认数据库（可选）</label>
          <input v-model="form.database" type="text" placeholder="连接后使用的库" />
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
import { ref, computed, reactive, onMounted, onUnmounted, watch, nextTick } from "vue";

interface Conn {
  id: string;
  name: string;
  driver: string;
  host: string;
  port: number;
  user: string;
  password: string;
  database?: string;
}

interface TreeItem {
  id: string;
  label: string;
  type: string;
  children?: TreeItem[];
}

interface DbTabState {
  selectedConnId: string;
  treeDatabases: TreeItem[];
  treeLoading: boolean;
  expandedDb: Record<string, boolean>;
  selectedDatabase: string;
  sql: string;
  queryError: string;
  queryColumns: string[];
  queryRows: unknown[][];
  querying: boolean;
  pageSize: number;
  currentPage: number;
  lastQueriedTableName: string;
}

function createDefaultTabState(): DbTabState {
  return reactive({
    selectedConnId: "",
    treeDatabases: [],
    treeLoading: false,
    expandedDb: {},
    selectedDatabase: "",
    sql: "",
    queryError: "",
    queryColumns: [],
    queryRows: [],
    querying: false,
    pageSize: 25,
    currentPage: 1,
    lastQueriedTableName: "",
  });
}

const tabs = ref<{ id: string; name: string }[]>([]);
const activeId = ref<string | null>(null);
const stateByTabId = reactive<Record<string, DbTabState>>({});
const currentState = computed(() => (activeId.value ? stateByTabId[activeId.value] : null));

const connections = ref<Conn[]>([]);
const sqlInputRef = ref<HTMLTextAreaElement | null>(null);
const showHintDropdown = ref(false);
const hintList = ref<string[]>([]);
const hintSelectedIndex = ref(0);
const hintWordStart = ref(0);
const hintPosition = ref<{ top: string; left: string; width: string }>({
  top: "0",
  left: "0",
  width: "100%",
});

function updateHintPosition() {
  const el = sqlInputRef.value;
  if (!el) return;
  const rect = el.getBoundingClientRect();
  const pad = 12;
  hintPosition.value = {
    top: `${rect.bottom + 2}px`,
    left: `${rect.left + pad}px`,
    width: `${Math.max(200, rect.width - pad * 2)}px`,
  };
}

const SQL_KEYWORDS = [
  "SELECT", "FROM", "WHERE", "AND", "OR", "NOT", "IN", "LIKE", "BETWEEN", "IS", "NULL",
  "INSERT", "INTO", "VALUES", "UPDATE", "SET", "DELETE", "CREATE", "TABLE", "DROP", "ALTER",
  "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "ON", "AS", "ORDER", "BY", "GROUP", "HAVING",
  "ASC", "DESC", "LIMIT", "OFFSET", "DISTINCT", "COUNT", "SUM", "AVG", "MIN", "MAX",
  "UNION", "ALL", "CASE", "WHEN", "THEN", "ELSE", "END", "EXISTS",
];
const hintTableNames = computed(() => {
  const s = currentState.value;
  if (!s) return [];
  const names: string[] = [];
  for (const db of s.treeDatabases) {
    if (db.children) {
      for (const t of db.children) names.push(t.label);
    }
  }
  return names;
});
const hintAllSuggestions = computed(() => {
  const set = new Set<string>([...SQL_KEYWORDS, ...hintTableNames.value]);
  return [...set].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));
});

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
  const id = `db-${Date.now()}`;
  const name = `SQL ${tabs.value.length + 1}`;
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
const connModal = ref(false);
const connManageModal = ref(false);
const connSaveSuccess = ref("");
let connSaveSuccessTimer: ReturnType<typeof setTimeout> | null = null;
const editingConnId = ref("");
const form = ref({
  name: "",
  driver: "mysql",
  host: "127.0.0.1",
  port: 3306,
  user: "",
  password: "",
  database: "",
});

async function loadConnections() {
  try {
    const res = await fetch("/api/db/connections");
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
  s.expandedDb = {};
  if (!s.selectedConnId) return;
  s.treeLoading = true;
  fetch(`/api/db/tree?connectionId=${encodeURIComponent(s.selectedConnId)}`)
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

function selectTable(databaseName: string, tableName: string) {
  const s = currentState.value;
  if (!s) return;
  s.selectedDatabase = databaseName;
  s.lastQueriedTableName = tableName;
  const conn = connections.value.find((c) => c.id === s.selectedConnId);
  const driver = conn?.driver ?? "mysql";
  const quoted =
    driver === "mysql"
      ? "`" + tableName.replace(/`/g, "``") + "`"
      : '"' + tableName.replace(/"/g, '""') + '"';
  s.sql = `SELECT * FROM ${quoted} LIMIT 1000`;
}

function showConnModal(id?: string) {
  editingConnId.value = id || "";
  if (id) {
    const c = connections.value.find((x) => x.id === id);
    if (c) {
      form.value = {
        name: c.name,
        driver: c.driver,
        host: c.host,
        port: c.port || (c.driver === "mysql" ? 3306 : 5432),
        user: c.user,
        password: "",
        database: c.database || "",
      };
    }
  } else {
    form.value = {
      name: "",
      driver: "mysql",
      host: "127.0.0.1",
      port: 3306,
      user: "",
      password: "",
      database: "",
    };
  }
  connModal.value = true;
}

function closeConnModal() {
  connModal.value = false;
  editingConnId.value = "";
}

function openConnManage() {
  connManageModal.value = true;
  cancelConnEdit();
}

function closeConnManageModal() {
  connManageModal.value = false;
  cancelConnEdit();
}

function editConnFromList(c: Conn) {
  editingConnId.value = c.id;
  form.value = {
    name: c.name,
    driver: c.driver,
    host: c.host,
    port: c.port || (c.driver === "mysql" ? 3306 : 5432),
    user: c.user,
    password: "",
    database: c.database || "",
  };
}

function cancelConnEdit() {
  editingConnId.value = "";
  form.value = {
    name: "",
    driver: "mysql",
    host: "127.0.0.1",
    port: 3306,
    user: "",
    password: "",
    database: "",
  };
}

async function deleteConnFromList(id: string) {
  if (!confirm("确定删除该连接？")) return;
  try {
    const res = await fetch(`/api/db/connections/${id}`, { method: "DELETE" });
    if (!res.ok) throw new Error("删除失败");
    await loadConnections();
    for (const tid of Object.keys(stateByTabId)) {
      const s = stateByTabId[tid];
      if (s.selectedConnId === id) {
        s.selectedConnId = "";
        s.treeDatabases = [];
        s.selectedDatabase = "";
      }
    }
    cancelConnEdit();
  } catch (e) {
    const s = currentState.value;
    if (s) s.queryError = (e as Error).message;
  }
}

async function saveConnFromManage() {
  const body: Record<string, unknown> = {
    name: form.value.name,
    driver: form.value.driver,
    host: form.value.host,
    port: form.value.port || (form.value.driver === "mysql" ? 3306 : 5432),
    user: form.value.user,
    database: form.value.database || undefined,
  };
  if (editingConnId.value) body.id = editingConnId.value;
  if (form.value.password) body.password = form.value.password;
  try {
    const res = await fetch("/api/db/connections", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "保存失败");
    await loadConnections();
    cancelConnEdit();
    showConnSaveSuccess();
  } catch (e) {
    const s = currentState.value;
    if (s) s.queryError = (e as Error).message;
  }
}

function showConnSaveSuccess() {
  if (connSaveSuccessTimer) clearTimeout(connSaveSuccessTimer);
  connSaveSuccess.value = "保存成功";
  connSaveSuccessTimer = setTimeout(() => {
    connSaveSuccessTimer = null;
    connSaveSuccess.value = "";
  }, 2000);
}

async function saveConn() {
  const body: Record<string, unknown> = {
    name: form.value.name,
    driver: form.value.driver,
    host: form.value.host,
    port: form.value.port || (form.value.driver === "mysql" ? 3306 : 5432),
    user: form.value.user,
    database: form.value.database || undefined,
  };
  if (editingConnId.value) body.id = editingConnId.value;
  if (form.value.password) body.password = form.value.password;
  try {
    const res = await fetch("/api/db/connections", {
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
    showConnSaveSuccess();
  } catch (e) {
    const s = currentState.value;
    if (s) s.queryError = (e as Error).message;
  }
}

async function deleteConn() {
  const s = currentState.value;
  if (!s?.selectedConnId || !confirm("确定删除该连接？")) return;
  try {
    const res = await fetch(`/api/db/connections/${s.selectedConnId}`, { method: "DELETE" });
    if (!res.ok) throw new Error("删除失败");
    s.selectedConnId = "";
    s.treeDatabases = [];
    s.selectedDatabase = "";
    await loadConnections();
  } catch (e) {
    if (s) s.queryError = (e as Error).message;
  }
}

function getCurrentWord(): { word: string; start: number } {
  const el = sqlInputRef.value;
  if (!el) return { word: "", start: 0 };
  const pos = el.selectionStart;
  const textBefore = el.value.slice(0, pos);
  const m = textBefore.match(/[\w*]+$/);
  const word = m ? m[0] : "";
  return { word, start: pos - word.length };
}

function onSqlInput() {
  const { word, start } = getCurrentWord();
  if (!word) {
    showHintDropdown.value = false;
    return;
  }
  const upper = word.toUpperCase();
  const list = hintAllSuggestions.value.filter((s) => s.toUpperCase().startsWith(upper));
  if (list.length === 0) {
    showHintDropdown.value = false;
    return;
  }
  hintList.value = list.slice(0, 20);
  hintSelectedIndex.value = 0;
  hintWordStart.value = start;
  showHintDropdown.value = true;
  nextTick(updateHintPosition);
}

function onSqlKeydown(e: KeyboardEvent) {
  if (!showHintDropdown.value || !hintList.value.length) return;
  const el = sqlInputRef.value;
  if (!el) return;
  if (e.key === "ArrowDown") {
    e.preventDefault();
    hintSelectedIndex.value = Math.min(hintSelectedIndex.value + 1, hintList.value.length - 1);
    return;
  }
  if (e.key === "ArrowUp") {
    e.preventDefault();
    hintSelectedIndex.value = Math.max(hintSelectedIndex.value - 1, 0);
    return;
  }
  if (e.key === "Enter" || e.key === "Tab") {
    e.preventDefault();
    applyHint(hintList.value[hintSelectedIndex.value]);
    return;
  }
  if (e.key === "Escape") {
    showHintDropdown.value = false;
  }
}

function onSqlBlur() {
  setTimeout(() => {
    showHintDropdown.value = false;
  }, 150);
}

function applyHint(item: string) {
  const el = sqlInputRef.value;
  const s = currentState.value;
  if (!el || !s) return;
  const start = hintWordStart.value;
  const end = el.selectionStart;
  const before = s.sql.slice(0, start);
  const after = s.sql.slice(end);
  s.sql = before + item + after;
  showHintDropdown.value = false;
  nextTick(() => {
    el.focus();
    const newPos = start + item.length;
    el.setSelectionRange(newPos, newPos);
  });
}

function getSqlToRun(): string {
  const el = sqlInputRef.value;
  const s = currentState.value;
  if (!s) return "";
  if (el && el.selectionStart !== el.selectionEnd) {
    const start = el.selectionStart;
    const end = el.selectionEnd;
    return el.value.slice(start, end).trim();
  }
  return s.sql.trim();
}

async function runQuery() {
  const sqlToRun = getSqlToRun();
  const s = currentState.value;
  if (!s || !s.selectedConnId || !sqlToRun) return;
  s.queryError = "";
  s.queryColumns = [];
  s.queryRows = [];
  s.querying = true;
  try {
    const res = await fetch("/api/db/query", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        connectionId: s.selectedConnId,
        database: s.selectedDatabase || undefined,
        sql: sqlToRun,
      }),
    });
    const data = await res.json();
    if (data.error) {
      s.queryError = data.error;
      return;
    }
    s.queryColumns = data.columns || [];
    s.queryRows = data.rows || [];
    s.currentPage = 1;
  } catch (e) {
    s.queryError = (e as Error).message;
  } finally {
    s.querying = false;
  }
}

function formatCell(v: unknown): string {
  if (v == null || v === undefined) return "NULL";
  if (typeof v === "object" && v !== null) {
    const o = v as Record<string, unknown>;
    if ("Valid" in o && "String" in o && typeof o.Valid === "boolean")
      return o.Valid ? String(o.String) : "NULL";
    if ("Valid" in o && "Int64" in o && typeof o.Valid === "boolean")
      return o.Valid ? String(o.Int64) : "NULL";
    if ("Valid" in o && "Float64" in o && typeof o.Valid === "boolean")
      return o.Valid ? String(o.Float64) : "NULL";
    if ("Valid" in o && "Bool" in o && typeof o.Valid === "boolean")
      return o.Valid ? String(o.Bool) : "NULL";
    return JSON.stringify(v);
  }
  if (typeof v === "number" && Number.isNaN(v)) return "NULL";
  return String(v);
}

function cellToRaw(v: unknown): string {
  if (v == null || v === undefined) return "";
  if (typeof v === "object" && v !== null) {
    const o = v as Record<string, unknown>;
    if ("Valid" in o && "String" in o && typeof o.Valid === "boolean")
      return o.Valid ? String(o.String) : "";
    if ("Valid" in o && "Int64" in o && typeof o.Valid === "boolean")
      return o.Valid ? String(o.Int64) : "";
    if ("Valid" in o && "Float64" in o && typeof o.Valid === "boolean")
      return o.Valid ? String(o.Float64) : "";
    if ("Valid" in o && "Bool" in o && typeof o.Valid === "boolean")
      return o.Valid ? String(o.Bool) : "";
    return JSON.stringify(v);
  }
  if (typeof v === "number" && Number.isNaN(v)) return "";
  return String(v);
}

function downloadFile(filename: string, mimeType: string, content: string) {
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

function escapeCsvCell(s: string): string {
  if (!/[\n,"]/.test(s)) return s;
  return '"' + s.replace(/"/g, '""') + '"';
}

function exportCSV() {
  const s = currentState.value;
  if (!s || !s.queryColumns.length) return;
  const cols = s.queryColumns;
  const rows = s.queryRows;
  const lines: string[] = [];
  lines.push(cols.map(escapeCsvCell).join(","));
  for (const row of rows) {
    lines.push(
      cols.map((_, j) => escapeCsvCell(cellToRaw(row[j]))).join(",")
    );
  }
  const bom = "\uFEFF";
  const csv = bom + lines.join("\n");
  const name = s.lastQueriedTableName || "export";
  downloadFile(`${name}.csv`, "text/csv;charset=utf-8", csv);
}

function exportSQL() {
  const s = currentState.value;
  if (!s || !s.queryColumns.length) return;
  const cols = s.queryColumns;
  const rows = s.queryRows;
  const conn = connections.value.find((c) => c.id === s.selectedConnId);
  const driver = conn?.driver ?? "mysql";
  const tableName = s.lastQueriedTableName || "exported_table";
  const quoteId = (id: string) =>
    driver === "mysql"
      ? "`" + id.replace(/`/g, "``") + "`"
      : '"' + id.replace(/"/g, '""') + '"';
  const quotedCols = cols.map(quoteId).join(", ");
  const lines: string[] = [];
  for (const row of rows) {
    const vals = cols.map((_, j) => {
      const v = row[j];
      if (v == null || v === undefined) return "NULL";
      if (typeof v === "object" && v !== null) {
        const o = v as Record<string, unknown>;
        if ("Valid" in o && typeof o.Valid === "boolean" && !o.Valid)
          return "NULL";
        if ("String" in o) return "'" + String(o.String).replace(/'/g, "''") + "'";
        if ("Int64" in o) return String(o.Int64);
        if ("Float64" in o) return String(o.Float64);
        if ("Bool" in o) return o.Bool ? "1" : "0";
        return "'" + JSON.stringify(v).replace(/'/g, "''") + "'";
      }
      if (typeof v === "number") return Number.isNaN(v) ? "NULL" : String(v);
      if (typeof v === "boolean") return v ? "1" : "0";
      return "'" + String(v).replace(/'/g, "''") + "'";
    });
    lines.push(`INSERT INTO ${quoteId(tableName)} (${quotedCols}) VALUES (${vals.join(", ")});`);
  }
  const sqlContent = lines.join("\n");
  downloadFile(`${tableName}.sql`, "application/sql;charset=utf-8", sqlContent);
}

watch(
  () => [currentState.value?.pageSize, currentState.value?.queryRows.length],
  () => {
    const s = currentState.value;
    if (s) s.currentPage = 1;
  }
);

function onScrollOrResize() {
  if (showHintDropdown.value) updateHintPosition();
}

onMounted(() => {
  loadConnections();
  if (tabs.value.length === 0) addTab();
  window.addEventListener("scroll", onScrollOrResize, true);
  window.addEventListener("resize", onScrollOrResize);
});
onUnmounted(() => {
  window.removeEventListener("scroll", onScrollOrResize, true);
  window.removeEventListener("resize", onScrollOrResize);
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
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}
.db-current-db {
  font-size: 12px;
  color: #94a3b8;
}
.db-current-db + .btn-run {
  margin-left: auto;
}
.btn-run {
  margin-left: auto;
}
.db-sql-input-wrap {
  position: relative;
  flex-shrink: 0;
}
.db-sql-input {
  display: block;
  width: 100%;
  height: 120px;
  max-height: 200px;
  min-height: 80px;
  padding: 12px;
  background: transparent;
  border: none;
  color: #e2e8f0;
  font-family: ui-monospace, monospace;
  font-size: 14px;
  resize: vertical;
  outline: none;
}
.db-sql-hint {
  position: fixed;
  max-height: 200px;
  overflow-y: auto;
  background: #1e293b;
  border: 1px solid rgba(148, 163, 184, 0.3);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
  z-index: 1000;
}
.db-sql-hint-item {
  padding: 6px 10px;
  font-family: ui-monospace, monospace;
  font-size: 13px;
  color: #e2e8f0;
  cursor: pointer;
}
.db-sql-hint-item:hover,
.db-sql-hint-item-active {
  background: rgba(59, 130, 246, 0.35);
  color: #fff;
}
.db-query-error {
  flex-shrink: 0;
  padding: 8px 12px;
  color: #f87171;
  font-size: 13px;
}
.db-result-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 12px;
  border-top: 1px solid rgba(148, 163, 184, 0.2);
  overflow: hidden;
}
.db-result-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.db-result-table {
  width: 100%;
  min-width: max-content;
  border-collapse: collapse;
  font-size: 14px;
  table-layout: auto;
}
.db-result-table thead th {
  position: sticky;
  top: 0;
  z-index: 1;
  background: #1e293b;
  color: #94a3b8;
  font-weight: 500;
  padding: 2px 6px;
  text-align: left;
  border-bottom: 1px solid rgba(148, 163, 184, 0.3);
  white-space: nowrap;
  min-width: 3em;
  box-shadow: 0 1px 0 rgba(148, 163, 184, 0.2);
}
.db-result-table td {
  padding: 2px 6px;
  text-align: left;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  white-space: nowrap;
  min-width: 3em;
}
.db-result-footer {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  padding: 8px 0 0;
  border-top: 1px solid rgba(148, 163, 184, 0.2);
  margin-top: 8px;
}
.db-result-meta {
  font-size: 12px;
  color: #64748b;
}
.db-pagination {
  display: flex;
  align-items: center;
  gap: 12px;
}
.db-page-size {
  font-size: 12px;
  color: #94a3b8;
}
.db-page-size select {
  margin: 0 4px;
  padding: 2px 6px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 4px;
  color: #e2e8f0;
  font-size: 12px;
}
.db-page-info {
  font-size: 12px;
  color: #94a3b8;
}
.db-export-sep {
  color: rgba(148, 163, 184, 0.5);
  margin: 0 4px;
}
.btn-export {
  background: rgba(100, 116, 139, 0.5);
  border-color: rgba(148, 163, 184, 0.4);
}
.btn-export:hover:not(:disabled) {
  background: rgba(100, 116, 139, 0.7);
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
  max-width: 400px;
  border: 1px solid rgba(148, 163, 184, 0.3);
}
.modal-box h3 {
  margin: 0 0 16px;
  font-size: 18px;
  color: #e2e8f0;
}
.modal-conn-manage {
  max-width: 520px;
  max-height: 90vh;
  overflow-y: auto;
}
.modal-conn-manage .conn-form h4 {
  margin: 16px 0 8px;
  font-size: 14px;
  color: #94a3b8;
}
.conn-list {
  margin-bottom: 16px;
  max-height: 200px;
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
  min-width: 90px;
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
.conn-form input,
.conn-form select {
  width: 100%;
  padding: 8px 10px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 6px;
  color: #e2e8f0;
  font-size: 14px;
}
.modal-actions {
  margin-top: 20px;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
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
.btn:hover:not(:disabled) {
  background: rgba(59, 130, 246, 0.8);
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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
.btn-danger:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.8);
}
</style>
