<template>
  <div class="editor-page">
    <div class="editor-toolbar">
      <router-link to="/files" class="back-link">← 返回文件管理</router-link>
      <span class="editor-path">{{ filePath || "/" }}</span>
      <button
        type="button"
        class="btn btn-save"
        :disabled="saving || !dirty"
        @click="save"
      >
        {{ saving ? "保存中…" : "保存" }}
      </button>
    </div>

    <div v-if="loadError" class="editor-error">{{ loadError }}</div>
    <div v-else class="editor-wrap">
      <textarea
        v-model="content"
        class="editor-textarea"
        spellcheck="false"
        @input="dirty = true"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

const route = useRoute();
const router = useRouter();
const filePath = ref("");
const content = ref("");
const dirty = ref(false);
const saving = ref(false);
const loadError = ref("");

function load() {
  const path = route.query.path as string;
  const host = route.query.host as string | undefined;
  if (!path) {
    loadError.value = "缺少 path 参数";
    return;
  }
  filePath.value = path;
  loadError.value = "";
  const url = host
    ? `/api/ssh/fs/read?host=${encodeURIComponent(host)}&path=${encodeURIComponent(path)}`
    : `/api/fs/read?path=${encodeURIComponent(path)}`;
  fetch(url)
    .then((res) => res.json())
    .then((data) => {
      if (data.error) {
        loadError.value = data.error;
        return;
      }
      content.value = data.content ?? "";
      dirty.value = false;
    })
    .catch((e) => {
      loadError.value = (e as Error).message || "加载失败";
    });
}

async function save() {
  if (!filePath.value || saving.value) return;
  const host = route.query.host as string | undefined;
  saving.value = true;
  try {
    const body: Record<string, string> = { path: filePath.value, content: content.value };
    if (host) body.host = host;
    const url = host ? "/api/ssh/fs/write" : "/api/fs/write";
    const res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) {
      loadError.value = data.error || "保存失败";
      return;
    }
    dirty.value = false;
  } catch (e) {
    loadError.value = (e as Error).message || "保存失败";
  } finally {
    saving.value = false;
  }
}

onMounted(load);
watch([() => route.query.path, () => route.query.host], load);
</script>

<style scoped>
.editor-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 16px;
}
.editor-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}
.back-link {
  color: #94a3b8;
  text-decoration: none;
  font-size: 14px;
}
.back-link:hover {
  color: #e2e8f0;
}
.editor-path {
  flex: 1;
  font-size: 13px;
  color: #94a3b8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
  opacity: 0.6;
  cursor: not-allowed;
}
.btn-save {
  min-width: 72px;
}
.editor-error {
  padding: 16px;
  color: #f87171;
}
.editor-wrap {
  flex: 1;
  min-height: 300px;
  display: flex;
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.3);
  overflow: hidden;
  background: rgba(15, 23, 42, 0.6);
}
.editor-textarea {
  flex: 1;
  width: 100%;
  min-height: 300px;
  padding: 12px;
  background: transparent;
  border: none;
  color: #e2e8f0;
  font-family: ui-monospace, "Cascadia Code", "Source Code Pro", Menlo, monospace;
  font-size: 14px;
  line-height: 1.5;
  resize: none;
  outline: none;
}
.editor-textarea::placeholder {
  color: #64748b;
}
</style>
