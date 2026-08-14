package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/cryptoutil"
	"github.com/changerr/dishflow-grok/internal/domain"
)

func (a *App) AdminStore(ctx context.Context, storeID string) (map[string]any, error) {
	s, err := a.PublicStore(ctx, storeID)
	if err != nil {
		return nil, err
	}
	p, _ := a.Policies(ctx, storeID)
	b, _ := json.Marshal(s)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["privacy_policy"] = p.PrivacyPolicy
	m["refund_policy"] = p.RefundPolicy
	m["qualifications"] = p.Qualifications
	return m, nil
}

func (a *App) PatchStore(ctx context.Context, storeID string, in map[string]any) (map[string]any, error) {
	fields := map[string]string{
		"name": "name", "phone": "phone", "address": "address", "business_hours": "business_hours",
		"announcement": "announcement", "timezone": "timezone",
		"privacy_policy": "privacy_policy", "refund_policy": "refund_policy", "qualifications": "qualifications",
	}
	sets := []string{"updated_at=?"}
	args := []any{a.Now()}
	if v, ok := in["name"].(string); ok && v != "" {
		sets = append(sets, "name=?")
		args = append(args, v)
	}
	for k, col := range fields {
		if k == "name" {
			continue
		}
		if v, ok := in[k].(string); ok {
			sets = append(sets, col+"=?")
			args = append(args, v)
		}
	}
	if v, ok := in["is_open"].(bool); ok {
		sets = append(sets, "is_open=?")
		args = append(args, v)
	}
	if v, ok := in["pickup_minutes"].(float64); ok {
		sets = append(sets, "pickup_minutes=?")
		args = append(args, int(v))
	}
	if v, ok := in["scheduled_pickup_enabled"].(bool); ok {
		sets = append(sets, "scheduled_pickup_enabled=?")
		args = append(args, v)
	}
	for _, key := range []string{"pickup_advance_days", "pickup_slot_minutes", "pickup_slot_capacity", "pickup_min_lead_minutes"} {
		if v, ok := in[key].(float64); ok {
			sets = append(sets, key+"=?")
			args = append(args, int(v))
		}
	}
	if hours, ok := in["business_hours"].(string); ok && hours != "" {
		if _, _, err := domain.ParseHours(hours); err != nil {
			return nil, err
		}
	}
	args = append(args, storeID)
	_, err := a.DB.ExecContext(ctx, "UPDATE stores SET "+strings.Join(sets, ",")+" WHERE id=?", args...)
	if err != nil {
		return nil, err
	}
	return a.AdminStore(ctx, storeID)
}

func (a *App) MiniprogramConfig(ctx context.Context, storeID string) (map[string]any, error) {
	var appid, brand, color, logo sql.NullString
	err := a.DB.QueryRowContext(ctx, `SELECT wechat_appid, brand_name, theme_color, logo_url FROM stores WHERE id=?`, storeID).
		Scan(&appid, &brand, &color, &logo)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"wechat_appid": scanNullString(appid),
		"brand_name":   scanNullString(brand),
		"theme_color":  scanNullString(color),
		"logo_url":     scanNullString(logo),
		"appsecret_configured": a.appSecretConfigured(ctx, storeID),
	}, nil
}

func (a *App) appSecretConfigured(ctx context.Context, storeID string) bool {
	var s sql.NullString
	_ = a.DB.QueryRowContext(ctx, `SELECT wechat_appsecret_enc FROM stores WHERE id=?`, storeID).Scan(&s)
	return s.Valid && s.String != ""
}

func (a *App) PatchMiniprogramConfig(ctx context.Context, storeID string, in map[string]any) (map[string]any, error) {
	if v, ok := in["wechat_appid"].(string); ok {
		if v != "" {
			var other sql.NullString
			err := a.DB.QueryRowContext(ctx, `SELECT id FROM stores WHERE wechat_appid=? AND id<>?`, v, storeID).Scan(&other)
			if err == nil {
				return nil, apperr.WechatAppIDConflict
			}
		}
		_, err := a.DB.ExecContext(ctx, `UPDATE stores SET wechat_appid=?, updated_at=? WHERE id=?`, nullS(v), a.Now(), storeID)
		if err != nil {
			if isDup(err) {
				return nil, apperr.WechatAppIDConflict
			}
			return nil, err
		}
	}
	if v, ok := in["brand_name"].(string); ok {
		_, _ = a.DB.ExecContext(ctx, `UPDATE stores SET brand_name=?, updated_at=? WHERE id=?`, v, a.Now(), storeID)
	}
	if v, ok := in["theme_color"].(string); ok {
		_, _ = a.DB.ExecContext(ctx, `UPDATE stores SET theme_color=?, updated_at=? WHERE id=?`, v, a.Now(), storeID)
	}
	if v, ok := in["logo_url"].(string); ok {
		_, _ = a.DB.ExecContext(ctx, `UPDATE stores SET logo_url=?, updated_at=? WHERE id=?`, v, a.Now(), storeID)
	}
	return a.MiniprogramConfig(ctx, storeID)
}

func (a *App) PaymentConfigGet(ctx context.Context, storeID string) (map[string]any, error) {
	var mch, serial, status sql.NullString
	var mock bool
	err := a.DB.QueryRowContext(ctx, `SELECT mch_id, serial_no, status, mock_payment FROM payment_configs WHERE store_id=?`, storeID).
		Scan(&mch, &serial, &status, &mock)
	if err == sql.ErrNoRows {
		return map[string]any{"status": "draft", "mock_payment": false, "ready_checks": map[string]bool{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var appid, secret sql.NullString
	_ = a.DB.QueryRowContext(ctx, `SELECT wechat_appid, wechat_appsecret_enc FROM stores WHERE id=?`, storeID).Scan(&appid, &secret)
	var api, priv, pubid, pub, plat sql.NullString
	_ = a.DB.QueryRowContext(ctx, `SELECT api_v3_key_enc, private_key_enc, pub_key_id, pub_key_pem_enc, platform_cert_enc FROM payment_configs WHERE store_id=?`, storeID).
		Scan(&api, &priv, &pubid, &pub, &plat)
	loginReady := appid.Valid && appid.String != "" && secret.Valid && secret.String != ""
	payReady := appid.Valid && mch.Valid && serial.Valid && api.Valid && priv.Valid && (pub.Valid || plat.Valid)
	return map[string]any{
		"status": scanNullString(status), "mock_payment": mock,
		"mch_id": scanNullString(mch), "serial_no": scanNullString(serial),
		"appid": scanNullString(appid),
		"wechat_login_ready": loginReady,
		"has_api_v3_key": api.Valid && api.String != "",
		"has_private_key": priv.Valid && priv.String != "",
		"has_pub_key": pub.Valid && pub.String != "",
		"has_platform_cert": plat.Valid && plat.String != "",
		"pay_ready": payReady,
	}, nil
}

func (a *App) PaymentConfigPut(ctx context.Context, storeID string, in map[string]any) (map[string]any, error) {
	now := a.Now()
	_, _ = a.DB.ExecContext(ctx, `INSERT IGNORE INTO payment_configs (store_id, status, updated_at) VALUES (?, 'draft', ?)`, storeID, now)
	if v, ok := in["mch_id"].(string); ok && v != "" {
		_, _ = a.DB.ExecContext(ctx, `UPDATE payment_configs SET mch_id=?, updated_at=? WHERE store_id=?`, v, now, storeID)
	}
	if v, ok := in["serial_no"].(string); ok && v != "" {
		_, _ = a.DB.ExecContext(ctx, `UPDATE payment_configs SET serial_no=?, updated_at=? WHERE store_id=?`, v, now, storeID)
	}
	if v, ok := in["app_secret"].(string); ok && v != "" {
		enc, err := cryptoutil.Encrypt(a.Cfg.CredentialKey, []byte(v))
		if err != nil {
			return nil, err
		}
		_, _ = a.DB.ExecContext(ctx, `UPDATE stores SET wechat_appsecret_enc=?, updated_at=? WHERE id=?`, enc, now, storeID)
	}
	for _, key := range []struct{ json, col string }{
		{"api_v3_key", "api_v3_key_enc"}, {"private_key", "private_key_enc"}, {"pub_key_pem", "pub_key_pem_enc"}, {"platform_cert", "platform_cert_enc"},
	} {
		if v, ok := in[key.json].(string); ok && v != "" {
			enc, err := cryptoutil.Encrypt(a.Cfg.CredentialKey, []byte(v))
			if err != nil {
				return nil, err
			}
			_, _ = a.DB.ExecContext(ctx, "UPDATE payment_configs SET "+key.col+"=?, updated_at=? WHERE store_id=?", enc, now, storeID)
		}
	}
	if v, ok := in["pub_key_id"].(string); ok {
		_, _ = a.DB.ExecContext(ctx, `UPDATE payment_configs SET pub_key_id=?, updated_at=? WHERE store_id=?`, v, now, storeID)
	}
	if v, ok := in["status"].(string); ok && (v == "disabled" || v == "draft" || v == "ready") {
		_, _ = a.DB.ExecContext(ctx, `UPDATE payment_configs SET status=?, updated_at=? WHERE store_id=?`, v, now, storeID)
	} else {
		cfg, _ := a.PaymentConfigGet(ctx, storeID)
		if ready, _ := cfg["pay_ready"].(bool); ready {
			_, _ = a.DB.ExecContext(ctx, `UPDATE payment_configs SET status='ready', updated_at=? WHERE store_id=? AND status<>'disabled'`, now, storeID)
		}
	}
	return a.PaymentConfigGet(ctx, storeID)
}

func (a *App) MockPaymentGet(ctx context.Context, storeID string) (map[string]any, error) {
	var mock bool
	_ = a.DB.QueryRowContext(ctx, `SELECT COALESCE(mock_payment,0) FROM payment_configs WHERE store_id=?`, storeID).Scan(&mock)
	return map[string]any{"enabled": mock, "dev_mode": a.Cfg.DevMode}, nil
}

func (a *App) MockPaymentPut(ctx context.Context, storeID, confirm string, enabled bool) error {
	if confirm != "MOCK" {
		return apperr.Validation("请输入确认词 MOCK")
	}
	now := a.Now()
	_, _ = a.DB.ExecContext(ctx, `INSERT IGNORE INTO payment_configs (store_id, status, updated_at) VALUES (?, 'draft', ?)`, storeID, now)
	_, err := a.DB.ExecContext(ctx, `UPDATE payment_configs SET mock_payment=?, updated_at=? WHERE store_id=?`, enabled, now, storeID)
	return err
}

func (a *App) ListMembers(ctx context.Context, storeID string) ([]map[string]any, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT m.admin_user_id, u.login_name, u.display_name, m.role FROM shop_members m JOIN admin_users u ON u.id=m.admin_user_id WHERE m.store_id=?`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, login, name, role string
		if err := rows.Scan(&id, &login, &name, &role); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"admin_user_id": id, "login_name": login, "display_name": name, "role": role})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) ChangeMemberRole(ctx context.Context, storeID, actorRole, targetID, role string) error {
	if actorRole != domain.RoleOwner {
		return apperr.Forbidden
	}
	if role != domain.RoleStaff && role != domain.RoleManager && role != domain.RoleOwner {
		return apperr.Validation("角色无效")
	}
	now := a.Now()
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		if role == domain.RoleOwner {
			if _, err := tx.Exec(`UPDATE shop_members SET role=? WHERE store_id=? AND role=?`, domain.RoleManager, storeID, domain.RoleOwner); err != nil {
				return err
			}
		}
		res, err := tx.Exec(`UPDATE shop_members SET role=? WHERE store_id=? AND admin_user_id=?`, role, storeID, targetID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return apperr.NotFound
		}
		_ = now
		return nil
	})
}

func (a *App) RemoveMember(ctx context.Context, storeID, actorID, actorRole, targetID string) error {
	if actorRole != domain.RoleOwner {
		return apperr.Forbidden
	}
	var role string
	if err := a.DB.QueryRowContext(ctx, `SELECT role FROM shop_members WHERE store_id=? AND admin_user_id=?`, storeID, targetID).Scan(&role); err != nil {
		return apperr.NotFound
	}
	if role == domain.RoleOwner {
		return apperr.New(409, "OWNER_TRANSFER_REQUIRED", "请先转移店主身份再移除")
	}
	_, err := a.DB.ExecContext(ctx, `DELETE FROM shop_members WHERE store_id=? AND admin_user_id=?`, storeID, targetID)
	return err
}

func (a *App) AuditLogs(ctx context.Context, storeID string) ([]map[string]any, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, actor_id, action, resource, summary, created_at FROM audit_logs WHERE store_id=? ORDER BY created_at DESC LIMIT 100`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, action, resource, summary string
		var actor sql.NullString
		var t time.Time
		if err := rows.Scan(&id, &actor, &action, &resource, &summary, &t); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "actor_id": scanNullString(actor), "action": action, "resource": resource, "summary": summary, "created_at": t.UTC().Format(time.RFC3339)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) MemberSettingsGet(ctx context.Context, storeID string) (map[string]any, error) {
	var per int
	var newbie sql.NullString
	err := a.DB.QueryRowContext(ctx, `SELECT points_per_yuan, newbie_coupon_template_id FROM member_settings WHERE store_id=?`, storeID).Scan(&per, &newbie)
	if err == sql.ErrNoRows {
		return map[string]any{"points_per_yuan": 1}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"points_per_yuan": per, "newbie_coupon_template_id": scanNullString(newbie)}, nil
}

func (a *App) MemberSettingsPut(ctx context.Context, storeID string, per int, newbie string) (map[string]any, error) {
	if per < 0 {
		return nil, apperr.Validation("积分比例无效")
	}
	now := a.Now()
	_, err := a.DB.ExecContext(ctx, `INSERT INTO member_settings (store_id, points_per_yuan, newbie_coupon_template_id, updated_at) VALUES (?,?,?,?)
		ON DUPLICATE KEY UPDATE points_per_yuan=VALUES(points_per_yuan), newbie_coupon_template_id=VALUES(newbie_coupon_template_id), updated_at=VALUES(updated_at)`,
		storeID, per, nullS(newbie), now)
	if err != nil {
		return nil, err
	}
	return a.MemberSettingsGet(ctx, storeID)
}

func (a *App) ListCustomerMembers(ctx context.Context, storeID, q string) ([]map[string]any, error) {
	query := `SELECT id, customer_id, member_no, phone_last4, status, points_balance, joined_at FROM customer_memberships WHERE store_id=?`
	args := []any{storeID}
	if q != "" {
		query += ` AND (member_no LIKE ? OR phone_last4=?)`
		args = append(args, q+"%", q)
	}
	query += ` ORDER BY joined_at DESC LIMIT 100`
	rows, err := a.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, cid, no, last4, status string
		var pts int
		var joined time.Time
		if err := rows.Scan(&id, &cid, &no, &last4, &status, &pts, &joined); err != nil {
			return nil, err
		}
		var orders int
		var net sql.NullInt64
		_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(payable_cents),0) FROM orders WHERE store_id=? AND customer_id=? AND payment_status='SUCCESS'`, storeID, cid).Scan(&orders, &net)
		var refunded sql.NullInt64
		_ = a.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM refunds WHERE store_id=? AND order_id IN (SELECT id FROM orders WHERE customer_id=?) AND status='SUCCESS'`, storeID, cid).Scan(&refunded)
		netv := net.Int64 - refunded.Int64
		out = append(out, map[string]any{
			"id": id, "customer_id": cid, "member_no": no, "phone_masked": "****" + last4,
			"status": status, "points_balance": pts, "joined_at": joined.UTC().Format(time.RFC3339),
			"order_count": orders, "net_spend_cents": netv,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) GetCustomerMember(ctx context.Context, storeID, customerID string) (map[string]any, error) {
	list, err := a.ListCustomerMembers(ctx, storeID, "")
	if err != nil {
		return nil, err
	}
	for _, m := range list {
		if m["customer_id"] == customerID {
			ledger, _ := a.PointsLedger(ctx, storeID, customerID)
			m["ledger"] = ledger
			return m, nil
		}
	}
	return nil, apperr.NotFound
}

func (a *App) AdjustPoints(ctx context.Context, storeID, customerID, actorID, reason string, delta int) error {
	if delta == 0 || strings.TrimSpace(reason) == "" {
		return apperr.Validation("delta 不能为 0，原因必填")
	}
	now := a.Now()
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		var mid string
		var bal int
		if err := tx.QueryRow(`SELECT id, points_balance FROM customer_memberships WHERE store_id=? AND customer_id=? FOR UPDATE`, storeID, customerID).Scan(&mid, &bal); err != nil {
			return apperr.NotFound
		}
		if bal+delta < 0 {
			return apperr.Validation("积分余额不能为负")
		}
		if _, err := tx.Exec(`UPDATE customer_memberships SET points_balance=points_balance+? WHERE id=?`, delta, mid); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO member_points_ledger (id, store_id, membership_id, delta, balance_after, reason_type, ref_id, note, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			a.NewID(), storeID, mid, delta, bal+delta, "ADJUST", actorID, reason, now)
		return err
	})
}
