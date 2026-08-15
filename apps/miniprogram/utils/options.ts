export type OptionItem = { id: string; name: string; price_cents: number; enabled: boolean; is_default: boolean };
export type OptionGroup = {
  id: string;
  name: string;
  selection_type: "SINGLE" | "MULTI" | string;
  required: boolean;
  min_select: number;
  max_select: number;
  items: OptionItem[];
};
export type Dish = {
  id: string;
  name: string;
  description?: string;
  from_price_cents: number;
  sold_out: boolean;
  option_count: number;
  skus: { id: string; name: string; price_cents: number; remaining?: number; enabled: boolean; is_default: boolean }[];
  option_groups: OptionGroup[];
};
export type Category = { id: string; name: string; dishes: Dish[] };

export function filterMenu(categories: Category[], keyword: string): Category[] {
  const k = keyword.trim().toLowerCase();
  if (!k) return categories;
  return categories
    .map((c) => ({
      ...c,
      dishes: c.dishes.filter((d) => {
        const parts = [d.name, d.description || ""];
        for (const s of d.skus) parts.push(s.name);
        for (const g of d.option_groups || []) {
          parts.push(g.name);
          for (const i of g.items || []) parts.push(i.name);
        }
        return parts.join(" ").toLowerCase().includes(k);
      }),
    }))
    .filter((c) => c.dishes.length > 0);
}

export function defaultOptionIds(groups: OptionGroup[]): string[] {
  const ids: string[] = [];
  for (const g of groups || []) {
    for (const it of g.items || []) {
      if (it.enabled && it.is_default) ids.push(it.id);
    }
  }
  return ids;
}

export function defaultSkuId(dish: Dish): string {
  const def = dish.skus.find((s) => s.enabled && s.is_default) || dish.skus.find((s) => s.enabled);
  return def?.id || "";
}

export function toggleOption(groups: OptionGroup[], selected: string[], groupId: string, optionId: string): string[] {
  const g = groups.find((x) => x.id === groupId);
  if (!g) return selected;
  const inGroup = new Set((g.items || []).map((i) => i.id));
  const rest = selected.filter((id) => !inGroup.has(id));
  const currently = selected.filter((id) => inGroup.has(id));
  if (g.selection_type === "SINGLE") {
    return currently[0] === optionId ? rest : [...rest, optionId];
  }
  if (currently.includes(optionId)) {
    return selected.filter((id) => id !== optionId);
  }
  if (currently.length >= g.max_select) return selected;
  return [...selected, optionId];
}

export function optionsValid(groups: OptionGroup[], selected: string[]): boolean {
  for (const g of groups || []) {
    const inGroup = new Set((g.items || []).filter((i) => i.enabled).map((i) => i.id));
    const n = selected.filter((id) => inGroup.has(id)).length;
    const min = g.required ? Math.max(g.min_select, 1) : g.min_select;
    if (n < min) return false;
    if (g.max_select > 0 && n > g.max_select) return false;
  }
  return true;
}
