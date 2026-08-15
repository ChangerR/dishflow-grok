import { useEffect, useMemo, useState } from "react";
import { Link, Navigate, Outlet, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { Alert, Btn, Field, Icon, Input, Loading, cx } from "./components/kit";
import { api, type Principal } from "./lib/api";
import { NAV, PAGE_COPY, PLATFORM_NAV, canSee, groupNav, roleLabel, type Role } from "./lib/ui";
import {
  Analytics,
  Backup,
  Board,
  Customers,
  JoinPage,
  Materials,
  Members,
  Menu,
  Orders,
  PlatformApps,
  PlatformStores,
  PlatformUsers,
  PrintPage,
  Promos,
  Refunds,
  Security,
  StoreSettings,
  Tables,
} from "./pages/Pages";

function homeOf(me: Principal) {
  if (me.is_platform_admin) return "/platform/stores";
  if (me.store_id) return "/board";
  return "/join";
}

export default function App() {
  const [me, setMe] = useState<Principal | null | undefined>(undefined);
  useEffect(() => {
    api.me().then(setMe).catch(() => setMe(null));
  }, []);
  if (me === undefined) {
    return (
      <div className="grid min-h-screen place-items-center bg-canvas">
        <Loading label="正在进入控制台…" />
      </div>
    );
  }
  return (
    <Routes>
      <Route path="/login" element={<Login onOk={setMe} />} />
      <Route element={me ? <Shell me={me} onLogout={() => setMe(null)} /> : <Navigate to="/login" replace />}>
        <Route path="/" element={<Navigate to={me ? homeOf(me) : "/login"} replace />} />
        <Route path="/join" element={<JoinPage />} />
        <Route path="/board" element={<Board storeId={me?.store_id} />} />
        <Route path="/orders" element={<Orders storeId={me?.store_id} />} />
        <Route path="/refunds" element={<Refunds storeId={me?.store_id} />} />
        <Route path="/analytics" element={<Analytics storeId={me?.store_id} />} />
        <Route path="/customers" element={<Customers storeId={me?.store_id} />} />
        <Route path="/menu" element={<Menu storeId={me?.store_id} />} />
        <Route path="/promos" element={<Promos storeId={me?.store_id} role={me?.role} />} />
        <Route path="/tables" element={<Tables storeId={me?.store_id} />} />
        <Route path="/store" element={<StoreSettings storeId={me?.store_id} />} />
        <Route path="/backup" element={<Backup storeId={me?.store_id} role={me?.role} />} />
        <Route path="/print" element={<PrintPage storeId={me?.store_id} role={me?.role} />} />
        <Route path="/materials" element={<Materials storeId={me?.store_id} />} />
        <Route path="/members" element={<Members storeId={me?.store_id} role={me?.role} />} />
        <Route path="/security" element={<Security storeId={me?.store_id} />} />
        <Route path="/platform/stores" element={<PlatformStores />} />
        <Route path="/platform/users" element={<PlatformUsers />} />
        <Route path="/platform/apps" element={<PlatformApps />} />
      </Route>
    </Routes>
  );
}

function Login({ onOk }: { onOk: (p: Principal) => void }) {
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const nav = useNavigate();
  return (
    <div className="min-h-screen lg:grid lg:grid-cols-[1.08fr_1fr]">
      <aside className="relative hidden overflow-hidden bg-[#1a1613] px-12 py-12 text-white lg:flex lg:flex-col lg:justify-between">
        <div className="pointer-events-none absolute -left-24 -top-24 h-80 w-80 rounded-full bg-brand/30 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-28 right-0 h-72 w-72 rounded-full bg-orange-300/10 blur-3xl" />
        <div className="relative">
          <div className="flex items-center gap-3">
            <span className="grid h-10 w-10 place-items-center rounded-xl bg-brand text-lg font-bold shadow-lg">D</span>
            <div>
              <div className="text-lg font-semibold tracking-tight">DishFlow</div>
              <div className="text-xs text-stone-400">餐饮门店运营控制台</div>
            </div>
          </div>
          <h2 className="mt-16 max-w-md text-4xl font-semibold leading-tight tracking-tight">
            接单、出餐、管菜单
            <span className="mt-2 block text-orange-300">像真正的后厨台面。</span>
          </h2>
          <p className="mt-5 max-w-sm text-sm leading-7 text-stone-400">
            平台管门店与账号，店长管履约与商品。工作台按待接单、制作中、待取餐流转，避免把后台做成填表作业。
          </p>
        </div>
        <ul className="relative grid gap-3 text-sm text-stone-300">
          {["四列履约看板，3 秒刷新", "菜单、优惠、桌台与会员同屏管理", "平台 / 门店权限隔离"].map((t) => (
            <li key={t} className="flex items-center gap-2">
              <span className="h-1.5 w-1.5 rounded-full bg-brand" />
              {t}
            </li>
          ))}
        </ul>
      </aside>
      <main className="grid place-items-center px-6 py-16">
        <form
          className="w-full max-w-[400px]"
          onSubmit={async (e) => {
            e.preventDefault();
            setErr("");
            setBusy(true);
            try {
              const p = await api.login(login, password);
              onOk(p);
              nav(homeOf(p));
            } catch (ex) {
              setErr(ex instanceof Error ? ex.message : "登录失败");
            } finally {
              setBusy(false);
            }
          }}
        >
          <div className="mb-8 lg:hidden">
            <div className="flex items-center gap-3">
              <span className="grid h-10 w-10 place-items-center rounded-xl bg-brand text-lg font-bold text-white">D</span>
              <div>
                <div className="text-lg font-semibold">DishFlow</div>
                <div className="text-xs text-stone-500">门店运营控制台</div>
              </div>
            </div>
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">登录后台</h1>
          <p className="mt-2 text-sm text-stone-500">使用门店或平台账号进入对应工作台</p>
          <div className="mt-8 space-y-4">
            <Field label="账号">
              <Input autoComplete="username" placeholder="登录名" value={login} onChange={(e) => setLogin(e.target.value)} />
            </Field>
            <Field label="密码">
              <Input autoComplete="current-password" type="password" placeholder="密码" value={password} onChange={(e) => setPassword(e.target.value)} />
            </Field>
            {err && <Alert>{err}</Alert>}
            <Btn type="submit" className="w-full py-2.5" disabled={busy}>
              {busy ? "登录中…" : "进入控制台"}
            </Btn>
          </div>
        </form>
      </main>
    </div>
  );
}

function Shell({ me, onLogout }: { me: Principal; onLogout: () => void }) {
  const [open, setOpen] = useState(false);
  const loc = useLocation();
  const role: Role = me.is_platform_admin ? "PLATFORM" : ((me.role as Role) || "STAFF");
  const items = useMemo(
    () =>
      role === "PLATFORM"
        ? PLATFORM_NAV
        : !me.store_id
          ? [{ to: "/join", label: "开店与加入", icon: "join", group: "入驻" }]
          : NAV.filter((n) => canSee(role, n)),
    [role, me.store_id],
  );
  const groups = groupNav(items);
  const copy = PAGE_COPY[loc.pathname];
  return (
    <div className="min-h-screen md:flex">
      {open && <button className="fixed inset-0 z-20 bg-stone-900/40 md:hidden" aria-label="关闭菜单" onClick={() => setOpen(false)} />}
      <aside
        className={cx(
          "fixed inset-y-0 z-30 flex w-[248px] flex-col bg-[#1a1613] text-stone-300 md:static",
          open ? "flex" : "hidden md:flex",
        )}
      >
        <div className="flex items-center gap-3 px-4 py-4">
          <span className="grid h-9 w-9 place-items-center rounded-xl bg-brand text-sm font-bold text-white">D</span>
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-semibold text-white">DishFlow</div>
            <div className="truncate text-[11px] text-stone-500">{me.is_platform_admin ? "平台控制台" : "门店运营"}</div>
          </div>
          <button className="rounded-lg p-1 text-stone-400 hover:bg-white/10 md:hidden" onClick={() => setOpen(false)}>
            ×
          </button>
        </div>
        <nav className="df-scroll flex-1 space-y-5 overflow-y-auto px-3 pb-8">
          {groups.map((g) => (
            <div key={g.name || g.items[0].to}>
              {g.name && <div className="mb-1.5 px-2 text-[11px] font-medium uppercase tracking-wider text-stone-500">{g.name}</div>}
              <div className="space-y-0.5">
                {g.items.map((it) => {
                  const active = loc.pathname === it.to || (it.to !== "/" && loc.pathname.startsWith(it.to));
                  return (
                    <Link
                      key={it.to}
                      to={it.to}
                      onClick={() => setOpen(false)}
                      className={cx(
                        "flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm transition",
                        active ? "bg-brand text-white shadow-sm" : "hover:bg-white/5 hover:text-white",
                      )}
                    >
                      <Icon name={it.icon || "board"} />
                      {it.label}
                    </Link>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-10 flex items-center justify-between gap-3 border-b border-stone-200/80 bg-white/90 px-4 py-3 backdrop-blur">
          <div className="flex min-w-0 items-center gap-3">
            <button className="rounded-lg p-1.5 text-stone-600 hover:bg-stone-100 md:hidden" onClick={() => setOpen(true)} aria-label="打开菜单">
              <Icon name="menuFold" />
            </button>
            <div className="min-w-0">
              <div className="truncate text-sm font-semibold text-stone-900">{copy?.title || "控制台"}</div>
              <div className="truncate text-xs text-stone-500">
                {me.store_name || (me.is_platform_admin ? "平台运营" : "尚未加入门店")}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {me.store_id && (
              <span className={cx("hidden rounded-full px-2.5 py-1 text-[11px] font-medium sm:inline", me.store_open === false ? "bg-amber-50 text-amber-800" : "bg-emerald-50 text-emerald-700")}>
                {me.store_open === false ? "休息中" : "营业中"}
              </span>
            )}
            <div className="hidden items-center gap-2 rounded-full border border-stone-200 bg-stone-50 py-1 pl-1 pr-3 sm:flex">
              <span className="grid h-7 w-7 place-items-center rounded-full bg-brand text-[11px] font-semibold text-white">
                {(me.display_name || me.login_name || "?").slice(0, 1)}
              </span>
              <div className="leading-tight">
                <div className="text-xs font-medium text-stone-800">{me.display_name}</div>
                <div className="text-[10px] text-stone-500">{roleLabel(role)}</div>
              </div>
            </div>
            <Btn
              variant="ghost"
              className="text-stone-500"
              onClick={async () => {
                await api.logout();
                onLogout();
              }}
            >
              <Icon name="logout" />
              退出
            </Btn>
          </div>
        </header>
        {(me.mock_payment || me.mock_print) && (
          <div className="border-b border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-950">
            当前门店启用了{me.mock_payment ? "测试支付" : ""}
            {me.mock_payment && me.mock_print ? " / " : ""}
            {me.mock_print ? "模拟打印" : ""}
            ，订单将带测试标记，不能替代生产。
          </div>
        )}
        <main className="flex-1 p-4 md:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
