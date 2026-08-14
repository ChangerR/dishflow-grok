import { request, ensureSession } from "../../utils/api";
import { fenToYuan } from "../../utils/cart";

Page({
  data: { items: [] as unknown[], tab: "ALL" },
  async onShow() {
    await ensureSession();
    this.load();
  },
  async load() {
    const r = await request<{ items: { id: string; status: string; payable_cents: number; pickup_type: string; scheduled_for?: string }[] }>("/api/v1/orders?status=" + this.data.tab, "GET", undefined, true);
    this.setData({ items: r.items });
  },
  switchTab(e: WechatMiniprogram.TouchEvent) {
    this.setData({ tab: e.currentTarget.dataset.tab as string });
    this.load();
  },
  open(e: WechatMiniprogram.TouchEvent) {
    wx.navigateTo({ url: "/pages/order-detail/index?id=" + e.currentTarget.dataset.id });
  },
  fenToYuan,
});
