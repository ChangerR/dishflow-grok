import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";
import type { Tone } from "../lib/ui";

export function cx(...xs: Array<string | false | null | undefined>) {
  return xs.filter(Boolean).join(" ");
}

export function Icon({ name, className }: { name: string; className?: string }) {
  return (
    <svg className={className} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      {ICONS[name] || ICONS.board}
    </svg>
  );
}

const ICONS: Record<string, ReactNode> = {
  board: (
    <>
      <rect x="3" y="4" width="5.5" height="16" rx="1.2" />
      <rect x="9.25" y="4" width="5.5" height="11" rx="1.2" />
      <rect x="15.5" y="4" width="5.5" height="14" rx="1.2" />
    </>
  ),
  orders: (
    <>
      <path d="M8 6h13M8 12h13M8 18h13" />
      <circle cx="4" cy="6" r=".8" fill="currentColor" />
      <circle cx="4" cy="12" r=".8" fill="currentColor" />
      <circle cx="4" cy="18" r=".8" fill="currentColor" />
    </>
  ),
  refunds: (
    <>
      <path d="M3 12a9 9 0 1 0 3-6.7" />
      <path d="M3 4v5h5" />
    </>
  ),
  analytics: <path d="M4 19V9m6 10V5m6 14v-7m6 7H2" />,
  customers: (
    <>
      <circle cx="9" cy="8" r="3" />
      <path d="M3 19a6 6 0 0 1 12 0" />
      <circle cx="17" cy="9" r="2.2" />
      <path d="M21 19a4.5 4.5 0 0 0-4-4.4" />
    </>
  ),
  menu: (
    <>
      <path d="M4 4h7v16H4zM15 8h5M15 12h5M15 16h3" />
    </>
  ),
  promo: (
    <>
      <path d="M3 10.5 10.5 3h7.2L21 6.3v7.2L13.5 21 3 10.5z" />
      <circle cx="15" cy="8" r="1.2" />
    </>
  ),
  tables: (
    <>
      <rect x="3" y="3" width="7" height="7" rx="1.2" />
      <rect x="14" y="3" width="7" height="7" rx="1.2" />
      <rect x="3" y="14" width="7" height="7" rx="1.2" />
      <rect x="14" y="14" width="7" height="7" rx="1.2" />
    </>
  ),
  store: (
    <>
      <path d="M3 10 5 4h14l2 6" />
      <path d="M4 10v10h16V10" />
      <path d="M10 20v-6h4v6" />
    </>
  ),
  backup: (
    <>
      <path d="M12 3v12" />
      <path d="m8 11 4 4 4-4" />
      <path d="M4 19h16" />
    </>
  ),
  print: (
    <>
      <rect x="6" y="3" width="12" height="6" rx="1" />
      <path d="M6 14H4a1 1 0 0 1-1-1v-3a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v3a1 1 0 0 1-1 1h-2" />
      <rect x="6" y="13" width="12" height="8" rx="1" />
    </>
  ),
  materials: (
    <>
      <path d="M3 8.5 12 4l9 4.5-9 4.5L3 8.5z" />
      <path d="M3 8.5v7L12 20" />
      <path d="M21 8.5v7L12 20" />
    </>
  ),
  members: (
    <>
      <circle cx="12" cy="8" r="3" />
      <path d="M5 19a7 7 0 0 1 14 0" />
    </>
  ),
  security: (
    <>
      <path d="M12 3 5 6.5v5.2c0 4.2 3 7.4 7 8.8 4-1.4 7-4.6 7-8.8V6.5L12 3z" />
      <path d="m9 12 2 2 4-4" />
    </>
  ),
  stores: (
    <>
      <path d="M3 21V8l4-4h10l4 4v13" />
      <path d="M9 21v-7h6v7" />
    </>
  ),
  users: (
    <>
      <circle cx="12" cy="8" r="3" />
      <path d="M4 20a8 8 0 0 1 16 0" />
    </>
  ),
  apps: (
    <>
      <rect x="5" y="3" width="14" height="18" rx="2" />
      <path d="M9 8h6M9 12h6M9 16h4" />
    </>
  ),
  join: (
    <>
      <path d="M10 8V6a2 2 0 0 1 2-2h7v16h-7a2 2 0 0 1-2-2v-2" />
      <path d="M3 12h11" />
      <path d="m10 9 4 3-4 3" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="6.5" />
      <path d="m20 20-3.2-3.2" />
    </>
  ),
  logout: (
    <>
      <path d="M10 8V6a2 2 0 0 1 2-2h7v16h-7a2 2 0 0 1-2-2v-2" />
      <path d="M3 12h11" />
      <path d="m7 8-4 4 4 4" />
    </>
  ),
  menuFold: (
    <>
      <path d="M4 6h16M4 12h16M4 18h16" />
    </>
  ),
};

export function PageHeader({ title, desc, extra }: { title: string; desc?: string; extra?: ReactNode }) {
  return (
    <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 className="text-[22px] font-semibold tracking-tight text-stone-900">{title}</h1>
        {desc && <p className="mt-1 max-w-2xl text-sm leading-6 text-stone-500">{desc}</p>}
      </div>
      {extra && <div className="flex flex-wrap items-center gap-2">{extra}</div>}
    </div>
  );
}

export function Card({ children, className, padded = true }: { children: ReactNode; className?: string; padded?: boolean }) {
  return <section className={cx("df-card", padded && "p-5", className)}>{children}</section>;
}

export function CardHead({ title, desc, extra }: { title: string; desc?: string; extra?: ReactNode }) {
  return (
    <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 className="text-[15px] font-semibold text-stone-900">{title}</h2>
        {desc && <p className="mt-0.5 text-xs leading-5 text-stone-500">{desc}</p>}
      </div>
      {extra}
    </div>
  );
}

type BtnVariant = "primary" | "secondary" | "ghost" | "danger" | "success";

export function Btn({ variant = "primary", className, type = "button", ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: BtnVariant }) {
  return <button type={type} className={cx("df-btn", `df-btn-${variant}`, className)} {...props} />;
}

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={cx("df-input", className)} {...props} />;
}

export function Select({ className, ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className={cx("df-input", className)} {...props} />;
}

export function Textarea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={cx("df-input min-h-[108px] resize-y", className)} {...props} />;
}

export function Field({ label, children, hint }: { label: string; children: ReactNode; hint?: string }) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-xs font-medium text-stone-600">{label}</span>
      {children}
      {hint && <span className="mt-1 block text-xs text-stone-400">{hint}</span>}
    </label>
  );
}

export function Empty({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="grid place-items-center py-12 text-center">
      <div className="mb-3 grid h-12 w-12 place-items-center rounded-full bg-stone-100 text-stone-400">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6">
          <rect x="3" y="4" width="18" height="16" rx="2" />
          <path d="M8 9h8M8 13h5" />
        </svg>
      </div>
      <p className="text-sm font-medium text-stone-600">{title}</p>
      {hint && <p className="mt-1 max-w-xs text-xs leading-5 text-stone-400">{hint}</p>}
    </div>
  );
}

export function Alert({ tone = "error", children }: { tone?: "error" | "warn" | "ok"; children: ReactNode }) {
  const map = {
    error: "bg-red-50 text-red-800 border-red-100",
    warn: "bg-amber-50 text-amber-950 border-amber-100",
    ok: "bg-emerald-50 text-emerald-800 border-emerald-100",
  };
  return <div className={cx("rounded-xl border px-3.5 py-2.5 text-sm leading-6", map[tone])}>{children}</div>;
}

export function Badge({ tone = "neutral", children }: { tone?: Tone; children: ReactNode }) {
  const map: Record<Tone, string> = {
    neutral: "bg-stone-100 text-stone-600",
    ok: "bg-emerald-50 text-emerald-700",
    warn: "bg-amber-50 text-amber-800",
    danger: "bg-red-50 text-red-700",
    brand: "bg-orange-50 text-orange-800",
    info: "bg-sky-50 text-sky-800",
  };
  return <span className={cx("inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium", map[tone])}>{children}</span>;
}

export function Loading({ label = "加载中…" }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-2 py-16 text-sm text-stone-500">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-stone-300 border-t-brand" />
      {label}
    </div>
  );
}

export function Toolbar({ children }: { children: ReactNode }) {
  return <div className="mb-4 flex flex-wrap items-end gap-2 rounded-xl bg-stone-50 p-3">{children}</div>;
}

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  emptyTitle,
  emptyHint,
}: {
  columns: { key: string; title: string; className?: string; render: (row: T) => ReactNode }[];
  rows: T[];
  rowKey: (row: T) => string;
  emptyTitle?: string;
  emptyHint?: string;
}) {
  if (!rows.length) return <Empty title={emptyTitle || "暂无数据"} hint={emptyHint} />;
  return (
    <div className="overflow-x-auto">
      <table className="df-table">
        <thead>
          <tr>
            {columns.map((c) => (
              <th key={c.key} className={c.className}>
                {c.title}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={rowKey(r)}>
              {columns.map((c) => (
                <td key={c.key} className={c.className}>
                  {c.render(r)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
