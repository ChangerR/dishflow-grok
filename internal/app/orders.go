package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/domain"
	"github.com/changerr/dishflow-grok/internal/ids"
	"github.com/changerr/dishflow-grok/internal/wechat"
)

type OrderDTO struct {
	ID                  string         `json:"id"`
	StoreID             string         `json:"store_id"`
	Status              string         `json:"status"`
	PaymentStatus       string         `json:"payment_status"`
	RefundStatus        string         `json:"refund_status,omitempty"`
	Scene               string         `json:"scene"`
	PickupType          string         `json:"pickup_type"`
	ScheduledFor        *string        `json:"scheduled_for,omitempty"`
	PickupBusinessDate  string         `json:"pickup_business_date"`
	PickupNumber        string         `json:"pickup_number,omitempty"`
	TableNo             string         `json:"table_no,omitempty"`
	GoodsCents          int64          `json:"goods_cents"`
	PackingCents        int64          `json:"packing_cents"`
	DiscountCents       int64          `json:"discount_cents"`
	PayableCents        int64          `json:"payable_cents"`
	Remark              string         `json:"remark,omitempty"`
	Version             int            `json:"version"`
	IsMock              bool           `json:"is_mock"`
	PointsAwarded       int            `json:"points_awarded"`
	CreatedAt           string         `json:"created_at"`
	PaidAt              *string        `json:"paid_at,omitempty"`
	Items               []OrderItemDTO `json:"items"`
	Events              []OrderEventDTO `json:"events,omitempty"`
}

type OrderItemDTO struct {
	ProductName     string   `json:"product_name"`
	SKUName         string   `json:"sku_name"`
	Options         []string `json:"options"`
	Qty             int      `json:"qty"`
	UnitPriceCents  int64    `json:"unit_price_cents"`
	LineTotalCents  int64    `json:"line_total_cents"`
	SKUID           string   `json:"sku_id"`
	ProductID       string   `json:"product_id"`
}

type OrderEventDTO struct {
	FromStatus string `json:"from_status,omitempty"`
	ToStatus   string `json:"to_status"`
	ActorType  string `json:"actor_type"`
	CreatedAt  string `json:"created_at"`
}

func (a *App) GetOrder(ctx context.Context, storeID, orderID, customerID string, customerScope bool) (OrderDTO, error) {
	var d OrderDTO
	var sched sql.NullTime
	var tableNo, remark, refund sql.NullString
	var pickupNo sql.NullString
	var paid sql.NullTime
	var created, updated time.Time
	err := a.DB.QueryRowContext(ctx, `SELECT id, store_id, status, payment_status, refund_status, scene, pickup_type, scheduled_for, pickup_business_date, pickup_number, table_no, goods_cents, packing_cents, discount_cents, payable_cents, remark, version, is_mock, points_awarded, created_at, paid_at
		FROM orders WHERE id=? AND store_id=?`, orderID, storeID).
		Scan(&d.ID, &d.StoreID, &d.Status, &d.PaymentStatus, &refund, &d.Scene, &d.PickupType, &sched, &d.PickupBusinessDate, &pickupNo, &tableNo, &d.GoodsCents, &d.PackingCents, &d.DiscountCents, &d.PayableCents, &remark, &d.Version, &d.IsMock, &d.PointsAwarded, &created, &paid)
	if err == sql.ErrNoRows {
		return d, apperr.NotFound
	}
	if err != nil {
		return d, err
	}
	if customerScope {
		var cid string
		_ = a.DB.QueryRowContext(ctx, `SELECT customer_id FROM orders WHERE id=?`, orderID).Scan(&cid)
		if cid != customerID {
			return d, apperr.NotFound
		}
	}
	d.RefundStatus = scanNullString(refund)
	d.TableNo = scanNullString(tableNo)
	d.Remark = scanNullString(remark)
	d.PickupNumber = scanNullString(pickupNo)
	d.PickupBusinessDate = bizDate(d.PickupBusinessDate)
	d.CreatedAt = created.UTC().Format(time.RFC3339)
	if paid.Valid {
		s := paid.Time.UTC().Format(time.RFC3339)
		d.PaidAt = &s
	}
	if sched.Valid {
		h, _ := a.storeHours(ctx, storeID)
		loc, _ := domain.LoadLocation(h.Timezone)
		s := sched.Time.In(loc).Format(time.RFC3339)
		d.ScheduledFor = &s
	}
	rows, err := a.DB.QueryContext(ctx, `SELECT product_id, sku_id, product_name, sku_name, options_json, qty, unit_price_cents, line_total_cents FROM order_items WHERE order_id=?`, orderID)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var it OrderItemDTO
		var opt string
		if err := rows.Scan(&it.ProductID, &it.SKUID, &it.ProductName, &it.SKUName, &opt, &it.Qty, &it.UnitPriceCents, &it.LineTotalCents); err != nil {
			return d, err
		}
		_ = json.Unmarshal([]byte(opt), &it.Options)
		if it.Options == nil {
			it.Options = []string{}
		}
		d.Items = append(d.Items, it)
	}
	if d.Items == nil {
		d.Items = []OrderItemDTO{}
	}
	ev, _ := a.DB.QueryContext(ctx, `SELECT from_status, to_status, actor_type, created_at FROM order_events WHERE order_id=? ORDER BY created_at`, orderID)
	if ev != nil {
		defer ev.Close()
		for ev.Next() {
			var e OrderEventDTO
			var from sql.NullString
			var t time.Time
			if err := ev.Scan(&from, &e.ToStatus, &e.ActorType, &t); err != nil {
				return d, err
			}
			e.FromStatus = scanNullString(from)
			e.CreatedAt = t.UTC().Format(time.RFC3339)
			d.Events = append(d.Events, e)
		}
	}
	if d.Events == nil {
		d.Events = []OrderEventDTO{}
	}
	_ = updated
	return d, nil
}

func (a *App) ListCustomerOrders(ctx context.Context, storeID, customerID, status string) ([]OrderDTO, error) {
	q := `SELECT id FROM orders WHERE store_id=? AND customer_id=?`
	args := []any{storeID, customerID}
	if status != "" && status != "ALL" {
		switch status {
		case "PENDING_PAYMENT":
			q += ` AND status='PENDING_PAYMENT'`
		case "PREPARING":
			q += ` AND status IN ('PAID','ACCEPTED','PREPARING')`
		case "READY":
			q += ` AND status='READY'`
		case "COMPLETED":
			q += ` AND status='COMPLETED'`
		case "CANCELLED":
			q += ` AND status IN ('CANCELLED','REFUNDED','REFUNDING','CANCEL_REQUESTED')`
		}
	}
	q += ` ORDER BY created_at DESC LIMIT 100`
	rows, err := a.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrderDTO
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		o, err := a.GetOrder(ctx, storeID, id, customerID, true)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if out == nil {
		out = []OrderDTO{}
	}
	return out, rows.Err()
}

func (a *App) Prepay(ctx context.Context, storeID, customerID, orderID string) (map[string]any, error) {
	o, err := a.GetOrder(ctx, storeID, orderID, customerID, true)
	if err != nil {
		return nil, err
	}
	if o.Status != domain.OrderPendingPayment {
		return nil, apperr.StateConflict
	}
	var payID, status string
	var prepay sql.NullString
	var exp time.Time
	var mock bool
	var amount int64
	if err := a.DB.QueryRowContext(ctx, `SELECT id, status, prepay_id, expires_at, is_mock, amount_cents FROM payments WHERE order_id=?`, orderID).
		Scan(&payID, &status, &prepay, &exp, &mock, &amount); err != nil {
		return nil, err
	}
	if a.Now().Add(30 * time.Second).After(exp) {
		return nil, apperr.PaymentUnavailable
	}
	var mockPay bool
	_ = a.DB.QueryRowContext(ctx, `SELECT COALESCE(mock_payment,0) FROM payment_configs WHERE store_id=?`, storeID).Scan(&mockPay)
	if a.Cfg.DevMode {
		mockPay = true
	}
	if amount == 0 {
		if err := a.ConfirmPaid(ctx, storeID, orderID, "ZERO", false); err != nil {
			return nil, err
		}
		return map[string]any{"zero_order": true, "mock_payment": false}, nil
	}
	if mockPay {
		_, _ = a.DB.ExecContext(ctx, `UPDATE payments SET status=?, prepay_id=?, is_mock=1, updated_at=? WHERE id=?`, domain.PayPrepayCreated, "mock_"+orderID, a.Now(), payID)
		_, _ = a.DB.ExecContext(ctx, `UPDATE orders SET is_mock=1, payment_status=?, updated_at=? WHERE id=?`, domain.PayPrepayCreated, a.Now(), orderID)
		return map[string]any{"mock_payment": true, "prepay_id": "mock_" + orderID}, nil
	}
	cfg, err := a.payConfig(ctx, storeID)
	if err != nil {
		return nil, apperr.PaymentUnavailable
	}
	var openid string
	_ = a.DB.QueryRowContext(ctx, `SELECT wechat_openid FROM customers WHERE id=?`, customerID).Scan(&openid)
	prepayID, params, err := a.Wechat.JSAPIPrepay(ctx, cfg, openid, amount, orderID, "DishFlow", a.Cfg.PublicBaseURL+"/callbacks/wechat-pay/transactions/"+storeID)
	if err != nil {
		return nil, err
	}
	_, _ = a.DB.ExecContext(ctx, `UPDATE payments SET status=?, prepay_id=?, updated_at=? WHERE id=?`, domain.PayPrepayCreated, prepayID, a.Now(), payID)
	params["mock_payment"] = "false"
	out := map[string]any{"mock_payment": false, "prepay_id": prepayID, "pay_params": params}
	return out, nil
}

func (a *App) ConfirmMockPayment(ctx context.Context, storeID, customerID, orderID string) error {
	o, err := a.GetOrder(ctx, storeID, orderID, customerID, true)
	if err != nil {
		return err
	}
	if !o.IsMock && !a.Cfg.DevMode {
		var mock bool
		_ = a.DB.QueryRowContext(ctx, `SELECT is_mock FROM payments WHERE order_id=?`, orderID).Scan(&mock)
		if !mock {
			return apperr.Validation("非 mock 订单不能走确认接口")
		}
	}
	return a.ConfirmPaid(ctx, storeID, orderID, "MOCK", true)
}

func (a *App) ConfirmPaid(ctx context.Context, storeID, orderID, txnID string, mock bool) error {
	now := a.Now()
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		var status, payStatus, couponID string
		var customerID string
		var payable int64
		var pointsAlready int
		var biz string
		var coupon sql.NullString
		err := tx.QueryRow(`SELECT status, payment_status, customer_id, payable_cents, points_awarded, pickup_business_date, applied_coupon_id FROM orders WHERE id=? AND store_id=? FOR UPDATE`, orderID, storeID).
			Scan(&status, &payStatus, &customerID, &payable, &pointsAlready, &biz, &coupon)
		if err == sql.ErrNoRows {
			return apperr.NotFound
		}
		if err != nil {
			return err
		}
		biz = bizDate(biz)
		if payStatus == domain.PaySuccess {
			return nil
		}
		if status != domain.OrderPendingPayment {
			return apperr.StateConflict
		}
		couponID = scanNullString(coupon)
		rows, err := tx.Query(`SELECT sku_id, qty FROM order_items WHERE order_id=?`, orderID)
		if err != nil {
			return err
		}
		type line struct {
			sku string
			qty int
		}
		var lines []line
		for rows.Next() {
			var l line
			if err := rows.Scan(&l.sku, &l.qty); err != nil {
				rows.Close()
				return err
			}
			lines = append(lines, l)
		}
		rows.Close()
		for _, l := range lines {
			if err := sellStockFixed(tx, storeID, l.sku, biz, l.qty, now, orderID); err != nil {
				return err
			}
		}
		if couponID != "" {
			if _, err := tx.Exec(`UPDATE customer_coupons SET status='USED', used_at=?, order_id=? WHERE id=? AND status='AVAILABLE'`, now, orderID, couponID); err != nil {
				return err
			}
		}
		points := 0
		var membershipID string
		var perYuan int
		var mstatus string
		err = tx.QueryRow(`SELECT m.id, m.status, s.points_per_yuan FROM customer_memberships m JOIN member_settings s ON s.store_id=m.store_id WHERE m.store_id=? AND m.customer_id=?`, storeID, customerID).
			Scan(&membershipID, &mstatus, &perYuan)
		if err == nil && mstatus == "ACTIVE" && pointsAlready == 0 {
			points = domain.PointsForPayment(payable, perYuan)
			if points > 0 {
				if _, err := tx.Exec(`UPDATE customer_memberships SET points_balance=points_balance+? WHERE id=?`, points, membershipID); err != nil {
					return err
				}
				var bal int
				_ = tx.QueryRow(`SELECT points_balance FROM customer_memberships WHERE id=?`, membershipID).Scan(&bal)
				if _, err := tx.Exec(`INSERT INTO member_points_ledger (id, store_id, membership_id, delta, balance_after, reason_type, ref_id, created_at) VALUES (?,?,?,?,?,?,?,?)`,
					ids.New(), storeID, membershipID, points, bal, "ORDER_PAID", orderID, now); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(`UPDATE orders SET status=?, payment_status=?, paid_at=?, points_awarded=?, is_mock=?, updated_at=? WHERE id=?`,
			domain.OrderPaid, domain.PaySuccess, now, points, mock, now, orderID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE payments SET status=?, wechat_transaction_id=?, is_mock=?, updated_at=? WHERE order_id=?`, domain.PaySuccess, txnID, mock, now, orderID); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO order_events (id, order_id, store_id, from_status, to_status, actor_type, created_at) VALUES (?,?,?,?,?,?,?)`,
			ids.New(), orderID, storeID, domain.OrderPendingPayment, domain.OrderPaid, "SYSTEM", now); err != nil {
			return err
		}
		return a.Outbox(tx, storeID, "order.paid", map[string]string{"order_id": orderID})
	})
}

func sellStockFixed(tx *sql.Tx, storeID, skuID, biz string, qty int, now time.Time, orderID string) error {
	biz = bizDate(biz)
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
		ids.New(), storeID, skuID, biz, 0, "SOLD", orderID, now)
	return err
}

func (a *App) CancelCustomerOrder(ctx context.Context, storeID, customerID, orderID string) (map[string]any, error) {
	o, err := a.GetOrder(ctx, storeID, orderID, customerID, true)
	if err != nil {
		return nil, err
	}
	now := a.Now()
	switch o.Status {
	case domain.OrderPendingPayment:
		if err := a.closeUnpaid(ctx, storeID, orderID, "CUSTOMER"); err != nil {
			return nil, err
		}
		return map[string]any{"status": domain.OrderCancelled}, nil
	case domain.OrderPaid:
		if err := a.startRefund(ctx, storeID, orderID, "顾客取消未接单", "CUSTOMER_AUTO", "", true); err != nil {
			return nil, err
		}
		return map[string]any{"status": domain.OrderRefunding}, nil
	case domain.OrderAccepted:
		err := persistTx(ctx, a, func(tx *sql.Tx) error {
			res, err := tx.Exec(`UPDATE orders SET status=?, version=version+1, updated_at=? WHERE id=? AND status=?`, domain.OrderCancelRequested, now, orderID, domain.OrderAccepted)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return apperr.StateConflict
			}
			_, err = tx.Exec(`INSERT INTO order_events (id, order_id, store_id, from_status, to_status, actor_type, created_at) VALUES (?,?,?,?,?,?,?)`,
				ids.New(), orderID, storeID, domain.OrderAccepted, domain.OrderCancelRequested, "CUSTOMER", now)
			return err
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": domain.OrderCancelRequested, "message": "已受理，等待门店审核"}, nil
	case domain.OrderCancelRequested, domain.OrderRefunding:
		return map[string]any{"status": o.Status, "message": "处理中"}, nil
	default:
		return nil, apperr.New(409, "CANCEL_FORBIDDEN", "当前状态不可取消")
	}
}

func (a *App) closeUnpaid(ctx context.Context, storeID, orderID, actor string) error {
	now := a.Now()
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		var status, biz string
		if err := tx.QueryRow(`SELECT status, pickup_business_date FROM orders WHERE id=? AND store_id=? FOR UPDATE`, orderID, storeID).Scan(&status, &biz); err != nil {
			return err
		}
		biz = bizDate(biz)
		if status == domain.OrderCancelled {
			return nil
		}
		if status != domain.OrderPendingPayment {
			return apperr.StateConflict
		}
		rows, err := tx.Query(`SELECT sku_id, qty FROM order_items WHERE order_id=?`, orderID)
		if err != nil {
			return err
		}
		var sku string
		var qty int
		type l struct {
			s string
			q int
		}
		var ls []l
		for rows.Next() {
			if err := rows.Scan(&sku, &qty); err != nil {
				rows.Close()
				return err
			}
			ls = append(ls, l{sku, qty})
		}
		rows.Close()
		for _, x := range ls {
			if err := releaseStockFixed(tx, storeID, x.s, biz, x.q, now, orderID, "RELEASE"); err != nil {
				return err
			}
		}
		if err := a.releaseCapacity(tx, orderID, storeID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE orders SET status=?, payment_status=?, updated_at=? WHERE id=?`, domain.OrderCancelled, domain.PayClosed, now, orderID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE payments SET status=?, updated_at=? WHERE order_id=?`, domain.PayClosed, now, orderID); err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO order_events (id, order_id, store_id, from_status, to_status, actor_type, created_at) VALUES (?,?,?,?,?,?,?)`,
			ids.New(), orderID, storeID, domain.OrderPendingPayment, domain.OrderCancelled, actor, now)
		return err
	})
}

func releaseStockFixed(tx *sql.Tx, storeID, skuID, biz string, qty int, now time.Time, orderID, reason string) error {
	biz = bizDate(biz)
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
		ids.New(), storeID, skuID, biz, qty, reason, orderID, now)
	return err
}

func restockSold(tx *sql.Tx, storeID, skuID, biz string, qty int, now time.Time, orderID string) error {
	biz = bizDate(biz)
	var mode string
	if err := tx.QueryRow(`SELECT stock_mode FROM skus WHERE id=?`, skuID).Scan(&mode); err != nil {
		return err
	}
	if mode != domain.StockDaily {
		return nil
	}
	if _, err := tx.Exec(`UPDATE daily_inventory SET sold_qty=GREATEST(sold_qty-?,0) WHERE store_id=? AND sku_id=? AND business_date=?`, qty, storeID, skuID, biz); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO inventory_movements (id, store_id, sku_id, business_date, delta, reason, order_id, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		ids.New(), storeID, skuID, biz, qty, "REFUND", orderID, now)
	return err
}

func (a *App) TransitionOrder(ctx context.Context, storeID, actorID, orderID, to string, version int) (OrderDTO, error) {
	now := a.Now()
	err := persistTx(ctx, a, func(tx *sql.Tx) error {
		var status string
		var ver int
		if err := tx.QueryRow(`SELECT status, version FROM orders WHERE id=? AND store_id=? FOR UPDATE`, orderID, storeID).Scan(&status, &ver); err != nil {
			if err == sql.ErrNoRows {
				return apperr.NotFound
			}
			return err
		}
		if ver != version || !domain.CanStaffTransition(status, to) {
			return apperr.StateConflict
		}
		if _, err := tx.Exec(`UPDATE orders SET status=?, version=version+1, updated_at=? WHERE id=? AND version=?`, to, now, orderID, version); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO order_events (id, order_id, store_id, from_status, to_status, actor_type, actor_id, created_at) VALUES (?,?,?,?,?,?,?,?)`,
			ids.New(), orderID, storeID, status, to, "ADMIN", actorID, now)
		return err
	})
	if err != nil {
		return OrderDTO{}, err
	}
	return a.GetOrder(ctx, storeID, orderID, "", false)
}

func (a *App) Board(ctx context.Context, storeID, q string) (map[string][]OrderDTO, error) {
	query := `SELECT id FROM orders WHERE store_id=? AND status IN ('PAID','ACCEPTED','PREPARING','READY')`
	args := []any{storeID}
	if strings.TrimSpace(q) != "" {
		query += ` AND (id LIKE ? OR pickup_number LIKE ? OR table_no LIKE ?)`
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	query += ` ORDER BY (pickup_type='SCHEDULED') ASC, scheduled_for ASC, created_at ASC`
	rows, err := a.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]OrderDTO{
		domain.OrderPaid: {}, domain.OrderAccepted: {}, domain.OrderPreparing: {}, domain.OrderReady: {},
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		o, err := a.GetOrder(ctx, storeID, id, "", false)
		if err != nil {
			return nil, err
		}
		out[o.Status] = append(out[o.Status], o)
	}
	return out, rows.Err()
}

func (a *App) payConfig(ctx context.Context, storeID string) (wechat.PayConfig, error) {
	var appid sql.NullString
	_ = a.DB.QueryRowContext(ctx, `SELECT wechat_appid FROM stores WHERE id=?`, storeID).Scan(&appid)
	var mch, serial sql.NullString
	err := a.DB.QueryRowContext(ctx, `SELECT mch_id, serial_no FROM payment_configs WHERE store_id=? AND status='ready'`, storeID).Scan(&mch, &serial)
	if err != nil {
		return wechat.PayConfig{}, apperr.PaymentUnavailable
	}
	return wechat.PayConfig{AppID: scanNullString(appid), MchID: scanNullString(mch), SerialNo: scanNullString(serial)}, nil
}
