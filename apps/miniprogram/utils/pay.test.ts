import { describe, expect, it } from "vitest";
import { defaultOptionIds, filterMenu, optionsValid, toggleOption, type Category } from "./options";
import { payAction, paymentStatusLabel } from "./pay";

const menu: Category[] = [
  {
    id: "c1",
    name: "面",
    dishes: [
      {
        id: "d1",
        name: "牛肉面",
        description: "红烧",
        from_price_cents: 2800,
        sold_out: false,
        option_count: 1,
        skus: [{ id: "s1", name: "标准", price_cents: 2800, enabled: true, is_default: true }],
        option_groups: [
          {
            id: "g1",
            name: "加料",
            selection_type: "MULTI",
            required: false,
            min_select: 0,
            max_select: 2,
            items: [
              { id: "o1", name: "加辣", price_cents: 0, enabled: true, is_default: true },
              { id: "o2", name: "加蛋", price_cents: 200, enabled: true, is_default: false },
            ],
          },
        ],
      },
    ],
  },
];

describe("menu options", () => {
  it("filters by name and option text", () => {
    expect(filterMenu(menu, "加蛋")[0].dishes).toHaveLength(1);
    expect(filterMenu(menu, "不存在")).toHaveLength(0);
  });
  it("toggles multi options without exceeding max", () => {
    const g = menu[0].dishes[0].option_groups;
    let sel = defaultOptionIds(g);
    expect(sel).toEqual(["o1"]);
    sel = toggleOption(g, sel, "g1", "o2");
    expect(sel.sort()).toEqual(["o1", "o2"]);
    sel = toggleOption(g, sel, "g1", "o1");
    expect(sel).toEqual(["o2"]);
    expect(optionsValid(g, ["o1", "o2"])).toBe(true);
  });
});

describe("pay", () => {
  it("does not treat arbitrary responses as mock success", () => {
    expect(payAction({ mock_payment: true })).toBe("mock");
    expect(payAction({ zero_order: true })).toBe("zero");
    expect(payAction({ pay_params: { timeStamp: "1", nonceStr: "n", package: "prepay_id=x", signType: "RSA", paySign: "s" } })).toBe("jsapi");
    expect(payAction({})).toBe("invalid");
    expect(paymentStatusLabel("SUCCESS")).toBe("SUCCESS");
    expect(paymentStatusLabel("CLOSED")).toBe("FAILED");
  });
});
