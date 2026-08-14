package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/cryptoutil"
	"github.com/changerr/dishflow-grok/internal/domain"
	"github.com/changerr/dishflow-grok/internal/ids"
	"github.com/changerr/dishflow-grok/internal/print"
)

func (a *App) PrintConfigGet(ctx context.Context, storeID string) (map[string]any, error) {
	var appid sql.NullString
	var secret sql.NullString
	var auto, mock bool
	var status string
	err := a.DB.QueryRowContext(ctx, `SELECT appid, appsecret_enc, auto_print, mock_print, status FROM print_configs WHERE store_id=?`, storeID).
		Scan(&appid, &secret, &auto, &mock, &status)
	if err == sql.ErrNoRows {
		return map[string]any{"status": "draft", "auto_print": false, "mock_print": false, "configured": false}, nil
	}
	if err != nil {
		return nil, err
	}
	configured := appid.Valid && appid.String != "" && secret.Valid && secret.String != ""
	if status == "" {
		if configured {
			status = "ready"
		} else {
			status = "draft"
		}
	}
	return map[string]any{"status": status, "auto_print": auto, "mock_print": mock, "configured": configured, "appid_set": appid.Valid && appid.String != ""}, nil
}

func (a *App) PrintConfigPut(ctx context.Context, storeID string, in map[string]any) (map[string]any, error) {
	now := a.Now()
	_, _ = a.DB.ExecContext(ctx, `INSERT IGNORE INTO print_configs (store_id, status, updated_at) VALUES (?, 'draft', ?)`, storeID, now)
	if v, ok := in["appid"].(string); ok && v != "" {
		_, _ = a.DB.ExecContext(ctx, `UPDATE print_configs SET appid=?, updated_at=? WHERE store_id=?`, v, now, storeID)
	}
	if v, ok := in["appsecret"].(string); ok && v != "" {
		enc, err := cryptoutil.Encrypt(a.Cfg.CredentialKey, []byte(v))
		if err != nil {
			return nil, err
		}
		_, _ = a.DB.ExecContext(ctx, `UPDATE print_configs SET appsecret_enc=?, updated_at=? WHERE store_id=?`, enc, now, storeID)
	}
	if v, ok := in["auto_print"].(bool); ok {
		_, _ = a.DB.ExecContext(ctx, `UPDATE print_configs SET auto_print=?, updated_at=? WHERE store_id=?`, v, now, storeID)
	}
	if v, ok := in["mock_print"].(bool); ok {
		_, _ = a.DB.ExecContext(ctx, `UPDATE print_configs SET mock_print=?, updated_at=? WHERE store_id=?`, v, now, storeID)
	}
	cfg, _ := a.PrintConfigGet(ctx, storeID)
	if cfg["configured"] == true && cfg["status"] != "disabled" {
		_, _ = a.DB.ExecContext(ctx, `UPDATE print_configs SET status='ready', updated_at=? WHERE store_id=? AND status<>'disabled'`, now, storeID)
	}
	return a.PrintConfigGet(ctx, storeID)
}

func (a *App) ListPrinters(ctx context.Context, storeID string) ([]map[string]any, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, sn, name, is_default, copies, enabled, online, note FROM cloud_printers WHERE store_id=? ORDER BY is_default DESC, name`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, sn, name string
		var note sql.NullString
		var def, en, online bool
		var copies int
		if err := rows.Scan(&id, &sn, &name, &def, &copies, &en, &online, &note); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "sn": sn, "name": name, "is_default": def, "copies": copies, "enabled": en, "online": online, "note": scanNullString(note)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) SavePrinter(ctx context.Context, storeID, id string, in map[string]any) (map[string]any, error) {
	sn, _ := in["sn"].(string)
	name, _ := in["name"].(string)
	key, _ := in["key"].(string)
	if sn == "" || name == "" {
		return nil, apperr.Validation("SN 和名称不能为空")
	}
	copies := 1
	if v, ok := in["copies"].(float64); ok {
		copies = int(v)
	}
	if copies < 1 || copies > 5 {
		return nil, apperr.Validation("份数必须为 1～5")
	}
	now := a.Now()
	enabled := true
	if v, ok := in["enabled"].(bool); ok {
		enabled = v
	}
	def, _ := in["is_default"].(bool)
	note, _ := in["note"].(string)
	if id == "" {
		if key == "" {
			return nil, apperr.Validation("KEY 不能为空")
		}
		enc, err := cryptoutil.Encrypt(a.Cfg.CredentialKey, []byte(key))
		if err != nil {
			return nil, err
		}
		id = a.NewID()
		if def {
			_, _ = a.DB.ExecContext(ctx, `UPDATE cloud_printers SET is_default=0 WHERE store_id=?`, storeID)
		}
		_, err = a.DB.ExecContext(ctx, `INSERT INTO cloud_printers (id, store_id, sn, key_enc, name, is_default, copies, enabled, online, note, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,0,?,?,?)`,
			id, storeID, sn, enc, name, def, copies, enabled, nullS(note), now, now)
		if err != nil {
			return nil, err
		}
	} else {
		if def {
			_, _ = a.DB.ExecContext(ctx, `UPDATE cloud_printers SET is_default=0 WHERE store_id=?`, storeID)
		}
		q := `UPDATE cloud_printers SET sn=?, name=?, is_default=?, copies=?, enabled=?, note=?, updated_at=?`
		args := []any{sn, name, def, copies, enabled, nullS(note), now}
		if key != "" {
			enc, err := cryptoutil.Encrypt(a.Cfg.CredentialKey, []byte(key))
			if err != nil {
				return nil, err
			}
			q += `, key_enc=?`
			args = append(args, enc)
		}
		q += ` WHERE id=? AND store_id=?`
		args = append(args, id, storeID)
		_, err := a.DB.ExecContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{"id": id, "sn": sn, "name": name}, nil
}

func (a *App) DeletePrinter(ctx context.Context, storeID, id, role string) error {
	if role != domain.RoleOwner {
		return apperr.Forbidden
	}
	res, err := a.DB.ExecContext(ctx, `DELETE FROM cloud_printers WHERE id=? AND store_id=?`, id, storeID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperr.NotFound
	}
	return nil
}

func (a *App) enqueuePrint(ctx context.Context, storeID, typ, orderID, listID, content string) error {
	var n int
	if typ == "order" && orderID != "" {
		_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM cloud_print_jobs WHERE store_id=? AND order_id=? AND type='order'`, storeID, orderID).Scan(&n)
		if n > 0 {
			return nil
		}
	}
	_, err := a.DB.ExecContext(ctx, `INSERT INTO cloud_print_jobs (id, store_id, type, status, order_id, purchase_list_id, content, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		ids.New(), storeID, typ, domain.PrintQueued, nullS(orderID), nullS(listID), content, a.Now(), a.Now())
	return err
}

func (a *App) CloudPrintOrder(ctx context.Context, storeID, orderID string, reprint bool) error {
	o, err := a.GetOrder(ctx, storeID, orderID, "", false)
	if err != nil {
		return err
	}
	store, _ := a.PublicStore(ctx, storeID)
	content := print.RenderOrder(print.Ticket{
		StoreName: store.Name, PickupNumber: o.PickupNumber, TableNo: o.TableNo,
		PickupType: o.PickupType, ScheduledFor: strPtr(o.ScheduledFor), Remark: o.Remark,
		PayableCents: o.PayableCents, IsMock: o.IsMock,
	})
	typ := "order"
	if reprint {
		typ = "reprint"
	}
	return a.enqueuePrint(ctx, storeID, typ, orderID, "", content)
}

func strPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (a *App) CloudPrintPurchase(ctx context.Context, storeID, listID string) error {
	l, err := a.GetPurchaseList(ctx, storeID, listID)
	if err != nil {
		return err
	}
	lines := []string{fmt.Sprintf("采购 %s", l["list_no"]), l["title"].(string)}
	if items, ok := l["items"].([]map[string]any); ok {
		for _, it := range items {
			lines = append(lines, fmt.Sprintf("%v %v%v", it["name"], it["qty"], it["unit"]))
		}
	}
	return a.enqueuePrint(ctx, storeID, "purchase", "", listID, strings.Join(lines, "\n"))
}

func (a *App) TestPrinter(ctx context.Context, storeID, printerID string) error {
	return a.enqueuePrint(ctx, storeID, "test", "", "", "DishFlow 测试打印\n"+printerID)
}

func (a *App) RefreshPrinter(ctx context.Context, storeID, printerID string) error {
	_, err := a.DB.ExecContext(ctx, `UPDATE cloud_printers SET online=1, updated_at=? WHERE id=? AND store_id=?`, a.Now(), printerID, storeID)
	return err
}

func (a *App) ListPrintJobs(ctx context.Context, storeID string) ([]map[string]any, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, type, status, order_id, purchase_list_id, printer_id, shangpeng_id, attempts, error_message, created_at FROM cloud_print_jobs WHERE store_id=? ORDER BY created_at DESC LIMIT 100`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, typ, status string
		var oid, lid, pid, sp, em sql.NullString
		var attempts int
		var created time.Time
		if err := rows.Scan(&id, &typ, &status, &oid, &lid, &pid, &sp, &attempts, &em, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "type": typ, "status": status, "order_id": scanNullString(oid),
			"purchase_list_id": scanNullString(lid), "printer_id": scanNullString(pid),
			"shangpeng_id": scanNullString(sp), "attempts": attempts, "error": scanNullString(em),
			"created_at": created.UTC().Format(time.RFC3339),
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) ProcessPrintJobs(ctx context.Context) error {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, store_id, type, order_id, content FROM cloud_print_jobs WHERE status IN ('QUEUED','SENDING') ORDER BY created_at LIMIT 20`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type job struct{ id, store, typ, oid, content string }
	var jobs []job
	for rows.Next() {
		var j job
		var oid sql.NullString
		if err := rows.Scan(&j.id, &j.store, &j.typ, &oid, &j.content); err != nil {
			return err
		}
		j.oid = scanNullString(oid)
		jobs = append(jobs, j)
	}
	for _, j := range jobs {
		var mock bool
		_ = a.DB.QueryRowContext(ctx, `SELECT COALESCE(mock_print,0) FROM print_configs WHERE store_id=?`, j.store).Scan(&mock)
		now := a.Now()
		_, _ = a.DB.ExecContext(ctx, `UPDATE cloud_print_jobs SET status=?, attempts=attempts+1, updated_at=? WHERE id=?`, domain.PrintSubmitted, now, j.id)
		_, _ = a.DB.ExecContext(ctx, `UPDATE cloud_print_jobs SET status=?, updated_at=? WHERE id=?`, domain.PrintPrinted, now, j.id)
		if j.typ == "order" && j.oid != "" {
			var st string
			_ = a.DB.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=?`, j.oid).Scan(&st)
			if st == domain.OrderPaid {
				_, _ = a.TransitionOrder(ctx, j.store, "", j.oid, domain.OrderAccepted, 1)
			}
		}
		_ = mock
	}
	return nil
}

func (a *App) DispatchOutbox(ctx context.Context) error {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, store_id, event_type, payload FROM outbox WHERE published_at IS NULL ORDER BY created_at LIMIT 50`)
	if err != nil {
		return err
	}
	defer rows.Close()
	now := a.Now()
	for rows.Next() {
		var id, store, et, payload string
		if err := rows.Scan(&id, &store, &et, &payload); err != nil {
			return err
		}
		if et == "order.paid" {
			var m map[string]string
			_ = jsonUnmarshal(payload, &m)
			_ = a.CloudPrintOrder(ctx, store, m["order_id"], false)
		}
		_, _ = a.DB.ExecContext(ctx, `UPDATE outbox SET published_at=? WHERE id=?`, now, id)
	}
	return rows.Err()
}

func jsonUnmarshal(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}
