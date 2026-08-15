import { newIdempotencyKey } from "./pay";

const APPID = ""; // 构建时注入门店 AppID

export async function request<T>(path: string, method: string, data?: unknown, auth = false): Promise<T> {
  const app = getApp<{ globalData: { token: string } }>();
  const header: Record<string, string> = { "X-Wechat-Appid": APPID || wx.getAccountInfoSync().miniProgram.appId };
  if (auth && app.globalData.token) header.Authorization = `Bearer ${app.globalData.token}`;
  if (method !== "GET" && method !== "HEAD") header["Idempotency-Key"] = newIdempotencyKey();
  return new Promise((resolve, reject) => {
    wx.request({
      url: path.startsWith("http") ? path : path,
      method: method as WechatMiniprogram.RequestOption["method"],
      data: data as string | WechatMiniprogram.IAnyObject | ArrayBuffer,
      header,
      success(res) {
        if (res.statusCode >= 400) reject(res.data);
        else resolve(res.data as T);
      },
      fail: reject,
    });
  });
}

export async function ensureSession(): Promise<void> {
  const app = getApp<{ globalData: { token: string } }>();
  if (app.globalData.token) return;
  const login = await wx.login();
  const data = await request<{ token: string }>("/api/v1/auth/wechat/session", "POST", { code: login.code });
  app.globalData.token = data.token;
}
