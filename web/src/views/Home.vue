<template>
  <main class="app-main home-main">
    <div class="home-grid">
      <button type="button" class="home-tile" @click="$router.push('/terminal')">
        <span class="home-tile-title">终端</span>
        <span class="home-tile-desc">Web 终端</span>
      </button>
      <button type="button" class="home-tile" @click="$router.push('/files')">
        <span class="home-tile-title">文件</span>
        <span class="home-tile-desc">上传下载</span>
      </button>
      <button type="button" class="home-tile" @click="$router.push('/database')">
        <span class="home-tile-title">SQL</span>
        <span class="home-tile-desc">MySQL / PG</span>
      </button>
      <button type="button" class="home-tile" @click="$router.push('/database-mongo')">
        <span class="home-tile-title">MongoDB</span>
        <span class="home-tile-desc">文档库</span>
      </button>
      <button type="button" class="home-tile" @click="$router.push('/redis')">
        <span class="home-tile-title">Redis</span>
        <span class="home-tile-desc">缓存 / Key</span>
      </button>
      <button type="button" class="home-tile" @click="$router.push('/vnc')">
        <span class="home-tile-title">VNC</span>
        <span class="home-tile-desc">远程桌面</span>
      </button>
    </div>
  </main>
</template>

<script setup lang="ts"></script>

<style scoped>
.home-main {
  padding: 20px 24px 24px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.home-grid {
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  gap: 14px;
  justify-content: center;
  align-items: stretch;
}
.home-tile {
  flex: 1;
  min-width: 160px;
  max-width: 220px;
  height: 96px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 14px 18px;
  background: radial-gradient(circle at top, rgba(148, 163, 184, 0.35), rgba(30, 64, 175, 0.35));
  border-radius: 12px;
  border: 1px solid rgba(129, 140, 248, 0.45);
  box-shadow: 0 6px 16px rgba(15, 23, 42, 0.5), inset 0 0 0 1px rgba(148, 163, 184, 0.15);
  color: #e5e7eb;
  cursor: pointer;
  font: inherit;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.home-tile:hover {
  border-color: rgba(129, 140, 248, 0.9);
  box-shadow: 0 0 20px rgba(99, 102, 241, 0.35);
}
.home-tile:active {
  transform: scale(0.98);
}
.home-tile-title {
  font-size: 17px;
  font-weight: 600;
  line-height: 1.2;
  text-align: center;
}
.home-tile-desc {
  font-size: 13px;
  color: #94a3b8;
  line-height: 1.2;
  text-align: center;
}
@media (max-width: 720px) {
  .home-grid {
    width: 100%;
  }
  .home-tile {
    min-width: calc(50% - 7px);
    max-width: none;
    height: 88px;
  }
}
</style>
