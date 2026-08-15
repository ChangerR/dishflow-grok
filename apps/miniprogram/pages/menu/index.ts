import { mergeCart, pruneCart, type CartItem } from "../../utils/cart";
import { request } from "../../utils/api";
import { defaultOptionIds, defaultSkuId, filterMenu, optionsValid, toggleOption, type Category, type Dish } from "../../utils/options";

type AppG = { globalData: { scene: string; tableToken: string; tableNo: string; sceneLocked: boolean } };

Page({
  data: {
    storeName: "",
    isOpen: false,
    announcement: "",
    scene: "PICKUP",
    tableNo: "",
    sceneLocked: false,
    categories: [] as Category[],
    shown: [] as Category[],
    keyword: "",
    cart: [] as CartItem[],
    picker: null as null | { dish: Dish; skuId: string; optionIds: string[]; qty: number },
  },
  onShow() {
    this.syncScene();
    this.load();
  },
  async syncScene() {
    const app = getApp<AppG>();
    if (app.globalData.tableToken && !app.globalData.tableNo) {
      try {
        const t = await request<{ table_no: string; enabled?: boolean }>(`/api/v1/tables/resolve?token=${encodeURIComponent(app.globalData.tableToken)}`, "GET");
        app.globalData.tableNo = t.table_no;
        app.globalData.scene = "DINE_IN";
        app.globalData.sceneLocked = true;
      } catch {
        wx.showToast({ title: "桌码无效，请重新扫码", icon: "none" });
        app.globalData.scene = "PICKUP";
        app.globalData.tableToken = "";
        app.globalData.tableNo = "";
        app.globalData.sceneLocked = false;
      }
    }
    this.setData({ scene: app.globalData.scene, tableNo: app.globalData.tableNo, sceneLocked: app.globalData.sceneLocked });
  },
  async load() {
    const boot = await request<{ store_name: string; is_open: boolean; announcement?: string }>("/api/v1/storefront/bootstrap", "GET");
    const menu = await request<{ categories: Category[] }>("/api/v1/menu", "GET");
    const valid = new Set<string>();
    const optionValid = new Set<string>();
    for (const c of menu.categories) {
      for (const d of c.dishes) {
        for (const s of d.skus) if (s.enabled) valid.add(s.id);
        for (const g of d.option_groups || []) for (const it of g.items || []) if (it.enabled) optionValid.add(it.id);
      }
    }
    const cart = wx.getStorageSync("cart") as CartItem[] | "";
    const raw = Array.isArray(cart) ? cart : [];
    const invalidOpts = raw.filter((it) => it.option_ids.some((id) => !optionValid.has(id))).map((it) => it.sku_id);
    const { kept, removed } = pruneCart(raw, valid, invalidOpts);
    if (removed.length) wx.showToast({ title: `${removed.length} 件已失效，请重新选择`, icon: "none" });
    wx.setStorageSync("cart", kept);
    this.setData({
      storeName: boot.store_name,
      isOpen: boot.is_open,
      announcement: boot.announcement || "",
      categories: menu.categories,
      shown: filterMenu(menu.categories, this.data.keyword),
      cart: kept,
    });
  },
  onKeyword(e: WechatMiniprogram.Input) {
    const keyword = e.detail.value;
    this.setData({ keyword, shown: filterMenu(this.data.categories as Category[], keyword) });
  },
  switchScene() {
    const app = getApp<AppG>();
    if (app.globalData.sceneLocked) {
      wx.showToast({ title: "堂食已锁定，不能换桌", icon: "none" });
      return;
    }
    app.globalData.scene = app.globalData.scene === "PICKUP" ? "PICKUP" : "PICKUP";
    this.setData({ scene: app.globalData.scene });
  },
  openPicker(e: WechatMiniprogram.TouchEvent) {
    const dish = e.currentTarget.dataset.dish as Dish;
    if (dish.sold_out) {
      wx.showToast({ title: "已售罄", icon: "none" });
      return;
    }
    this.setData({ picker: { dish, skuId: defaultSkuId(dish), optionIds: defaultOptionIds(dish.option_groups || []), qty: 1 } });
  },
  pickSku(e: WechatMiniprogram.TouchEvent) {
    const skuId = e.currentTarget.dataset.id as string;
    const picker = this.data.picker;
    if (!picker) return;
    this.setData({ picker: { ...picker, skuId } });
  },
  pickOption(e: WechatMiniprogram.TouchEvent) {
    const picker = this.data.picker;
    if (!picker) return;
    const gid = e.currentTarget.dataset.gid as string;
    const oid = e.currentTarget.dataset.oid as string;
    this.setData({ picker: { ...picker, optionIds: toggleOption(picker.dish.option_groups || [], picker.optionIds, gid, oid) } });
  },
  confirmAdd() {
    const picker = this.data.picker;
    if (!picker) return;
    if (!optionsValid(picker.dish.option_groups || [], picker.optionIds)) {
      wx.showToast({ title: "请完成必选加料", icon: "none" });
      return;
    }
    const cart = mergeCart([...(this.data.cart as CartItem[]), { sku_id: picker.skuId, option_ids: picker.optionIds, qty: picker.qty }]);
    wx.setStorageSync("cart", cart);
    this.setData({ cart, picker: null });
  },
  closePicker() {
    this.setData({ picker: null });
  },
  checkout() {
    wx.navigateTo({ url: "/pages/checkout/index" });
  },
});
