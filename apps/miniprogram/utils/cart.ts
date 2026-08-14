export type CartItem = { sku_id: string; option_ids: string[]; qty: number };

export function cartKey(item: CartItem): string {
  return `${item.sku_id}|${[...item.option_ids].sort().join(",")}`;
}

export function mergeCart(items: CartItem[]): CartItem[] {
  const map = new Map<string, CartItem>();
  for (const it of items) {
    if (it.qty <= 0) continue;
    const key = cartKey(it);
    const cur = map.get(key);
    if (cur) cur.qty = Math.min(99, cur.qty + it.qty);
    else map.set(key, { sku_id: it.sku_id, option_ids: [...it.option_ids].sort(), qty: Math.min(99, it.qty) });
  }
  return [...map.values()];
}

export function pruneCart(items: CartItem[], validSkuIds: Set<string>, invalid: string[] = []): { kept: CartItem[]; removed: CartItem[] } {
  const kept: CartItem[] = [];
  const removed: CartItem[] = [];
  for (const it of items) {
    if (!validSkuIds.has(it.sku_id) || invalid.includes(it.sku_id)) removed.push(it);
    else kept.push(it);
  }
  return { kept, removed };
}

export function fenToYuan(cents: number): string {
  return (cents / 100).toFixed(2);
}

export function canCancel(status: string): boolean {
  return ["PENDING_PAYMENT", "PAID", "ACCEPTED", "CANCEL_REQUESTED", "REFUNDING"].includes(status);
}

export function orderTab(status: string): string {
  if (status === "PENDING_PAYMENT") return "PENDING_PAYMENT";
  if (["PAID", "ACCEPTED", "PREPARING"].includes(status)) return "PREPARING";
  if (status === "READY") return "READY";
  if (status === "COMPLETED") return "COMPLETED";
  return "CANCELLED";
}
