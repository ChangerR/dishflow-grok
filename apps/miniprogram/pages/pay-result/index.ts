import { request, ensureSession } from "../../utils/api";

Page({
  data: { status: "CONFIRMING", pickup_number: "", points_awarded: 0, table_no: "", scheduled_for: "" },
  async onLoad(q: Record<string, string>) {
    await ensureSession();
    this.poll(q.id);
  },
  async poll(id: string) {
    for (let i = 0; i < 10; i++) {
      const o = await request<{ payment_status: string; pickup_number?: string; points_awarded: number; scheduled_for?: string; table_no?: string }>(`/api/v1/orders/${id}`, "GET", undefined, true);
      if (o.payment_status === "SUCCESS") {
        this.setData({ status: "SUCCESS", pickup_number: o.pickup_number || "", points_awarded: o.points_awarded, table_no: o.table_no || "", scheduled_for: o.scheduled_for || "" });
        wx.removeStorageSync("cart");
        return;
      }
      if (o.payment_status === "CLOSED") {
        this.setData({ status: "FAILED" });
        return;
      }
      await new Promise((r) => setTimeout(r, 800));
    }
    this.setData({ status: "CONFIRMING" });
  },
});
