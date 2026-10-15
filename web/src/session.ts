import { reactive } from "vue";
import type { Router } from "vue-router";

const originalFetch = window.fetch.bind(window);

export const authState = reactive({
  username: "",
});

export async function refreshSession(): Promise<boolean> {
  const res = await originalFetch("/api/auth/me");
  if (!res.ok) {
    authState.username = "";
    return false;
  }
  const data = (await res.json()) as { username?: string };
  authState.username = typeof data.username === "string" ? data.username : "";
  return authState.username !== "";
}

export async function logout(): Promise<void> {
  await originalFetch("/api/logout", { method: "POST" });
  authState.username = "";
}

export async function redirectIfLoggedOut(router: Router): Promise<void> {
  const ok = await refreshSession();
  if (!ok) {
    await router.push("/login");
  }
}

export function installFetchGuard(router: Router): void {
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const res = await originalFetch(input, init);
    const url = requestURL(input);
    const method = (init?.method || (input instanceof Request ? input.method : "GET")).toUpperCase();
    const isLogin = method === "POST" && url.includes("/api/login");
    if (res.status === 401 && url.includes("/api/") && !isLogin && window.location.pathname !== "/login") {
      authState.username = "";
      await router.push("/login");
    }
    return res;
  };
}

function requestURL(input: RequestInfo | URL): string {
  if (typeof input === "string") return input;
  if (input instanceof URL) return input.href;
  return input.url;
}
