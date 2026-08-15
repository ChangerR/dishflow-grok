import { useEffect, useMemo, useState } from "react";
import { Alert, Badge, Btn, Card, CardHead, DataTable, Empty, Field, Input, Loading, PageHeader, Select, Textarea, Toolbar } from "../components/kit";
import { api } from "../lib/api";
import { fenToYuan, nextTransition, PAGE_COPY, pickupLabel, roleLabel, statusLabel, statusTone, transitionLabel } from "../lib/ui";

function useLoad<T>(fn: () => Promise<T>, deps: unknown[]) {
  const [data, setData] = useState<T | null>(null);
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(true);
  const reload = () =>
    fn()
      .then((d) => {
        setData(d);
        setErr("");
      })
      .catch((e: Error) => setErr(e.message))
      .finally(() => setLoading(false));
  useEffect(() => {
    reload();
  }, deps);
  return { data, err, loading, reload, setErr };
}

function Err({ err }: { err: string }) {
  if (!err) return null;
  return (
    <div className="mb-3">
      <Alert>{err}</Alert>
    </div>
  );
}

async function act(fn: () => Promise<unknown>, reload?: () => void, setErr?: (s: string) => void) {
  try {
    await fn();
    reload?.();
  } catch (e) {
    const msg = e instanceof Error ? e.message : "操作失败";
    setErr?.(msg);
    window.alert(msg);
  }
}

function copy(path: string) {
  return PAGE_COPY[path] || { title: "", desc: "" };
}

type Order = {
  id: string;
  status: string;
  version: number;
  pickup_number?: string;
  table_no?: string;
  pickup_type: string;
  scheduled_for?: string;
  payable_cents: number;
  remark?: string;
  is_mock?: boolean;
  items: { product_name: string; qty: number }[];
};

const BOARD_COLS = [
  { key: "PAID", accent: "border-t-brand", count: "bg-orange-50 text-orange-800" },
  { key: "ACCEPTED", accent: "border-t-sky-500", count: "bg-sky-50 text-sky-800" },
  { key: "PREPARING", accent: "border-t-amber-500", count: "bg-amber-50 text-amber-800" },
  { key: "READY", accent: "border-t-emerald-500", count: "bg-emerald-50 text-emerald-800" },
];

export function Board({ storeId }: { storeId?: string }) {
  const { data, err, reload } = useLoad(() => api.get<Record<string, Order[]>>("/api/v1/admin/orders/board", storeId), [storeId]);
  const [q, setQ] = useState("");
  useEffect(() => {
    const t = setInterval(reload, 3000);
    return () => clearInterval(t);
  }, [storeId]);
  const match = (o: Order) => {
    const s = q.trim().toLowerCase();
    if (!s) return true;
    return [o.pickup_number, o.table_no, o.id, o.remark, ...(o.items || []).map((i) => i.product_name)].some((x) => String(x || "").toLowerCase().includes(s));
  };
  const meta = copy("/board");
  return (
    <div>
      <PageHeader
        title={meta.title}
        desc={meta.desc}
        extra={
          <>
            <div className="flex items-center gap-1.5 text-xs text-stone-500">
              <span className="h-2 w-2 animate-pulse rounded-full bg-emerald-500" />
              实时
            </div>
            <Input className="w-52" placeholder="取餐号 / 桌号 / 菜品" value={q} onChange={(e) => setQ(e.target.value)} />
            <Btn variant="secondary" onClick={reload}>
              刷新
            </Btn>
          </>
        }
      />
      <Err err={err} />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        {BOARD_COLS.map((c) => {
          const list = (data?.[c.key] || []).filter(match);
          return (
            <div key={c.key} className={`df-card border-t-4 p-3 ${c.accent}`}>
              <div className="mb-3 flex items-center justify-between">
                <h3 className="font-semibold text-stone-900">{statusLabel(c.key)}</h3>
                <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${c.count}`}>{list.length}</span>
              </div>
              <div className="space-y-2">
                {list.map((o) => (
                  <OrderCard key={o.id} o={o} storeId={storeId} onDone={reload} />
                ))}
                {list.length === 0 && <p className="rounded-xl bg-stone-50 px-3 py-8 text-center text-sm text-stone-400">暂无订单</p>}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function OrderCard({ o, storeId, onDone }: { o: Order; storeId?: string; onDone: () => void }) {
  const next = nextTransition(o.status);
  return (
    <div className="rounded-xl border border-stone-100 bg-stone-50/70 p-3">
      <div className="flex items-start justify-between gap-2">
        <div className="text-lg font-semibold tracking-tight text-stone-900">#{o.pickup_number || o.id.slice(-6)}</div>
        <div className="flex flex-wrap justify-end gap-1">
          {o.table_no && <Badge tone="info">桌 {o.table_no}</Badge>}
          {o.is_mock && <Badge tone="warn">测试</Badge>}
        </div>
      </div>
      {o.pickup_type === "SCHEDULED" && <div className="mt-1 text-xs font-medium text-brand">预约 {o.scheduled_for}</div>}
      <div className="mt-2 space-y-0.5 text-sm text-stone-600">
        {(o.items || []).map((i, idx) => (
          <div key={idx} className="flex justify-between gap-2">
            <span className="truncate">{i.product_name}</span>
            <span className="text-stone-400">×{i.qty}</span>
          </div>
        ))}
      </div>
      <div className="mt-2 flex items-center justify-between text-sm">
        <span className="font-semibold text-stone-900">¥{fenToYuan(o.payable_cents)}</span>
        <span className="text-[11px] text-stone-400">{pickupLabel(o.pickup_type)}</span>
      </div>
      {o.remark && <div className="mt-1 rounded-lg bg-white px-2 py-1 text-xs text-stone-500">备注 {o.remark}</div>}
      {next && (
        <Btn
          className="mt-3 w-full"
          variant={o.status === "PAID" ? "primary" : o.status === "READY" ? "success" : "secondary"}
          onClick={() => act(() => api.send(`/api/v1/admin/orders/${o.id}/transitions`, "POST", { to_status: next, expected_version: o.version }, storeId), onDone)}
        >
          {transitionLabel(next)}
        </Btn>
      )}
    </div>
  );
}

export function Orders({ storeId }: { storeId?: string }) {
  const { data, err, loading } = useLoad(() => api.get<{ items: Order[] }>("/api/v1/admin/orders", storeId), [storeId]);
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const items = useMemo(() => {
    return (data?.items || []).filter((o) => {
      if (status && o.status !== status) return false;
      const s = q.trim().toLowerCase();
      if (!s) return true;
      return [o.pickup_number, o.table_no, o.id].some((x) => String(x || "").toLowerCase().includes(s));
    });
  }, [data, q, status]);
  const meta = copy("/orders");
  return (
    <div>
      <PageHeader
        title={meta.title}
        desc={meta.desc}
        extra={
          <a className="df-btn df-btn-secondary" href="/api/v1/admin/orders/export">
            导出 CSV
          </a>
        }
      />
      <Card>
        <Toolbar>
          <Field label="检索">
            <Input className="w-52" placeholder="取餐号 / 桌号 / 单号" value={q} onChange={(e) => setQ(e.target.value)} />
          </Field>
          <Field label="状态">
            <Select className="w-40" value={status} onChange={(e) => setStatus(e.target.value)}>
              <option value="">全部状态</option>
              {["PAID", "ACCEPTED", "PREPARING", "READY", "COMPLETED", "CANCELLED", "REFUNDING", "REFUNDED"].map((s) => (
                <option key={s} value={s}>
                  {statusLabel(s)}
                </option>
              ))}
            </Select>
          </Field>
        </Toolbar>
        <Err err={err} />
        {loading && !data ? (
          <Loading />
        ) : (
          <DataTable
            rows={items}
            rowKey={(o) => o.id}
            emptyTitle="暂无历史订单"
            emptyHint="支付成功后的订单会出现在这里"
            columns={[
              { key: "no", title: "取餐号", render: (o) => <span className="font-medium">{o.pickup_number || o.id.slice(-8)}</span> },
              { key: "st", title: "状态", render: (o) => <Badge tone={statusTone(o.status)}>{statusLabel(o.status)}</Badge> },
              { key: "sc", title: "场景", render: (o) => pickupLabel(o.pickup_type) },
              { key: "tb", title: "桌号", render: (o) => o.table_no || "—" },
              { key: "amt", title: "金额", render: (o) => `¥${fenToYuan(o.payable_cents)}` },
              { key: "mk", title: "标记", render: (o) => (o.is_mock ? <Badge tone="warn">测试</Badge> : "—") },
            ]}
          />
        )}
      </Card>
    </div>
  );
}

export function Refunds({ storeId }: { storeId?: string }) {
  const { data, err, reload, setErr, loading } = useLoad(
    () => api.get<{ items: { id: string; order_id: string; amount_cents: number; status: string; reason: string }[] }>("/api/v1/admin/refunds", storeId),
    [storeId],
  );
  const ex = useLoad(
    () => api.get<{ items: { id: string; biz_no: string; error_type: string; status: string; message: string }[] }>("/api/v1/admin/exceptions", storeId),
    [storeId],
  );
  const meta = copy("/refunds");
  return (
    <div className="space-y-4">
      <PageHeader title={meta.title} desc={meta.desc} />
      <Card>
        <CardHead title="退款审核" desc="顾客取消待审与门店主动退款" />
        <Err err={err} />
        {loading && !data ? (
          <Loading />
        ) : (
          <DataTable
            rows={data?.items || []}
            rowKey={(r) => r.id}
            emptyTitle="暂无退款"
            columns={[
              { key: "oid", title: "订单", render: (r) => <span className="font-mono text-xs">{r.order_id.slice(0, 8)}</span> },
              { key: "amt", title: "金额", render: (r) => `¥${fenToYuan(r.amount_cents)}` },
              { key: "st", title: "状态", render: (r) => <Badge tone={statusTone(r.status)}>{statusLabel(r.status)}</Badge> },
              { key: "rs", title: "原因", render: (r) => r.reason || "—" },
              {
                key: "op",
                title: "操作",
                render: (r) =>
                  r.status === "PENDING" ? (
                    <div className="flex gap-2">
                      <Btn variant="success" onClick={() => act(() => api.send(`/api/v1/admin/refunds/${r.id}/review`, "POST", { decision: "APPROVED" }, storeId), reload, setErr)}>
                        通过
                      </Btn>
                      <Btn variant="secondary" onClick={() => act(() => api.send(`/api/v1/admin/refunds/${r.id}/review`, "POST", { decision: "REJECTED" }, storeId), reload, setErr)}>
                        驳回
                      </Btn>
                    </div>
                  ) : (
                    "—"
                  ),
              },
            ]}
          />
        )}
      </Card>
      <Card>
        <CardHead title="支付异常" desc="查单失败后可重试补偿，最终状态仍以微信为准" extra={<Btn variant="secondary" onClick={ex.reload}>刷新</Btn>} />
        <Err err={ex.err} />
        <DataTable
          rows={ex.data?.items || []}
          rowKey={(e) => e.id}
          emptyTitle="暂无异常"
          columns={[
            { key: "biz", title: "业务号", render: (e) => <span className="font-mono text-xs">{e.biz_no}</span> },
            { key: "tp", title: "类型", render: (e) => e.error_type },
            { key: "st", title: "状态", render: (e) => <Badge tone={statusTone(e.status)}>{statusLabel(e.status)}</Badge> },
            { key: "msg", title: "信息", render: (e) => <span className="max-w-xs truncate text-stone-500">{e.message}</span> },
            {
              key: "op",
              title: "操作",
              render: (e) => (
                <Btn variant="secondary" onClick={() => act(() => api.send(`/api/v1/admin/exceptions/${e.id}/retry`, "POST", {}, storeId), ex.reload, ex.setErr)}>
                  重试查单
                </Btn>
              ),
            },
          ]}
        />
      </Card>
    </div>
  );
}

export function Analytics({ storeId }: { storeId?: string }) {
  const { data, err, loading } = useLoad(() => api.get<{ current: Record<string, number> }>("/api/v1/admin/analytics/overview", storeId), [storeId]);
  const cur = data?.current || {};
  const meta = copy("/analytics");
  const cards = [
    { k: "净收入", v: `¥${fenToYuan(Number(cur.net_amount_cents || 0))}`, hint: "扣除退款后的实收" },
    { k: "支付订单", v: String(cur.paid_orders || 0), hint: "支付成功笔数" },
    { k: "客单价", v: `¥${fenToYuan(Number(cur.aov_cents || 0))}`, hint: "平均每单净额" },
  ];
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <Err err={err} />
      {loading && !data ? (
        <Loading />
      ) : (
        <div className="grid gap-3 md:grid-cols-3">
          {cards.map((c) => (
            <Card key={c.k}>
              <div className="text-sm text-stone-500">{c.k}</div>
              <div className="mt-2 text-3xl font-semibold tracking-tight">{c.v}</div>
              <div className="mt-1 text-xs text-stone-400">{c.hint}</div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

export function Customers({ storeId }: { storeId?: string }) {
  const { data, err, loading } = useLoad(
    () => api.get<{ items: { customer_id: string; member_no: string; phone_masked: string; points_balance: number }[] }>("/api/v1/admin/customer-members", storeId),
    [storeId],
  );
  const [q, setQ] = useState("");
  const rows = (data?.items || []).filter((m) => {
    const s = q.trim();
    if (!s) return true;
    return [m.member_no, m.phone_masked].some((x) => String(x || "").includes(s));
  });
  const meta = copy("/customers");
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <Card>
        <Toolbar>
          <Field label="检索">
            <Input className="w-56" placeholder="会员号 / 手机后四位" value={q} onChange={(e) => setQ(e.target.value)} />
          </Field>
        </Toolbar>
        <Err err={err} />
        {loading && !data ? (
          <Loading />
        ) : (
          <DataTable
            rows={rows}
            rowKey={(m) => m.customer_id}
            emptyTitle="暂无会员"
            emptyHint="顾客入会后会出现在此列表"
            columns={[
              { key: "no", title: "会员号", render: (m) => <span className="font-medium">{m.member_no}</span> },
              { key: "ph", title: "手机", render: (m) => m.phone_masked },
              { key: "pt", title: "积分", render: (m) => m.points_balance },
            ]}
          />
        )}
      </Card>
    </div>
  );
}

type Cat = { id: string; name: string; enabled: boolean; deleted?: boolean };
type Dish = { id: string; name: string; enabled: boolean; skus?: { price_cents: number }[] };

export function Menu({ storeId }: { storeId?: string }) {
  const cats = useLoad(() => api.get<{ items: Cat[] }>("/api/v1/admin/categories?include_deleted=1", storeId), [storeId]);
  const dishes = useLoad(() => api.get<{ items: Dish[] }>("/api/v1/admin/dishes?include_deleted=1", storeId), [storeId]);
  const [name, setName] = useState("");
  const [dishName, setDishName] = useState("");
  const [catId, setCatId] = useState("");
  const [price, setPrice] = useState("12");
  const liveCats = (cats.data?.items || []).filter((c) => !c.deleted);
  const meta = copy("/menu");
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHead title="分类" desc="删除进入 30 天回收站" />
          <Err err={cats.err} />
          <Toolbar>
            <Field label="分类名">
              <Input placeholder="例如：热销 / 饮品" value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <Btn
              onClick={() =>
                act(() => api.send("/api/v1/admin/categories", "POST", { name, enabled: true }, storeId), () => {
                  setName("");
                  cats.reload();
                }, cats.setErr)
              }
            >
              新建分类
            </Btn>
          </Toolbar>
          <DataTable
            rows={cats.data?.items || []}
            rowKey={(c) => c.id}
            emptyTitle="暂无分类"
            columns={[
              { key: "n", title: "名称", render: (c) => c.name },
              {
                key: "st",
                title: "状态",
                render: (c) => (c.deleted ? <Badge>已回收</Badge> : <Badge tone={c.enabled ? "ok" : "neutral"}>{c.enabled ? "启用" : "停用"}</Badge>),
              },
              {
                key: "op",
                title: "操作",
                render: (c) =>
                  c.deleted ? (
                    <Btn variant="secondary" onClick={() => act(() => api.send(`/api/v1/admin/categories/${c.id}/restore`, "POST", {}, storeId), cats.reload, cats.setErr)}>
                      恢复
                    </Btn>
                  ) : (
                    <Btn variant="ghost" className="text-red-600" onClick={() => act(() => api.send(`/api/v1/admin/categories/${c.id}`, "DELETE", undefined, storeId), cats.reload, cats.setErr)}>
                      回收
                    </Btn>
                  ),
              },
            ]}
          />
        </Card>
        <Card>
          <CardHead title="菜品" desc="新建时写入一个默认规格" />
          <Err err={dishes.err} />
          <Toolbar>
            <Field label="分类">
              <Select value={catId} onChange={(e) => setCatId(e.target.value)}>
                <option value="">选择分类</option>
                {liveCats.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="菜品名">
              <Input placeholder="菜品名" value={dishName} onChange={(e) => setDishName(e.target.value)} />
            </Field>
            <Field label="价格（元）">
              <Input className="w-28" placeholder="价格" value={price} onChange={(e) => setPrice(e.target.value)} />
            </Field>
            <Btn
              onClick={() =>
                act(
                  () =>
                    api.send(
                      "/api/v1/admin/dishes",
                      "POST",
                      {
                        name: dishName,
                        category_id: catId,
                        enabled: true,
                        skus: [{ name: "默认", price_cents: Math.round(Number(price) * 100), stock_mode: "UNLIMITED", enabled: true, is_default: true, sort_order: 0 }],
                      },
                      storeId,
                    ),
                  () => {
                    setDishName("");
                    dishes.reload();
                  },
                  dishes.setErr,
                )
              }
            >
              新建菜品
            </Btn>
          </Toolbar>
          <DataTable
            rows={dishes.data?.items || []}
            rowKey={(d) => d.id}
            emptyTitle="暂无菜品"
            columns={[
              { key: "n", title: "名称", render: (d) => d.name },
              { key: "st", title: "售卖", render: (d) => <Badge tone={d.enabled ? "ok" : "neutral"}>{d.enabled ? "在售" : "下架"}</Badge> },
              { key: "p", title: "价格", render: (d) => (d.skus?.[0] ? `¥${fenToYuan(d.skus[0].price_cents)}` : "—") },
            ]}
          />
        </Card>
      </div>
    </div>
  );
}

export function Promos({ storeId, role }: { storeId?: string; role?: string }) {
  const { data, err, reload, setErr } = useLoad(() => api.get<{ items: { id: string; name: string; discount_cents: number }[] }>("/api/v1/admin/coupon-templates", storeId), [storeId]);
  const [name, setName] = useState("");
  const [yuan, setYuan] = useState("5");
  const iso = (d: Date) => d.toISOString();
  const meta = copy("/promos");
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <Card>
        <Err err={err} />
        <Toolbar>
          <Field label="券名">
            <Input placeholder="例如：新客减 5 元" value={name} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field label="减免（元）">
            <Input className="w-28" placeholder="减免元" value={yuan} onChange={(e) => setYuan(e.target.value)} />
          </Field>
          <Btn
            onClick={() => {
              const now = new Date();
              const end = new Date(now.getTime() + 30 * 24 * 3600 * 1000);
              act(
                () =>
                  api.send(
                    "/api/v1/admin/coupon-templates",
                    "POST",
                    { name, discount_cents: Math.round(Number(yuan) * 100), min_spend_cents: 0, scope: "ALL", audience: "ALL", enabled: true, starts_at: iso(now), ends_at: iso(end) },
                    storeId,
                  ),
                () => {
                  setName("");
                  reload();
                },
                setErr,
              );
            }}
          >
            新建模板
          </Btn>
        </Toolbar>
        <DataTable
          rows={data?.items || []}
          rowKey={(t) => t.id}
          emptyTitle="暂无优惠券模板"
          columns={[
            { key: "n", title: "名称", render: (t) => t.name },
            { key: "d", title: "减免", render: (t) => `¥${fenToYuan(t.discount_cents)}` },
            {
              key: "op",
              title: "操作",
              render: (t) =>
                role === "OWNER" ? (
                  <Btn variant="ghost" className="text-red-600" onClick={() => act(() => api.send(`/api/v1/admin/coupon-templates/${t.id}`, "DELETE", undefined, storeId), reload, setErr)}>
                    永久删除
                  </Btn>
                ) : (
                  "—"
                ),
            },
          ]}
        />
      </Card>
    </div>
  );
}

export function Tables({ storeId }: { storeId?: string }) {
  const { data, err, reload, setErr } = useLoad(() => api.get<{ items: { id: string; table_no: string }[] }>("/api/v1/admin/tables", storeId), [storeId]);
  const [no, setNo] = useState("");
  const meta = copy("/tables");
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <Card>
        <Err err={err} />
        <Toolbar>
          <Field label="桌号">
            <Input className="w-40" placeholder="例如 A1" value={no} onChange={(e) => setNo(e.target.value)} />
          </Field>
          <Btn
            onClick={() =>
              act(() => api.send("/api/v1/admin/tables", "POST", { table_no: no, enabled: true }, storeId), () => {
                setNo("");
                reload();
              }, setErr)
            }
          >
            新建桌台
          </Btn>
        </Toolbar>
        <DataTable
          rows={data?.items || []}
          rowKey={(t) => t.id}
          emptyTitle="暂无桌台"
          emptyHint="新建后可轮换堂食小程序码"
          columns={[
            { key: "n", title: "桌号", render: (t) => <span className="font-medium">{t.table_no}</span> },
            {
              key: "op",
              title: "操作",
              render: (t) => (
                <Btn variant="secondary" onClick={() => act(() => api.send(`/api/v1/admin/tables/${t.id}/rotate-token`, "POST", {}, storeId), reload, setErr)}>
                  换码
                </Btn>
              ),
            },
          ]}
        />
      </Card>
    </div>
  );
}

export function StoreSettings({ storeId }: { storeId?: string }) {
  const { data, err, reload, setErr, loading } = useLoad(() => api.get<Record<string, unknown>>("/api/v1/admin/store", storeId), [storeId]);
  const [ann, setAnn] = useState("");
  useEffect(() => {
    if (data?.announcement) setAnn(String(data.announcement));
  }, [data]);
  const meta = copy("/store");
  if (err && !data) {
    return (
      <div>
        <PageHeader title={meta.title} desc={meta.desc} />
        <Err err={err} />
      </div>
    );
  }
  if (loading && !data) return <Loading />;
  if (!data) return null;
  return (
    <div>
      <PageHeader title={String(data.name || meta.title)} desc={meta.desc} extra={<Badge tone={data.is_open === false ? "warn" : "ok"}>{data.is_open === false ? "休息中" : "营业中"}</Badge>} />
      <Card className="max-w-2xl">
        <CardHead title="门店公告" desc="展示在顾客小程序首页" />
        <Err err={err} />
        <Textarea value={ann} onChange={(e) => setAnn(e.target.value)} placeholder="今日供应、休息提醒等" />
        <div className="mt-3">
          <Btn onClick={() => act(() => api.send("/api/v1/admin/store", "PATCH", { announcement: ann }, storeId), reload, setErr)}>保存公告</Btn>
        </div>
      </Card>
    </div>
  );
}

export function Backup({ storeId, role }: { storeId?: string; role?: string }) {
  const [confirm, setConfirm] = useState(false);
  const [msg, setMsg] = useState("");
  const meta = copy("/backup");
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHead title="导出备份" desc="包含基础信息、菜单与小程序配置，不含密钥与订单" />
          <a className="df-btn df-btn-primary inline-flex" href="/api/v1/admin/store/export">
            下载备份文件
          </a>
        </Card>
        <Card>
          <CardHead title="导入覆盖" desc="仅店主可操作" />
          {role === "OWNER" ? (
            <div className="space-y-3">
              <Alert tone="warn">导入会把现有菜单放入回收站并以新 ID 替换，此操作不可撤销。</Alert>
              <label className="flex items-center gap-2 text-sm text-stone-700">
                <input type="checkbox" checked={confirm} onChange={(e) => setConfirm(e.target.checked)} />
                我已确认风险
              </label>
              <input
                type="file"
                disabled={!confirm}
                className="block text-sm text-stone-600 file:mr-3 file:rounded-lg file:border-0 file:bg-stone-100 file:px-3 file:py-1.5"
                onChange={async (e) => {
                  const f = e.target.files?.[0];
                  if (!f || !confirm) return;
                  try {
                    const res = await fetch("/api/v1/admin/store/import", { method: "POST", credentials: "include", headers: { "X-Store-Id": storeId || "" }, body: await f.arrayBuffer() });
                    if (!res.ok) throw new Error((await res.json().catch(() => ({}))).message || res.statusText);
                    setMsg("导入成功");
                  } catch (ex) {
                    setMsg(ex instanceof Error ? ex.message : "导入失败");
                  }
                }}
              />
              {msg && <Alert tone={msg.includes("成功") ? "ok" : "error"}>{msg}</Alert>}
            </div>
          ) : (
            <Empty title="仅店主可导入" hint="店长可以导出备份，但不能覆盖现有菜单" />
          )}
        </Card>
      </div>
    </div>
  );
}

export function PrintPage({ storeId, role }: { storeId?: string; role?: string }) {
  const { data, err, reload, setErr } = useLoad(() => api.get<{ items: { id: string; name: string; sn: string }[] }>("/api/v1/admin/print/printers", storeId), [storeId]);
  const meta = copy("/print");
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <Card>
        <Err err={err} />
        <DataTable
          rows={data?.items || []}
          rowKey={(p) => p.id}
          emptyTitle="暂无打印机"
          emptyHint="绑定商鹏云打印机后可自动出票"
          columns={[
            { key: "n", title: "名称", render: (p) => p.name },
            { key: "sn", title: "序列号", render: (p) => <span className="font-mono text-xs">{p.sn}</span> },
            {
              key: "op",
              title: "操作",
              render: (p) =>
                role === "OWNER" ? (
                  <Btn variant="ghost" className="text-red-600" onClick={() => act(() => api.send(`/api/v1/admin/print/printers/${p.id}`, "DELETE", undefined, storeId), reload, setErr)}>
                    删除
                  </Btn>
                ) : (
                  "—"
                ),
            },
          ]}
        />
      </Card>
    </div>
  );
}

export function Materials({ storeId }: { storeId?: string }) {
  const { data, err, reload, setErr } = useLoad(() => api.get<{ items: { id: string; name: string; unit: string }[] }>("/api/v1/admin/materials", storeId), [storeId]);
  const lists = useLoad(() => api.get<{ items: { id: string; title: string; status: string }[] }>("/api/v1/admin/purchase-lists", storeId), [storeId]);
  const [mName, setMName] = useState("");
  const [unit, setUnit] = useState("份");
  const meta = copy("/materials");
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHead title="物料目录" />
          <Err err={err} />
          <Toolbar>
            <Field label="物料名">
              <Input placeholder="例如：鸡胸" value={mName} onChange={(e) => setMName(e.target.value)} />
            </Field>
            <Field label="单位">
              <Input className="w-20" value={unit} onChange={(e) => setUnit(e.target.value)} />
            </Field>
            <Btn
              onClick={() =>
                act(() => api.send("/api/v1/admin/materials", "POST", { name: mName, unit }, storeId), () => {
                  setMName("");
                  reload();
                }, setErr)
              }
            >
              新建
            </Btn>
          </Toolbar>
          <DataTable
            rows={data?.items || []}
            rowKey={(m) => m.id}
            emptyTitle="暂无物料"
            columns={[
              { key: "n", title: "名称", render: (m) => m.name },
              { key: "u", title: "单位", render: (m) => m.unit },
            ]}
          />
        </Card>
        <Card>
          <CardHead title="采购清单" extra={<Btn onClick={() => act(() => api.send("/api/v1/admin/purchase-lists", "POST", { title: "采购" }, storeId), lists.reload, lists.setErr)}>新建草稿</Btn>} />
          <Err err={lists.err} />
          <DataTable
            rows={lists.data?.items || []}
            rowKey={(l) => l.id}
            emptyTitle="暂无采购清单"
            columns={[
              { key: "t", title: "标题", render: (l) => l.title },
              { key: "s", title: "状态", render: (l) => <Badge tone={statusTone(l.status)}>{statusLabel(l.status)}</Badge> },
            ]}
          />
        </Card>
      </div>
    </div>
  );
}

export function Members({ storeId, role }: { storeId?: string; role?: string }) {
  const { data, err, reload, setErr } = useLoad(
    () => api.get<{ items: { admin_user_id: string; display_name: string; role: string }[] }>("/api/v1/admin/members", storeId),
    [storeId],
  );
  const joins = useLoad(
    () =>
      role === "OWNER"
        ? api.get<{ items: { id: string; applicant_admin_user_id: string; requested_role: string; status: string }[] }>("/api/v1/admin/join-requests", storeId)
        : Promise.resolve({ items: [] }),
    [storeId, role],
  );
  const meta = copy("/members");
  return (
    <div className="space-y-4">
      <PageHeader title={meta.title} desc={meta.desc} />
      <Card>
        <CardHead title="在职成员" />
        <Err err={err} />
        <DataTable
          rows={data?.items || []}
          rowKey={(m) => m.admin_user_id}
          emptyTitle="暂无成员"
          columns={[
            { key: "n", title: "姓名", render: (m) => m.display_name },
            { key: "r", title: "角色", render: (m) => <Badge tone={m.role === "OWNER" ? "brand" : "neutral"}>{roleLabel(m.role)}</Badge> },
            {
              key: "op",
              title: "操作",
              render: (m) =>
                role === "OWNER" ? (
                  <Btn variant="ghost" className="text-red-600" onClick={() => act(() => api.send(`/api/v1/admin/members/${m.admin_user_id}`, "DELETE", undefined, storeId), reload, setErr)}>
                    移除
                  </Btn>
                ) : (
                  "—"
                ),
            },
          ]}
        />
      </Card>
      {role === "OWNER" && (
        <Card>
          <CardHead title="加入申请" />
          <Err err={joins.err} />
          <DataTable
            rows={joins.data?.items || []}
            rowKey={(j) => j.id}
            emptyTitle="暂无申请"
            columns={[
              { key: "u", title: "申请人", render: (j) => <span className="font-mono text-xs">{j.applicant_admin_user_id.slice(0, 8)}</span> },
              { key: "r", title: "申请角色", render: (j) => roleLabel(j.requested_role) },
              { key: "s", title: "状态", render: (j) => <Badge tone={statusTone(j.status)}>{statusLabel(j.status)}</Badge> },
              {
                key: "op",
                title: "操作",
                render: (j) =>
                  j.status === "PENDING" ? (
                    <div className="flex gap-2">
                      <Btn variant="success" onClick={() => act(() => api.send(`/api/v1/admin/join-requests/${j.id}/review`, "POST", { decision: "APPROVED" }, storeId), () => { joins.reload(); reload(); }, joins.setErr)}>
                        通过
                      </Btn>
                      <Btn variant="secondary" onClick={() => act(() => api.send(`/api/v1/admin/join-requests/${j.id}/review`, "POST", { decision: "REJECTED" }, storeId), joins.reload, joins.setErr)}>
                        驳回
                      </Btn>
                    </div>
                  ) : (
                    "—"
                  ),
              },
            ]}
          />
        </Card>
      )}
    </div>
  );
}

export function Security({ storeId }: { storeId?: string }) {
  const { data, err, reload, setErr } = useLoad(() => api.get<Record<string, unknown>>("/api/v1/admin/payment-config", storeId), [storeId]);
  const [secret, setSecret] = useState("");
  const meta = copy("/security");
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHead title="微信支付 / 登录" desc="密钥只写不读，保存后不会回显" />
          <Err err={err} />
          <div className="mb-4 flex gap-2">
            <Badge tone={data?.wechat_login_ready ? "ok" : "warn"}>登录 {data?.wechat_login_ready ? "就绪" : "未就绪"}</Badge>
            <Badge tone={data?.pay_ready ? "ok" : "warn"}>支付 {data?.pay_ready ? "就绪" : "未就绪"}</Badge>
          </div>
          <Field label="AppSecret" hint="留空表示保留现有密钥">
            <Input type="password" placeholder="AppSecret" value={secret} onChange={(e) => setSecret(e.target.value)} />
          </Field>
          <div className="mt-3">
            <Btn onClick={() => act(() => api.send("/api/v1/admin/payment-config", "PUT", { app_secret: secret }, storeId), reload, setErr)}>保存密钥</Btn>
          </div>
        </Card>
        <Card>
          <CardHead title="测试支付" desc="仅开发联调用，生产订单不可用 mock" />
          <Alert tone="warn">开启后订单会永久带测试标记，不能替代真实微信支付。</Alert>
          <div className="mt-4">
            <Btn
              variant="secondary"
              onClick={() => {
                const ok = window.prompt("输入 MOCK 确认") === "MOCK";
                if (ok) act(() => api.send("/api/v1/admin/mock-payment", "PUT", { enabled: true, confirm: "MOCK" }, storeId), reload, setErr);
              }}
            >
              开启 mock 支付
            </Btn>
          </div>
        </Card>
      </div>
    </div>
  );
}

export function PlatformStores() {
  const { data, err, reload, setErr } = useLoad(() => api.get<{ items: { id: string; name: string; enabled: boolean }[] }>("/api/v1/admin/platform/stores"), []);
  const [name, setName] = useState("");
  const [q, setQ] = useState("");
  const rows = (data?.items || []).filter((s) => !q.trim() || s.name.includes(q.trim()) || s.id.includes(q.trim()));
  const meta = copy("/platform/stores");
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <Card>
        <Err err={err} />
        <Toolbar>
          <Field label="检索">
            <Input className="w-56" placeholder="门店名 / ID" value={q} onChange={(e) => setQ(e.target.value)} />
          </Field>
          <Field label="新门店名">
            <Input placeholder="例如：南山店" value={name} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Btn
            onClick={() =>
              act(() => api.send("/api/v1/admin/platform/stores", "POST", { name }), () => {
                setName("");
                reload();
              }, setErr)
            }
          >
            创建门店
          </Btn>
        </Toolbar>
        <DataTable
          rows={rows}
          rowKey={(s) => s.id}
          emptyTitle="暂无门店"
          columns={[
            { key: "n", title: "名称", render: (s) => <span className="font-medium">{s.name}</span> },
            { key: "st", title: "状态", render: (s) => <Badge tone={s.enabled ? "ok" : "neutral"}>{s.enabled ? "启用" : "停用"}</Badge> },
            { key: "id", title: "门店 ID", render: (s) => <span className="break-all font-mono text-xs text-stone-400">{s.id}</span> },
            {
              key: "op",
              title: "操作",
              render: (s) => (
                <Btn variant="secondary" onClick={() => act(() => api.send(`/api/v1/admin/platform/stores/${s.id}`, "PATCH", { enabled: !s.enabled }), reload, setErr)}>
                  {s.enabled ? "停用" : "启用"}
                </Btn>
              ),
            },
          ]}
        />
      </Card>
    </div>
  );
}

export function PlatformUsers() {
  const stores = useLoad(() => api.get<{ items: { id: string; name: string }[] }>("/api/v1/admin/platform/stores"), []);
  const { data, err, reload, setErr } = useLoad(
    () => api.get<{ items: { id: string; login_name: string; display_name: string; store_id?: string; role?: string; is_platform_admin?: boolean }[] }>("/api/v1/admin/platform/users"),
    [],
  );
  const [login, setLogin] = useState("");
  const [display, setDisplay] = useState("");
  const [password, setPassword] = useState("");
  const [storeId, setStoreId] = useState("");
  const [q, setQ] = useState("");
  const storeName = (id?: string) => stores.data?.items.find((s) => s.id === id)?.name || id || "";
  const rows = (data?.items || []).filter((u) => !q.trim() || [u.login_name, u.display_name, u.store_id].some((x) => String(x || "").includes(q.trim())));
  const meta = copy("/platform/users");
  return (
    <div className="space-y-4">
      <PageHeader title={meta.title} desc={meta.desc} />
      <Card>
        <CardHead title="创建账号" desc="初始密码不少于 12 位；平台账号不能绑定门店" />
        <Err err={err} />
        <div className="grid gap-3 md:grid-cols-3">
          <Field label="登录名">
            <Input placeholder="login_name" value={login} onChange={(e) => setLogin(e.target.value)} />
          </Field>
          <Field label="显示名">
            <Input placeholder="例如：张店长" value={display} onChange={(e) => setDisplay(e.target.value)} />
          </Field>
          <Field label="初始密码">
            <Input type="password" placeholder="12 位以上" value={password} onChange={(e) => setPassword(e.target.value)} />
          </Field>
        </div>
        <div className="mt-3">
          <Btn
            onClick={() =>
              act(() => api.send("/api/v1/admin/platform/users", "POST", { login_name: login, display_name: display, password, enabled: true }), () => {
                setLogin("");
                setDisplay("");
                setPassword("");
                reload();
              }, setErr)
            }
          >
            创建账号
          </Btn>
        </div>
      </Card>
      <Card>
        <CardHead title="账号列表" />
        <Toolbar>
          <Field label="检索">
            <Input className="w-52" placeholder="登录名 / 显示名" value={q} onChange={(e) => setQ(e.target.value)} />
          </Field>
          <Field label="指定店主时的门店">
            <Select className="min-w-[180px]" value={storeId} onChange={(e) => setStoreId(e.target.value)}>
              <option value="">选择门店</option>
              {(stores.data?.items || []).map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </Select>
          </Field>
        </Toolbar>
        <DataTable
          rows={rows}
          rowKey={(u) => u.id}
          emptyTitle="暂无账号"
          columns={[
            { key: "l", title: "登录名", render: (u) => <span className="font-medium">{u.login_name}</span> },
            { key: "d", title: "显示名", render: (u) => u.display_name },
            {
              key: "r",
              title: "身份",
              render: (u) =>
                u.is_platform_admin ? (
                  <Badge tone="brand">平台</Badge>
                ) : u.role ? (
                  <span>
                    <Badge>{roleLabel(u.role)}</Badge>
                    <span className="ml-2 text-xs text-stone-400">{storeName(u.store_id)}</span>
                  </span>
                ) : (
                  <Badge tone="warn">未绑定门店</Badge>
                ),
            },
            {
              key: "op",
              title: "操作",
              render: (u) =>
                u.is_platform_admin ? (
                  "—"
                ) : (
                  <Btn variant="secondary" onClick={() => act(() => api.send(`/api/v1/admin/platform/users/${u.id}/assign-store-owner`, "POST", { store_id: storeId }), reload, setErr)}>
                    设为店主
                  </Btn>
                ),
            },
          ]}
        />
      </Card>
    </div>
  );
}

export function PlatformApps() {
  const { data, err, reload, setErr } = useLoad(() => api.get<{ items: { id: string; store_name: string; status: string }[] }>("/api/v1/admin/platform/shop-applications"), []);
  const meta = copy("/platform/apps");
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <Card>
        <Err err={err} />
        <DataTable
          rows={data?.items || []}
          rowKey={(a) => a.id}
          emptyTitle="暂无开店申请"
          columns={[
            { key: "n", title: "拟开门店", render: (a) => <span className="font-medium">{a.store_name}</span> },
            { key: "s", title: "状态", render: (a) => <Badge tone={statusTone(a.status)}>{statusLabel(a.status)}</Badge> },
            {
              key: "op",
              title: "操作",
              render: (a) =>
                a.status === "PENDING" ? (
                  <div className="flex gap-2">
                    <Btn variant="success" onClick={() => act(() => api.send(`/api/v1/admin/platform/shop-applications/${a.id}/review`, "POST", { decision: "APPROVED" }), reload, setErr)}>
                      通过
                    </Btn>
                    <Btn variant="secondary" onClick={() => act(() => api.send(`/api/v1/admin/platform/shop-applications/${a.id}/review`, "POST", { decision: "REJECTED" }), reload, setErr)}>
                      驳回
                    </Btn>
                  </div>
                ) : (
                  "—"
                ),
            },
          ]}
        />
      </Card>
    </div>
  );
}

export function JoinPage() {
  const [name, setName] = useState("");
  const [contact, setContact] = useState("");
  const [storeId, setStoreId] = useState("");
  const [role, setRole] = useState("STAFF");
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");
  const meta = copy("/join");
  return (
    <div>
      <PageHeader title={meta.title} desc={meta.desc} />
      <Err err={err} />
      {msg && (
        <div className="mb-3">
          <Alert tone="ok">{msg}</Alert>
        </div>
      )}
      <div className="grid max-w-4xl gap-4 md:grid-cols-2">
        <Card>
          <CardHead title="申请开店" desc="平台通过后会创建门店并把你设为店主" />
          <div className="space-y-3">
            <Field label="拟开门店名">
              <Input placeholder="门店名称" value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <Field label="联系方式">
              <Input placeholder="手机或微信" value={contact} onChange={(e) => setContact(e.target.value)} />
            </Field>
            <Btn
              onClick={() =>
                act(
                  async () => {
                    await api.send("/api/v1/admin/shop-applications", "POST", { store_name: name, contact });
                    setMsg("已提交开店申请");
                  },
                  undefined,
                  setErr,
                )
              }
            >
              提交开店申请
            </Btn>
          </div>
        </Card>
        <Card>
          <CardHead title="申请加入门店" desc="由该店店主审批后生效" />
          <div className="space-y-3">
            <Field label="门店 ID">
              <Input placeholder="store_id" value={storeId} onChange={(e) => setStoreId(e.target.value)} />
            </Field>
            <Field label="申请角色">
              <Select value={role} onChange={(e) => setRole(e.target.value)}>
                <option value="STAFF">店员</option>
                <option value="MANAGER">店长</option>
                <option value="OWNER">店主</option>
              </Select>
            </Field>
            <Btn
              variant="secondary"
              onClick={() =>
                act(
                  async () => {
                    await api.send("/api/v1/admin/shop-join-requests", "POST", { store_id: storeId, requested_role: role });
                    setMsg("已提交加入申请");
                  },
                  undefined,
                  setErr,
                )
              }
            >
              提交加入申请
            </Btn>
          </div>
        </Card>
      </div>
    </div>
  );
}
