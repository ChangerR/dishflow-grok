import { request, ensureSession } from "../../utils/api";
import { canCancel } from "../../utils/cart";
import type { CartItem } from "../../utils/cart";

Page({
  data: { order: null as null | Record<string, unknown> },
  async onLoad(q: Record<string, string>) {
    await ensureSession();
    const order = await request<Record<string, unknown>>(`/api/v1/orders/${q.id}`, "GET", undefined, true);
    this.setData({ order });
  },
  async cancel() {
    const id = (this.data.order as { id: string }).id;
    await request(`/api/v1/orders/${id}/cancel`, "POST", {}, true);
    this.onLoad({ id });
  },
  reorder() {
    const items = ((this.data.order as { items: { sku_id: string }[] }).items || []).map((it) => ({ sku_id: it.sku_id, option_ids: [], qty: 1 })) as CartItem[];
    wx.setStorageSync("cart", items);
    wx.showToast({ title: "已加入购物车，失效项请重新选择", icon: "none" });
    wx.switchTab({ url: "/pages/menu/index" });
  },
  canCancel,
});
