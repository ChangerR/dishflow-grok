import { useEffect, useMemo, useRef, useState } from "react";
import { Route, Routes } from "react-router-dom";
import { api, type Principal } from "../lib/api";
import { playBeep, inspectMerchantCert } from "../lib/ops";
import { fenToYuan, isOverdue, nextTransition, statusLabel, transitionLabel } from "../lib/ui";

type Props = { me: Principal };

export function Pages({ me }: Props) {
  const storeId = me.store_id;
  return (
    <Routes>
      <Route path="/board" element={<Board storeId={storeId} />} />
      <Route path="/orders" element={<Orders storeId={storeId} />} />
      <Route path="/refunds" element={<Refunds storeId={storeId} />} />
      <Route path="/analytics" element={<Analytics storeId={storeId} />} />
      <Route path="/customers" element={<Customers storeId={storeId} />} />
      <Route path="/menu" element={<Menu storeId={storeId} />} />
      <Route path="/promos" element={<Promos storeId={storeId} role={me.role} />} />
      <Route path="/tables" element={<Tables storeId={storeId} />} />
      <Route path="/store" element={<StoreSettings storeId={storeId} />} />
      <Route path="/backup" element={<Backup storeId={storeId} role={me.role} />} />
      <Route path="/print" element={<PrintPage storeId={storeId} role={me.role} />} />
      <Route path="/materials" element={<Materials storeId={storeId} />} />
      <Route path="/members" element={<Members storeId={storeId} role={me.role} />} />
      <Route path="/security" element={<Security storeId={storeId} />} />
      <Route path="/platform/stores" element={<PlatformStores />} />
      <Route path="/platform/users" element={<PlatformUsers />} />
      <Route path="/platform/apps" element={<PlatformApps />} />
    </Routes>
  );
}

function useLoad<T>(fn: () => Promise<T>, deps: unknown[]) {
  const [data, setData] = useState<T | null>(null);
  const [err, setErr] = useState("");
  const reload = () => fn().then(setData).catch((e) => setErr(e instanceof Error ? e.message : "加载失败"));
  useEffect(() => { reload(); }, deps);
  return { data, err, reload, setErr };
}

function Board({ storeId }: { storeId?: string }) {
  const [q, setQ] = useState("");
  const store = useLoad(() => api.get<{ pickup_minutes: number }>("/api/v1/admin/store", storeId), [storeId]);
  const { data, reload, err } = useLoad(() => api.get<Record<string, Order[]>>(`/api/v1/admin/orders/board?q=${encodeURIComponent(q)}`, storeId), [storeId, q]);
  const seen = useRef<Set<string>>(new Set());
  const overdueSeen = useRef<Set<string>>(new Set());
  const primed = useRef(false);
  const [vol, setVol] = useState(0.4);
  const [detail, setDetail] = useState<Order | null>(null);
  useEffect(() => {
    const t = setInterval(reload, 3000);
    return () => clearInterval(t);
  }, [storeId, q]);
  useEffect(() => {
    const ids = Object.values(data || {}).flat().map((o) => o.id);
    if (!primed.current) {
      ids.forEach((id) => seen.current.add(id));
      primed.current = true;
      return;
    }
    const fresh = ids.filter((id) => !seen.current.has(id));
    fresh.forEach((id) => seen.current.add(id));
    const minutes = store.data?.pickup_minutes || 15;
    const overdue = Object.values(data || {}).flat().filter((o) => isOverdue(o.paid_at || o.created_at, minutes) && !overdueSeen.current.has(o.id));
    overdue.forEach((o) => overdueSeen.current.add(o.id));
    if (fresh.length || overdue.length) playBeep(vol);
  }, [data, vol, store.data]);
  const cols = ["PAID", "ACCEPTED", "PREPARING", "READY"];
  const minutes = store.data?.pickup_minutes || 15;
  return (
    <div>
      <div className="mb-2 flex flex-wrap items-center gap-2 text-sm">
        <input className="rounded border px-2 py-1" placeholder="订单号/取餐号/桌号" value={q} onChange={(e) => setQ(e.target.value)} />
        <button className="rounded border px-2" onClick={reload}>刷新</button>
        音量 <input type="range" min={0} max={1} step={0.1} value={vol} onChange={(e) => setVol(Number(e.target.value))} />
        <button className="rounded border px-2" onClick={() => playBeep(vol)}>试听</button>
      </div>
      {err && <p className="mb-2 text-sm text-red-600">{err}</p>}
      <div className="grid gap-3 md:grid-cols-4">
        {cols.map((c) => (
          <div key={c} className="rounded-xl bg-white p-3 shadow-sm">
            <h3 className="mb-2 font-semibold">{statusLabel(c)}</h3>
            {(data?.[c] || []).map((o) => (
              <OrderCard key={o.id} o={o} storeId={storeId} overdue={isOverdue(o.paid_at || o.created_at, minutes)} onDone={reload} onOpen={setDetail} />
            ))}
            {(data?.[c] || []).length === 0 && <p className="text-sm text-stone-400">暂无</p>}
          </div>
        ))}
      </div>
      {detail && <OrderDetailModal o={detail} storeId={storeId} onClose={() => { setDetail(null); reload(); }} />}
    </div>
  );
}

type Order = {
  id: string; status: string; version: number; pickup_number?: string; table_no?: string;
  pickup_type: string; scheduled_for?: string; payable_cents: number; remark?: string; is_mock?: boolean;
  created_at?: string; paid_at?: string; scene?: string; goods_cents?: number; packing_cents?: number; discount_cents?: number;
  items: { product_name: string; sku_name?: string; options?: string[]; qty: number }[];
  events?: { to_status: string; created_at: string }[];
};

function OrderCard({ o, storeId, overdue, onDone, onOpen }: { o: Order; storeId?: string; overdue?: boolean; onDone: () => void; onOpen: (o: Order) => void }) {
  const next = nextTransition(o.status);
  return (
    <div className={`mb-2 rounded border p-2 text-sm ${overdue ? "border-red-500 bg-red-50" : ""}`}>
      <button className="w-full text-left" onClick={() => onOpen(o)}>
        <div className="font-medium">#{o.pickup_number || o.id.slice(-6)} {o.table_no && `桌${o.table_no}`}</div>
        {o.pickup_type === "SCHEDULED" && <div className="text-brand">预约 {o.scheduled_for}</div>}
        <div>{o.items.map((i) => `${i.product_name}×${i.qty}`).join("、")}</div>
        <div>¥{fenToYuan(o.payable_cents)} {o.is_mock && "测试"} {overdue && <span className="text-red-600">超时</span>}</div>
        {o.remark && <div className="text-stone-500">备注 {o.remark}</div>}
      </button>
      {next && (
        <button className="mt-1 rounded bg-brand px-2 py-1 text-white" onClick={async () => {
          await api.send(`/api/v1/admin/orders/${o.id}/transitions`, "POST", { to_status: next, expected_version: o.version }, storeId);
          onDone();
        }}>{transitionLabel(next)}</button>
      )}
    </div>
  );
}

function OrderDetailModal({ o, storeId, onClose }: { o: Order; storeId?: string; onClose: () => void }) {
  const { data } = useLoad(() => api.get<Order>(`/api/v1/admin/orders/${o.id}`, storeId), [o.id, storeId]);
  const d = data || o;
  return (
    <div className="fixed inset-0 z-30 flex items-start justify-center overflow-auto bg-black/40 p-4">
      <div className="w-full max-w-lg space-y-2 rounded-xl bg-white p-4 text-sm">
        <h3 className="font-semibold">订单 {d.pickup_number || d.id}</h3>
        <p>{statusLabel(d.status)} · ¥{fenToYuan(d.payable_cents)} {d.is_mock && "测试"}</p>
        {(d.items || []).map((i, idx) => <div key={idx}>{i.product_name} {i.sku_name} {(i.options || []).join("/")} ×{i.qty}</div>)}
        <p>商品 ¥{fenToYuan(d.goods_cents || 0)} 包装 ¥{fenToYuan(d.packing_cents || 0)} 优惠 ¥{fenToYuan(d.discount_cents || 0)}</p>
        {d.remark && <p>备注 {d.remark}</p>}
        <div>{(d.events || []).map((e, i) => <div key={i}>{e.to_status} {e.created_at}</div>)}</div>
        <div className="flex flex-wrap gap-2">
          <button className="rounded border px-2 py-1" onClick={() => api.send(`/api/v1/admin/orders/${d.id}/cloud-print`, "POST", {}, storeId)}>云打印</button>
          <button className="rounded border px-2 py-1" onClick={() => window.print()}>本地打印</button>
          {["ACCEPTED", "PREPARING", "READY", "COMPLETED"].includes(d.status) && (
            <button className="rounded border px-2 py-1 text-red-700" onClick={async () => {
              const reason = window.prompt("退款原因") || "门店主动退款";
              await api.send(`/api/v1/admin/orders/${d.id}/refunds`, "POST", { reason }, storeId);
              onClose();
            }}>全额退款</button>
          )}
          <button onClick={onClose}>关闭</button>
        </div>
      </div>
    </div>
  );
}

function Orders({ storeId }: { storeId?: string }) {
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [scene, setScene] = useState("");
  const [pay, setPay] = useState("");
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [cursor, setCursor] = useState("");
  const params = useMemo(() => {
    const p = new URLSearchParams();
    if (q) p.set("q", q);
    if (status) p.set("status", status);
    if (scene) p.set("scene", scene);
    if (pay) p.set("payment_status", pay);
    if (start) p.set("start", start);
    if (end) p.set("end", end);
    if (cursor) p.set("cursor", cursor);
    return p.toString();
  }, [q, status, scene, pay, start, end, cursor]);
  const { data } = useLoad(() => api.get<{ items: Order[]; next_cursor?: string }>(`/api/v1/admin/orders?${params}`, storeId), [storeId, params]);
  return (
    <div className="overflow-x-auto rounded-xl bg-white p-4">
      <div className="mb-3 flex flex-wrap gap-2 text-sm">
        <h2 className="mr-auto font-semibold">历史订单</h2>
        <input className="rounded border px-2 py-1" placeholder="单号/取餐号/桌号" value={q} onChange={(e) => { setCursor(""); setQ(e.target.value); }} />
        <select className="rounded border" value={status} onChange={(e) => { setCursor(""); setStatus(e.target.value); }}>
          <option value="">全部状态</option>
          {["PENDING_PAYMENT", "PAID", "ACCEPTED", "PREPARING", "READY", "COMPLETED", "CANCELLED", "REFUNDED"].map((s) => <option key={s} value={s}>{statusLabel(s)}</option>)}
        </select>
        <select className="rounded border" value={scene} onChange={(e) => { setCursor(""); setScene(e.target.value); }}>
          <option value="">全部场景</option><option value="DINE_IN">堂食</option><option value="PICKUP">自取</option>
        </select>
        <select className="rounded border" value={pay} onChange={(e) => { setCursor(""); setPay(e.target.value); }}>
          <option value="">支付状态</option><option value="SUCCESS">已支付</option><option value="UNPAID">未支付</option><option value="CLOSED">已关闭</option>
        </select>
        <input type="date" className="rounded border" value={start} onChange={(e) => { setCursor(""); setStart(e.target.value); }} />
        <input type="date" className="rounded border" value={end} onChange={(e) => { setCursor(""); setEnd(e.target.value); }} />
        <a className="text-brand" href={`/api/v1/admin/orders/export?${params}`}>导出 CSV</a>
      </div>
      <table className="w-full text-left text-sm">
        <thead><tr><th>单号</th><th>状态</th><th>场景</th><th>金额</th></tr></thead>
        <tbody>
          {(data?.items || []).map((o) => (
            <tr key={o.id} className="border-t"><td>{o.pickup_number || o.id}</td><td>{statusLabel(o.status)}</td><td>{o.scene}</td><td>¥{fenToYuan(o.payable_cents)}</td></tr>
          ))}
        </tbody>
      </table>
      {(data?.items || []).length === 0 && <p className="py-4 text-sm text-stone-400">暂无订单</p>}
      <div className="mt-2 flex gap-2 text-sm">
        <button disabled={!cursor} onClick={() => setCursor("")}>上一页</button>
        <button disabled={!data?.next_cursor} onClick={() => setCursor(data?.next_cursor || "")}>下一页</button>
      </div>
    </div>
  );
}

function Refunds({ storeId }: { storeId?: string }) {
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; order_id: string; amount_cents: number; status: string; reason: string; is_mock?: boolean }[] }>("/api/v1/admin/refunds", storeId), [storeId]);
  const ex = useLoad(() => api.get<{ items: { id: string; biz_no: string; error_type: string; status: string; message: string; retry_count?: number }[] }>("/api/v1/admin/exceptions", storeId), [storeId]);
  useEffect(() => {
    const t = setInterval(() => { reload(); ex.reload(); }, 5000);
    return () => clearInterval(t);
  }, [storeId]);
  return (
    <div className="space-y-4">
      <section className="rounded-xl bg-white p-4">
        <h2 className="mb-2 font-semibold">退款</h2>
        {(data?.items || []).length === 0 && <p className="text-sm text-stone-400">暂无退款</p>}
        {(data?.items || []).map((r) => (
          <div key={r.id} className="flex justify-between border-b py-2 text-sm">
            <span>{r.order_id} ¥{fenToYuan(r.amount_cents)} {r.status} {r.is_mock && "测试"} {r.reason}</span>
            <span>
              <button className="mr-2 text-brand" onClick={() => api.send(`/api/v1/admin/refunds/${r.id}/review`, "POST", { decision: "APPROVED" }, storeId).then(reload)}>通过</button>
              <button onClick={() => api.send(`/api/v1/admin/refunds/${r.id}/review`, "POST", { decision: "REJECTED" }, storeId).then(reload)}>驳回</button>
            </span>
          </div>
        ))}
      </section>
      <section className="rounded-xl bg-white p-4">
        <h2 className="mb-2 font-semibold">支付异常</h2>
        {(ex.data?.items || []).length === 0 && <p className="text-sm text-stone-400">暂无异常</p>}
        {(ex.data?.items || []).map((e) => (
          <div key={e.id} className="flex justify-between text-sm">
            <span>{e.biz_no} {e.error_type} {e.message} 重试 {e.retry_count || 0}</span>
            <button className="text-brand" onClick={() => api.send(`/api/v1/admin/exceptions/${e.id}/retry`, "POST", {}, storeId).then(ex.reload)}>重试查单</button>
          </div>
        ))}
      </section>
    </div>
  );
}

function Analytics({ storeId }: { storeId?: string }) {
  const [start, setStart] = useState(() => new Date(Date.now() - 7 * 86400000).toISOString().slice(0, 10));
  const [end, setEnd] = useState(() => new Date().toISOString().slice(0, 10));
  const qs = `start=${start}&end=${end}`;
  const { data } = useLoad(() => api.get<{ current: Record<string, number>; previous: Record<string, number> }>(`/api/v1/admin/analytics/overview?${qs}`, storeId), [storeId, qs]);
  const trends = useLoad(() => api.get<{ items: { bucket: string; paid_amount_cents: number }[] }>(`/api/v1/admin/analytics/trends?${qs}&grain=day`, storeId), [storeId, qs]);
  const br = useLoad(() => api.get<{ products?: { product_name: string; qty: number }[] }>(`/api/v1/admin/analytics/breakdown?${qs}`, storeId), [storeId, qs]);
  const cur = data?.current || {};
  return (
    <div className="space-y-4">
      <div className="flex gap-2 text-sm">
        <input type="date" className="rounded border" value={start} onChange={(e) => setStart(e.target.value)} />
        <input type="date" className="rounded border" value={end} onChange={(e) => setEnd(e.target.value)} />
      </div>
      <div className="grid gap-3 md:grid-cols-3">
        {[["净收入", cur.net_amount_cents], ["支付订单", cur.paid_orders], ["客单价", cur.aov_cents]].map(([k, v]) => (
          <div key={String(k)} className="rounded-xl bg-white p-4"><div className="text-stone-500">{k}</div><div className="text-2xl font-semibold">{typeof k === "string" && k.includes("订单") ? v : `¥${fenToYuan(Number(v || 0))}`}</div></div>
        ))}
      </div>
      <section className="rounded-xl bg-white p-4 text-sm">
        <h3 className="mb-2 font-semibold">按日趋势</h3>
        {(trends.data?.items || []).map((t) => <div key={t.bucket}>{t.bucket} ¥{fenToYuan(t.paid_amount_cents)}</div>)}
        {(br.data?.products || []).map((h) => <div key={h.product_name}>热销 {h.product_name} ×{h.qty}</div>)}
      </section>
    </div>
  );
}

function Customers({ storeId }: { storeId?: string }) {
  const [q, setQ] = useState("");
  const { data, reload } = useLoad(() => api.get<{ items: { customer_id: string; member_no: string; phone_last4?: string; phone_masked?: string; points_balance: number; status: string }[] }>(`/api/v1/admin/customer-members?q=${encodeURIComponent(q)}`, storeId), [storeId, q]);
  const settings = useLoad(() => api.get<{ points_per_yuan: number; newbie_coupon_template_id?: string }>("/api/v1/admin/member-settings", storeId), [storeId]);
  const tpls = useLoad(() => api.get<{ items: { id: string; name: string }[] }>("/api/v1/admin/coupon-templates", storeId), [storeId]);
  const [per, setPer] = useState(1);
  const [newbie, setNewbie] = useState("");
  useEffect(() => {
    if (settings.data) {
      setPer(settings.data.points_per_yuan);
      setNewbie(settings.data.newbie_coupon_template_id || "");
    }
  }, [settings.data]);
  return (
    <div className="space-y-4">
      <section className="rounded-xl bg-white p-4 space-y-2">
        <h2 className="font-semibold">会员规则</h2>
        <label className="text-sm">每元积分 <input className="rounded border px-2" type="number" value={per} onChange={(e) => setPer(Number(e.target.value))} /></label>
        <select className="w-full rounded border px-2 py-1" value={newbie} onChange={(e) => setNewbie(e.target.value)}>
          <option value="">无新人券</option>
          {(tpls.data?.items || []).map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
        </select>
        <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/member-settings", "PUT", { points_per_yuan: per, newbie_coupon_template_id: newbie }, storeId).then(settings.reload)}>保存规则</button>
      </section>
      <section className="rounded-xl bg-white p-4">
        <input className="mb-2 rounded border px-2 py-1" placeholder="会员号前缀或手机后四位" value={q} onChange={(e) => setQ(e.target.value)} />
        {(data?.items || []).map((m) => (
          <div key={m.customer_id} className="flex justify-between border-b py-2 text-sm">
            <span>{m.member_no} ****{m.phone_last4 || m.phone_masked} 积分 {m.points_balance} {m.status}</span>
            <button className="text-brand" onClick={async () => {
              const delta = Number(window.prompt("积分增减（非 0）") || "0");
              const reason = window.prompt("原因") || "";
              if (!delta || !reason) return;
              await api.send(`/api/v1/admin/customer-members/${m.customer_id}/points-adjustments`, "POST", { delta, reason }, storeId);
              reload();
            }}>调积分</button>
          </div>
        ))}
      </section>
    </div>
  );
}

function Menu({ storeId }: { storeId?: string }) {
  const cats = useLoad(() => api.get<{ items: { id: string; name: string; enabled: boolean; deleted?: boolean; sort_order?: number }[] }>("/api/v1/admin/categories?include_deleted=1", storeId), [storeId]);
  const dishes = useLoad(() => api.get<{ items: Dish[] }>("/api/v1/admin/dishes?include_deleted=1", storeId), [storeId]);
  const [name, setName] = useState("");
  const [editing, setEditing] = useState<Partial<Dish> | null>(null);
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <section className="rounded-xl bg-white p-4">
        <h2 className="font-semibold">分类</h2>
        <div className="mt-2 flex gap-2">
          <input className="flex-1 rounded border px-2 py-1" value={name} onChange={(e) => setName(e.target.value)} />
          <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/categories", "POST", { name, enabled: true, sort_order: (cats.data?.items || []).length + 1 }, storeId).then(cats.reload)}>新建</button>
        </div>
        {(cats.data?.items || []).map((c) => (
          <div key={c.id} className="flex justify-between py-1 text-sm">
            <span>{c.name} {c.enabled ? "" : "停用"} {c.deleted ? "（回收站）" : ""}</span>
            {c.deleted
              ? <button className="text-brand" onClick={() => api.send(`/api/v1/admin/categories/${c.id}/restore`, "POST", {}, storeId).then(() => { cats.reload(); dishes.reload(); })}>恢复</button>
              : <span className="space-x-2">
                <button onClick={() => api.send(`/api/v1/admin/categories/${c.id}`, "PATCH", { name: c.name, enabled: !c.enabled, sort_order: c.sort_order || 0 }, storeId).then(cats.reload)}>{c.enabled ? "停用" : "启用"}</button>
                <button className="text-red-600" onClick={() => api.send(`/api/v1/admin/categories/${c.id}`, "DELETE", undefined, storeId).then(() => { cats.reload(); dishes.reload(); })}>回收</button>
              </span>}
          </div>
        ))}
      </section>
      <section className="rounded-xl bg-white p-4">
        <div className="mb-2 flex justify-between">
          <h2 className="font-semibold">菜品</h2>
          <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => setEditing({ name: "", enabled: true, packing_fee_cents: 0, skus: [{ name: "标准", price_cents: 100, stock_mode: "UNLIMITED", daily_stock: 0, enabled: true, is_default: true, sort_order: 1 }], option_groups: [], category_id: cats.data?.items?.find((c) => !c.deleted)?.id })}>新建菜品</button>
        </div>
        {(dishes.data?.items || []).map((d) => (
          <div key={d.id} className="flex justify-between py-1 text-sm">
            <button className="text-left" onClick={() => setEditing(d)}>{d.name} {d.enabled ? "在售" : "下架"} {d.sold_out ? "售罄" : ""}</button>
            {d.deleted
              ? <button onClick={() => api.send(`/api/v1/admin/dishes/${d.id}/restore`, "POST", {}, storeId).then(dishes.reload)}>恢复</button>
              : <button className="text-red-600" onClick={() => api.send(`/api/v1/admin/dishes/${d.id}`, "DELETE", undefined, storeId).then(dishes.reload)}>回收</button>}
          </div>
        ))}
      </section>
      {editing && <DishEditor storeId={storeId} cats={(cats.data?.items || []).filter((c) => !c.deleted)} dish={editing} onClose={() => { setEditing(null); dishes.reload(); }} />}
    </div>
  );
}

type Dish = {
  id?: string; category_id: string; code?: string; name: string; description?: string; image_url?: string; enabled: boolean; sold_out?: boolean;
  packing_fee_cents: number; deleted?: boolean; sort_order?: number;
  skus: { id?: string; name: string; price_cents: number; stock_mode: string; daily_stock: number; enabled: boolean; is_default: boolean; sort_order: number }[];
  option_groups: { id?: string; name: string; selection_type: string; required: boolean; min_select: number; max_select: number; sort_order: number; items: { name: string; price_cents: number; enabled: boolean; is_default: boolean; sort_order: number }[] }[];
};

function DishEditor({ storeId, cats, dish, onClose }: { storeId?: string; cats: { id: string; name: string }[]; dish: Partial<Dish>; onClose: () => void }) {
  const [d, setD] = useState<Dish>({
    category_id: dish.category_id || cats[0]?.id || "",
    name: dish.name || "",
    enabled: dish.enabled ?? true,
    packing_fee_cents: dish.packing_fee_cents || 0,
    skus: dish.skus || [],
    option_groups: dish.option_groups || [],
    ...dish,
  } as Dish);
  const [err, setErr] = useState("");
  return (
    <div className="fixed inset-0 z-30 flex items-start justify-center overflow-auto bg-black/40 p-4">
      <form className="w-full max-w-lg space-y-2 rounded-xl bg-white p-4" onSubmit={async (e) => {
        e.preventDefault();
        setErr("");
        try {
          const path = d.id ? `/api/v1/admin/dishes/${d.id}` : "/api/v1/admin/dishes";
          await api.send(path, d.id ? "PATCH" : "POST", d, storeId);
          onClose();
        } catch (ex) {
          setErr(ex instanceof Error ? ex.message : "保存失败");
        }
      }}>
        <h3 className="font-semibold">{d.id ? "编辑菜品" : "新建菜品"}</h3>
        {err && <p className="text-sm text-red-600">{err}</p>}
        <select className="w-full rounded border px-2 py-1" value={d.category_id} onChange={(e) => setD({ ...d, category_id: e.target.value })}>
          {cats.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
        <input className="w-full rounded border px-2 py-1" placeholder="编码" value={d.code || ""} onChange={(e) => setD({ ...d, code: e.target.value })} />
        <input className="w-full rounded border px-2 py-1" placeholder="名称" value={d.name} onChange={(e) => setD({ ...d, name: e.target.value })} />
        <textarea className="w-full rounded border p-2" placeholder="描述" value={d.description || ""} onChange={(e) => setD({ ...d, description: e.target.value })} />
        <input className="w-full rounded border px-2 py-1" type="number" placeholder="包装费分" value={d.packing_fee_cents} onChange={(e) => setD({ ...d, packing_fee_cents: Number(e.target.value) })} />
        <label className="text-sm"><input type="checkbox" checked={d.enabled} onChange={(e) => setD({ ...d, enabled: e.target.checked })} /> 上架</label>
        <label className="ml-2 text-sm"><input type="checkbox" checked={!!d.sold_out} onChange={(e) => setD({ ...d, sold_out: e.target.checked })} /> 人工售罄</label>
        <h4 className="pt-2 font-medium">SKU</h4>
        {(d.skus || []).map((s, i) => (
          <div key={i} className="grid grid-cols-2 gap-1 text-sm">
            <input className="rounded border px-2 py-1" value={s.name} onChange={(e) => { const skus = [...d.skus]; skus[i] = { ...s, name: e.target.value }; setD({ ...d, skus }); }} />
            <input className="rounded border px-2 py-1" type="number" value={s.price_cents} onChange={(e) => { const skus = [...d.skus]; skus[i] = { ...s, price_cents: Number(e.target.value) }; setD({ ...d, skus }); }} />
            <select className="rounded border px-2 py-1" value={s.stock_mode} onChange={(e) => { const skus = [...d.skus]; skus[i] = { ...s, stock_mode: e.target.value }; setD({ ...d, skus }); }}>
              <option value="UNLIMITED">无限</option><option value="DAILY">每日库存</option>
            </select>
            <input className="rounded border px-2 py-1" type="number" value={s.daily_stock} onChange={(e) => { const skus = [...d.skus]; skus[i] = { ...s, daily_stock: Number(e.target.value) }; setD({ ...d, skus }); }} />
            {d.id && s.id && s.stock_mode === "DAILY" && (
              <button type="button" className="text-brand" onClick={async () => {
                const delta = Number(window.prompt("库存增减") || "0");
                const reason = window.prompt("原因") || "";
                if (!delta || !reason) return;
                await api.send(`/api/v1/admin/dishes/${d.id}/stock-adjustments`, "POST", { sku_id: s.id, delta, reason }, storeId);
              }}>调库存</button>
            )}
            <button type="button" className="text-red-600" onClick={() => setD({ ...d, skus: d.skus.filter((_, j) => j !== i) })}>删规格</button>
          </div>
        ))}
        <button type="button" className="text-sm text-brand" onClick={() => setD({ ...d, skus: [...d.skus, { name: "规格", price_cents: 0, stock_mode: "UNLIMITED", daily_stock: 0, enabled: true, is_default: d.skus.length === 0, sort_order: d.skus.length + 1 }] })}>加 SKU</button>
        <h4 className="pt-2 font-medium">选项组</h4>
        {(d.option_groups || []).map((g, gi) => (
          <div key={gi} className="space-y-1 rounded border p-2 text-sm">
            <input className="w-full rounded border px-2 py-1" value={g.name} onChange={(e) => { const option_groups = [...d.option_groups]; option_groups[gi] = { ...g, name: e.target.value }; setD({ ...d, option_groups }); }} />
            <select className="rounded border" value={g.selection_type} onChange={(e) => { const option_groups = [...d.option_groups]; option_groups[gi] = { ...g, selection_type: e.target.value }; setD({ ...d, option_groups }); }}>
              <option value="SINGLE">单选</option><option value="MULTI">多选</option>
            </select>
            <label><input type="checkbox" checked={g.required} onChange={(e) => { const option_groups = [...d.option_groups]; option_groups[gi] = { ...g, required: e.target.checked }; setD({ ...d, option_groups }); }} /> 必选</label>
            <input className="w-16 rounded border px-1" type="number" value={g.min_select} onChange={(e) => { const option_groups = [...d.option_groups]; option_groups[gi] = { ...g, min_select: Number(e.target.value) }; setD({ ...d, option_groups }); }} />
            <input className="w-16 rounded border px-1" type="number" value={g.max_select} onChange={(e) => { const option_groups = [...d.option_groups]; option_groups[gi] = { ...g, max_select: Number(e.target.value) }; setD({ ...d, option_groups }); }} />
            {(g.items || []).map((it, ii) => (
              <div key={ii} className="flex gap-1">
                <input className="flex-1 rounded border px-1" value={it.name} onChange={(e) => { const option_groups = [...d.option_groups]; const items = [...g.items]; items[ii] = { ...it, name: e.target.value }; option_groups[gi] = { ...g, items }; setD({ ...d, option_groups }); }} />
                <input className="w-20 rounded border px-1" type="number" value={it.price_cents} onChange={(e) => { const option_groups = [...d.option_groups]; const items = [...g.items]; items[ii] = { ...it, price_cents: Number(e.target.value) }; option_groups[gi] = { ...g, items }; setD({ ...d, option_groups }); }} />
              </div>
            ))}
            <button type="button" className="text-brand" onClick={() => { const option_groups = [...d.option_groups]; option_groups[gi] = { ...g, items: [...(g.items || []), { name: "选项", price_cents: 0, enabled: true, is_default: false, sort_order: (g.items || []).length + 1 }] }; setD({ ...d, option_groups }); }}>加选项</button>
          </div>
        ))}
        <button type="button" className="text-sm text-brand" onClick={() => setD({ ...d, option_groups: [...(d.option_groups || []), { name: "加料", selection_type: "MULTI", required: false, min_select: 0, max_select: 3, sort_order: (d.option_groups || []).length + 1, items: [] }] })}>加选项组</button>
        <div className="flex gap-2">
          <button className="rounded bg-brand px-3 py-1 text-white">保存</button>
          <button type="button" onClick={onClose}>取消</button>
        </div>
      </form>
    </div>
  );
}

function Promos({ storeId, role }: { storeId?: string; role?: string }) {
  const promos = useLoad(() => api.get<{ items: { id: string; name: string; threshold_cents: number; discount_cents: number; enabled: boolean }[] }>("/api/v1/admin/promotions", storeId), [storeId]);
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; name: string; discount_cents: number; min_spend_cents: number; public_claim: boolean }[] }>("/api/v1/admin/coupon-templates", storeId), [storeId]);
  const [pname, setPname] = useState("满减");
  const [th, setTh] = useState(3000);
  const [disc, setDisc] = useState(500);
  const [cname, setCname] = useState("优惠券");
  const [cmin, setCmin] = useState(0);
  const [cdisc, setCdisc] = useState(800);
  const nowISO = () => new Date().toISOString();
  const laterISO = () => new Date(Date.now() + 30 * 86400000).toISOString();
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <section className="rounded-xl bg-white p-4 space-y-2">
        <h2 className="font-semibold">满减</h2>
        <input className="w-full rounded border px-2 py-1" value={pname} onChange={(e) => setPname(e.target.value)} />
        <input className="w-full rounded border px-2 py-1" type="number" placeholder="门槛分" value={th} onChange={(e) => setTh(Number(e.target.value))} />
        <input className="w-full rounded border px-2 py-1" type="number" placeholder="减免分" value={disc} onChange={(e) => setDisc(Number(e.target.value))} />
        <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/promotions", "POST", {
          name: pname, threshold_cents: th, discount_cents: disc, scope: "ALL", stack_policy: "BEST_OF", enabled: true, starts_at: nowISO(), ends_at: laterISO(),
        }, storeId).then(promos.reload)}>创建满减</button>
        {(promos.data?.items || []).map((p) => <div key={p.id} className="text-sm">{p.name} 满{fenToYuan(p.threshold_cents)}减{fenToYuan(p.discount_cents)}</div>)}
      </section>
      <section className="rounded-xl bg-white p-4 space-y-2">
        <h2 className="font-semibold">优惠券模板</h2>
        <input className="w-full rounded border px-2 py-1" value={cname} onChange={(e) => setCname(e.target.value)} />
        <input className="w-full rounded border px-2 py-1" type="number" placeholder="最低消费分" value={cmin} onChange={(e) => setCmin(Number(e.target.value))} />
        <input className="w-full rounded border px-2 py-1" type="number" placeholder="减免分" value={cdisc} onChange={(e) => setCdisc(Number(e.target.value))} />
        <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/coupon-templates", "POST", {
          name: cname, min_spend_cents: cmin, discount_cents: cdisc, scope: "ALL", enabled: true, public_claim: true, audience: "ALL", redeemable: false, points_cost: 0, starts_at: nowISO(), ends_at: laterISO(),
        }, storeId).then(reload)}>创建券模板</button>
        {(data?.items || []).map((t) => (
          <div key={t.id} className="flex justify-between py-2 text-sm">
            <span>{t.name} -¥{fenToYuan(t.discount_cents)}</span>
            <span className="space-x-2">
              <button onClick={() => api.send(`/api/v1/admin/coupon-templates/${t.id}/issue`, "POST", { audience: "ALL" }, storeId)}>发放全部</button>
              {role === "OWNER" && <button onClick={() => api.send(`/api/v1/admin/coupon-templates/${t.id}`, "DELETE", undefined, storeId).then(reload)}>永久删除</button>}
            </span>
          </div>
        ))}
      </section>
    </div>
  );
}

function Tables({ storeId }: { storeId?: string }) {
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; table_no: string; area?: string; enabled?: boolean }[] }>("/api/v1/admin/tables", storeId), [storeId]);
  const [no, setNo] = useState("");
  const [area, setArea] = useState("");
  return (
    <div className="rounded-xl bg-white p-4">
      <div className="mb-2 flex gap-2">
        <input className="rounded border px-2 py-1" placeholder="桌号" value={no} onChange={(e) => setNo(e.target.value)} />
        <input className="rounded border px-2 py-1" placeholder="区域" value={area} onChange={(e) => setArea(e.target.value)} />
        <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/tables", "POST", { table_no: no, area, enabled: true }, storeId).then(reload)}>新建</button>
      </div>
      {(data?.items || []).map((t) => (
        <div key={t.id} className="flex justify-between py-1 text-sm">
          {t.table_no} {t.area}
          <span className="space-x-2">
            <button className="text-brand" onClick={() => api.send(`/api/v1/admin/tables/${t.id}/rotate-token`, "POST", {}, storeId)}>换码</button>
            <a className="text-brand" href={`/api/v1/admin/tables/${t.id}/miniprogram-code`} target="_blank" rel="noreferrer">下载小程序码</a>
          </span>
        </div>
      ))}
    </div>
  );
}

function StoreSettings({ storeId }: { storeId?: string }) {
  const { data, reload } = useLoad(() => api.get<Record<string, unknown>>("/api/v1/admin/store", storeId), [storeId]);
  const [form, setForm] = useState<Record<string, string | number | boolean>>({});
  useEffect(() => {
    if (!data) return;
    setForm({
      name: String(data.name || ""),
      phone: String(data.phone || ""),
      address: String(data.address || ""),
      business_hours: String(data.business_hours || "09:00-21:00"),
      announcement: String(data.announcement || ""),
      timezone: String(data.timezone || "Asia/Shanghai"),
      pickup_minutes: Number(data.pickup_minutes || 15),
      is_open: Boolean(data.is_open),
      scheduled_pickup_enabled: Boolean(data.scheduled_pickup_enabled),
      pickup_advance_days: Number(data.pickup_advance_days || 7),
      pickup_slot_minutes: Number(data.pickup_slot_minutes || 15),
      pickup_slot_capacity: Number(data.pickup_slot_capacity || 10),
      pickup_min_lead_minutes: Number(data.pickup_min_lead_minutes || 30),
      privacy_policy: String(data.privacy_policy || ""),
      refund_policy: String(data.refund_policy || ""),
    });
  }, [data]);
  if (!data) return <p>加载中</p>;
  const set = (k: string, v: string | number | boolean) => setForm({ ...form, [k]: v });
  return (
    <div className="rounded-xl bg-white p-4 space-y-2">
      <h2 className="font-semibold">门店设置</h2>
      {["name", "phone", "address", "business_hours", "timezone"].map((k) => (
        <input key={k} className="w-full rounded border px-2 py-1" placeholder={k} value={String(form[k] || "")} onChange={(e) => set(k, e.target.value)} />
      ))}
      <textarea className="w-full rounded border p-2" placeholder="公告" value={String(form.announcement || "")} onChange={(e) => set("announcement", e.target.value)} />
      <label className="text-sm"><input type="checkbox" checked={Boolean(form.is_open)} onChange={(e) => set("is_open", e.target.checked)} /> 营业中</label>
      <label className="ml-2 text-sm"><input type="checkbox" checked={Boolean(form.scheduled_pickup_enabled)} onChange={(e) => set("scheduled_pickup_enabled", e.target.checked)} /> 开启预约</label>
      <div className="grid grid-cols-2 gap-2 text-sm">
        {[["pickup_minutes", "预计分钟"], ["pickup_advance_days", "可预约天数"], ["pickup_slot_minutes", "时段间隔"], ["pickup_slot_capacity", "时段容量"], ["pickup_min_lead_minutes", "最少提前分钟"]].map(([k, lab]) => (
          <label key={k}>{lab}<input className="ml-1 w-20 rounded border px-1" type="number" value={Number(form[k] || 0)} onChange={(e) => set(k, Number(e.target.value))} /></label>
        ))}
      </div>
      <textarea className="w-full rounded border p-2" placeholder="隐私政策" value={String(form.privacy_policy || "")} onChange={(e) => set("privacy_policy", e.target.value)} />
      <textarea className="w-full rounded border p-2" placeholder="退款政策" value={String(form.refund_policy || "")} onChange={(e) => set("refund_policy", e.target.value)} />
      <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/store", "PATCH", form, storeId).then(reload)}>保存</button>
    </div>
  );
}

function Backup({ storeId, role }: { storeId?: string; role?: string }) {
  const [confirm, setConfirm] = useState(false);
  const [preview, setPreview] = useState("");
  return (
    <div className="rounded-xl bg-white p-4 space-y-3">
      <a className="text-brand" href="/api/v1/admin/store/export?sections=base,menu,miniprogram">导出备份</a>
      {role === "OWNER" && (
        <div>
          <p className="text-sm text-red-700">导入将把现有菜单放入回收站并以新 ID 替换，此操作不可撤销。</p>
          <label className="block text-sm"><input type="checkbox" checked={confirm} onChange={(e) => setConfirm(e.target.checked)} /> 我已确认风险</label>
          {preview && <pre className="max-h-40 overflow-auto text-xs">{preview}</pre>}
          <input type="file" disabled={!confirm} onChange={async (e) => {
            const f = e.target.files?.[0];
            if (!f || !confirm) return;
            const text = await f.text();
            try {
              const j = JSON.parse(text);
              setPreview(`来源 ${j.source_store_id || ""} 分区 ${JSON.stringify(j.sections || j.partitions || [])} 版本 ${j.version}`);
            } catch { setPreview("无法预览"); }
            await fetch("/api/v1/admin/store/import", { method: "POST", credentials: "include", headers: { "X-Store-Id": storeId || "", "Idempotency-Key": crypto.randomUUID() }, body: await new Blob([text]).arrayBuffer() });
          }} />
        </div>
      )}
    </div>
  );
}

function PrintPage({ storeId, role }: { storeId?: string; role?: string }) {
  const cfg = useLoad(() => api.get<{ status: string; auto_print: boolean; mock_print: boolean; configured: boolean }>("/api/v1/admin/print/config", storeId), [storeId]);
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; name: string; sn: string; is_default?: boolean; copies?: number; enabled?: boolean }[] }>("/api/v1/admin/print/printers", storeId), [storeId]);
  const jobs = useLoad(() => api.get<{ items: { id: string; type: string; status: string; error?: string }[] }>("/api/v1/admin/print/jobs", storeId), [storeId]);
  const [appid, setAppid] = useState("");
  const [secret, setSecret] = useState("");
  const [sn, setSn] = useState("");
  const [key, setKey] = useState("");
  const [pname, setPname] = useState("默认机");
  return (
    <div className="space-y-4">
      <section className="rounded-xl bg-white p-4 space-y-2">
        <h2 className="font-semibold">商鹏配置 状态 {cfg.data?.status} {cfg.data?.configured ? "已配置" : "未配置"}</h2>
        <input className="w-full rounded border px-2 py-1" placeholder="AppID" value={appid} onChange={(e) => setAppid(e.target.value)} />
        <input className="w-full rounded border px-2 py-1" placeholder="AppSecret（只写）" value={secret} onChange={(e) => setSecret(e.target.value)} />
        <label className="text-sm"><input type="checkbox" defaultChecked={cfg.data?.auto_print} onChange={(e) => api.send("/api/v1/admin/print/config", "PUT", { auto_print: e.target.checked }, storeId).then(cfg.reload)} /> 自动打印</label>
        <label className="ml-2 text-sm"><input type="checkbox" defaultChecked={cfg.data?.mock_print} onChange={(e) => api.send("/api/v1/admin/print/config", "PUT", { mock_print: e.target.checked }, storeId).then(cfg.reload)} /> 模拟打印</label>
        <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/print/config", "PUT", { appid, appsecret: secret }, storeId).then(cfg.reload)}>保存密钥</button>
      </section>
      <section className="rounded-xl bg-white p-4 space-y-2">
        <h2 className="font-semibold">打印机</h2>
        <div className="flex flex-wrap gap-2">
          <input className="rounded border px-2 py-1" placeholder="SN" value={sn} onChange={(e) => setSn(e.target.value)} />
          <input className="rounded border px-2 py-1" placeholder="KEY" value={key} onChange={(e) => setKey(e.target.value)} />
          <input className="rounded border px-2 py-1" placeholder="名称" value={pname} onChange={(e) => setPname(e.target.value)} />
          <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/print/printers", "POST", { sn, key, name: pname, copies: 1, enabled: true, is_default: true }, storeId).then(reload)}>绑定</button>
        </div>
        {(data?.items || []).map((p) => (
          <div key={p.id} className="flex justify-between py-1 text-sm">
            {p.name} {p.sn} {p.is_default && "默认"}
            <span className="space-x-2">
              <button onClick={() => api.send(`/api/v1/admin/print/printers/${p.id}/test`, "POST", {}, storeId)}>测试</button>
              {role === "OWNER" && <button onClick={() => api.send(`/api/v1/admin/print/printers/${p.id}`, "DELETE", undefined, storeId).then(reload)}>删除</button>}
            </span>
          </div>
        ))}
      </section>
      <section className="rounded-xl bg-white p-4 text-sm">
        <h2 className="font-semibold">任务</h2>
        {(jobs.data?.items || []).map((j) => <div key={j.id}>{j.type} {j.status} {j.error}</div>)}
      </section>
    </div>
  );
}

function Materials({ storeId }: { storeId?: string }) {
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; name: string; unit: string; enabled: boolean; default_qty: number }[] }>("/api/v1/admin/materials", storeId), [storeId]);
  const lists = useLoad(() => api.get<{ items: { id: string; title: string; status: string; version: number }[] }>("/api/v1/admin/purchase-lists", storeId), [storeId]);
  const [name, setName] = useState("");
  const [unit, setUnit] = useState("份");
  const [open, setOpen] = useState<string>("");
  const detail = useLoad(() => open ? api.get<{ items: { id: string; name: string; qty: number; unit: string }[]; status: string; version: number }>(`/api/v1/admin/purchase-lists/${open}`, storeId) : Promise.resolve(null), [storeId, open]);
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <section className="rounded-xl bg-white p-4">
        <h2 className="font-semibold">物料</h2>
        <div className="mb-2 flex gap-2">
          <input className="rounded border px-2 py-1" value={name} onChange={(e) => setName(e.target.value)} placeholder="名称" />
          <input className="w-20 rounded border px-2 py-1" value={unit} onChange={(e) => setUnit(e.target.value)} />
          <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/materials", "POST", { name, unit, enabled: true, default_qty: 1 }, storeId).then(reload)}>新增</button>
        </div>
        {(data?.items || []).map((m) => <div key={m.id} className="flex justify-between text-sm">{m.name} / {m.unit} {m.enabled ? "" : "停用"}
          <button className="text-brand" onClick={() => open && api.send(`/api/v1/admin/purchase-lists/${open}/items`, "POST", { material_id: m.id, qty: m.default_qty || 1 }, storeId).then(detail.reload)}>加入清单</button>
        </div>)}
      </section>
      <section className="rounded-xl bg-white p-4">
        <h2 className="font-semibold">采购清单</h2>
        <button className="mb-2 rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/purchase-lists", "POST", { title: "采购" }, storeId).then(lists.reload)}>新建草稿</button>
        {(lists.data?.items || []).map((l) => (
          <div key={l.id} className="flex justify-between text-sm">
            <button onClick={() => setOpen(l.id)}>{l.title} {l.status}</button>
            {l.status === "DRAFT" && <button onClick={() => api.send(`/api/v1/admin/purchase-lists/${l.id}/submit`, "POST", { expected_version: l.version }, storeId).then(lists.reload)}>提交</button>}
            {l.status === "SUBMITTED" && <button onClick={() => api.send(`/api/v1/admin/purchase-lists/${l.id}/complete`, "POST", { expected_version: l.version, total_amount_cents: 0 }, storeId).then(lists.reload)}>完成</button>}
          </div>
        ))}
        {detail.data && <div className="mt-2 text-sm">{(detail.data.items || []).map((it) => <div key={it.id}>{it.name} {it.qty}{it.unit}</div>)}</div>}
      </section>
    </div>
  );
}

function Members({ storeId, role }: { storeId?: string; role?: string }) {
  const { data, reload } = useLoad(() => api.get<{ items: { admin_user_id: string; display_name: string; role: string }[] }>("/api/v1/admin/members", storeId), [storeId]);
  const joins = useLoad(() => api.get<{ items: { id: string; display_name?: string; requested_role: string; status: string }[] }>("/api/v1/admin/join-requests", storeId), [storeId]);
  return (
    <div className="space-y-4">
      <section className="rounded-xl bg-white p-4">
        {(data?.items || []).map((m) => (
          <div key={m.admin_user_id} className="flex justify-between py-1 text-sm">
            {m.display_name} {m.role}
            {role === "OWNER" && (
              <span className="space-x-2">
                <button onClick={() => api.send(`/api/v1/admin/members/${m.admin_user_id}`, "POST", { role: m.role === "STAFF" ? "MANAGER" : "STAFF" }, storeId).then(reload)}>改角色</button>
                <button onClick={() => api.send(`/api/v1/admin/members/${m.admin_user_id}`, "DELETE", undefined, storeId).then(reload)}>移除</button>
              </span>
            )}
          </div>
        ))}
      </section>
      <section className="rounded-xl bg-white p-4 text-sm">
        <h2 className="font-semibold">加入申请</h2>
        {(joins.data?.items || []).map((j) => (
          <div key={j.id} className="flex justify-between">
            {j.display_name} {j.requested_role} {j.status}
            {j.status === "PENDING" && role === "OWNER" && (
              <span>
                <button className="text-brand" onClick={() => api.send(`/api/v1/admin/join-requests/${j.id}/review`, "POST", { decision: "APPROVED" }, storeId).then(() => { joins.reload(); reload(); })}>通过</button>
                <button onClick={() => api.send(`/api/v1/admin/join-requests/${j.id}/review`, "POST", { decision: "REJECTED" }, storeId).then(joins.reload)}>驳回</button>
              </span>
            )}
          </div>
        ))}
      </section>
    </div>
  );
}

function Security({ storeId }: { storeId?: string }) {
  const { data, reload } = useLoad(() => api.get<Record<string, unknown>>("/api/v1/admin/payment-config", storeId), [storeId]);
  const mp = useLoad(() => api.get<Record<string, unknown>>("/api/v1/admin/miniprogram-config", storeId), [storeId]);
  const [secret, setSecret] = useState("");
  const [mch, setMch] = useState("");
  const [serial, setSerial] = useState("");
  const [apiV3, setApiV3] = useState("");
  const [priv, setPriv] = useState("");
  const [pub, setPub] = useState("");
  const [appid, setAppid] = useState("");
  const [brand, setBrand] = useState("");
  const [color, setColor] = useState("#c2410c");
  const certHint = inspectMerchantCert(priv || pub);
  useEffect(() => {
    if (mp.data?.wechat_appid) setAppid(String(mp.data.wechat_appid));
    if (mp.data?.brand_name) setBrand(String(mp.data.brand_name));
    if (mp.data?.theme_color) setColor(String(mp.data.theme_color));
  }, [mp.data]);
  return (
    <div className="space-y-4">
      <div className="rounded-xl bg-white p-4 space-y-2">
        <h2 className="font-semibold">小程序</h2>
        <input className="w-full rounded border px-2 py-1" placeholder="AppID" value={appid} onChange={(e) => setAppid(e.target.value)} />
        <input className="w-full rounded border px-2 py-1" placeholder="品牌名" value={brand} onChange={(e) => setBrand(e.target.value)} />
        <input className="w-full rounded border px-2 py-1" placeholder="主题色" value={color} onChange={(e) => setColor(e.target.value)} />
        <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/miniprogram-config", "PATCH", { wechat_appid: appid, brand_name: brand, theme_color: color }, storeId).then(mp.reload)}>保存小程序配置</button>
      </div>
      <div className="rounded-xl bg-white p-4 space-y-2">
        <p className="text-sm">密钥只写不读。登录就绪：{String(data?.wechat_login_ready)} 支付就绪：{String(data?.pay_ready)}</p>
        <input className="w-full rounded border px-2 py-1" placeholder="AppSecret（留空保留）" value={secret} onChange={(e) => setSecret(e.target.value)} />
        <input className="w-full rounded border px-2 py-1" placeholder="商户号" value={mch} onChange={(e) => setMch(e.target.value)} />
        <input className="w-full rounded border px-2 py-1" placeholder="证书序列号" value={serial} onChange={(e) => setSerial(e.target.value)} />
        <input className="w-full rounded border px-2 py-1" placeholder="APIv3 密钥" value={apiV3} onChange={(e) => setApiV3(e.target.value)} />
        <textarea className="w-full rounded border p-2 font-mono text-xs" rows={4} placeholder="商户私钥 PEM" value={priv} onChange={(e) => setPriv(e.target.value)} />
        <textarea className="w-full rounded border p-2 font-mono text-xs" rows={3} placeholder="微信支付公钥或平台证书 PEM（浏览器仅校验格式，不保存证书文件）" value={pub} onChange={(e) => setPub(e.target.value)} />
        <p className="text-xs text-stone-500">{certHint.reason}</p>
        <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/payment-config", "PUT", {
          app_secret: secret, mch_id: mch, serial_no: serial, api_v3_key: apiV3, private_key: priv, pub_key_pem: pub,
        }, storeId).then(reload)}>保存支付配置</button>
        <button className="ml-2 rounded border px-3 py-1" onClick={() => {
          const ok = window.prompt("输入 MOCK 确认") === "MOCK";
          if (ok) api.send("/api/v1/admin/mock-payment", "PUT", { enabled: true, confirm: "MOCK" }, storeId);
        }}>开启 mock 支付</button>
      </div>
    </div>
  );
}

function PlatformStores() {
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; name: string; enabled: boolean }[] }>("/api/v1/admin/platform/stores"), []);
  const [name, setName] = useState("");
  return (
    <div className="rounded-xl bg-white p-4">
      <div className="mb-2 flex gap-2">
        <input className="rounded border px-2 py-1" value={name} onChange={(e) => setName(e.target.value)} />
        <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/platform/stores", "POST", { name }).then(reload)}>创建门店</button>
      </div>
      {(data?.items || []).map((s) => (
        <div key={s.id} className="flex justify-between text-sm">
          {s.name} {s.enabled ? "启用" : "停用"} <span className="text-stone-400">{s.id}</span>
          <button onClick={() => api.send(`/api/v1/admin/platform/stores/${s.id}`, "PATCH", { enabled: !s.enabled }).then(reload)}>{s.enabled ? "停用" : "启用"}</button>
        </div>
      ))}
    </div>
  );
}

function PlatformUsers() {
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; login_name: string; display_name: string }[] }>("/api/v1/admin/platform/users"), []);
  const stores = useLoad(() => api.get<{ items: { id: string; name: string }[] }>("/api/v1/admin/platform/stores"), []);
  const [login, setLogin] = useState("");
  const [display, setDisplay] = useState("");
  const [password, setPassword] = useState("");
  const [storeId, setStoreId] = useState("");
  return (
    <div className="rounded-xl bg-white p-4 space-y-2">
      <input className="w-full rounded border px-2 py-1" placeholder="登录名" value={login} onChange={(e) => setLogin(e.target.value)} />
      <input className="w-full rounded border px-2 py-1" placeholder="显示名" value={display} onChange={(e) => setDisplay(e.target.value)} />
      <input className="w-full rounded border px-2 py-1" placeholder="初始密码 12+" value={password} onChange={(e) => setPassword(e.target.value)} />
      <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/platform/users", "POST", { login_name: login, display_name: display, password, enabled: true }).then(reload)}>创建账号</button>
      {(data?.items || []).map((u) => (
        <div key={u.id} className="flex flex-wrap items-center gap-2 py-1 text-sm">
          {u.login_name} {u.display_name}
          <select className="rounded border" value={storeId} onChange={(e) => setStoreId(e.target.value)}>
            <option value="">指定店主到…</option>
            {(stores.data?.items || []).map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
          <button className="text-brand" onClick={() => api.send(`/api/v1/admin/platform/users/${u.id}/assign-store-owner`, "POST", { store_id: storeId }).then(reload)}>指定店主</button>
        </div>
      ))}
    </div>
  );
}

function PlatformApps() {
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; store_name: string; status: string }[] }>("/api/v1/admin/platform/shop-applications"), []);
  return (
    <div className="rounded-xl bg-white p-4">
      {(data?.items || []).map((a) => (
        <div key={a.id} className="flex justify-between py-1 text-sm">
          {a.store_name} {a.status}
          {a.status === "PENDING" && (
            <span>
              <button className="text-brand" onClick={() => api.send(`/api/v1/admin/platform/shop-applications/${a.id}/review`, "POST", { decision: "APPROVED" }).then(reload)}>通过</button>
              <button onClick={() => api.send(`/api/v1/admin/platform/shop-applications/${a.id}/review`, "POST", { decision: "REJECTED" }).then(reload)}>驳回</button>
            </span>
          )}
        </div>
      ))}
    </div>
  );
}
