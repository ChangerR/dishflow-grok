import { request, ensureSession } from "../../utils/api";
import { canCancel } from "../../utils/cart";
import type { CartItem } from "../../utils/cart";
import { payAction, type PrepayResp } from "../../utils/pay";

Page({
  data: { order: null as null | Record<string, unknown>, canPay: false, canDoCancel: false },
  async onLoad(q: Record<string, string>) {
    await ensureSession();
    this.refresh(q.id);
  },
  async refresh(id: string) {
    const order = await request<Record<string, unknown>>(`/api/v1/orders/${id}`, "GET", undefined, true);
    this.setData({
      order,
      canPay: order.status === "PENDING_PAYMENT",
      canDoCancel: canCancel(String(order.status || "")),
    });
  },
  async cancel() {
    const id = (this.data.order as { id: string }).id;
    await request(`/api/v1/orders/${id}/cancel`, "POST", {}, true);
    this.refresh(id);
  },
  async continuePay() {
    const id = (this.data.order as { id: string }).id;
    const prepay = await request<PrepayResp>(`/api/v1/orders/${id}/prepay`, "POST", {}, true);
    const action = payAction(prepay);
    if (action === "mock") {
      await request(`/api/v1/orders/${id}/mock-payment/confirm`, "POST", {}, true);
    } else if (action === "jsapi" && prepay.pay_params) {
      await new Promise<void>((resolve, reject) => {
        wx.requestPayment({
          timeStamp: prepay.pay_params!.timeStamp,
          nonceStr: prepay.pay_params!.nonceStr,
          package: prepay.pay_params!.package,
          signType: prepay.pay_params!.signType as "RSA",
          paySign: prepay.pay_params!.paySign,
          success() { resolve(); },
          fail() { reject(new Error("cancelled")); },
        });
      });
    }
    wx.redirectTo({ url: "/pages/pay-result/index?id=" + id });
  },
  reorder() {
    const items = ((this.data.order as { items: { sku_id: string; option_ids?: string[]; qty: number }[] }).items || []).map((it) => ({
      sku_id: it.sku_id,
      option_ids: it.option_ids || [],
      qty: it.qty || 1,
    })) as CartItem[];
    wx.setStorageSync("cart", items);
    wx.showToast({ title: "已加入购物车，失效项请重新选择", icon: "none" });
    wx.switchTab({ url: "/pages/menu/index" });
  },
});
