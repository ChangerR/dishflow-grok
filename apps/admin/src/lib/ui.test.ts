import { describe, expect, it } from "vitest";
import { canSee, fenToYuan, isOverdue, nextTransition, statusLabel, type NavItem } from "./ui";
import { inspectMerchantCert } from "./ops";

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
  it("rejects non-PEM cert paste", () => {
    expect(inspectMerchantCert("hello").ok).toBe(false);
    expect(inspectMerchantCert("-----BEGIN CERTIFICATE-----\nabc").ok).toBe(true);
  });
  it("marks overdue after pickup minutes", () => {
    expect(isOverdue("2026-08-15T00:00:00Z", 15, Date.parse("2026-08-15T00:20:00Z"))).toBe(true);
    expect(isOverdue("2026-08-15T00:00:00Z", 15, Date.parse("2026-08-15T00:10:00Z"))).toBe(false);
  });
});
