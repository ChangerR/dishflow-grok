#!/usr/bin/env python3
"""Live HTTP smoke against a running DishFlow API (SHOP_DEV_MODE=true)."""

from __future__ import annotations

import json
import os
import sys
import time
import urllib.error
import urllib.request
from http.cookiejar import CookieJar
from typing import Any

BASE = os.environ.get("SMOKE_BASE", "http://127.0.0.1:8080").rstrip("/")
PLATFORM_LOGIN = os.environ.get("BOOTSTRAP_PLATFORM_LOGIN", "platform")
PLATFORM_PASSWORD = os.environ.get("BOOTSTRAP_PLATFORM_PASSWORD", "ChangeMeNow123")
APPID = os.environ.get("SMOKE_APPID", "wxSMOKE01")


class Fail(Exception):
    pass


class Client:
    def __init__(self) -> None:
        self.jar = CookieJar()
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.jar))
        self.bearer = ""
        self.appid = ""
        self.store_id = ""

    def request(
        self,
        method: str,
        path: str,
        body: Any | None = None,
        expect: int = 200,
        headers: dict[str, str] | None = None,
    ) -> tuple[int, Any]:
        data = None
        hdrs = {"Accept": "application/json"}
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            hdrs["Content-Type"] = "application/json"
        if self.store_id:
            hdrs["X-Store-Id"] = self.store_id
        if self.appid:
            hdrs["X-Wechat-Appid"] = self.appid
        if self.bearer:
            hdrs["Authorization"] = "Bearer " + self.bearer
        if method not in ("GET", "HEAD") and "Idempotency-Key" not in (headers or {}):
            hdrs["Idempotency-Key"] = f"smoke-{int(time.time()*1000)}-{method}-{path}"
        if headers:
            hdrs.update(headers)
        req = urllib.request.Request(BASE + path, data=data, headers=hdrs, method=method)
        try:
            with self.opener.open(req, timeout=20) as resp:
                raw = resp.read()
                status = resp.status
        except urllib.error.HTTPError as e:
            raw = e.read()
            status = e.code
        parsed: Any = raw
        if raw:
            try:
                parsed = json.loads(raw.decode("utf-8"))
            except json.JSONDecodeError:
                parsed = raw.decode("utf-8", "replace")
        if status != expect:
            raise Fail(f"{method} {path} expected {expect} got {status}: {parsed}")
        return status, parsed


passed: list[str] = []
failed: list[str] = []


def check(name: str, fn) -> None:
    try:
        fn()
        passed.append(name)
        print(f"PASS  {name}")
    except Exception as e:  # noqa: BLE001 — smoke must report every case
        failed.append(name)
        print(f"FAIL  {name}: {e}")


def remaining_of(menu: dict, sku_id: str) -> int | None:
    for cat in menu.get("categories") or []:
        for dish in cat.get("dishes") or []:
            for sku in dish.get("skus") or []:
                if sku.get("id") == sku_id:
                    return sku.get("remaining")
    return None


def main() -> int:
    stamp = str(int(time.time()))
    appid = os.environ.get("SMOKE_APPID") or ("wxSMOKE" + stamp[-8:])
    owner_name = "owner_" + stamp
    staff_name = "staff_" + stamp
    owner2_name = "owner2_" + stamp
    password = "ChangeMeNow123"

    health = Client()
    check("health live", lambda: health.request("GET", "/health/live"))
    check("health ready", lambda: health.request("GET", "/health/ready"))

    platform = Client()

    def platform_login() -> None:
        _, body = platform.request(
            "POST",
            "/api/v1/admin/session",
            {"login_name": PLATFORM_LOGIN, "password": PLATFORM_PASSWORD},
        )
        if not body.get("is_platform_admin"):
            raise Fail(f"expected platform admin, got {body}")

    check("platform login", platform_login)

    store_id = ""
    store2_id = ""
    owner_id = ""
    staff_id = ""
    owner2_id = ""
    cat_id = ""
    dish_id = ""
    sku_id = ""
    table_id = ""
    table_token = ""
    order_id = ""
    order_version = 1
    join_id = ""

    def create_store() -> None:
        nonlocal store_id
        _, body = platform.request("POST", "/api/v1/admin/platform/stores", {"name": "冒烟店", "timezone": "Asia/Shanghai"})
        store_id = body["id"]
        if body.get("name") != "冒烟店":
            raise Fail(body)

    check("create store", create_store)

    def platform_cannot_board() -> None:
        platform.store_id = store_id
        platform.request("GET", "/api/v1/admin/orders/board", expect=403)
        platform.store_id = ""

    check("platform admin forbidden on store board", platform_cannot_board)

    def create_owner() -> None:
        nonlocal owner_id
        _, body = platform.request(
            "POST",
            "/api/v1/admin/platform/users",
            {"login_name": owner_name, "display_name": "店主", "password": password, "enabled": True},
        )
        owner_id = body["id"]
        if body.get("login_name") != owner_name:
            raise Fail(body)

    check("create owner user (snake_case JSON)", create_owner)

    def assign_owner() -> None:
        platform.request(
            "POST",
            f"/api/v1/admin/platform/users/{owner_id}/assign-store-owner",
            {"store_id": store_id},
        )

    check("assign store owner", assign_owner)

    def create_staff_user() -> None:
        nonlocal staff_id
        _, body = platform.request(
            "POST",
            "/api/v1/admin/platform/users",
            {"login_name": staff_name, "display_name": "店员", "password": password, "enabled": True},
        )
        staff_id = body["id"]

    check("create staff user", create_staff_user)

    def create_store2() -> None:
        nonlocal store2_id, owner2_id
        _, body = platform.request("POST", "/api/v1/admin/platform/stores", {"name": "另一家店"})
        store2_id = body["id"]
        _, user = platform.request(
            "POST",
            "/api/v1/admin/platform/users",
            {"login_name": owner2_name, "display_name": "店主二", "password": password, "enabled": True},
        )
        owner2_id = user["id"]
        platform.request(
            "POST",
            f"/api/v1/admin/platform/users/{owner2_id}/assign-store-owner",
            {"store_id": store2_id},
        )

    check("create second store for cross-tenant", create_store2)

    owner = Client()
    owner.store_id = store_id

    def owner_login() -> None:
        _, body = owner.request("POST", "/api/v1/admin/session", {"login_name": owner_name, "password": password})
        if body.get("role") != "OWNER" or body.get("store_id") != store_id:
            raise Fail(body)

    check("owner login", owner_login)

    def configure_miniprogram() -> None:
        _, body = owner.request(
            "PATCH",
            "/api/v1/admin/miniprogram-config",
            {"wechat_appid": appid, "brand_name": "冒烟店"},
        )
        if body.get("wechat_appid") != appid:
            raise Fail(body)

    check("set wechat appid", configure_miniprogram)

    def enable_mock_pay() -> None:
        owner.request("PUT", "/api/v1/admin/mock-payment", {"enabled": True, "confirm": "MOCK"})
        _, body = owner.request("GET", "/api/v1/admin/mock-payment")
        if not body.get("enabled"):
            raise Fail(body)

    check("enable mock payment with confirm MOCK", enable_mock_pay)

    def create_category() -> None:
        nonlocal cat_id
        _, body = owner.request("POST", "/api/v1/admin/categories", {"name": "热销", "enabled": True, "sort_order": 1})
        cat_id = body["id"]
        if not body.get("enabled"):
            raise Fail("category created disabled")

    check("create category", create_category)

    def create_dish() -> None:
        nonlocal dish_id, sku_id
        _, body = owner.request(
            "POST",
            "/api/v1/admin/dishes",
            {
                "category_id": cat_id,
                "name": "牛肉面",
                "enabled": True,
                "packing_fee_cents": 100,
                "skus": [
                    {
                        "name": "标准",
                        "price_cents": 2800,
                        "stock_mode": "DAILY",
                        "daily_stock": 2,
                        "enabled": True,
                        "is_default": True,
                        "sort_order": 1,
                    }
                ],
            },
        )
        dish_id = body["id"]
        sku_id = body["skus"][0]["id"]
        if body.get("packing_fee_cents") != 100:
            raise Fail(body)

    check("create dish with daily stock 2", create_dish)

    def create_table() -> None:
        nonlocal table_id, table_token
        _, body = owner.request("POST", "/api/v1/admin/tables", {"table_no": "A08", "enabled": True})
        table_id = body["id"]
        _, rot = owner.request("POST", f"/api/v1/admin/tables/{table_id}/rotate-token", {})
        table_token = rot["token"]
        if not table_token:
            raise Fail(rot)

    check("create table and rotate token", create_table)

    sf = Client()
    sf.appid = appid

    def storefront_bootstrap() -> None:
        _, body = sf.request("GET", "/api/v1/storefront/bootstrap")
        if body.get("store_id") != store_id or not body.get("is_open"):
            raise Fail(body)

    check("storefront bootstrap via X-Wechat-Appid", storefront_bootstrap)

    def public_menu() -> None:
        _, body = sf.request("GET", "/api/v1/menu")
        cats = body.get("categories") or []
        if not cats or cats[0]["dishes"][0]["skus"][0]["id"] != sku_id:
            raise Fail(body)
        rem = remaining_of(body, sku_id)
        if rem != 2:
            raise Fail(f"expected remaining 2, got {rem}")

    check("customer menu remaining=2", public_menu)

    def resolve_table() -> None:
        _, body = sf.request("GET", f"/api/v1/tables/resolve?token={table_token}")
        if body.get("table_no") != "A08":
            raise Fail(body)

    check("resolve table token", resolve_table)

    def quote_dine_in() -> None:
        _, body = sf.request(
            "POST",
            "/api/v1/pricing/preview",
            {"scene": "DINE_IN", "table_token": table_token, "items": [{"sku_id": sku_id, "qty": 1}]},
        )
        if body.get("packing_cents") != 0:
            raise Fail(f"dine-in packing should be 0, got {body}")
        if body.get("goods_cents") != 2800 or body.get("payable_cents") != 2800:
            raise Fail(body)

    check("dine-in quote packing=0", quote_dine_in)

    def quote_pickup() -> None:
        _, body = sf.request(
            "POST",
            "/api/v1/pricing/preview",
            {"scene": "PICKUP", "items": [{"sku_id": sku_id, "qty": 1}]},
        )
        if body.get("packing_cents") != 100:
            raise Fail(f"pickup packing should be 100, got {body}")
        if body.get("payable_cents") != 2900:
            raise Fail(body)

    check("pickup quote packing=100", quote_pickup)

    cust = Client()
    cust.appid = appid

    def wechat_session() -> None:
        _, body = cust.request("POST", "/api/v1/auth/wechat/session", {"code": "smoke-user-1"})
        cust.bearer = body["token"]
        if body.get("store_id") != store_id:
            raise Fail(body)

    check("wechat session in dev mode", wechat_session)

    def place_and_pay() -> None:
        nonlocal order_id, order_version
        _, quote = cust.request(
            "POST",
            "/api/v1/pricing/preview",
            {"scene": "DINE_IN", "table_token": table_token, "items": [{"sku_id": sku_id, "qty": 1}]},
        )
        _, order = cust.request(
            "POST",
            "/api/v1/orders",
            {"quote_token": quote["quote_token"], "items": [{"sku_id": sku_id, "qty": 1}]},
        )
        order_id = order["id"]
        if order.get("status") != "PENDING_PAYMENT":
            raise Fail(order)
        if order.get("packing_cents") != 0 or order.get("table_no") != "A08":
            raise Fail(order)
        _, prepay = cust.request("POST", f"/api/v1/orders/{order_id}/prepay", {})
        if not prepay.get("mock_payment"):
            raise Fail(prepay)
        cust.request("POST", f"/api/v1/orders/{order_id}/mock-payment/confirm", {})
        _, paid = cust.request("GET", f"/api/v1/orders/{order_id}")
        if paid.get("status") != "PAID" or paid.get("payment_status") != "SUCCESS":
            raise Fail(paid)
        order_version = int(paid.get("version") or 1)

    check("create order + mock prepay + confirm", place_and_pay)

    def stock_after_pay() -> None:
        _, body = sf.request("GET", "/api/v1/menu")
        rem = remaining_of(body, sku_id)
        if rem != 1:
            raise Fail(f"expected remaining 1 after selling 1 of 2, got {rem} body={body}")

    check("remaining stock after payment = 1", stock_after_pay)

    def skip_level_forbidden() -> None:
        owner.request(
            "POST",
            f"/api/v1/admin/orders/{order_id}/transitions",
            {"to_status": "READY", "expected_version": order_version},
            expect=409,
        )

    check("skip-level PAID→READY is 409", skip_level_forbidden)

    def board_and_advance() -> None:
        nonlocal order_version
        _, board = owner.request("GET", "/api/v1/admin/orders/board")
        paid_col = board.get("PAID") or []
        if not any(o.get("id") == order_id for o in paid_col):
            raise Fail(board)
        for to in ("ACCEPTED", "PREPARING", "READY", "COMPLETED"):
            _, nxt = owner.request(
                "POST",
                f"/api/v1/admin/orders/{order_id}/transitions",
                {"to_status": to, "expected_version": order_version},
            )
            if nxt.get("status") != to:
                raise Fail(nxt)
            order_version = int(nxt["version"])

    check("board lists paid order and sequential transitions", board_and_advance)

    staff = Client()
    staff.store_id = store_id

    def staff_join() -> None:
        nonlocal join_id
        _, sess = staff.request("POST", "/api/v1/admin/session", {"login_name": staff_name, "password": password})
        if sess.get("role"):
            raise Fail(f"staff should not have a store role yet: {sess}")
        _, req = staff.request(
            "POST",
            "/api/v1/admin/shop-join-requests",
            {"store_id": store_id, "requested_role": "STAFF"},
        )
        join_id = req["id"]
        _, reviewed = owner.request(
            "POST",
            f"/api/v1/admin/join-requests/{join_id}/review",
            {"decision": "APPROVED"},
        )
        if reviewed.get("status") != "APPROVED":
            raise Fail(reviewed)
        _, again = staff.request("GET", "/api/v1/admin/session")
        if again.get("role") != "STAFF" or again.get("store_id") != store_id:
            raise Fail(again)

    check("staff join request approved by owner", staff_join)

    def staff_board_ok_history_forbidden() -> None:
        staff.request("GET", "/api/v1/admin/orders/board")
        staff.request("GET", "/api/v1/admin/orders", expect=403)

    check("staff can open board but not history", staff_board_ok_history_forbidden)

    def cross_store_forbidden() -> None:
        owner.store_id = store2_id
        owner.request("GET", "/api/v1/admin/orders/board", expect=403)
        owner.store_id = store_id

    check("owner cannot access another store", cross_store_forbidden)

    def closed_store_blocks_checkout() -> None:
        owner.request("PATCH", "/api/v1/admin/store", {"is_open": False})
        sf.request(
            "POST",
            "/api/v1/pricing/preview",
            {"scene": "PICKUP", "items": [{"sku_id": sku_id, "qty": 1}]},
            expect=409,
        )
        owner.request("PATCH", "/api/v1/admin/store", {"is_open": True})

    check("closed store cannot checkout", closed_store_blocks_checkout)

    def history_lists_completed() -> None:
        _, body = owner.request("GET", "/api/v1/admin/orders")
        ids = [o.get("id") for o in (body.get("items") or [])]
        if order_id not in ids:
            raise Fail(body)

    check("admin history lists completed order without empty status filter", history_lists_completed)

    def promo_and_coupon_choose_better() -> None:
        owner.request(
            "POST",
            "/api/v1/admin/promotions",
            {
                "name": "满20减3",
                "threshold_cents": 2000,
                "discount_cents": 300,
                "scope": "ALL",
                "stack_policy": "BEST_OF",
                "enabled": True,
                "starts_at": "2020-01-01T00:00:00Z",
                "ends_at": "2099-01-01T00:00:00Z",
            },
        )
        _, tpl = owner.request(
            "POST",
            "/api/v1/admin/coupon-templates",
            {
                "name": "减8券",
                "min_spend_cents": 0,
                "discount_cents": 800,
                "scope": "ALL",
                "enabled": True,
                "public_claim": True,
                "audience": "ALL",
                "starts_at": "2020-01-01T00:00:00Z",
                "ends_at": "2099-01-01T00:00:00Z",
            },
        )
        cust.request("POST", f"/api/v1/coupon-offers/{tpl['id']}/claim", {})
        _, coupons = cust.request("GET", "/api/v1/coupons?status=AVAILABLE")
        cid = (coupons.get("items") or [{}])[0].get("id")
        _, quote = cust.request(
            "POST",
            "/api/v1/pricing/preview",
            {"scene": "PICKUP", "items": [{"sku_id": sku_id, "qty": 1}], "customer_coupon_id": cid},
        )
        # goods 2800 + packing 100 = 2900; coupon 800 > promo 300
        if quote.get("discount_cents") != 800:
            raise Fail(quote)

    check("quote prefers larger coupon over full-reduction", promo_and_coupon_choose_better)

    def membership_join_dev_phone() -> None:
        _, me = cust.request("POST", "/api/v1/me/membership", {"phone_code": "13800138000", "agreed": True})
        if not me.get("is_member"):
            raise Fail(me)
        _, again = cust.request("POST", "/api/v1/me/membership", {"phone_code": "13800138000", "agreed": True})
        if again.get("member_no") != me.get("member_no"):
            raise Fail(again)

    check("membership join is idempotent in dev", membership_join_dev_phone)

    def idempotent_preview_order() -> None:
        _, quote = cust.request(
            "POST",
            "/api/v1/pricing/preview",
            {"scene": "PICKUP", "items": [{"sku_id": sku_id, "qty": 1}]},
        )
        key = "idem-order-" + stamp
        _, first = cust.request(
            "POST",
            "/api/v1/orders",
            {"quote_token": quote["quote_token"], "items": [{"sku_id": sku_id, "qty": 1}], "remark": "少辣"},
            headers={"Idempotency-Key": key},
        )
        _, second = cust.request(
            "POST",
            "/api/v1/orders",
            {"quote_token": quote["quote_token"], "items": [{"sku_id": sku_id, "qty": 1}], "remark": "少辣"},
            headers={"Idempotency-Key": key},
        )
        if first.get("id") != second.get("id"):
            raise Fail({"first": first, "second": second})
        if first.get("remark") != "少辣":
            raise Fail(first)

    check("Idempotency-Key returns same order and persists remark", idempotent_preview_order)

    print()
    print(f"{len(passed)} passed, {len(failed)} failed")
    if failed:
        print("failed cases: " + ", ".join(failed))
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
