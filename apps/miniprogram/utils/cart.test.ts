import { describe, expect, it } from "vitest";
import { canCancel, fenToYuan, mergeCart, orderTab, pruneCart } from "./cart";

describe("cart", () => {
  it("merges same sku+options regardless of order", () => {
    const got = mergeCart([
      { sku_id: "s1", option_ids: ["b", "a"], qty: 1 },
      { sku_id: "s1", option_ids: ["a", "b"], qty: 2 },
    ]);
    expect(got).toEqual([{ sku_id: "s1", option_ids: ["a", "b"], qty: 3 }]);
  });
  it("prunes invalid lines and reports them", () => {
    const r = pruneCart(
      [{ sku_id: "s1", option_ids: [], qty: 1 }, { sku_id: "gone", option_ids: [], qty: 1 }],
      new Set(["s1"]),
    );
    expect(r.kept).toHaveLength(1);
    expect(r.removed[0].sku_id).toBe("gone");
  });
  it("formats money and cancel rules", () => {
    expect(fenToYuan(2500)).toBe("25.00");
    expect(canCancel("PREPARING")).toBe(false);
    expect(canCancel("PAID")).toBe(true);
    expect(orderTab("ACCEPTED")).toBe("PREPARING");
  });
});
