<template>
  <div class="app">
    <header class="app-header">
      <h1>Quay</h1>
      <p v-if="route.path !== '/login'" class="app-subtitle">终端 · 文件 · 数据库 · 远程桌面</p>
      <button
        v-if="authState.username && route.path !== '/login'"
        type="button"
        class="logout"
        @click="onLogout"
      >
        退出
      </button>
    </header>

    <router-view />
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";
import { authState, logout } from "./session";

const route = useRoute();
const router = useRouter();

async function onLogout() {
  await logout();
  await router.push("/login");
}
</script>

<style scoped>
.logout {
  margin-left: auto;
  height: 30px;
  padding: 0 12px;
  border-radius: 8px;
  border: 1px solid rgba(226, 232, 240, 0.35);
  background: transparent;
  color: #e5e7eb;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}
.logout:hover {
  border-color: rgba(226, 232, 240, 0.7);
}
</style>
