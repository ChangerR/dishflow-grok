import { useEffect, useMemo, useState } from "react";
import { Link, Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { api, type Principal } from "./lib/api";
import { NAV, canSee, type Role } from "./lib/ui";
import { Pages } from "./pages/Pages";

export default function App() {
  const [me, setMe] = useState<Principal | null | undefined>(undefined);
  useEffect(() => {
    api.me().then(setMe).catch(() => setMe(null));
  }, []);
  if (me === undefined) return <div className="p-8 text-stone-500">加载中…</div>;
  return (
    <Routes>
      <Route path="/login" element={<Login onOk={setMe} />} />
      <Route path="/*" element={me ? <Shell me={me} onLogout={() => setMe(null)} /> : <Navigate to="/login" replace />} />
    </Routes>
  );
}

function Login({ onOk }: { onOk: (p: Principal) => void }) {
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState("");
  const nav = useNavigate();
  return (
    <form
      className="mx-auto mt-24 max-w-sm space-y-3 rounded-xl bg-white p-6 shadow"
      onSubmit={async (e) => {
        e.preventDefault();
        setErr("");
        try {
          const p = await api.login(login, password);
          onOk(p);
          nav("/");
        } catch (ex) {
          setErr(ex instanceof Error ? ex.message : "登录失败");
        }
      }}
    >
      <h1 className="text-xl font-semibold">DishFlow 管理后台</h1>
      <input className="w-full rounded border px-3 py-2" placeholder="账号" value={login} onChange={(e) => setLogin(e.target.value)} />
      <input className="w-full rounded border px-3 py-2" type="password" placeholder="密码" value={password} onChange={(e) => setPassword(e.target.value)} />
      {err && <p className="text-sm text-red-600">{err}</p>}
      <button className="w-full rounded bg-brand py-2 text-white">登录</button>
    </form>
  );
}

function Shell({ me, onLogout }: { me: Principal; onLogout: () => void }) {
  const [open, setOpen] = useState(false);
  const loc = useLocation();
  const role: Role = me.is_platform_admin ? "PLATFORM" : ((me.role as Role) || "STAFF");
  const items = useMemo(() => (role === "PLATFORM" ? [
    { to: "/platform/stores", label: "门店" },
    { to: "/platform/users", label: "账号" },
    { to: "/platform/apps", label: "开店申请" },
  ] : !me.store_id ? [{ to: "/join", label: "开店与加入" }] : NAV.filter((n) => canSee(role, n))), [role, me.store_id]);
  const home = role === "PLATFORM" ? "/platform/stores" : me.store_id ? "/board" : "/join";
  return (
    <div className="min-h-screen md:flex">
      <aside className={`fixed inset-y-0 z-20 w-64 bg-stone-900 text-white md:static ${open ? "block" : "hidden md:block"}`}>
        <div className="flex items-center justify-between p-4 font-semibold">
          DishFlow
          <button className="md:hidden" onClick={() => setOpen(false)}>关闭</button>
        </div>
        <nav className="space-y-1 px-2 pb-8">
          {items.map((it) => (
            <Link key={it.to} to={it.to} className={`block rounded px-3 py-2 text-sm ${loc.pathname.startsWith(it.to) ? "bg-brand" : "hover:bg-stone-800"}`} onClick={() => setOpen(false)}>
              {it.label}
            </Link>
          ))}
        </nav>
      </aside>
      <div className="flex-1">
        <header className="flex items-center justify-between border-b bg-white px-4 py-3">
          <button className="md:hidden" onClick={() => setOpen(true)}>菜单</button>
          <div className="text-sm">
            {me.store_name || (me.is_platform_admin ? "平台" : "未加入门店")} · {me.display_name} · {role}
            {me.store_open === false && <span className="ml-2 text-amber-700">休息中</span>}
          </div>
          <button
            className="text-sm text-stone-500"
            onClick={async () => {
              await api.logout();
              onLogout();
            }}
          >
            退出
          </button>
        </header>
        {(me.mock_payment || me.mock_print) && (
          <div className="bg-amber-100 px-4 py-2 text-sm text-amber-900">当前门店启用了测试支付/模拟打印，订单将带测试标记，不能替代生产。</div>
        )}
        <main className="p-4">
          <Routes>
            <Route path="/" element={<Navigate to={home} replace />} />
            <Route path="/join" element={<JoinPage />} />
            <Route path="/*" element={<Pages me={me} />} />
          </Routes>
        </main>
      </div>
    </div>
  );
}

function JoinPage() {
  const [name, setName] = useState("");
  const [contact, setContact] = useState("");
  const [storeId, setStoreId] = useState("");
  const [role, setRole] = useState("STAFF");
  const [msg, setMsg] = useState("");
  return (
    <div className="grid max-w-xl gap-6">
      <section className="rounded-xl bg-white p-4 shadow-sm">
        <h2 className="mb-2 font-semibold">申请开店</h2>
        <input className="mb-2 w-full rounded border px-3 py-2" placeholder="拟开门店名" value={name} onChange={(e) => setName(e.target.value)} />
        <input className="mb-2 w-full rounded border px-3 py-2" placeholder="联系方式" value={contact} onChange={(e) => setContact(e.target.value)} />
        <button className="rounded bg-brand px-4 py-2 text-white" onClick={async () => { await api.send("/api/v1/admin/shop-applications", "POST", { store_name: name, contact }); setMsg("已提交开店申请"); }}>提交</button>
      </section>
      <section className="rounded-xl bg-white p-4 shadow-sm">
        <h2 className="mb-2 font-semibold">申请加入门店</h2>
        <input className="mb-2 w-full rounded border px-3 py-2" placeholder="门店 ID" value={storeId} onChange={(e) => setStoreId(e.target.value)} />
        <select className="mb-2 w-full rounded border px-3 py-2" value={role} onChange={(e) => setRole(e.target.value)}>
          <option>STAFF</option><option>MANAGER</option><option>OWNER</option>
        </select>
        <button className="rounded bg-brand px-4 py-2 text-white" onClick={async () => { await api.send("/api/v1/admin/shop-join-requests", "POST", { store_id: storeId, requested_role: role }); setMsg("已提交加入申请"); }}>提交</button>
      </section>
      {msg && <p className="text-green-700">{msg}</p>}
    </div>
  );
}
