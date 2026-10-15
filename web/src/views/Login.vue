<template>
  <main class="login-main">
    <form class="login-card" @submit.prevent="submit">
      <label>
        用户名
        <input v-model="username" name="username" autocomplete="username" />
      </label>
      <label>
        密码
        <input v-model="password" name="password" type="password" autocomplete="current-password" />
      </label>
      <p v-if="error" class="login-error">{{ error }}</p>
      <button type="submit" :disabled="submitting">{{ submitting ? "登录中…" : "登录" }}</button>
    </form>
  </main>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { refreshSession } from "../session";

const router = useRouter();
const username = ref("");
const password = ref("");
const error = ref("");
const submitting = ref(false);

async function submit() {
  error.value = "";
  submitting.value = true;
  try {
    const res = await fetch("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: username.value, password: password.value }),
    });
    if (!res.ok) {
      const data = (await res.json().catch(() => null)) as { error?: string } | null;
      error.value = data?.error || "用户名或密码错误";
      return;
    }
    await refreshSession();
    await router.push("/");
  } finally {
    submitting.value = false;
  }
}
</script>

<style scoped>
.login-main {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
.login-card {
  width: min(360px, 100%);
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 22px 22px 18px;
  border-radius: 12px;
  border: 1px solid rgba(129, 140, 248, 0.45);
  background: radial-gradient(circle at top, rgba(148, 163, 184, 0.28), rgba(30, 64, 175, 0.28));
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.55);
}
.login-card label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
  color: #cbd5e1;
}
.login-card input {
  height: 36px;
  padding: 0 10px;
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.45);
  background: rgba(15, 23, 42, 0.72);
  color: #e5e7eb;
  font: inherit;
}
.login-card button {
  height: 38px;
  border: 0;
  border-radius: 8px;
  background: #4f46e5;
  color: white;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
.login-card button:disabled {
  opacity: 0.7;
  cursor: default;
}
.login-error {
  margin: 0;
  color: #fca5a5;
  font-size: 13px;
}
</style>
