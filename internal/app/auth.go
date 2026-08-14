package app

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/cryptoutil"
	"github.com/changerr/dishflow-grok/internal/domain"
)

const adminCookie = "dishflow_admin"

func (a *App) LoginAdmin(ctx context.Context, login, password, ip string) (token string, principal AdminPrincipal, err error) {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" || password == "" {
		return "", principal, apperr.Validation("账号和密码不能为空")
	}
	if err := a.checkLoginLimit(ctx, ip, login); err != nil {
		return "", principal, err
	}
	var u AdminPrincipal
	var hash string
	err = a.DB.QueryRowContext(ctx, `SELECT id, login_name, display_name, password_hash, is_platform_admin, enabled FROM admin_users WHERE login_name=?`, login).
		Scan(&u.UserID, &u.LoginName, &u.DisplayName, &hash, &u.IsPlatformAdmin, &u.Enabled)
	if err == sql.ErrNoRows || !cryptoutil.VerifyPassword(hash, password) {
		return "", principal, apperr.Unauthorized
	}
	if err != nil {
		return "", principal, err
	}
	if !u.Enabled {
		return "", principal, apperr.Forbidden
	}
	tok, err := cryptoutil.RandomHex(32)
	if err != nil {
		return "", principal, err
	}
	now := a.Now()
	_, err = a.DB.ExecContext(ctx, `INSERT INTO admin_sessions (id, admin_user_id, token_hash, created_at, last_seen_at, last_renewed_at, absolute_expires_at, idle_expires_at)
		VALUES (?,?,?,?,?,?,?,?)`, a.NewID(), u.UserID, cryptoutil.HashToken(tok), now, now, now, now.Add(a.Cfg.AbsoluteSession), now.Add(a.Cfg.IdleSession))
	if err != nil {
		return "", principal, err
	}
	p, err := a.hydrateAdmin(ctx, u.UserID)
	if err != nil {
		return "", principal, err
	}
	a.Audit(ctx, p.StoreID, p.UserID, "ADMIN", "admin.login", "admin_user", p.LoginName, "")
	return tok, p, nil
}

func (a *App) checkLoginLimit(ctx context.Context, ip, login string) error {
	if a.Redis == nil {
		return nil
	}
	keys := []string{"login:ip:" + ip, "login:user:" + login}
	for _, k := range keys {
		n, err := a.Redis.Incr(ctx, k).Result()
		if err != nil {
			continue
		}
		if n == 1 {
			_ = a.Redis.Expire(ctx, k, 15*time.Minute).Err()
		}
		if n > 10 {
			return apperr.RateLimited
		}
	}
	return nil
}

func (a *App) LogoutAdmin(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, _ = a.DB.ExecContext(ctx, `UPDATE admin_sessions SET revoked_at=? WHERE token_hash=? AND revoked_at IS NULL`, a.Now(), cryptoutil.HashToken(token))
	return nil
}

func (a *App) AdminFromToken(ctx context.Context, token string) (AdminPrincipal, error) {
	if token == "" {
		return AdminPrincipal{}, apperr.Unauthorized
	}
	var sessID, userID string
	var lastSeen, lastRenew, absExp, idleExp time.Time
	var revoked sql.NullTime
	err := a.DB.QueryRowContext(ctx, `SELECT id, admin_user_id, last_seen_at, last_renewed_at, absolute_expires_at, idle_expires_at, revoked_at
		FROM admin_sessions WHERE token_hash=?`, cryptoutil.HashToken(token)).
		Scan(&sessID, &userID, &lastSeen, &lastRenew, &absExp, &idleExp, &revoked)
	if err == sql.ErrNoRows {
		return AdminPrincipal{}, apperr.Unauthorized
	}
	if err != nil {
		return AdminPrincipal{}, err
	}
	now := a.Now()
	if revoked.Valid || now.After(absExp) || now.After(idleExp) {
		return AdminPrincipal{}, apperr.Unauthorized
	}
	if now.Sub(lastRenew) >= a.Cfg.SessionRenewEvery {
		_, _ = a.DB.ExecContext(ctx, `UPDATE admin_sessions SET last_seen_at=?, last_renewed_at=?, idle_expires_at=? WHERE id=?`,
			now, now, now.Add(a.Cfg.IdleSession), sessID)
	} else {
		_, _ = a.DB.ExecContext(ctx, `UPDATE admin_sessions SET last_seen_at=?, idle_expires_at=? WHERE id=?`,
			now, now.Add(a.Cfg.IdleSession), sessID)
	}
	return a.hydrateAdmin(ctx, userID)
}

func (a *App) hydrateAdmin(ctx context.Context, userID string) (AdminPrincipal, error) {
	var p AdminPrincipal
	err := a.DB.QueryRowContext(ctx, `SELECT id, login_name, display_name, is_platform_admin, enabled FROM admin_users WHERE id=?`, userID).
		Scan(&p.UserID, &p.LoginName, &p.DisplayName, &p.IsPlatformAdmin, &p.Enabled)
	if err != nil {
		return p, err
	}
	if !p.Enabled {
		return p, apperr.Unauthorized
	}
	var storeID, role, storeName sql.NullString
	var isOpen, mockPay, mockPrint sql.NullBool
	_ = a.DB.QueryRowContext(ctx, `SELECT m.store_id, m.role, s.name, s.is_open,
		COALESCE(pc.mock_payment,0), COALESCE(pr.mock_print,0)
		FROM shop_members m
		JOIN stores s ON s.id=m.store_id
		LEFT JOIN payment_configs pc ON pc.store_id=s.id
		LEFT JOIN print_configs pr ON pr.store_id=s.id
		WHERE m.admin_user_id=?`, userID).Scan(&storeID, &role, &storeName, &isOpen, &mockPay, &mockPrint)
	p.StoreID = scanNullString(storeID)
	p.Role = scanNullString(role)
	p.StoreName = scanNullString(storeName)
	p.StoreOpen = isOpen.Valid && isOpen.Bool
	p.MockPayment = mockPay.Valid && mockPay.Bool
	p.MockPrint = mockPrint.Valid && mockPrint.Bool
	return p, nil
}

func (a *App) RequireStoreRole(p AdminPrincipal, storeID, need string) (AdminPrincipal, error) {
	if p.IsPlatformAdmin {
		return p, apperr.Forbidden
	}
	if storeID == "" || p.StoreID == "" || p.StoreID != storeID {
		return p, apperr.Forbidden
	}
	if err := requireRole(p.Role, need); err != nil {
		return p, err
	}
	return p, nil
}

func (a *App) RequirePlatform(p AdminPrincipal) error {
	if !p.IsPlatformAdmin {
		return apperr.Forbidden
	}
	return nil
}

func (a *App) WechatLogin(ctx context.Context, appid, code string) (token string, customerID, storeID string, err error) {
	if strings.TrimSpace(appid) == "" || strings.TrimSpace(code) == "" {
		return "", "", "", apperr.Validation("缺少 AppID 或 code")
	}
	var storeName, secretEnc sql.NullString
	err = a.DB.QueryRowContext(ctx, `SELECT id, name, wechat_appsecret_enc FROM stores WHERE wechat_appid=? AND enabled=1`, appid).
		Scan(&storeID, &storeName, &secretEnc)
	if err == sql.ErrNoRows {
		return "", "", "", apperr.NotFound
	}
	if err != nil {
		return "", "", "", err
	}
	secret := ""
	if secretEnc.Valid && secretEnc.String != "" {
		b, decErr := cryptoutil.Decrypt(a.Cfg.CredentialKey, secretEnc.String)
		if decErr != nil {
			return "", "", "", apperr.Internal
		}
		secret = string(b)
	}
	if secret == "" && !a.Cfg.DevMode {
		return "", "", "", apperr.New(409, "WECHAT_LOGIN_UNAVAILABLE", "门店未配置小程序密钥")
	}
	openid, err := a.Wechat.Code2Session(ctx, appid, secret, code)
	if err != nil {
		return "", "", "", err
	}
	now := a.Now()
	err = a.DB.QueryRowContext(ctx, `SELECT id FROM customers WHERE store_id=? AND wechat_openid=?`, storeID, openid).Scan(&customerID)
	if err == sql.ErrNoRows {
		customerID = a.NewID()
		_, err = a.DB.ExecContext(ctx, `INSERT INTO customers (id, store_id, wechat_openid, created_at) VALUES (?,?,?,?)`, customerID, storeID, openid, now)
		if err != nil {
			return "", "", "", err
		}
	} else if err != nil {
		return "", "", "", err
	}
	tok, err := cryptoutil.RandomHex(32)
	if err != nil {
		return "", "", "", err
	}
	_, err = a.DB.ExecContext(ctx, `INSERT INTO customer_sessions (id, customer_id, store_id, token_hash, expires_at, created_at) VALUES (?,?,?,?,?,?)`,
		a.NewID(), customerID, storeID, cryptoutil.HashToken(tok), now.Add(30*24*time.Hour), now)
	return tok, customerID, storeID, err
}

func (a *App) CustomerFromToken(ctx context.Context, appid, token string) (CustomerPrincipal, error) {
	if token == "" {
		return CustomerPrincipal{}, apperr.Unauthorized
	}
	var storeID string
	err := a.DB.QueryRowContext(ctx, `SELECT id FROM stores WHERE wechat_appid=? AND enabled=1`, appid).Scan(&storeID)
	if err == sql.ErrNoRows {
		return CustomerPrincipal{}, apperr.NotFound
	}
	if err != nil {
		return CustomerPrincipal{}, err
	}
	var p CustomerPrincipal
	var exp time.Time
	err = a.DB.QueryRowContext(ctx, `SELECT customer_id, store_id, expires_at FROM customer_sessions WHERE token_hash=?`, cryptoutil.HashToken(token)).
		Scan(&p.CustomerID, &p.StoreID, &exp)
	if err == sql.ErrNoRows || a.Now().After(exp) {
		return CustomerPrincipal{}, apperr.Unauthorized
	}
	if err != nil {
		return CustomerPrincipal{}, err
	}
	if p.StoreID != storeID {
		return CustomerPrincipal{}, apperr.Forbidden
	}
	_ = a.DB.QueryRowContext(ctx, `SELECT wechat_openid FROM customers WHERE id=?`, p.CustomerID).Scan(&p.OpenID)
	return p, nil
}

func (a *App) StoreByAppID(ctx context.Context, appid string) (string, error) {
	var id string
	err := a.DB.QueryRowContext(ctx, `SELECT id FROM stores WHERE wechat_appid=? AND enabled=1`, appid).Scan(&id)
	if err == sql.ErrNoRows {
		return "", apperr.NotFound
	}
	return id, err
}

func (a *App) CookieName() string { return adminCookie }

func NeedOwnerFor(action string) bool {
	switch action {
	case "store.import", "payment.write", "mock.write", "member.delete", "printer.delete", "coupon.purge":
		return true
	default:
		return false
	}
}

func StaffForbidden(role, resource string) bool {
	if role != domain.RoleStaff {
		return false
	}
	switch resource {
	case "board", "order.detail", "order.transition", "materials", "purchase":
		return false
	default:
		return true
	}
}
