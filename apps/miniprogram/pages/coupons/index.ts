import { request, ensureSession } from "../../utils/api";

Page({
  data: { offers: [] as unknown[], mine: [] as unknown[], tab: "AVAILABLE" },
  async onShow() {
    const offers = await request<{ items: unknown[] }>("/api/v1/coupon-offers", "GET");
    this.setData({ offers: offers.items });
    this.loadMine();
  },
  async loadMine() {
    try {
      await ensureSession();
      const mine = await request<{ items: unknown[] }>(`/api/v1/coupons?status=${this.data.tab}`, "GET", undefined, true);
      this.setData({ mine: mine.items });
    } catch {
      this.setData({ mine: [] });
    }
  },
  switchTab(e: WechatMiniprogram.TouchEvent) {
    this.setData({ tab: e.currentTarget.dataset.tab as string });
    this.loadMine();
  },
  async claim(e: WechatMiniprogram.TouchEvent) {
    await ensureSession();
    await request(`/api/v1/coupon-offers/${e.currentTarget.dataset.id}/claim`, "POST", {}, true);
    this.onShow();
  },
  async redeem(e: WechatMiniprogram.TouchEvent) {
    await ensureSession();
    await request(`/api/v1/me/rewards/${e.currentTarget.dataset.id}/redeem`, "POST", {}, true);
    this.onShow();
  },
});
