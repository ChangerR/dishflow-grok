package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/cryptoutil"
	"github.com/changerr/dishflow-grok/internal/ids"
)

type PromotionDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ThresholdCents int64  `json:"threshold_cents"`
	DiscountCents  int64  `json:"discount_cents"`
	Scope          string `json:"scope"`
	StackPolicy    string `json:"stack_policy"`
	StartsAt       string `json:"starts_at"`
	EndsAt         string `json:"ends_at"`
	Enabled        bool   `json:"enabled"`
}

type CouponTemplateDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	MinSpendCents  int64  `json:"min_spend_cents"`
	DiscountCents  int64  `json:"discount_cents"`
	Scope          string `json:"scope"`
	StartsAt       string `json:"starts_at"`
	EndsAt         string `json:"ends_at"`
	Enabled        bool   `json:"enabled"`
	PublicClaim    bool   `json:"public_claim"`
	Audience       string `json:"audience"`
	Redeemable     bool   `json:"redeemable"`
	PointsCost     int    `json:"points_cost"`
}

func (a *App) ListPromotions(ctx context.Context, storeID string) ([]PromotionDTO, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, name, threshold_cents, discount_cents, scope, stack_policy, starts_at, ends_at, enabled FROM promotions WHERE store_id=? ORDER BY created_at DESC`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PromotionDTO
	for rows.Next() {
		var p PromotionDTO
		var s, e time.Time
		if err := rows.Scan(&p.ID, &p.Name, &p.ThresholdCents, &p.DiscountCents, &p.Scope, &p.StackPolicy, &s, &e, &p.Enabled); err != nil {
			return nil, err
		}
		p.StartsAt, p.EndsAt = s.UTC().Format(time.RFC3339), e.UTC().Format(time.RFC3339)
		out = append(out, p)
	}
	if out == nil {
		out = []PromotionDTO{}
	}
	return out, rows.Err()
}

func (a *App) SavePromotion(ctx context.Context, storeID, id string, p PromotionDTO) (PromotionDTO, error) {
	if p.DiscountCents <= 0 || p.ThresholdCents < 0 || p.Name == "" {
		return p, apperr.Validation("满减字段无效")
	}
	st, err := time.Parse(time.RFC3339, p.StartsAt)
	if err != nil {
		return p, apperr.Validation("开始时间无效")
	}
	en, err := time.Parse(time.RFC3339, p.EndsAt)
	if err != nil {
		return p, apperr.Validation("结束时间无效")
	}
	if p.Scope == "" {
		p.Scope = "ALL"
	}
	if p.StackPolicy == "" {
		p.StackPolicy = "BEST_OF"
	}
	now := a.Now()
	if id == "" {
		id = a.NewID()
		_, err = a.DB.ExecContext(ctx, `INSERT INTO promotions (id, store_id, name, threshold_cents, discount_cents, scope, stack_policy, starts_at, ends_at, enabled, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, id, storeID, p.Name, p.ThresholdCents, p.DiscountCents, p.Scope, p.StackPolicy, st.UTC(), en.UTC(), p.Enabled, now, now)
	} else {
		_, err = a.DB.ExecContext(ctx, `UPDATE promotions SET name=?, threshold_cents=?, discount_cents=?, scope=?, stack_policy=?, starts_at=?, ends_at=?, enabled=?, updated_at=? WHERE id=? AND store_id=?`,
			p.Name, p.ThresholdCents, p.DiscountCents, p.Scope, p.StackPolicy, st.UTC(), en.UTC(), p.Enabled, now, id, storeID)
	}
	p.ID = id
	return p, err
}

func (a *App) ListCouponTemplates(ctx context.Context, storeID string) ([]CouponTemplateDTO, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, name, min_spend_cents, discount_cents, scope, starts_at, ends_at, enabled, public_claim, audience, redeemable, points_cost FROM coupon_templates WHERE store_id=? AND permanently_deleted=0 ORDER BY created_at DESC`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CouponTemplateDTO
	for rows.Next() {
		var t CouponTemplateDTO
		var s, e time.Time
		if err := rows.Scan(&t.ID, &t.Name, &t.MinSpendCents, &t.DiscountCents, &t.Scope, &s, &e, &t.Enabled, &t.PublicClaim, &t.Audience, &t.Redeemable, &t.PointsCost); err != nil {
			return nil, err
		}
		t.StartsAt, t.EndsAt = s.UTC().Format(time.RFC3339), e.UTC().Format(time.RFC3339)
		out = append(out, t)
	}
	if out == nil {
		out = []CouponTemplateDTO{}
	}
	return out, rows.Err()
}

func (a *App) SaveCouponTemplate(ctx context.Context, storeID, id string, t CouponTemplateDTO) (CouponTemplateDTO, error) {
	if t.Name == "" || t.DiscountCents <= 0 {
		return t, apperr.Validation("券模板字段无效")
	}
	st, err := time.Parse(time.RFC3339, t.StartsAt)
	if err != nil {
		return t, apperr.Validation("开始时间无效")
	}
	en, err := time.Parse(time.RFC3339, t.EndsAt)
	if err != nil {
		return t, apperr.Validation("结束时间无效")
	}
	if t.Audience == "" {
		t.Audience = "ALL"
	}
	if t.Scope == "" {
		t.Scope = "ALL"
	}
	now := a.Now()
	if id == "" {
		id = a.NewID()
		_, err = a.DB.ExecContext(ctx, `INSERT INTO coupon_templates (id, store_id, name, min_spend_cents, discount_cents, scope, starts_at, ends_at, enabled, public_claim, audience, redeemable, points_cost, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, storeID, t.Name, t.MinSpendCents, t.DiscountCents, t.Scope, st.UTC(), en.UTC(), t.Enabled, t.PublicClaim, t.Audience, t.Redeemable, t.PointsCost, now, now)
	} else {
		_, err = a.DB.ExecContext(ctx, `UPDATE coupon_templates SET name=?, min_spend_cents=?, discount_cents=?, scope=?, starts_at=?, ends_at=?, enabled=?, public_claim=?, audience=?, redeemable=?, points_cost=?, updated_at=? WHERE id=? AND store_id=? AND permanently_deleted=0`,
			t.Name, t.MinSpendCents, t.DiscountCents, t.Scope, st.UTC(), en.UTC(), t.Enabled, t.PublicClaim, t.Audience, t.Redeemable, t.PointsCost, now, id, storeID)
	}
	t.ID = id
	return t, err
}

func (a *App) DeleteCouponTemplate(ctx context.Context, storeID, id, role string) error {
	if role != "OWNER" {
		return apperr.Forbidden
	}
	var n int
	_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_coupons WHERE template_id=?`, id).Scan(&n)
	if n > 0 {
		return apperr.New(409, "TEMPLATE_IN_USE", "模板已发放，不能永久删除")
	}
	var newbie sql.NullString
	_ = a.DB.QueryRowContext(ctx, `SELECT newbie_coupon_template_id FROM member_settings WHERE store_id=?`, storeID).Scan(&newbie)
	if newbie.Valid && newbie.String == id {
		return apperr.New(409, "TEMPLATE_IN_USE", "模板被引用为新人券")
	}
	res, err := a.DB.ExecContext(ctx, `UPDATE coupon_templates SET permanently_deleted=1, updated_at=? WHERE id=? AND store_id=?`, a.Now(), id, storeID)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return apperr.NotFound
	}
	return nil
}

func (a *App) PublicCouponOffers(ctx context.Context, storeID, customerID string) ([]CouponTemplateDTO, error) {
	all, err := a.ListCouponTemplates(ctx, storeID)
	if err != nil {
		return nil, err
	}
	now := a.Now()
	var out []CouponTemplateDTO
	for _, t := range all {
		st, _ := time.Parse(time.RFC3339, t.StartsAt)
		en, _ := time.Parse(time.RFC3339, t.EndsAt)
		if !t.Enabled || !t.PublicClaim || now.Before(st) || !now.Before(en) {
			continue
		}
		if customerID != "" {
			var n int
			_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_coupons WHERE customer_id=? AND template_id=?`, customerID, t.ID).Scan(&n)
			if n > 0 {
				continue
			}
		}
		out = append(out, t)
	}
	if out == nil {
		out = []CouponTemplateDTO{}
	}
	return out, nil
}

func (a *App) ClaimCoupon(ctx context.Context, storeID, customerID, templateID string) (map[string]any, error) {
	if customerID == "" {
		return nil, apperr.Unauthorized
	}
	now := a.Now()
	var enabled, pub bool
	var st, en time.Time
	err := a.DB.QueryRowContext(ctx, `SELECT enabled, public_claim, starts_at, ends_at FROM coupon_templates WHERE id=? AND store_id=? AND permanently_deleted=0`, templateID, storeID).
		Scan(&enabled, &pub, &st, &en)
	if err == sql.ErrNoRows {
		return nil, apperr.NotFound
	}
	if err != nil {
		return nil, err
	}
	if !enabled || !pub || now.Before(st) || !now.Before(en) {
		return nil, apperr.Validation("券不可领取")
	}
	id := a.NewID()
	_, err = a.DB.ExecContext(ctx, `INSERT INTO customer_coupons (id, store_id, customer_id, template_id, status, claimed_at) VALUES (?,?,?,?, 'AVAILABLE', ?)`,
		id, storeID, customerID, templateID, now)
	if err != nil {
		if isDup(err) {
			var existing string
			_ = a.DB.QueryRowContext(ctx, `SELECT id FROM customer_coupons WHERE customer_id=? AND template_id=?`, customerID, templateID).Scan(&existing)
			return map[string]any{"id": existing, "idempotent": true}, nil
		}
		return nil, err
	}
	return map[string]any{"id": id, "status": "AVAILABLE"}, nil
}

func (a *App) MyCoupons(ctx context.Context, storeID, customerID, tab string) ([]map[string]any, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT cc.id, cc.status, cc.claimed_at, ct.name, ct.min_spend_cents, ct.discount_cents, ct.starts_at, ct.ends_at
		FROM customer_coupons cc JOIN coupon_templates ct ON ct.id=cc.template_id WHERE cc.store_id=? AND cc.customer_id=? ORDER BY cc.claimed_at DESC`, storeID, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := a.Now()
	var out []map[string]any
	for rows.Next() {
		var id, status, name string
		var claimed, st, en time.Time
		var min, disc int64
		if err := rows.Scan(&id, &status, &claimed, &name, &min, &disc, &st, &en); err != nil {
			return nil, err
		}
		if status == "AVAILABLE" && now.After(en) {
			status = "EXPIRED"
		}
		keep := tab == "" || tab == "ALL" || tab == status || (tab == "OFFERS")
		if tab == "AVAILABLE" && status != "AVAILABLE" {
			keep = false
		}
		if !keep {
			continue
		}
		out = append(out, map[string]any{
			"id": id, "status": status, "name": name, "min_spend_cents": min, "discount_cents": disc,
			"starts_at": st.UTC().Format(time.RFC3339), "ends_at": en.UTC().Format(time.RFC3339),
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) IssueCoupons(ctx context.Context, storeID, templateID, audience string) (map[string]any, error) {
	now := a.Now()
	rows, err := a.DB.QueryContext(ctx, `SELECT id FROM customers WHERE store_id=?`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err != nil {
			return nil, err
		}
		if !a.matchAudience(ctx, storeID, cid, audience) {
			continue
		}
		_, err := a.DB.ExecContext(ctx, `INSERT IGNORE INTO customer_coupons (id, store_id, customer_id, template_id, status, claimed_at) VALUES (?,?,?,?, 'AVAILABLE', ?)`,
			ids.New(), storeID, cid, templateID, now)
		if err != nil {
			return nil, err
		}
		n++
	}
	return map[string]any{"issued": n}, rows.Err()
}

func (a *App) matchAudience(ctx context.Context, storeID, customerID, audience string) bool {
	if audience == "" || audience == "ALL" {
		return true
	}
	var paid int
	_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE store_id=? AND customer_id=? AND payment_status='SUCCESS'`, storeID, customerID).Scan(&paid)
	switch audience {
	case "NEW":
		return paid == 0
	case "OLD":
		return paid > 0
	case "ACTIVE_180":
		var n int
		_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE store_id=? AND customer_id=? AND payment_status='SUCCESS' AND paid_at>=?`, storeID, customerID, a.Now().Add(-180*24*time.Hour)).Scan(&n)
		return n > 0
	case "SILENT_180":
		var last sql.NullTime
		_ = a.DB.QueryRowContext(ctx, `SELECT MAX(paid_at) FROM orders WHERE store_id=? AND customer_id=? AND payment_status='SUCCESS'`, storeID, customerID).Scan(&last)
		return last.Valid && last.Time.Before(a.Now().Add(-180*24*time.Hour))
	default:
		return true
	}
}

func (a *App) JoinMembership(ctx context.Context, storeID, customerID, phoneCode string, agreed bool) (map[string]any, error) {
	if !agreed {
		return nil, apperr.Validation("请同意会员协议")
	}
	var appid, secretEnc sql.NullString
	_ = a.DB.QueryRowContext(ctx, `SELECT wechat_appid, wechat_appsecret_enc FROM stores WHERE id=?`, storeID).Scan(&appid, &secretEnc)
	secret := ""
	if secretEnc.Valid {
		b, _ := cryptoutil.Decrypt(a.Cfg.CredentialKey, secretEnc.String)
		secret = string(b)
	}
	e164, country, err := a.Wechat.GetPhoneNumber(ctx, scanNullString(appid), secret, phoneCode)
	if err != nil {
		return nil, err
	}
	digits := strings.TrimPrefix(e164, "+")
	if strings.HasPrefix(digits, "86") {
		digits = digits[2:]
	}
	last4 := digits
	if len(last4) >= 4 {
		last4 = last4[len(last4)-4:]
	}
	phoneHash := cryptoutil.PhoneHash(a.Cfg.PhoneHashPepper, storeID, e164)
	enc, err := cryptoutil.Encrypt(a.Cfg.CredentialKey, []byte(e164))
	if err != nil {
		return nil, err
	}
	now := a.Now()
	var existing string
	err = a.DB.QueryRowContext(ctx, `SELECT id FROM customer_memberships WHERE store_id=? AND phone_hash=?`, storeID, phoneHash).Scan(&existing)
	if err == nil {
		return a.Me(ctx, storeID, customerID)
	}
	var already string
	_ = a.DB.QueryRowContext(ctx, `SELECT id FROM customer_memberships WHERE store_id=? AND customer_id=?`, storeID, customerID).Scan(&already)
	if already != "" {
		return a.Me(ctx, storeID, customerID)
	}
	memberNo := fmt.Sprintf("M%s", now.Format("060102150405"))
	id := a.NewID()
	err = persistTx(ctx, a, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO customer_memberships (id, store_id, customer_id, member_no, phone_enc, phone_hash, phone_last4, phone_country_code, status, points_balance, joined_at)
			VALUES (?,?,?,?,?,?,?,?, 'ACTIVE', 0, ?)`, id, storeID, customerID, memberNo, enc, phoneHash, last4, country, now); err != nil {
			if isDup(err) {
				return nil
			}
			return err
		}
		var newbie sql.NullString
		_ = tx.QueryRow(`SELECT newbie_coupon_template_id FROM member_settings WHERE store_id=?`, storeID).Scan(&newbie)
		if newbie.Valid && newbie.String != "" {
			var enabled bool
			var st, en time.Time
			err := tx.QueryRow(`SELECT enabled, starts_at, ends_at FROM coupon_templates WHERE id=? AND permanently_deleted=0`, newbie.String).Scan(&enabled, &st, &en)
			if err == nil && enabled && !now.Before(st) && now.Before(en) {
				_, _ = tx.Exec(`INSERT IGNORE INTO customer_coupons (id, store_id, customer_id, template_id, status, claimed_at) VALUES (?,?,?,?, 'AVAILABLE', ?)`,
					ids.New(), storeID, customerID, newbie.String, now)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return a.Me(ctx, storeID, customerID)
}

func (a *App) Me(ctx context.Context, storeID, customerID string) (map[string]any, error) {
	out := map[string]any{"customer_id": customerID, "is_member": false, "points_balance": 0}
	var no, status string
	var pts int
	var joined time.Time
	err := a.DB.QueryRowContext(ctx, `SELECT member_no, status, points_balance, joined_at FROM customer_memberships WHERE store_id=? AND customer_id=?`, storeID, customerID).
		Scan(&no, &status, &pts, &joined)
	if err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	out["is_member"] = true
	out["member_no"] = no
	out["status"] = status
	out["points_balance"] = pts
	out["joined_at"] = joined.UTC().Format(time.RFC3339)
	return out, nil
}

func (a *App) PointsLedger(ctx context.Context, storeID, customerID string) ([]map[string]any, error) {
	var mid string
	if err := a.DB.QueryRowContext(ctx, `SELECT id FROM customer_memberships WHERE store_id=? AND customer_id=?`, storeID, customerID).Scan(&mid); err != nil {
		if err == sql.ErrNoRows {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	rows, err := a.DB.QueryContext(ctx, `SELECT delta, balance_after, reason_type, created_at FROM member_points_ledger WHERE membership_id=? ORDER BY created_at DESC LIMIT 100`, mid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var delta, bal int
		var reason string
		var t time.Time
		if err := rows.Scan(&delta, &bal, &reason, &t); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"delta": delta, "balance_after": bal, "reason_type": reason, "created_at": t.UTC().Format(time.RFC3339)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) Rewards(ctx context.Context, storeID string) ([]CouponTemplateDTO, error) {
	all, err := a.ListCouponTemplates(ctx, storeID)
	if err != nil {
		return nil, err
	}
	now := a.Now()
	var out []CouponTemplateDTO
	for _, t := range all {
		st, _ := time.Parse(time.RFC3339, t.StartsAt)
		en, _ := time.Parse(time.RFC3339, t.EndsAt)
		if t.Enabled && t.Redeemable && t.PointsCost > 0 && !now.Before(st) && now.Before(en) {
			out = append(out, t)
		}
	}
	if out == nil {
		out = []CouponTemplateDTO{}
	}
	return out, nil
}

func (a *App) RedeemReward(ctx context.Context, storeID, customerID, templateID string) error {
	now := a.Now()
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		var cost int
		var enabled, redeemable bool
		var st, en time.Time
		if err := tx.QueryRow(`SELECT points_cost, enabled, redeemable, starts_at, ends_at FROM coupon_templates WHERE id=? AND store_id=? AND permanently_deleted=0`, templateID, storeID).
			Scan(&cost, &enabled, &redeemable, &st, &en); err != nil {
			return apperr.NotFound
		}
		if !enabled || !redeemable || cost <= 0 || now.Before(st) || !now.Before(en) {
			return apperr.Validation("权益不可兑换")
		}
		var mid string
		var bal int
		if err := tx.QueryRow(`SELECT id, points_balance FROM customer_memberships WHERE store_id=? AND customer_id=? AND status='ACTIVE' FOR UPDATE`, storeID, customerID).
			Scan(&mid, &bal); err != nil {
			return apperr.Forbidden
		}
		if bal < cost {
			return apperr.InsufficientPoints
		}
		if _, err := tx.Exec(`UPDATE customer_memberships SET points_balance=points_balance-? WHERE id=?`, cost, mid); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO member_points_ledger (id, store_id, membership_id, delta, balance_after, reason_type, ref_id, created_at) VALUES (?,?,?,?,?,?,?,?)`,
			ids.New(), storeID, mid, -cost, bal-cost, "REDEEM", templateID, now); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO customer_coupons (id, store_id, customer_id, template_id, status, claimed_at) VALUES (?,?,?,?, 'AVAILABLE', ?)`,
			ids.New(), storeID, customerID, templateID, now)
		return err
	})
}
