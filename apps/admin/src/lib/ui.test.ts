import { describe, expect, it } from "vitest";
import { canSee, fenToYuan, nextTransition, statusLabel, type NavItem } from "./ui";

describe("money and labels", () => {
  it("formats fen", () => {
    expect(fenToYuan(199)).toBe("1.99");
  });
  it("maps status", () => {
    expect(statusLabel("PAID")).toBe("待接单");
  });
  it("optimistic next step", () => {
    expect(nextTransition("PAID")).toBe("ACCEPTED");
    expect(nextTransition("COMPLETED")).toBeNull();
  });
  it("hides manager pages from staff", () => {
    const item: NavItem = { to: "/orders", label: "历史", min: "MANAGER" };
    expect(canSee("STAFF", item)).toBe(false);
    expect(canSee("MANAGER", item)).toBe(true);
  });
  it("platform cannot see store nav", () => {
    expect(canSee("PLATFORM", { to: "/board", label: "工作台", min: "STAFF" })).toBe(false);
  });
});
