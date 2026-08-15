import { request, ensureSession } from "../../utils/api";
import type { CartItem } from "../../utils/cart";
import { fenToYuan } from "../../utils/cart";
import { payAction, type PayParams, type PrepayResp } from "../../utils/pay";

type AppG = { globalData: { scene: string; tableToken: string } };

Page({
  data: {
    pickupType: "IMMEDIATE",
    slots: [] as { starts_at: string; label: string; remaining: number; available: boolean }[],
    quote: null as null | { quote_token: string; payable_cents: number; packing_cents: number; discount_cents: number; goods_cents: number; lines?: { product_name: string; qty: number }[] },
    remark: "",
    scheduled: "",
    coupons: [] as { id: string; name: string; status: string }[],
    couponId: "",
    scene: "PICKUP",
    paying: false,
  },
  onShow() {
    const app = getApp<AppG>();
    this.setData({ scene: app.globalData.scene });
    this.loadCoupons();
    this.preview();
  },
  async loadCoupons() {
    try {
      await ensureSession();
      const mine = await request<{ items: { id: string; name: string; status: string }[] }>("/api/v1/coupons?status=AVAILABLE", "GET", undefined, true);
      this.setData({ coupons: mine.items });
    } catch {
      this.setData({ coupons: [] });
    }
  },
  async preview() {
    const app = getApp<AppG>();
    const items = (wx.getStorageSync("cart") || []) as CartItem[];
    const body: Record<string, unknown> = { scene: app.globalData.scene, items };
    if (app.globalData.scene === "DINE_IN") body.table_token = app.globalData.tableToken;
    if (this.data.pickupType === "SCHEDULED" && this.data.scheduled) body.scheduled_for = this.data.scheduled;
    if (this.data.couponId) body.customer_coupon_id = this.data.couponId;
    try {
      const quote = await request<{ quote_token: string; payable_cents: number; packing_cents: number; discount_cents: number; goods_cents: number }>("/api/v1/pricing/preview", "POST", body);
      this.setData({ quote });
    } catch {
      wx.showToast({ title: "请重新确认金额", icon: "none" });
    }
  },
  async loadSlots() {
    const slots = await request<{ slots: { starts_at: string; label: string; remaining: number; available: boolean }[] }>("/api/v1/pickup-slots", "GET");
    this.setData({ slots: slots.slots, pickupType: "SCHEDULED" });
  },
  immediate() {
    this.setData({ pickupType: "IMMEDIATE", scheduled: "", slots: [] });
    this.preview();
  },
  chooseSlot(e: WechatMiniprogram.TouchEvent) {
    this.setData({ scheduled: e.currentTarget.dataset.at as string, pickupType: "SCHEDULED" });
    this.preview();
  },
  chooseCoupon(e: WechatMiniprogram.TouchEvent) {
    this.setData({ couponId: e.currentTarget.dataset.id as string });
    this.preview();
  },
  onRemark(e: WechatMiniprogram.Input) {
    this.setData({ remark: e.detail.value.slice(0, 100) });
  },
  async pay() {
    if (this.data.paying) return;
    this.setData({ paying: true });
    try {
      await ensureSession();
      const items = (wx.getStorageSync("cart") || []) as CartItem[];
      const order = await request<{ id: string }>("/api/v1/orders", "POST", { quote_token: this.data.quote?.quote_token, items, remark: this.data.remark }, true);
      const prepay = await request<PrepayResp>("/api/v1/orders/" + order.id + "/prepay", "POST", {}, true);
      const action = payAction(prepay);
      if (action === "zero") {
        wx.redirectTo({ url: "/pages/pay-result/index?id=" + order.id });
        return;
      }
      if (action === "mock") {
        await request("/api/v1/orders/" + order.id + "/mock-payment/confirm", "POST", {}, true);
        wx.redirectTo({ url: "/pages/pay-result/index?id=" + order.id });
        return;
      }
      if (action !== "jsapi" || !prepay.pay_params) {
        wx.showToast({ title: "支付参数无效", icon: "none" });
        wx.redirectTo({ url: "/pages/order-detail/index?id=" + order.id });
        return;
      }
      await this.requestPay(prepay.pay_params);
      wx.redirectTo({ url: "/pages/pay-result/index?id=" + order.id });
    } catch {
      wx.showToast({ title: "支付未完成，可在订单中继续", icon: "none" });
    } finally {
      this.setData({ paying: false });
    }
  },
  requestPay(params: PayParams) {
    return new Promise<void>((resolve, reject) => {
      wx.requestPayment({
        timeStamp: params.timeStamp,
        nonceStr: params.nonceStr,
        package: params.package,
        signType: params.signType as "RSA",
        paySign: params.paySign,
        success() { resolve(); },
        fail() { reject(new Error("cancelled")); },
      });
    });
  },
  fenToYuan,
});
