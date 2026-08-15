package app

import (
	"context"
	"database/sql"
	"time"
)

func (a *App) AnalyticsOverview(ctx context.Context, storeID, start, end string) (map[string]any, error) {
	cur, err := a.periodStats(ctx, storeID, start, end)
	if err != nil {
		return nil, err
	}
	st, _ := time.Parse("2006-01-02", start)
	en, _ := time.Parse("2006-01-02", end)
	dur := en.Sub(st)
	prevStart := st.Add(-dur - 24*time.Hour).Format("2006-01-02")
	prevEnd := st.Add(-24 * time.Hour).Format("2006-01-02")
	prev, _ := a.periodStats(ctx, storeID, prevStart, prevEnd)
	return map[string]any{"current": cur, "previous": prev, "changes": pctMap(cur, prev)}, nil
}

func pctMap(cur, prev map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range cur {
		cv, ok1 := asFloat(v)
		pv, ok2 := asFloat(prev[k])
		if !ok1 || !ok2 || pv == 0 {
			out[k] = nil
			continue
		}
		out[k] = (cv - pv) / pv
	}
	return out
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case float64:
		return t, true
	default:
		return 0, false
	}
}

func (a *App) periodStats(ctx context.Context, storeID, start, end string) (map[string]any, error) {
	var paidAmt, refundAmt sql.NullInt64
	var paidN, refundN, customers, newC, oldC sql.NullInt64
	_ = a.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(payable_cents),0), COUNT(*) FROM orders WHERE store_id=? AND payment_status='SUCCESS' AND paid_at>=? AND paid_at<?`,
		storeID, start, end+" 23:59:59").Scan(&paidAmt, &paidN)
	_ = a.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0), COUNT(*) FROM refunds WHERE store_id=? AND status='SUCCESS' AND updated_at>=? AND updated_at<?`,
		storeID, start, end+" 23:59:59").Scan(&refundAmt, &refundN)
	_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(DISTINCT customer_id) FROM orders WHERE store_id=? AND payment_status='SUCCESS' AND paid_at>=? AND paid_at<?`,
		storeID, start, end+" 23:59:59").Scan(&customers)
	aov := int64(0)
	if paidN.Int64 > 0 {
		aov = (paidAmt.Int64 - refundAmt.Int64) / paidN.Int64
	}
	return map[string]any{
		"paid_amount_cents": paidAmt.Int64, "refund_amount_cents": refundAmt.Int64,
		"net_amount_cents": paidAmt.Int64 - refundAmt.Int64,
		"paid_orders": paidN.Int64, "refund_orders": refundN.Int64,
		"customers": customers.Int64, "new_customers": newC.Int64, "old_customers": oldC.Int64,
		"aov_cents": aov,
	}, nil
}

func (a *App) AnalyticsTrends(ctx context.Context, storeID, start, end, grain string) ([]map[string]any, error) {
	q := `SELECT DATE(paid_at) d, COALESCE(SUM(payable_cents),0) FROM orders WHERE store_id=? AND payment_status='SUCCESS' AND paid_at>=? AND paid_at<? GROUP BY d ORDER BY d`
	if grain == "hour" {
		q = `SELECT DATE_FORMAT(paid_at, '%Y-%m-%d %H:00:00') d, COALESCE(SUM(payable_cents),0) FROM orders WHERE store_id=? AND payment_status='SUCCESS' AND paid_at>=? AND paid_at<? GROUP BY d ORDER BY d`
	}
	rows, err := a.DB.QueryContext(ctx, q, storeID, start, end+" 23:59:59")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var bucket string
		var amt int64
		if err := rows.Scan(&bucket, &amt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"bucket": bucket, "paid_amount_cents": amt})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) AnalyticsBreakdown(ctx context.Context, storeID, start, end string) (map[string]any, error) {
	scene := []map[string]any{}
	rows, err := a.DB.QueryContext(ctx, `SELECT scene, COUNT(*), COALESCE(SUM(payable_cents),0) FROM orders WHERE store_id=? AND payment_status='SUCCESS' AND paid_at>=? AND paid_at<? GROUP BY scene`, storeID, start, end+" 23:59:59")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s string
		var n, amt int64
		if err := rows.Scan(&s, &n, &amt); err != nil {
			rows.Close()
			return nil, err
		}
		scene = append(scene, map[string]any{"scene": s, "orders": n, "amount_cents": amt})
	}
	rows.Close()
	hours := []map[string]any{}
	rows, err = a.DB.QueryContext(ctx, `SELECT HOUR(paid_at), COUNT(*), COALESCE(SUM(payable_cents),0) FROM orders WHERE store_id=? AND payment_status='SUCCESS' AND paid_at>=? AND paid_at<? GROUP BY HOUR(paid_at)`, storeID, start, end+" 23:59:59")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var h int
		var n, amt int64
		if err := rows.Scan(&h, &n, &amt); err != nil {
			rows.Close()
			return nil, err
		}
		hours = append(hours, map[string]any{"hour": h, "orders": n, "amount_cents": amt})
	}
	rows.Close()
	items := []map[string]any{}
	rows, err = a.DB.QueryContext(ctx, `SELECT oi.product_name, SUM(oi.qty), SUM(oi.line_total_cents) FROM order_items oi JOIN orders o ON o.id=oi.order_id WHERE o.store_id=? AND o.payment_status='SUCCESS' AND o.paid_at>=? AND o.paid_at<? GROUP BY oi.product_name ORDER BY SUM(oi.qty) DESC LIMIT 20`, storeID, start, end+" 23:59:59")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var qty, amt int64
		if err := rows.Scan(&name, &qty, &amt); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"product_name": name, "qty": qty, "amount_cents": amt})
	}
	return map[string]any{"scenes": scene, "hours": hours, "products": items}, rows.Err()
}

func (a *App) AnalyticsCustomers(ctx context.Context, storeID string) (map[string]any, error) {
	since := a.Now().Add(-180 * 24 * time.Hour)
	var ordered, active, silent, neu, old int
	_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(DISTINCT customer_id) FROM orders WHERE store_id=? AND payment_status='SUCCESS'`, storeID).Scan(&ordered)
	_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(DISTINCT customer_id) FROM orders WHERE store_id=? AND payment_status='SUCCESS' AND paid_at>=?`, storeID, since).Scan(&active)
	silent = ordered - active
	if silent < 0 {
		silent = 0
	}
	return map[string]any{
		"ordered": ordered, "active_180": active, "silent_180": silent,
		"new_customers": neu, "old_customers": old,
	}, nil
}

func (a *App) ExportOrdersCSV(ctx context.Context, storeID string, statuses []string, scene, pay, q, start, end string) (string, error) {
	query := `SELECT id, pickup_number, scene, pickup_type, scheduled_for, status, payment_status, payable_cents, created_at FROM orders WHERE store_id=?`
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
	query += ` ORDER BY created_at DESC LIMIT 5000`
	rows, err := a.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	out := "id,pickup_number,scene,pickup_type,scheduled_for,status,payment_status,payable_cents,created_at\n"
	for rows.Next() {
		var id, scene, ptype, status, pay string
		var no sql.NullString
		var sched sql.NullTime
		var amt int64
		var created time.Time
		if err := rows.Scan(&id, &no, &scene, &ptype, &sched, &status, &pay, &amt, &created); err != nil {
			return "", err
		}
		sf := ""
		if sched.Valid {
			sf = sched.Time.UTC().Format(time.RFC3339)
		}
		out += id + "," + scanNullString(no) + "," + scene + "," + ptype + "," + sf + "," + status + "," + pay + "," + itoa(amt) + "," + created.UTC().Format(time.RFC3339) + "\n"
	}
	return out, rows.Err()
}

func itoa(v int64) string {
	return sql.NullString{String: "", Valid: false}.String + func() string {
		s := time.Unix(0, 0).Format("")
		_ = s
		return formatInt(v)
	}()
}

func formatInt(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [32]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
