import { request, ensureSession } from "../../utils/api";

Page({
  data: { me: {} as Record<string, unknown>, agreed: false, joining: false },
  async onShow() {
    try {
      await ensureSession();
      const me = await request<Record<string, unknown>>("/api/v1/me", "GET", undefined, true);
      this.setData({ me });
    } catch {
      this.setData({ me: { is_member: false } });
    }
  },
  toggleAgree(e: WechatMiniprogram.CheckboxGroupChange) {
    this.setData({ agreed: (e.detail.value || []).includes("yes") });
  },
  onPhone(e: WechatMiniprogram.ButtonGetPhoneNumber) {
    const code = e.detail.code;
    if (!code) {
      wx.showToast({ title: "需要手机号授权才能入会", icon: "none" });
      return;
    }
    this.join(code);
  },
  async join(phoneCode: string) {
    if (!this.data.agreed || this.data.joining) {
      wx.showToast({ title: "请先勾选会员协议", icon: "none" });
      return;
    }
    this.setData({ joining: true });
    try {
      await ensureSession();
      await request("/api/v1/me/membership", "POST", { phone_code: phoneCode, agreed: true }, true);
      this.onShow();
    } catch {
      wx.showToast({ title: "入会失败", icon: "none" });
    } finally {
      this.setData({ joining: false });
    }
  },
  goCoupons() { wx.navigateTo({ url: "/pages/coupons/index" }); },
  goStore() { wx.navigateTo({ url: "/pages/store/index" }); },
  goOrders() { wx.switchTab({ url: "/pages/orders/index" }); },
});
