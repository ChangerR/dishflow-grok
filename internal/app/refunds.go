package app

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/domain"
	"github.com/changerr/dishflow-grok/internal/ids"
)

func (a *App) startRefund(ctx context.Context, storeID, orderID, reason, source, actorID string, auto bool) error {
	now := a.Now()
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		var status, payStatus string
		var payable int64
		var mock bool
		var existing sql.NullString
		err := tx.QueryRow(`SELECT status, payment_status, payable_cents, is_mock FROM orders WHERE id=? AND store_id=? FOR UPDATE`, orderID, storeID).
			Scan(&status, &payStatus, &payable, &mock)
		if err == sql.ErrNoRows {
			return apperr.NotFound
		}
		if err != nil {
			return err
		}
		if payStatus != domain.PaySuccess && status != domain.OrderPaid && status != domain.OrderAccepted && status != domain.OrderPreparing && status != domain.OrderReady && status != domain.OrderCompleted && status != domain.OrderCancelRequested {
			return apperr.StateConflict
		}
		err = tx.QueryRow(`SELECT id FROM refunds WHERE order_id=?`, orderID).Scan(&existing)
		if err == nil {
			return apperr.RefundConflict
		}
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		refundID := ids.New()
		no := "R" + refundID
		if _, err := tx.Exec(`INSERT INTO refunds (id, store_id, order_id, merchant_refund_no, amount_cents, reason, status, source, is_mock, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`, refundID, storeID, orderID, no, payable, reason, domain.RefundCreated, source, mock, now, now); err != nil {
			if isDup(err) {
				return apperr.RefundConflict
			}
			return err
		}
		if _, err := tx.Exec(`UPDATE orders SET status=?, refund_status=?, version=version+1, updated_at=? WHERE id=?`, domain.OrderRefunding, domain.RefundCreated, now, orderID); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO order_events (id, order_id, store_id, from_status, to_status, actor_type, actor_id, note, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			ids.New(), orderID, storeID, status, domain.OrderRefunding, actorType(source), nullS(actorID), reason, now); err != nil {
			return err
		}
		return a.Outbox(tx, storeID, "refund.created", map[string]string{"refund_id": refundID, "order_id": orderID})
	})
}

func actorType(source string) string {
	if strings.HasPrefix(source, "CUSTOMER") {
		return "CUSTOMER"
	}
	if source == "SYSTEM" {
		return "SYSTEM"
	}
	return "ADMIN"
}

func (a *App) StoreRefund(ctx context.Context, storeID, actorID, orderID, reason string) error {
	if strings.TrimSpace(reason) == "" {
		reason = "门店主动全额退款"
	}
	return a.startRefund(ctx, storeID, orderID, reason, "STORE", actorID, false)
}

func (a *App) ReviewCancelRefund(ctx context.Context, storeID, actorID, refundOrOrderID, decision, note string) error {
	now := a.Now()
	var orderID, status string
	err := a.DB.QueryRowContext(ctx, `SELECT id, status FROM orders WHERE store_id=? AND id=?`, storeID, refundOrOrderID).Scan(&orderID, &status)
	if err == sql.ErrNoRows {
		err = a.DB.QueryRowContext(ctx, `SELECT order_id FROM refunds WHERE id=? AND store_id=?`, refundOrOrderID, storeID).Scan(&orderID)
		if err != nil {
			return apperr.NotFound
		}
		_ = a.DB.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=?`, orderID).Scan(&status)
	} else if err != nil {
		return err
	}
	if status != domain.OrderCancelRequested {
		return a.reviewByRefundID(ctx, storeID, actorID, refundOrOrderID, decision, note)
	}
	if decision == "REJECTED" {
		return persistTx(ctx, a, func(tx *sql.Tx) error {
			if _, err := tx.Exec(`UPDATE orders SET status=?, version=version+1, updated_at=? WHERE id=? AND status=?`, domain.OrderAccepted, now, orderID, domain.OrderCancelRequested); err != nil {
				return err
			}
			_, err := tx.Exec(`INSERT INTO order_events (id, order_id, store_id, from_status, to_status, actor_type, actor_id, note, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
				ids.New(), orderID, storeID, domain.OrderCancelRequested, domain.OrderAccepted, "ADMIN", actorID, note, now)
			return err
		})
	}
	if decision != "APPROVED" {
		return apperr.Validation("decision 无效")
	}
	return a.startRefund(ctx, storeID, orderID, firstNonEmpty(note, "审核通过顾客取消"), "REVIEW", actorID, false)
}

func (a *App) reviewByRefundID(ctx context.Context, storeID, actorID, refundID, decision, note string) error {
	var orderID, st string
	err := a.DB.QueryRowContext(ctx, `SELECT r.order_id, o.status FROM refunds r JOIN orders o ON o.id=r.order_id WHERE r.id=? AND r.store_id=?`, refundID, storeID).
		Scan(&orderID, &st)
	if err != nil {
		return apperr.NotFound
	}
	_ = actorID
	if decision == "REJECTED" && st == domain.OrderCancelRequested {
		return a.ReviewCancelRefund(ctx, storeID, actorID, orderID, decision, note)
	}
	return apperr.StateConflict
}

func (a *App) ConfirmRefundSuccess(ctx context.Context, storeID, orderID, wechatRefundID string) error {
	now := a.Now()
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		var status string
		var points int
		var reversed bool
		var customerID, biz string
		err := tx.QueryRow(`SELECT status, points_awarded, points_reversed, customer_id, pickup_business_date FROM orders WHERE id=? AND store_id=? FOR UPDATE`, orderID, storeID).
			Scan(&status, &points, &reversed, &customerID, &biz)
		if err != nil {
			return err
		}
		biz = bizDate(biz)
		if status == domain.OrderRefunded {
			return nil
		}
		if _, err := tx.Exec(`UPDATE refunds SET status=?, wechat_refund_id=?, updated_at=? WHERE order_id=?`, domain.RefundSuccess, wechatRefundID, now, orderID); err != nil {
			return err
		}
		if err := a.releaseCapacity(tx, orderID, storeID); err != nil {
			return err
		}
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
			if err := restockSold(tx, storeID, l.sku, biz, l.qty, now, orderID); err != nil {
				return err
			}
		}
		if !reversed && points > 0 {
			var membershipID string
			err := tx.QueryRow(`SELECT id FROM customer_memberships WHERE store_id=? AND customer_id=?`, storeID, customerID).Scan(&membershipID)
			if err == nil {
				if _, err := tx.Exec(`UPDATE customer_memberships SET points_balance=GREATEST(points_balance-?,0) WHERE id=?`, points, membershipID); err != nil {
					return err
				}
				var bal int
				_ = tx.QueryRow(`SELECT points_balance FROM customer_memberships WHERE id=?`, membershipID).Scan(&bal)
				if _, err := tx.Exec(`INSERT INTO member_points_ledger (id, store_id, membership_id, delta, balance_after, reason_type, ref_id, created_at) VALUES (?,?,?,?,?,?,?,?)`,
					ids.New(), storeID, membershipID, -points, bal, "ORDER_REFUND", orderID, now); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(`UPDATE orders SET points_reversed=1 WHERE id=?`, orderID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`UPDATE orders SET status=?, refund_status=?, version=version+1, updated_at=? WHERE id=?`, domain.OrderRefunded, domain.RefundSuccess, now, orderID); err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO order_events (id, order_id, store_id, from_status, to_status, actor_type, created_at) VALUES (?,?,?,?,?,?,?)`,
			ids.New(), orderID, storeID, status, domain.OrderRefunded, "SYSTEM", now)
		return err
	})
}

func (a *App) ListRefunds(ctx context.Context, storeID string) ([]map[string]any, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, order_id, merchant_refund_no, amount_cents, reason, status, source, is_mock, created_at FROM refunds WHERE store_id=? ORDER BY created_at DESC LIMIT 200`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, oid, no, reason, status, source string
		var amount int64
		var mock bool
		var created time.Time
		if err := rows.Scan(&id, &oid, &no, &amount, &reason, &status, &source, &mock, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "order_id": oid, "merchant_refund_no": no, "amount_cents": amount,
			"reason": reason, "status": status, "source": source, "is_mock": mock,
			"created_at": created.UTC().Format(time.RFC3339),
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) ListExceptions(ctx context.Context, storeID string) ([]map[string]any, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, biz_no, error_type, status, message, retries, created_at FROM exceptions WHERE store_id=? ORDER BY created_at DESC LIMIT 200`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, biz, et, st, msg string
		var retries int
		var created time.Time
		if err := rows.Scan(&id, &biz, &et, &st, &msg, &retries, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "biz_no": biz, "error_type": et, "status": st, "message": msg, "retries": retries, "created_at": created.UTC().Format(time.RFC3339)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) RetryException(ctx context.Context, storeID, id string) error {
	res, err := a.DB.ExecContext(ctx, `UPDATE exceptions SET status='QUEUED', retries=retries+1, updated_at=? WHERE id=? AND store_id=?`, a.Now(), id, storeID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperr.NotFound
	}
	return nil
}

func (a *App) AdminListOrders(ctx context.Context, storeID, cursor string, statuses []string, scene, pay, q string, start, end string) (map[string]any, error) {
	query := `SELECT id FROM orders WHERE store_id=?`
	args := []any{storeID}
	statuses = cleanList(statuses)
	if len(statuses) > 0 {
		query += ` AND status IN (` + placeholders(len(statuses)) + `)`
		for _, s := range statuses {
			args = append(args, s)
		}
	}
	if scene != "" {
		query += ` AND scene=?`
		args = append(args, scene)
	}
	if pay != "" {
		query += ` AND payment_status=?`
		args = append(args, pay)
	}
	if q != "" {
		query += ` AND (id LIKE ? OR pickup_number LIKE ? OR table_no LIKE ?)`
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	if start != "" {
		query += ` AND created_at>=?`
		args = append(args, start)
	}
	if end != "" {
		query += ` AND created_at<=?`
		args = append(args, end)
	}
	if cursor != "" {
		query += ` AND id < ?`
		args = append(args, cursor)
	}
	query += ` ORDER BY id DESC LIMIT 21`
	rows, err := a.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var idsList []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		idsList = append(idsList, id)
	}
	next := ""
	if len(idsList) > 20 {
		next = idsList[20]
		idsList = idsList[:20]
	}
	items := []OrderDTO{}
	for _, id := range idsList {
		o, err := a.GetOrder(ctx, storeID, id, "", false)
		if err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return map[string]any{"items": items, "next_cursor": next}, nil
}

func cleanList(ss []string) []string {
	var out []string
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
