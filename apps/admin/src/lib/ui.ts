export function fenToYuan(cents: number): string {
  return (cents / 100).toFixed(2);
}

export function statusLabel(status: string): string {
  const map: Record<string, string> = {
    PENDING_PAYMENT: "待支付",
    PAID: "待接单",
    ACCEPTED: "已接单",
    PREPARING: "制作中",
    READY: "待取餐",
    COMPLETED: "已完成",
    CANCELLED: "已取消",
    CANCEL_REQUESTED: "取消待审",
    REFUNDING: "退款中",
    REFUNDED: "已退款",
  };
  return map[status] ?? status;
}

export type Role = "STAFF" | "MANAGER" | "OWNER" | "PLATFORM";

export type NavItem = { to: string; label: string; min?: Role };

export const NAV: NavItem[] = [
  { to: "/board", label: "订单工作台", min: "STAFF" },
  { to: "/orders", label: "历史订单", min: "MANAGER" },
  { to: "/refunds", label: "退款异常", min: "MANAGER" },
  { to: "/analytics", label: "经营分析", min: "MANAGER" },
  { to: "/customers", label: "客户会员", min: "MANAGER" },
  { to: "/menu", label: "菜单定价", min: "MANAGER" },
  { to: "/promos", label: "优惠", min: "MANAGER" },
  { to: "/tables", label: "桌台", min: "MANAGER" },
  { to: "/store", label: "门店设置", min: "MANAGER" },
  { to: "/backup", label: "备份导入", min: "MANAGER" },
  { to: "/print", label: "云打印", min: "MANAGER" },
  { to: "/materials", label: "物料买菜", min: "STAFF" },
  { to: "/members", label: "门店成员", min: "MANAGER" },
  { to: "/security", label: "安全中心", min: "OWNER" },
];

const rank: Record<Role, number> = { STAFF: 1, MANAGER: 2, OWNER: 3, PLATFORM: 0 };

export function canSee(role: Role | undefined, item: NavItem): boolean {
  if (!role || role === "PLATFORM") return false;
  if (!item.min) return true;
  return rank[role] >= rank[item.min];
}

export function nextTransition(status: string): string | null {
  if (status === "PAID") return "ACCEPTED";
  if (status === "ACCEPTED") return "PREPARING";
  if (status === "PREPARING") return "READY";
  if (status === "READY") return "COMPLETED";
  return null;
}

export function transitionLabel(to: string): string {
  return ({ ACCEPTED: "接单", PREPARING: "开始制作", READY: "出餐", COMPLETED: "完成" } as Record<string, string>)[to] ?? to;
}

export function isOverdue(anchorISO: string | undefined, pickupMinutes: number, nowMs = Date.now()): boolean {
  if (!anchorISO || pickupMinutes <= 0) return false;
  const t = Date.parse(anchorISO);
  if (Number.isNaN(t)) return false;
  return nowMs - t > pickupMinutes * 60_000;
}

export function toRFC3339(local: string): string {
  if (!local) return "";
  const d = new Date(local);
  if (Number.isNaN(d.getTime())) return local;
  return d.toISOString();
}
