package app

import (
	"context"
	"database/sql"

	"github.com/changerr/dishflow-grok/internal/domain"
	"github.com/changerr/dishflow-grok/internal/ids"
)

func (a *App) ReconcilePayments(ctx context.Context) error {
	rows, err := a.DB.QueryContext(ctx, `SELECT p.order_id, p.store_id, p.status, p.expires_at, p.is_mock
		FROM payments p JOIN orders o ON o.id=p.order_id
		WHERE o.status='PENDING_PAYMENT' AND p.status IN ('UNPAID','PREPAY_CREATED')
		ORDER BY p.created_at LIMIT 50`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		oid, store, status string
		mock               bool
	}
	var list []row
	for rows.Next() {
		var r row
		var exp sql.NullTime
		if err := rows.Scan(&r.oid, &r.store, &r.status, &exp, &r.mock); err != nil {
			return err
		}
		_ = exp
		list = append(list, r)
	}
	for _, r := range list {
		if r.mock || a.Cfg.DevMode && r.status != domain.PayPrepayCreated {
			var expAt sql.NullTime
			_ = a.DB.QueryRowContext(ctx, `SELECT expires_at FROM payments WHERE order_id=?`, r.oid).Scan(&expAt)
			if expAt.Valid && a.Now().After(expAt.Time) {
				_ = a.closeUnpaid(ctx, r.store, r.oid, "SYSTEM")
			}
			continue
		}
		cfg, err := a.payConfig(ctx, r.store)
		if err != nil {
			a.recordException(ctx, r.store, r.oid, "PAY_QUERY", err.Error())
			continue
		}
		q, err := a.Wechat.QueryOrder(ctx, cfg, r.oid)
		if err != nil {
			a.recordException(ctx, r.store, r.oid, "PAY_QUERY", err.Error())
			continue
		}
		switch q.TradeState {
		case "SUCCESS":
			if q.Amount != 0 {
				var payable int64
				_ = a.DB.QueryRowContext(ctx, `SELECT payable_cents FROM orders WHERE id=?`, r.oid).Scan(&payable)
				if q.Amount != payable {
					a.recordException(ctx, r.store, r.oid, "PAY_QUERY", "金额不符")
					continue
				}
			}
			_ = a.ConfirmPaid(ctx, r.store, r.oid, q.TxnID, false)
		case "CLOSED", "REVOKED", "PAYERROR":
			_ = a.closeUnpaid(ctx, r.store, r.oid, "SYSTEM")
		case "NOTPAY":
			var expAt sql.NullTime
			_ = a.DB.QueryRowContext(ctx, `SELECT expires_at FROM payments WHERE order_id=?`, r.oid).Scan(&expAt)
			if expAt.Valid && a.Now().After(expAt.Time) {
				_ = a.Wechat.CloseOrder(ctx, cfg, r.oid)
				_ = a.closeUnpaid(ctx, r.store, r.oid, "SYSTEM")
			}
		}
	}
	return nil
}

func (a *App) ReconcileRefunds(ctx context.Context) error {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, store_id, order_id, merchant_refund_no, amount_cents, status, is_mock FROM refunds WHERE status IN ('CREATED','PROCESSING','ABNORMAL') ORDER BY created_at LIMIT 50`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		id, store, oid, no, status string
		amount                     int64
		mock                       bool
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.store, &r.oid, &r.no, &r.amount, &r.status, &r.mock); err != nil {
			return err
		}
		list = append(list, r)
	}
	for _, r := range list {
		if r.mock || a.Cfg.DevMode {
			_ = a.ConfirmRefundSuccess(ctx, r.store, r.oid, "MOCK_"+r.no)
			continue
		}
		cfg, err := a.payConfig(ctx, r.store)
		if err != nil {
			a.recordException(ctx, r.store, r.no, "REFUND_QUERY", err.Error())
			continue
		}
		if r.status == domain.RefundCreated {
			notify := a.Cfg.PublicBaseURL + "/callbacks/wechat-pay/refunds/" + r.store
			if err := a.Wechat.Refund(ctx, cfg, r.oid, r.no, r.amount, "退款", notify); err != nil {
				a.recordException(ctx, r.store, r.no, "REFUND_CREATE", err.Error())
				_, _ = a.DB.ExecContext(ctx, `UPDATE refunds SET status=?, updated_at=? WHERE id=?`, domain.RefundAbnormal, a.Now(), r.id)
				continue
			}
			_, _ = a.DB.ExecContext(ctx, `UPDATE refunds SET status=?, updated_at=? WHERE id=?`, domain.RefundProcessing, a.Now(), r.id)
			r.status = domain.RefundProcessing
		}
		q, err := a.Wechat.QueryRefund(ctx, cfg, r.no)
		if err != nil {
			a.recordException(ctx, r.store, r.no, "REFUND_QUERY", err.Error())
			continue
		}
		switch q.TradeState {
		case "SUCCESS":
			_ = a.ConfirmRefundSuccess(ctx, r.store, r.oid, q.TxnID)
		case "ABNORMAL", "CLOSED":
			_, _ = a.DB.ExecContext(ctx, `UPDATE refunds SET status=?, updated_at=? WHERE id=?`, domain.RefundAbnormal, a.Now(), r.id)
			a.recordException(ctx, r.store, r.no, "REFUND_QUERY", q.TradeState)
		}
	}
	return nil
}

func (a *App) recordException(ctx context.Context, storeID, bizNo, typ, msg string) {
	now := a.Now()
	_, _ = a.DB.ExecContext(ctx, `INSERT INTO exceptions (id, store_id, biz_no, error_type, status, message, retries, created_at, updated_at)
		VALUES (?,?,?,?, 'OPEN', ?, 0, ?, ?)`, ids.New(), storeID, bizNo, typ, clip(msg, 512), now, now)
}
