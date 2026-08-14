import { request } from "../../utils/api";

Page({
  data: { store: {} as Record<string, unknown>, policies: {} as Record<string, unknown> },
  async onShow() {
    const store = await request<Record<string, unknown>>("/api/v1/store", "GET");
    const policies = await request<Record<string, unknown>>("/api/v1/store/policies", "GET");
    this.setData({ store, policies });
  },
  call() {
    const phone = this.data.store.phone as string;
    if (!phone) { wx.showToast({ title: "门店未设置电话", icon: "none" }); return; }
    wx.makePhoneCall({ phoneNumber: phone });
  },
  map() {
    const addr = this.data.store.address as string;
    if (!addr) { wx.showToast({ title: "门店未设置地址", icon: "none" }); return; }
    wx.openLocation({ latitude: 0, longitude: 0, name: String(this.data.store.name || ""), address: addr });
  },
});
