import { request, ensureSession } from "../../utils/api";
import type { CartItem } from "../../utils/cart";
import { fenToYuan } from "../../utils/cart";

Page({
  data: {
    pickupType: "IMMEDIATE",
    slots: [] as { starts_at: string; label: string; remaining: number; available: boolean }[],
    quote: null as null | { quote_token: string; payable_cents: number; packing_cents: number; discount_cents: number },
    remark: "",
    scheduled: "",
  },
  onShow() {
    this.preview();
  },
  async preview() {
    const app = getApp<{ globalData: { scene: string; tableToken: string } }>();
    const items = (wx.getStorageSync("cart") || []) as CartItem[];
    const body: Record<string, unknown> = { scene: app.globalData.scene, items };
    if (app.globalData.scene === "DINE_IN") body.table_token = app.globalData.tableToken;
    if (this.data.pickupType === "SCHEDULED" && this.data.scheduled) body.scheduled_for = this.data.scheduled;
    try {
      const quote = await request<{ quote_token: string; payable_cents: number; packing_cents: number; discount_cents: number }>("/api/v1/pricing/preview", "POST", body);
      this.setData({ quote });
    } catch (e) {
      wx.showToast({ title: "请重新确认金额", icon: "none" });
    }
  },
  async loadSlots() {
    const slots = await request<{ slots: { starts_at: string; label: string; remaining: number; available: boolean }[] }>("/api/v1/pickup-slots", "GET");
    this.setData({ slots: slots.slots, pickupType: "SCHEDULED" });
  },
  chooseSlot(e: WechatMiniprogram.TouchEvent) {
    this.setData({ scheduled: e.currentTarget.dataset.at as string });
    this.preview();
  },
  onRemark(e: WechatMiniprogram.Input) {
    this.setData({ remark: e.detail.value.slice(0, 100) });
  },
  async pay() {
    await ensureSession();
    const items = (wx.getStorageSync("cart") || []) as CartItem[];
    const order = await request<{ id: string }>("/api/v1/orders", "POST", { quote_token: this.data.quote?.quote_token, items, remark: this.data.remark }, true);
    const prepay = await request<{ mock_payment?: boolean; zero_order?: boolean }>("/api/v1/orders/" + order.id + "/prepay", "POST", {}, true);
    if (prepay.mock_payment) {
      await request("/api/v1/orders/" + order.id + "/mock-payment/confirm", "POST", {}, true);
    }
    wx.redirectTo({ url: "/pages/pay-result/index?id=" + order.id });
  },
  fenToYuan,
});
