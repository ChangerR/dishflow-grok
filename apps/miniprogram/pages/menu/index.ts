import { mergeCart, pruneCart, type CartItem } from "../../utils/cart";
import { request } from "../../utils/api";

Page({
  data: { storeName: "", isOpen: false, announcement: "", scene: "PICKUP", tableNo: "", categories: [] as unknown[], keyword: "", cart: [] as CartItem[] },
  onShow() {
    const app = getApp<{ globalData: { scene: string; tableToken: string; tableNo: string } }>();
    this.setData({ scene: app.globalData.scene, tableNo: app.globalData.tableNo });
    this.load();
  },
  async load() {
    const boot = await request<{ store_name: string; is_open: boolean; announcement?: string }>("/api/v1/storefront/bootstrap", "GET");
    const menu = await request<{ categories: { id: string; dishes: { id: string; name: string; skus: { id: string }[] }[] }[] }>("/api/v1/menu", "GET");
    const valid = new Set<string>();
    for (const c of menu.categories) for (const d of c.dishes) for (const s of d.skus) valid.add(s.id);
    const cart = wx.getStorageSync("cart") as CartItem[] | "";
    const { kept, removed } = pruneCart(Array.isArray(cart) ? cart : [], valid);
    if (removed.length) wx.showToast({ title: `${removed.length} 件已失效`, icon: "none" });
    wx.setStorageSync("cart", kept);
    this.setData({ storeName: boot.store_name, isOpen: boot.is_open, announcement: boot.announcement || "", categories: menu.categories, cart: kept });
  },
  onKeyword(e: WechatMiniprogram.Input) {
    this.setData({ keyword: e.detail.value });
  },
  add(e: WechatMiniprogram.TouchEvent) {
    const sku = e.currentTarget.dataset.sku as string;
    const cart = mergeCart([...(this.data.cart as CartItem[]), { sku_id: sku, option_ids: [], qty: 1 }]);
    wx.setStorageSync("cart", cart);
    this.setData({ cart });
  },
  checkout() {
    wx.navigateTo({ url: "/pages/checkout/index" });
  },
});
