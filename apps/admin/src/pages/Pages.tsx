import { useEffect, useState } from "react";
import { Route, Routes } from "react-router-dom";
import { api, type Principal } from "../lib/api";
import { fenToYuan, nextTransition, statusLabel, transitionLabel } from "../lib/ui";

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
  const reload = () => fn().then(setData).catch((e) => setErr(e.message));
  useEffect(() => { reload(); }, deps);
  return { data, err, reload, setErr };
}

function Board({ storeId }: { storeId?: string }) {
  const { data, reload } = useLoad(() => api.get<Record<string, Order[]>>("/api/v1/admin/orders/board", storeId), [storeId]);
  useEffect(() => {
    const t = setInterval(reload, 3000);
    return () => clearInterval(t);
  }, [storeId]);
  const cols = ["PAID", "ACCEPTED", "PREPARING", "READY"];
  return (
    <div className="grid gap-3 md:grid-cols-4">
      {cols.map((c) => (
        <div key={c} className="rounded-xl bg-white p-3 shadow-sm">
          <h3 className="mb-2 font-semibold">{statusLabel(c)}</h3>
          {(data?.[c] || []).map((o) => (
            <OrderCard key={o.id} o={o} storeId={storeId} onDone={reload} />
          ))}
          {(data?.[c] || []).length === 0 && <p className="text-sm text-stone-400">暂无</p>}
        </div>
      ))}
    </div>
  );
}

type Order = {
  id: string; status: string; version: number; pickup_number?: string; table_no?: string;
  pickup_type: string; scheduled_for?: string; payable_cents: number; remark?: string; is_mock?: boolean;
  items: { product_name: string; qty: number }[];
};

function OrderCard({ o, storeId, onDone }: { o: Order; storeId?: string; onDone: () => void }) {
  const next = nextTransition(o.status);
  return (
    <div className="mb-2 rounded border p-2 text-sm">
      <div className="font-medium">#{o.pickup_number || o.id.slice(-6)} {o.table_no && `桌${o.table_no}`}</div>
      {o.pickup_type === "SCHEDULED" && <div className="text-brand">预约 {o.scheduled_for}</div>}
      <div>{o.items.map((i) => `${i.product_name}×${i.qty}`).join("、")}</div>
      <div>¥{fenToYuan(o.payable_cents)} {o.is_mock && "测试"}</div>
      {o.remark && <div className="text-stone-500">备注 {o.remark}</div>}
      {next && (
        <button className="mt-1 rounded bg-brand px-2 py-1 text-white" onClick={async () => {
          await api.send(`/api/v1/admin/orders/${o.id}/transitions`, "POST", { to_status: next, expected_version: o.version }, storeId);
          onDone();
        }}>{transitionLabel(next)}</button>
      )}
    </div>
  );
}

function Orders({ storeId }: { storeId?: string }) {
  const { data } = useLoad(() => api.get<{ items: Order[] }>("/api/v1/admin/orders", storeId), [storeId]);
  return (
    <div className="overflow-x-auto rounded-xl bg-white p-4">
      <div className="mb-3 flex justify-between">
        <h2 className="font-semibold">历史订单</h2>
        <a className="text-brand" href="/api/v1/admin/orders/export">导出 CSV</a>
      </div>
      <table className="w-full text-left text-sm">
        <thead><tr><th>单号</th><th>状态</th><th>金额</th></tr></thead>
        <tbody>
          {(data?.items || []).map((o) => (
            <tr key={o.id} className="border-t"><td>{o.pickup_number || o.id}</td><td>{statusLabel(o.status)}</td><td>¥{fenToYuan(o.payable_cents)}</td></tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Refunds({ storeId }: { storeId?: string }) {
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; order_id: string; amount_cents: number; status: string; reason: string }[] }>("/api/v1/admin/refunds", storeId), [storeId]);
  const ex = useLoad(() => api.get<{ items: { id: string; biz_no: string; error_type: string; status: string; message: string }[] }>("/api/v1/admin/exceptions", storeId), [storeId]);
  return (
    <div className="space-y-4">
      <section className="rounded-xl bg-white p-4">
        <h2 className="mb-2 font-semibold">退款</h2>
        {(data?.items || []).map((r) => (
          <div key={r.id} className="flex justify-between border-b py-2 text-sm">
            <span>{r.order_id} ¥{fenToYuan(r.amount_cents)} {r.status}</span>
            <span>
              <button className="mr-2 text-brand" onClick={() => api.send(`/api/v1/admin/refunds/${r.id}/review`, "POST", { decision: "APPROVED" }, storeId).then(reload)}>通过</button>
              <button onClick={() => api.send(`/api/v1/admin/refunds/${r.id}/review`, "POST", { decision: "REJECTED" }, storeId).then(reload)}>驳回</button>
            </span>
          </div>
        ))}
      </section>
      <section className="rounded-xl bg-white p-4">
        <h2 className="mb-2 font-semibold">支付异常</h2>
        {(ex.data?.items || []).map((e) => (
          <div key={e.id} className="flex justify-between text-sm">
            <span>{e.biz_no} {e.error_type} {e.message}</span>
            <button className="text-brand" onClick={() => api.send(`/api/v1/admin/exceptions/${e.id}/retry`, "POST", {}, storeId)}>重试查单</button>
          </div>
        ))}
      </section>
    </div>
  );
}

function Analytics({ storeId }: { storeId?: string }) {
  const { data } = useLoad(() => api.get<{ current: Record<string, number> }>("/api/v1/admin/analytics/overview", storeId), [storeId]);
  const cur = data?.current || {};
  return (
    <div className="grid gap-3 md:grid-cols-3">
      {[["净收入", cur.net_amount_cents], ["支付订单", cur.paid_orders], ["客单价", cur.aov_cents]].map(([k, v]) => (
        <div key={String(k)} className="rounded-xl bg-white p-4"><div className="text-stone-500">{k}</div><div className="text-2xl font-semibold">{typeof k === "string" && k.includes("订单") ? v : `¥${fenToYuan(Number(v || 0))}`}</div></div>
      ))}
    </div>
  );
}

function Customers({ storeId }: { storeId?: string }) {
  const { data } = useLoad(() => api.get<{ items: { customer_id: string; member_no: string; phone_masked: string; points_balance: number }[] }>("/api/v1/admin/customer-members", storeId), [storeId]);
  return (
    <div className="rounded-xl bg-white p-4">
      {(data?.items || []).map((m) => (
        <div key={m.customer_id} className="border-b py-2 text-sm">{m.member_no} {m.phone_masked} 积分 {m.points_balance}</div>
      ))}
    </div>
  );
}

function Menu({ storeId }: { storeId?: string }) {
  const cats = useLoad(() => api.get<{ items: { id: string; name: string; enabled: boolean }[] }>("/api/v1/admin/categories?include_deleted=1", storeId), [storeId]);
  const dishes = useLoad(() => api.get<{ items: { id: string; name: string; from_price_cents?: number; enabled: boolean }[] }>("/api/v1/admin/dishes?include_deleted=1", storeId), [storeId]);
  const [name, setName] = useState("");
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <section className="rounded-xl bg-white p-4">
        <h2 className="font-semibold">分类</h2>
        <div className="mt-2 flex gap-2">
          <input className="flex-1 rounded border px-2 py-1" value={name} onChange={(e) => setName(e.target.value)} />
          <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/categories", "POST", { name, enabled: true }, storeId).then(cats.reload)}>新建</button>
        </div>
        {(cats.data?.items || []).map((c) => (
          <div key={c.id} className="flex justify-between py-1 text-sm">
            <span>{c.name}</span>
            <button className="text-red-600" onClick={() => api.send(`/api/v1/admin/categories/${c.id}`, "DELETE", undefined, storeId).then(cats.reload)}>回收</button>
          </div>
        ))}
      </section>
      <section className="rounded-xl bg-white p-4">
        <h2 className="font-semibold">菜品</h2>
        {(dishes.data?.items || []).map((d) => (
          <div key={d.id} className="py-1 text-sm">{d.name} {d.enabled ? "在售" : "下架"}</div>
        ))}
      </section>
    </div>
  );
}

function Promos({ storeId, role }: { storeId?: string; role?: string }) {
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; name: string; discount_cents: number }[] }>("/api/v1/admin/coupon-templates", storeId), [storeId]);
  return (
    <div className="rounded-xl bg-white p-4">
      <h2 className="font-semibold">优惠券模板</h2>
      {(data?.items || []).map((t) => (
        <div key={t.id} className="flex justify-between py-2 text-sm">
          <span>{t.name} -¥{fenToYuan(t.discount_cents)}</span>
          {role === "OWNER" && <button onClick={() => api.send(`/api/v1/admin/coupon-templates/${t.id}`, "DELETE", undefined, storeId).then(reload)}>永久删除</button>}
        </div>
      ))}
    </div>
  );
}

function Tables({ storeId }: { storeId?: string }) {
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; table_no: string }[] }>("/api/v1/admin/tables", storeId), [storeId]);
  const [no, setNo] = useState("");
  return (
    <div className="rounded-xl bg-white p-4">
      <div className="mb-2 flex gap-2">
        <input className="rounded border px-2 py-1" placeholder="桌号" value={no} onChange={(e) => setNo(e.target.value)} />
        <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/tables", "POST", { table_no: no, enabled: true }, storeId).then(reload)}>新建</button>
      </div>
      {(data?.items || []).map((t) => (
        <div key={t.id} className="flex justify-between py-1 text-sm">
          {t.table_no}
          <button className="text-brand" onClick={() => api.send(`/api/v1/admin/tables/${t.id}/rotate-token`, "POST", {}, storeId)}>换码</button>
        </div>
      ))}
    </div>
  );
}

function StoreSettings({ storeId }: { storeId?: string }) {
  const { data, reload } = useLoad(() => api.get<Record<string, unknown>>("/api/v1/admin/store", storeId), [storeId]);
  const [ann, setAnn] = useState("");
  useEffect(() => { if (data?.announcement) setAnn(String(data.announcement)); }, [data]);
  if (!data) return <p>加载中</p>;
  return (
    <div className="rounded-xl bg-white p-4">
      <h2 className="font-semibold">{String(data.name)}</h2>
      <textarea className="mt-2 w-full rounded border p-2" value={ann} onChange={(e) => setAnn(e.target.value)} />
      <button className="mt-2 rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/store", "PATCH", { announcement: ann }, storeId).then(reload)}>保存</button>
    </div>
  );
}

function Backup({ storeId, role }: { storeId?: string; role?: string }) {
  const [confirm, setConfirm] = useState(false);
  return (
    <div className="rounded-xl bg-white p-4 space-y-3">
      <a className="text-brand" href="/api/v1/admin/store/export">导出备份</a>
      {role === "OWNER" && (
        <div>
          <p className="text-sm text-red-700">导入将把现有菜单放入回收站并以新 ID 替换，此操作不可撤销。</p>
          <label className="block text-sm"><input type="checkbox" checked={confirm} onChange={(e) => setConfirm(e.target.checked)} /> 我已确认风险</label>
          <input type="file" disabled={!confirm} onChange={async (e) => {
            const f = e.target.files?.[0];
            if (!f || !confirm) return;
            await fetch("/api/v1/admin/store/import", { method: "POST", credentials: "include", headers: { "X-Store-Id": storeId || "" }, body: await f.arrayBuffer() });
          }} />
        </div>
      )}
    </div>
  );
}

function PrintPage({ storeId, role }: { storeId?: string; role?: string }) {
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; name: string; sn: string }[] }>("/api/v1/admin/print/printers", storeId), [storeId]);
  return (
    <div className="rounded-xl bg-white p-4">
      {(data?.items || []).map((p) => (
        <div key={p.id} className="flex justify-between py-1 text-sm">
          {p.name} {p.sn}
          {role === "OWNER" && <button onClick={() => api.send(`/api/v1/admin/print/printers/${p.id}`, "DELETE", undefined, storeId).then(reload)}>删除</button>}
        </div>
      ))}
    </div>
  );
}

function Materials({ storeId }: { storeId?: string }) {
  const { data } = useLoad(() => api.get<{ items: { id: string; name: string; unit: string }[] }>("/api/v1/admin/materials", storeId), [storeId]);
  const lists = useLoad(() => api.get<{ items: { id: string; title: string; status: string }[] }>("/api/v1/admin/purchase-lists", storeId), [storeId]);
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <section className="rounded-xl bg-white p-4">
        <h2 className="font-semibold">物料</h2>
        {(data?.items || []).map((m) => <div key={m.id} className="text-sm">{m.name} / {m.unit}</div>)}
      </section>
      <section className="rounded-xl bg-white p-4">
        <h2 className="font-semibold">采购清单</h2>
        <button className="mb-2 rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/purchase-lists", "POST", { title: "采购" }, storeId).then(lists.reload)}>新建草稿</button>
        {(lists.data?.items || []).map((l) => <div key={l.id} className="text-sm">{l.title} {l.status}</div>)}
      </section>
    </div>
  );
}

function Members({ storeId, role }: { storeId?: string; role?: string }) {
  const { data, reload } = useLoad(() => api.get<{ items: { admin_user_id: string; display_name: string; role: string }[] }>("/api/v1/admin/members", storeId), [storeId]);
  return (
    <div className="rounded-xl bg-white p-4">
      {(data?.items || []).map((m) => (
        <div key={m.admin_user_id} className="flex justify-between py-1 text-sm">
          {m.display_name} {m.role}
          {role === "OWNER" && <button onClick={() => api.send(`/api/v1/admin/members/${m.admin_user_id}`, "DELETE", undefined, storeId).then(reload)}>移除</button>}
        </div>
      ))}
    </div>
  );
}

function Security({ storeId }: { storeId?: string }) {
  const { data } = useLoad(() => api.get<Record<string, unknown>>("/api/v1/admin/payment-config", storeId), [storeId]);
  const [secret, setSecret] = useState("");
  return (
    <div className="rounded-xl bg-white p-4 space-y-2">
      <p className="text-sm">密钥只写不读。登录就绪：{String(data?.wechat_login_ready)} 支付就绪：{String(data?.pay_ready)}</p>
      <input className="w-full rounded border px-2 py-1" placeholder="AppSecret（留空保留）" value={secret} onChange={(e) => setSecret(e.target.value)} />
      <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/payment-config", "PUT", { app_secret: secret }, storeId)}>保存</button>
      <button className="ml-2 rounded border px-3 py-1" onClick={() => {
        const ok = window.prompt("输入 MOCK 确认") === "MOCK";
        if (ok) api.send("/api/v1/admin/mock-payment", "PUT", { enabled: true, confirm: "MOCK" }, storeId);
      }}>开启 mock 支付</button>
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
      {(data?.items || []).map((s) => <div key={s.id} className="text-sm">{s.name} {s.enabled ? "启用" : "停用"} <span className="text-stone-400">{s.id}</span></div>)}
    </div>
  );
}

function PlatformUsers() {
  const { data, reload } = useLoad(() => api.get<{ items: { id: string; login_name: string; display_name: string }[] }>("/api/v1/admin/platform/users"), []);
  const [login, setLogin] = useState("");
  const [display, setDisplay] = useState("");
  const [password, setPassword] = useState("");
  return (
    <div className="rounded-xl bg-white p-4 space-y-2">
      <input className="rounded border px-2 py-1" placeholder="登录名" value={login} onChange={(e) => setLogin(e.target.value)} />
      <input className="rounded border px-2 py-1" placeholder="显示名" value={display} onChange={(e) => setDisplay(e.target.value)} />
      <input className="rounded border px-2 py-1" placeholder="初始密码 12+" value={password} onChange={(e) => setPassword(e.target.value)} />
      <button className="rounded bg-brand px-3 py-1 text-white" onClick={() => api.send("/api/v1/admin/platform/users", "POST", { login_name: login, display_name: display, password, enabled: true }).then(reload)}>创建账号</button>
      {(data?.items || []).map((u) => <div key={u.id} className="text-sm">{u.login_name} {u.display_name}</div>)}
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
          {a.status === "PENDING" && <button className="text-brand" onClick={() => api.send(`/api/v1/admin/platform/shop-applications/${a.id}/review`, "POST", { decision: "APPROVED" }).then(reload)}>通过</button>}
        </div>
      ))}
    </div>
  );
}
