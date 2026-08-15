package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/cryptoutil"
	"github.com/changerr/dishflow-grok/internal/domain"
)

type PreviewReq struct {
	Scene            string           `json:"scene"`
	Items            []domain.CartItem `json:"items"`
	TableToken       string           `json:"table_token"`
	ScheduledFor     *string          `json:"scheduled_for"`
	CustomerCouponID string           `json:"customer_coupon_id"`
}

type PreviewResp struct {
	QuoteToken        string              `json:"quote_token"`
	ExpiresAt         string              `json:"expires_at"`
	Scene             string              `json:"scene"`
	PickupType        string              `json:"pickup_type"`
	ScheduledFor      *string             `json:"scheduled_for,omitempty"`
	TableNo           string              `json:"table_no,omitempty"`
	Lines             []domain.QuoteLine  `json:"lines"`
	GoodsCents        int64               `json:"goods_cents"`
	PackingCents      int64               `json:"packing_cents"`
	DiscountCents     int64               `json:"discount_cents"`
	PayableCents      int64               `json:"payable_cents"`
	Discounts         []domain.DiscountDetail `json:"discounts"`
	UnavailableCoupon string              `json:"unavailable_coupon_reason,omitempty"`
}

func (a *App) Preview(ctx context.Context, storeID, customerID string, req PreviewReq) (PreviewResp, error) {
	var open bool
	if err := a.DB.QueryRowContext(ctx, `SELECT is_open FROM stores WHERE id=?`, storeID).Scan(&open); err != nil {
		return PreviewResp{}, err
	}
	if !open {
		return PreviewResp{}, apperr.StoreClosed
	}
	items, err := domain.NormalizeCart(req.Items)
	if err != nil {
		return PreviewResp{}, err
	}
	if len(items) == 0 {
		return PreviewResp{}, apperr.Validation("购物车为空")
	}
	if req.Scene != domain.SceneDineIn && req.Scene != domain.ScenePickup {
		return PreviewResp{}, apperr.Validation("用餐方式无效")
	}
	q := domain.Quote{ID: a.NewID(), StoreID: storeID, CustomerID: customerID, Scene: req.Scene, PickupType: domain.PickupImmediate}
	if req.Scene == domain.SceneDineIn {
		if req.ScheduledFor != nil {
			return PreviewResp{}, apperr.PickupTimeInvalid
		}
		tab, err := a.ResolveTable(ctx, storeID, req.TableToken)
		if err != nil {
			return PreviewResp{}, err
		}
		q.TableID = tab["id"].(string)
		q.TableNo = tab["table_no"].(string)
	}
	var scheduled *time.Time
	if req.Scene == domain.ScenePickup && req.ScheduledFor != nil && strings.TrimSpace(*req.ScheduledFor) != "" {
		t, err := time.Parse(time.RFC3339, *req.ScheduledFor)
		if err != nil {
			return PreviewResp{}, apperr.PickupTimeInvalid
		}
		t = t.UTC()
		scheduled = &t
		h, err := a.storeHours(ctx, storeID)
		if err != nil {
			return PreviewResp{}, err
		}
		if err := domain.ValidateScheduledFor(h, scheduled, a.Now(), 0); err != nil {
			return PreviewResp{}, err
		}
		q.PickupType = domain.PickupScheduled
		q.ScheduledFor = scheduled
	}
	biz, err := a.businessDate(ctx, storeID, scheduled)
	if err != nil {
		return PreviewResp{}, err
	}
	q.PickupBusinessDay = biz
	digest, _ := domain.CartDigest(items)
	q.CartDigest = digest

	var goods, packing int64
	for _, it := range items {
		line, err := a.buildLine(ctx, storeID, it, biz, req.Scene)
		if err != nil {
			return PreviewResp{}, err
		}
		q.Lines = append(q.Lines, line)
		goods += line.UnitPriceCents * int64(line.Qty)
		packing += line.PackingCents * int64(line.Qty)
	}
	q.GoodsCents = goods
	if req.Scene == domain.SceneDineIn {
		packing = 0
		for i := range q.Lines {
			q.Lines[i].PackingCents = 0
		}
	}
	q.PackingCents = packing

	promos, err := a.activePromotions(ctx, storeID)
	if err != nil {
		return PreviewResp{}, err
	}
	best := domain.BestPromotion(a.Now(), goods, promos)
	var coupon *domain.Coupon
	unavail := ""
	if req.CustomerCouponID != "" {
		c, reason, err := a.loadCustomerCoupon(ctx, storeID, customerID, req.CustomerCouponID)
		if err != nil {
			return PreviewResp{}, err
		}
		if reason != "" {
			unavail = reason
		} else {
			coupon = c
		}
	}
	couponAmt := domain.CouponDiscount(a.Now(), goods, coupon)
	kind, disc := domain.ChooseDiscount(best, couponAmt)
	disc, payable := domain.FinalizeAmounts(goods, packing, disc)
	q.DiscountCents = disc
	q.PayableCents = payable
	if kind == "PROMOTION" && best != nil {
		q.PromotionID = best.ID
		q.Discounts = []domain.DiscountDetail{{Type: "PROMOTION", ID: best.ID, Name: best.Name, Amount: disc}}
	} else if kind == "COUPON" && coupon != nil {
		q.CouponID = coupon.ID
		q.Discounts = []domain.DiscountDetail{{Type: "COUPON", ID: coupon.ID, Name: coupon.Name, Amount: disc}}
	} else {
		q.Discounts = []domain.DiscountDetail{}
	}
	q.ExpiresAt = a.Now().Add(a.Cfg.QuoteTTL)
	token, err := cryptoutil.RandomHex(24)
	if err != nil {
		return PreviewResp{}, err
	}
	raw, _ := json.Marshal(q)
	if a.Redis != nil {
		if err := a.Redis.Set(ctx, "quote:"+token, raw, a.Cfg.QuoteTTL).Err(); err != nil {
			return PreviewResp{}, err
		}
	}
	resp := PreviewResp{
		QuoteToken: token, ExpiresAt: q.ExpiresAt.UTC().Format(time.RFC3339),
		Scene: q.Scene, PickupType: q.PickupType, TableNo: q.TableNo,
		Lines: q.Lines, GoodsCents: q.GoodsCents, PackingCents: q.PackingCents,
		DiscountCents: q.DiscountCents, PayableCents: q.PayableCents, Discounts: q.Discounts,
		UnavailableCoupon: unavail,
	}
	if q.ScheduledFor != nil {
		h, _ := a.storeHours(ctx, storeID)
		loc, _ := domain.LoadLocation(h.Timezone)
		s := q.ScheduledFor.In(loc).Format(time.RFC3339)
		resp.ScheduledFor = &s
	}
	return resp, nil
}

func (a *App) loadQuote(ctx context.Context, token string) (domain.Quote, error) {
	if token == "" || a.Redis == nil {
		return domain.Quote{}, apperr.QuoteExpired
	}
	b, err := a.Redis.Get(ctx, "quote:"+token).Bytes()
	if err != nil {
		return domain.Quote{}, apperr.QuoteExpired
	}
	var q domain.Quote
	if err := json.Unmarshal(b, &q); err != nil {
		return domain.Quote{}, apperr.QuoteExpired
	}
	if a.Now().After(q.ExpiresAt) {
		return domain.Quote{}, apperr.QuoteExpired
	}
	return q, nil
}

func (a *App) buildLine(ctx context.Context, storeID string, it domain.CartItem, biz, scene string) (domain.QuoteLine, error) {
	var productID, skuName, stockMode, productName string
	var price, packing int64
	var skuEnabled, prodEnabled, soldOut bool
	var catEnabled bool
	var prodDeleted, catDeleted sql.NullTime
	var daily int
	err := a.DB.QueryRowContext(ctx, `SELECT s.product_id, s.name, s.price_cents, s.stock_mode, s.daily_stock, s.enabled,
		p.name, p.enabled, p.sold_out, p.packing_fee_cents, p.deleted_at, c.enabled, c.deleted_at
		FROM skus s JOIN products p ON p.id=s.product_id JOIN categories c ON c.id=p.category_id
		WHERE s.id=? AND s.store_id=?`, it.SKUID, storeID).
		Scan(&productID, &skuName, &price, &stockMode, &daily, &skuEnabled, &productName, &prodEnabled, &soldOut, &packing, &prodDeleted, &catEnabled, &catDeleted)
	if err == sql.ErrNoRows {
		return domain.QuoteLine{}, apperr.Validation("规格不可售")
	}
	if err != nil {
		return domain.QuoteLine{}, err
	}
	if !skuEnabled || !prodEnabled || soldOut || prodDeleted.Valid || !catEnabled || catDeleted.Valid {
		return domain.QuoteLine{}, apperr.Validation(productName + " 当前不可购买")
	}
	if stockMode == domain.StockDaily {
		if a.remainingStock(ctx, storeID, it.SKUID, biz, daily) < it.Qty {
			return domain.QuoteLine{}, apperr.Validation(productName + " 库存不足")
		}
	}
	optPrices, names, err := a.validateOptionsSelection(ctx, productID, it.OptionIDs)
	if err != nil {
		return domain.QuoteLine{}, err
	}
	unit := domain.LineUnitPrice(price, optPrices)
	line := domain.QuoteLine{
		SKUID: it.SKUID, ProductID: productID, ProductName: productName, SKUName: skuName,
		OptionIDs: it.OptionIDs, OptionNames: names, Qty: it.Qty,
		UnitPriceCents: unit, PackingCents: packing, LineTotalCents: unit * int64(it.Qty),
	}
	_ = scene
	return line, nil
}

func (a *App) validateOptionsSelection(ctx context.Context, productID string, selected []string) ([]int64, []string, error) {
	groups, err := a.customerOGs(ctx, productID)
	if err != nil {
		return nil, nil, err
	}
	sel := map[string]struct{}{}
	for _, id := range selected {
		sel[id] = struct{}{}
	}
	var prices []int64
	var names []string
	used := map[string]bool{}
	for _, g := range groups {
		count := 0
		for _, it := range g.Items {
			if _, ok := sel[it.ID]; ok {
				if !it.Enabled {
					return nil, nil, apperr.Validation("选项已失效")
				}
				count++
				prices = append(prices, it.PriceCents)
				names = append(names, it.Name)
				used[it.ID] = true
			}
		}
		if g.Required && count == 0 {
			return nil, nil, apperr.Validation("请选择 " + g.Name)
		}
		min, max := g.MinSelect, g.MaxSelect
		if g.SelectionType == "SINGLE" {
			max = 1
			if min < 1 && g.Required {
				min = 1
			}
		}
		if count < min || (max > 0 && count > max) {
			return nil, nil, apperr.Validation(g.Name + " 选择数量不符合规则")
		}
	}
	for id := range sel {
		if !used[id] {
			return nil, nil, apperr.Validation("存在无效选项")
		}
	}
	return prices, names, nil
}

func (a *App) activePromotions(ctx context.Context, storeID string) ([]domain.Promotion, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, name, threshold_cents, discount_cents, starts_at, ends_at, enabled FROM promotions WHERE store_id=?`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Promotion
	for rows.Next() {
		var p domain.Promotion
		if err := rows.Scan(&p.ID, &p.Name, &p.ThresholdCents, &p.DiscountCents, &p.StartsAt, &p.EndsAt, &p.Enabled); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (a *App) loadCustomerCoupon(ctx context.Context, storeID, customerID, couponID string) (*domain.Coupon, string, error) {
	if customerID == "" {
		return nil, "请先登录后使用优惠券", nil
	}
	var c domain.Coupon
	var enabled bool
	err := a.DB.QueryRowContext(ctx, `SELECT cc.id, cc.template_id, ct.name, ct.min_spend_cents, ct.discount_cents, ct.starts_at, ct.ends_at, ct.enabled, cc.status, cc.customer_id, cc.store_id
		FROM customer_coupons cc JOIN coupon_templates ct ON ct.id=cc.template_id
		WHERE cc.id=? AND cc.store_id=?`, couponID, storeID).
		Scan(&c.ID, &c.TemplateID, &c.Name, &c.MinSpendCents, &c.DiscountCents, &c.StartsAt, &c.EndsAt, &enabled, &c.Status, &c.CustomerID, &c.StoreID)
	if err == sql.ErrNoRows {
		return nil, "优惠券不存在", nil
	}
	if err != nil {
		return nil, "", err
	}
	c.Enabled = enabled
	if c.CustomerID != customerID {
		return nil, "优惠券不属于当前顾客", nil
	}
	if domain.CouponDiscount(a.Now(), 1<<62, &c) == 0 && c.Status != "AVAILABLE" {
		return nil, "优惠券不可用", nil
	}
	return &c, "", nil
}

func (a *App) CreateOrder(ctx context.Context, storeID, customerID, quoteToken string, items []domain.CartItem, remark string) (OrderDTO, error) {
	if customerID == "" {
		return OrderDTO{}, apperr.Unauthorized
	}
	q, err := a.loadQuote(ctx, quoteToken)
	if err != nil {
		return OrderDTO{}, err
	}
	if q.StoreID != storeID {
		return OrderDTO{}, apperr.Forbidden
	}
	if q.CustomerID != "" && q.CustomerID != customerID {
		return OrderDTO{}, apperr.Forbidden
	}
	if len(items) > 0 {
		digest, err := domain.CartDigest(items)
		if err != nil {
			return OrderDTO{}, err
		}
		if digest != q.CartDigest {
			return OrderDTO{}, apperr.QuoteMismatch
		}
	}
	var existing string
	err = a.DB.QueryRowContext(ctx, `SELECT id FROM orders WHERE quote_id=?`, q.ID).Scan(&existing)
	if err == nil {
		return a.GetOrder(ctx, storeID, existing, customerID, true)
	}
	now := a.Now()
	orderID := a.NewID()
	err = persistTx(ctx, a, func(tx *sql.Tx) error {
		pickupNo, err := nextPickup(tx, storeID, q.PickupBusinessDay)
		if err != nil {
			return err
		}
		if q.PickupType == domain.PickupScheduled && q.ScheduledFor != nil {
			h, err := a.storeHours(ctx, storeID)
			if err != nil {
				return err
			}
			if err := occupySlot(tx, storeID, *q.ScheduledFor, h.SlotCapacity); err != nil {
				return err
			}
		}
		for _, line := range q.Lines {
			if err := reserveStock(tx, storeID, line.SKUID, q.PickupBusinessDay, line.Qty, now, orderID); err != nil {
				return err
			}
		}
		_, err = tx.Exec(`INSERT INTO orders (id, store_id, customer_id, quote_id, scene, status, payment_status, table_id, table_no, pickup_type, scheduled_for, pickup_business_date, pickup_number, goods_cents, packing_cents, discount_cents, payable_cents, applied_promotion_id, applied_coupon_id, remark, version, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,?)`,
			orderID, storeID, customerID, q.ID, q.Scene, domain.OrderPendingPayment, domain.PayUnpaid,
			nullS(q.TableID), nullS(q.TableNo), q.PickupType, q.ScheduledFor, q.PickupBusinessDay, pickupNo,
			q.GoodsCents, q.PackingCents, q.DiscountCents, q.PayableCents, nullS(q.PromotionID), nullS(q.CouponID), nullS(clip(remark, 100)), now, now)
		if err != nil {
			if isDup(err) {
				return apperr.Conflict
			}
			return err
		}
		for _, line := range q.Lines {
			opt, _ := json.Marshal(optionSnapshot(line.OptionIDs, line.OptionNames))
			if _, err := tx.Exec(`INSERT INTO order_items (id, order_id, store_id, product_id, sku_id, product_name, sku_name, options_json, qty, unit_price_cents, packing_fee_cents, line_total_cents)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, a.NewID(), orderID, storeID, line.ProductID, line.SKUID, line.ProductName, line.SKUName, string(opt), line.Qty, line.UnitPriceCents, line.PackingCents, line.LineTotalCents); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`INSERT INTO order_events (id, order_id, store_id, from_status, to_status, actor_type, created_at) VALUES (?,?,?,?,?,?,?)`,
			a.NewID(), orderID, storeID, nil, domain.OrderPendingPayment, "CUSTOMER", now); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO payments (id, store_id, order_id, status, amount_cents, expires_at, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
			a.NewID(), storeID, orderID, domain.PayUnpaid, q.PayableCents, q.ExpiresAt, now, now); err != nil {
			return err
		}
		return a.Outbox(tx, storeID, "order.created", map[string]string{"order_id": orderID})
	})
	if err != nil {
		return OrderDTO{}, err
	}
	if a.Redis != nil {
		_ = a.Redis.Del(ctx, "quote:"+quoteToken).Err()
	}
	return a.GetOrder(ctx, storeID, orderID, customerID, true)
}

func optionSnapshot(ids, names []string) []map[string]string {
	n := len(names)
	if len(ids) > n {
		n = len(ids)
	}
	out := make([]map[string]string, 0, n)
	for i := 0; i < n; i++ {
		row := map[string]string{}
		if i < len(ids) {
			row["id"] = ids[i]
		}
		if i < len(names) {
			row["name"] = names[i]
		}
		out = append(out, row)
	}
	return out
}

func occupySlot(tx *sql.Tx, storeID string, start time.Time, capacity int) error {
	hStart := start.UTC()
	if capacity < 1 {
		capacity = 1
	}
	var capSnap, reserved int
	err := tx.QueryRow(`SELECT capacity_snapshot, reserved_orders FROM pickup_slot_capacity WHERE store_id=? AND slot_start=? FOR UPDATE`, storeID, hStart).
		Scan(&capSnap, &reserved)
	if err == sql.ErrNoRows {
		_, err = tx.Exec(`INSERT INTO pickup_slot_capacity (store_id, slot_start, capacity_snapshot, reserved_orders) VALUES (?,?,?,1)`,
			storeID, hStart, capacity)
		if err != nil {
			if isDup(err) {
				return occupySlot(tx, storeID, start, capacity)
			}
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	if reserved+1 > capSnap {
		return apperr.PickupSlotFull
	}
	_, err = tx.Exec(`UPDATE pickup_slot_capacity SET reserved_orders=reserved_orders+1 WHERE store_id=? AND slot_start=?`, storeID, hStart)
	return err
}

func nextPickup(tx *sql.Tx, storeID, biz string) (string, error) {
	var n int
	err := tx.QueryRow(`SELECT last_number FROM store_pickup_counters WHERE store_id=? AND business_date=? FOR UPDATE`, storeID, biz).Scan(&n)
	if err == sql.ErrNoRows {
		if _, err := tx.Exec(`INSERT INTO store_pickup_counters (store_id, business_date, last_number) VALUES (?,?,1)`, storeID, biz); err != nil {
			return "", err
		}
		return domain.PickupNumber(1), nil
	}
	if err != nil {
		return "", err
	}
	n++
	if _, err := tx.Exec(`UPDATE store_pickup_counters SET last_number=? WHERE store_id=? AND business_date=?`, n, storeID, biz); err != nil {
		return "", err
	}
	return domain.PickupNumber(n), nil
}

func reserveStock(tx *sql.Tx, storeID, skuID, biz string, qty int, now time.Time, orderID string) error {
	var mode string
	var daily int
	if err := tx.QueryRow(`SELECT stock_mode, daily_stock FROM skus WHERE id=?`, skuID).Scan(&mode, &daily); err != nil {
		return err
	}
	if mode != domain.StockDaily {
		return nil
	}
	if err := ensureInventory(tx, storeID, skuID, biz, daily); err != nil {
		return err
	}
	var avail, reserved, sold int
	if err := tx.QueryRow(`SELECT available_qty, reserved_qty, sold_qty FROM daily_inventory WHERE store_id=? AND sku_id=? AND business_date=? FOR UPDATE`, storeID, skuID, biz).
		Scan(&avail, &reserved, &sold); err != nil {
		return err
	}
	if reserved+sold+qty > avail {
		return apperr.Validation("库存不足")
	}
	if _, err := tx.Exec(`UPDATE daily_inventory SET reserved_qty=reserved_qty+? WHERE store_id=? AND sku_id=? AND business_date=?`, qty, storeID, skuID, biz); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO inventory_movements (id, store_id, sku_id, business_date, delta, reason, order_id, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		orderID[:26], storeID, skuID, biz, -qty, "RESERVE", orderID, now)
	return err
}

func releaseStock(tx *sql.Tx, storeID, skuID, biz string, qty int, now time.Time, orderID, reason string) error {
	var mode string
	if err := tx.QueryRow(`SELECT stock_mode FROM skus WHERE id=?`, skuID).Scan(&mode); err != nil {
		return err
	}
	if mode != domain.StockDaily {
		return nil
	}
	if _, err := tx.Exec(`UPDATE daily_inventory SET reserved_qty=GREATEST(reserved_qty-?,0) WHERE store_id=? AND sku_id=? AND business_date=?`, qty, storeID, skuID, biz); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO inventory_movements (id, store_id, sku_id, business_date, delta, reason, order_id, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		orderID[:16]+reason[:min(10, len(reason))], storeID, skuID, biz, qty, reason, orderID, now)
	return err
}

func sellStock(tx *sql.Tx, storeID, skuID, biz string, qty int, now time.Time, orderID string) error {
	var mode string
	if err := tx.QueryRow(`SELECT stock_mode FROM skus WHERE id=?`, skuID).Scan(&mode); err != nil {
		return err
	}
	if mode != domain.StockDaily {
		return nil
	}
	if _, err := tx.Exec(`UPDATE daily_inventory SET reserved_qty=GREATEST(reserved_qty-?,0), sold_qty=sold_qty+? WHERE store_id=? AND sku_id=? AND business_date=?`, qty, qty, storeID, skuID, biz); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO inventory_movements (id, store_id, sku_id, business_date, delta, reason, order_id, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		orderID+"sold"[:min(26, 4)], storeID, skuID, biz, 0, "SOLD", orderID, now)
	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (a *App) releaseCapacity(tx *sql.Tx, orderID, storeID string) error {
	var released sql.NullTime
	var pickupType string
	var scheduled sql.NullTime
	if err := tx.QueryRow(`SELECT pickup_capacity_released_at, pickup_type, scheduled_for FROM orders WHERE id=? FOR UPDATE`, orderID).
		Scan(&released, &pickupType, &scheduled); err != nil {
		return err
	}
	if released.Valid || pickupType != domain.PickupScheduled || !scheduled.Valid {
		return nil
	}
	if _, err := tx.Exec(`UPDATE pickup_slot_capacity SET reserved_orders=GREATEST(reserved_orders-1,0) WHERE store_id=? AND slot_start=?`, storeID, scheduled.Time); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE orders SET pickup_capacity_released_at=? WHERE id=?`, a.Now(), orderID)
	return err
}

func sortedKeys(m map[string]struct{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
