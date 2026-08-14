import { request, ensureSession } from "../../utils/api";

Page({
  data: { offers: [] as unknown[], mine: [] as unknown[] },
  async onShow() {
    const offers = await request<{ items: unknown[] }>("/api/v1/coupon-offers", "GET");
    this.setData({ offers: offers.items });
    try {
      await ensureSession();
      const mine = await request<{ items: unknown[] }>("/api/v1/coupons", "GET", undefined, true);
      this.setData({ mine: mine.items });
    } catch {
      /* guest */
    }
  },
  async claim(e: WechatMiniprogram.TouchEvent) {
    await ensureSession();
    await request(`/api/v1/coupon-offers/${e.currentTarget.dataset.id}/claim`, "POST", {}, true);
    this.onShow();
  },
});
