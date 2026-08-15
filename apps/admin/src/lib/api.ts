export type Principal = {
  id: string;
  login_name: string;
  display_name: string;
  is_platform_admin: boolean;
  store_id?: string;
  role?: string;
  store_name?: string;
  store_open?: boolean;
  mock_payment?: boolean;
  mock_print?: boolean;
};

async function request<T>(path: string, init: RequestInit = {}, storeId?: string): Promise<T> {
  const headers = new Headers(init.headers);
  if (!(init.body instanceof FormData) && init.body !== undefined) headers.set("Content-Type", "application/json");
  if (storeId) headers.set("X-Store-Id", storeId);
  let res: Response;
  try {
    res = await fetch(path, { ...init, headers, credentials: "include" });
  } catch {
    throw new Error("无法连接 API，请确认后台已启动且 Vite 代理指向正确端口");
  }
  if (res.status === 401) {
    if (!path.includes("/admin/session") || init.method !== "POST") {
      if (window.location.pathname !== "/login") window.location.assign("/login");
    }
    throw new Error("UNAUTHORIZED");
  }
  const text = await res.text();
  let data: { message?: string; code?: string } = {};
  if (text) {
    try {
      data = JSON.parse(text) as { message?: string; code?: string };
    } catch {
      throw new Error(res.ok ? "响应不是 JSON" : `HTTP ${res.status}`);
    }
  }
  if (!res.ok) throw new Error(data.message || data.code || res.statusText);
  return data as T;
}

export const api = {
  login: (login_name: string, password: string) =>
    request<Principal>("/api/v1/admin/session", { method: "POST", body: JSON.stringify({ login_name, password }) }),
  logout: () => request("/api/v1/admin/session", { method: "DELETE" }),
  me: () => request<Principal>("/api/v1/admin/session"),
  get: <T>(path: string, storeId?: string) => request<T>(path, {}, storeId),
  send: <T>(path: string, method: string, body?: unknown, storeId?: string) =>
    request<T>(path, { method, body: body === undefined ? undefined : JSON.stringify(body) }, storeId),
};
