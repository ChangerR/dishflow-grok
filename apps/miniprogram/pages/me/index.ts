import { request, ensureSession } from "../../utils/api";

Page({
  data: { me: {} as Record<string, unknown> },
  async onShow() {
    try {
      await ensureSession();
      const me = await request<Record<string, unknown>>("/api/v1/me", "GET", undefined, true);
      this.setData({ me });
    } catch {
      this.setData({ me: { is_member: false } });
    }
  },
  async join() {
    await ensureSession();
    await request("/api/v1/me/membership", "POST", { phone_code: "13800138000", agreed: true }, true);
    this.onShow();
  },
  goCoupons() { wx.navigateTo({ url: "/pages/coupons/index" }); },
  goStore() { wx.navigateTo({ url: "/pages/store/index" }); },
});
