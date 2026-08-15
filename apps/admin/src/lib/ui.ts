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
    PENDING: "待处理",
    APPROVED: "已通过",
    REJECTED: "已驳回",
    DRAFT: "草稿",
    SUBMITTED: "已提交",
    PRINTED: "已打印",
    DONE: "已完成",
    VOID: "已作废",
  };
  return map[status] ?? status;
}

export type Role = "STAFF" | "MANAGER" | "OWNER" | "PLATFORM";

export type NavItem = { to: string; label: string; min?: Role; icon?: string; group?: string };

export const NAV: NavItem[] = [
  { to: "/board", label: "订单工作台", min: "STAFF", icon: "board", group: "履约" },
  { to: "/orders", label: "历史订单", min: "MANAGER", icon: "orders", group: "履约" },
  { to: "/refunds", label: "退款异常", min: "MANAGER", icon: "refunds", group: "履约" },
  { to: "/analytics", label: "经营分析", min: "MANAGER", icon: "analytics", group: "经营" },
  { to: "/customers", label: "客户会员", min: "MANAGER", icon: "customers", group: "经营" },
  { to: "/menu", label: "菜单定价", min: "MANAGER", icon: "menu", group: "商品" },
  { to: "/promos", label: "优惠", min: "MANAGER", icon: "promo", group: "商品" },
  { to: "/materials", label: "物料买菜", min: "STAFF", icon: "materials", group: "商品" },
  { to: "/tables", label: "桌台", min: "MANAGER", icon: "tables", group: "门店" },
  { to: "/store", label: "门店设置", min: "MANAGER", icon: "store", group: "门店" },
  { to: "/print", label: "云打印", min: "MANAGER", icon: "print", group: "门店" },
  { to: "/members", label: "门店成员", min: "MANAGER", icon: "members", group: "门店" },
  { to: "/backup", label: "备份导入", min: "MANAGER", icon: "backup", group: "门店" },
  { to: "/security", label: "安全中心", min: "OWNER", icon: "security", group: "门店" },
];

export const PLATFORM_NAV: NavItem[] = [
  { to: "/platform/stores", label: "门店", icon: "stores", group: "平台" },
  { to: "/platform/users", label: "账号", icon: "users", group: "平台" },
  { to: "/platform/apps", label: "开店申请", icon: "apps", group: "平台" },
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

export function roleLabel(role: string): string {
  return ({ STAFF: "店员", MANAGER: "店长", OWNER: "店主", PLATFORM: "平台管理员" } as Record<string, string>)[role] ?? role;
}

export function pickupLabel(type: string): string {
  return ({ DINE_IN: "堂食", TAKEOUT: "自取", SCHEDULED: "预约自取" } as Record<string, string>)[type] ?? type;
}

export type Tone = "neutral" | "ok" | "warn" | "danger" | "brand" | "info";

export function statusTone(status: string): Tone {
  const map: Record<string, Tone> = {
    PAID: "brand",
    ACCEPTED: "info",
    PREPARING: "warn",
    READY: "ok",
    COMPLETED: "ok",
    CANCELLED: "neutral",
    REFUNDING: "warn",
    REFUNDED: "neutral",
    PENDING_PAYMENT: "warn",
    CANCEL_REQUESTED: "warn",
    PENDING: "warn",
    APPROVED: "ok",
    REJECTED: "danger",
    ENABLED: "ok",
    DRAFT: "neutral",
    SUBMITTED: "info",
    PRINTED: "info",
    DONE: "ok",
    VOID: "neutral",
  };
  return map[status] ?? "neutral";
}

const GROUP_ORDER = ["履约", "经营", "商品", "门店", "平台"];

export function groupNav(items: NavItem[]): { name: string; items: NavItem[] }[] {
  const map = new Map<string, NavItem[]>();
  for (const it of items) {
    const name = it.group || "";
    const list = map.get(name) || [];
    list.push(it);
    map.set(name, list);
  }
  const names = [...GROUP_ORDER.filter((g) => map.has(g)), ...[...map.keys()].filter((g) => !GROUP_ORDER.includes(g))];
  return names.map((name) => ({ name, items: map.get(name)! }));
}

export const PAGE_COPY: Record<string, { title: string; desc: string }> = {
  "/board": { title: "订单工作台", desc: "按履约进度接单、制作、出餐，约 3 秒自动刷新" },
  "/orders": { title: "历史订单", desc: "检索已发生订单，并按当前列表导出 CSV" },
  "/refunds": { title: "退款与异常", desc: "审核顾客取消退款，并补偿支付查单失败" },
  "/analytics": { title: "经营分析", desc: "净收入、支付订单与客单价总览" },
  "/customers": { title: "客户会员", desc: "会员号、脱敏手机与积分余额" },
  "/menu": { title: "菜单定价", desc: "分类、菜品与默认规格价格" },
  "/promos": { title: "优惠券", desc: "满减券模板，店主可永久删除未发放模板" },
  "/tables": { title: "桌台", desc: "维护桌号，并轮换堂食小程序码" },
  "/store": { title: "门店设置", desc: "公告与基础营业信息" },
  "/backup": { title: "备份导入", desc: "导出菜单与配置；导入会替换现有菜单" },
  "/print": { title: "云打印", desc: "商鹏云打印机与出票设备" },
  "/materials": { title: "物料买菜", desc: "物料目录与采购清单草稿" },
  "/members": { title: "门店成员", desc: "店员角色与加入申请审批" },
  "/security": { title: "安全中心", desc: "微信支付密钥与测试支付开关" },
  "/join": { title: "开店与加入", desc: "提交开店申请，或凭门店 ID 申请加入" },
  "/platform/stores": { title: "门店", desc: "创建、启停平台下的餐饮门店" },
  "/platform/users": { title: "后台账号", desc: "创建登录账号，并为普通账号指定唯一店主" },
  "/platform/apps": { title: "开店申请", desc: "审批拟开门店，通过后创建门店并指定店主" },
};
