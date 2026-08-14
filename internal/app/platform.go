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

type StoreDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Phone        string `json:"phone,omitempty"`
	Address      string `json:"address,omitempty"`
	Timezone     string `json:"timezone"`
	Enabled      bool   `json:"enabled"`
	BrandName    string `json:"brand_name,omitempty"`
	WechatAppID  string `json:"wechat_appid,omitempty"`
	WechatReady  bool   `json:"wechat_login_ready"`
	CreatedAt    string `json:"created_at"`
}

type AdminUserDTO struct {
	ID              string `json:"id"`
	LoginName       string `json:"login_name"`
	DisplayName     string `json:"display_name"`
	IsPlatformAdmin bool   `json:"is_platform_admin"`
	Enabled         bool   `json:"enabled"`
	StoreID         string `json:"store_id,omitempty"`
	Role            string `json:"role,omitempty"`
}

type ApplicationDTO struct {
	ID          string  `json:"id"`
	ApplicantID string  `json:"applicant_admin_user_id"`
	StoreName   string  `json:"store_name,omitempty"`
	StoreID     string  `json:"store_id,omitempty"`
	Contact     string  `json:"contact,omitempty"`
	Role        string  `json:"requested_role,omitempty"`
	Status      string  `json:"status"`
	Note        string  `json:"note,omitempty"`
	CreatedAt   string  `json:"created_at"`
	ReviewedAt  *string `json:"reviewed_at,omitempty"`
}

func (a *App) CreatePlatformStore(ctx context.Context, name, timezone string) (StoreDTO, error) {
	if strings.TrimSpace(name) == "" {
		return StoreDTO{}, apperr.Validation("门店名称不能为空")
	}
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	if _, err := domain.LoadLocation(timezone); err != nil {
		return StoreDTO{}, err
	}
	id := a.NewID()
	now := a.Now()
	_, err := a.DB.ExecContext(ctx, `INSERT INTO stores (id, name, timezone, created_at, updated_at) VALUES (?,?,?,?,?)`, id, name, timezone, now, now)
	if err != nil {
		return StoreDTO{}, err
	}
	_, _ = a.DB.ExecContext(ctx, `INSERT INTO member_settings (store_id, points_per_yuan, updated_at) VALUES (?,?,?)`, id, 1, now)
	_, _ = a.DB.ExecContext(ctx, `INSERT INTO payment_configs (store_id, status, updated_at) VALUES (?,?,?)`, id, "draft", now)
	_, _ = a.DB.ExecContext(ctx, `INSERT INTO print_configs (store_id, status, updated_at) VALUES (?,?,?)`, id, "draft", now)
	return a.GetPlatformStore(ctx, id)
}

func (a *App) GetPlatformStore(ctx context.Context, id string) (StoreDTO, error) {
	var s StoreDTO
	var phone, addr, brand, appid, secret sql.NullString
	var created time.Time
	err := a.DB.QueryRowContext(ctx, `SELECT id, name, phone, address, timezone, enabled, brand_name, wechat_appid, wechat_appsecret_enc, created_at FROM stores WHERE id=?`, id).
		Scan(&s.ID, &s.Name, &phone, &addr, &s.Timezone, &s.Enabled, &brand, &appid, &secret, &created)
	if err == sql.ErrNoRows {
		return s, apperr.NotFound
	}
	if err != nil {
		return s, err
	}
	s.Phone = scanNullString(phone)
	s.Address = scanNullString(addr)
	s.BrandName = scanNullString(brand)
	s.WechatAppID = scanNullString(appid)
	s.WechatReady = secret.Valid && secret.String != "" && s.WechatAppID != ""
	s.CreatedAt = created.UTC().Format(time.RFC3339)
	return s, nil
}

func (a *App) ListPlatformStores(ctx context.Context) ([]StoreDTO, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id FROM stores ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoreDTO
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		s, err := a.GetPlatformStore(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if out == nil {
		out = []StoreDTO{}
	}
	return out, rows.Err()
}

func (a *App) PatchPlatformStore(ctx context.Context, id, name string, enabled *bool) (StoreDTO, error) {
	s, err := a.GetPlatformStore(ctx, id)
	if err != nil {
		return s, err
	}
	if name != "" {
		s.Name = name
	}
	if enabled != nil {
		s.Enabled = *enabled
	}
	_, err = a.DB.ExecContext(ctx, `UPDATE stores SET name=?, enabled=?, updated_at=? WHERE id=?`, s.Name, s.Enabled, a.Now(), id)
	if err != nil {
		return s, err
	}
	return a.GetPlatformStore(ctx, id)
}

func (a *App) CreateAdminUser(ctx context.Context, login, display, password string, platform, enabled bool) (AdminUserDTO, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" || display == "" {
		return AdminUserDTO{}, apperr.Validation("登录名和显示名不能为空")
	}
	if err := domain.ValidPassword(password); err != nil {
		return AdminUserDTO{}, err
	}
	hash, err := cryptoutil.HashPassword(password)
	if err != nil {
		return AdminUserDTO{}, err
	}
	id := a.NewID()
	now := a.Now()
	_, err = a.DB.ExecContext(ctx, `INSERT INTO admin_users (id, login_name, display_name, password_hash, is_platform_admin, enabled, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`, id, login, display, hash, platform, enabled, now, now)
	if err != nil {
		if isDup(err) {
			return AdminUserDTO{}, apperr.Conflict
		}
		return AdminUserDTO{}, err
	}
	return a.GetAdminUser(ctx, id)
}

func (a *App) GetAdminUser(ctx context.Context, id string) (AdminUserDTO, error) {
	var u AdminUserDTO
	err := a.DB.QueryRowContext(ctx, `SELECT id, login_name, display_name, is_platform_admin, enabled FROM admin_users WHERE id=?`, id).
		Scan(&u.ID, &u.LoginName, &u.DisplayName, &u.IsPlatformAdmin, &u.Enabled)
	if err == sql.ErrNoRows {
		return u, apperr.NotFound
	}
	if err != nil {
		return u, err
	}
	var sid, role sql.NullString
	_ = a.DB.QueryRowContext(ctx, `SELECT store_id, role FROM shop_members WHERE admin_user_id=?`, id).Scan(&sid, &role)
	u.StoreID = scanNullString(sid)
	u.Role = scanNullString(role)
	return u, nil
}

func (a *App) ListAdminUsers(ctx context.Context) ([]AdminUserDTO, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id FROM admin_users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminUserDTO
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		u, err := a.GetAdminUser(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if out == nil {
		out = []AdminUserDTO{}
	}
	return out, rows.Err()
}

func (a *App) PatchAdminUser(ctx context.Context, id, display string, enabled *bool, platform *bool) (AdminUserDTO, error) {
	u, err := a.GetAdminUser(ctx, id)
	if err != nil {
		return u, err
	}
	if display != "" {
		u.DisplayName = display
	}
	if enabled != nil {
		u.Enabled = *enabled
	}
	if platform != nil {
		u.IsPlatformAdmin = *platform
	}
	_, err = a.DB.ExecContext(ctx, `UPDATE admin_users SET display_name=?, enabled=?, is_platform_admin=?, updated_at=? WHERE id=?`,
		u.DisplayName, u.Enabled, u.IsPlatformAdmin, a.Now(), id)
	if err != nil {
		return u, err
	}
	return a.GetAdminUser(ctx, id)
}

func (a *App) AssignStoreOwner(ctx context.Context, userID, storeID string) error {
	u, err := a.GetAdminUser(ctx, userID)
	if err != nil {
		return err
	}
	if u.IsPlatformAdmin {
		return apperr.Validation("平台管理员不能绑定门店")
	}
	if _, err := a.GetPlatformStore(ctx, storeID); err != nil {
		return err
	}
	if u.StoreID != "" && u.StoreID != storeID {
		return apperr.New(409, "MEMBER_CONFLICT", "该账号已绑定其他门店，请先解除关系")
	}
	now := a.Now()
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE shop_members SET role=? WHERE store_id=? AND role=?`, domain.RoleManager, storeID, domain.RoleOwner); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO shop_members (store_id, admin_user_id, role, created_at) VALUES (?,?,?,?)
			ON DUPLICATE KEY UPDATE role=VALUES(role)`, storeID, userID, domain.RoleOwner, now)
		return err
	})
}

func (a *App) SubmitShopApplication(ctx context.Context, userID, storeName, contact string) (ApplicationDTO, error) {
	if strings.TrimSpace(storeName) == "" || strings.TrimSpace(contact) == "" {
		return ApplicationDTO{}, apperr.Validation("门店名和联系方式不能为空")
	}
	id := a.NewID()
	now := a.Now()
	_, err := a.DB.ExecContext(ctx, `INSERT INTO shop_applications (id, applicant_admin_user_id, store_name, contact, status, created_at)
		VALUES (?,?,?,?, 'PENDING', ?)`, id, userID, storeName, contact, now)
	if err != nil {
		return ApplicationDTO{}, err
	}
	return a.getShopApplication(ctx, id)
}

func (a *App) getShopApplication(ctx context.Context, id string) (ApplicationDTO, error) {
	var d ApplicationDTO
	var reviewed sql.NullTime
	var note sql.NullString
	var created time.Time
	err := a.DB.QueryRowContext(ctx, `SELECT id, applicant_admin_user_id, store_name, contact, status, note, created_at, reviewed_at FROM shop_applications WHERE id=?`, id).
		Scan(&d.ID, &d.ApplicantID, &d.StoreName, &d.Contact, &d.Status, &note, &created, &reviewed)
	if err == sql.ErrNoRows {
		return d, apperr.NotFound
	}
	if err != nil {
		return d, err
	}
	d.Note = scanNullString(note)
	d.CreatedAt = created.UTC().Format(time.RFC3339)
	if reviewed.Valid {
		s := reviewed.Time.UTC().Format(time.RFC3339)
		d.ReviewedAt = &s
	}
	return d, nil
}

func (a *App) MyShopApplications(ctx context.Context, userID string) ([]ApplicationDTO, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id FROM shop_applications WHERE applicant_admin_user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanApps(rows, func(id string) (ApplicationDTO, error) { return a.getShopApplication(ctx, id) })
}

func (a *App) ListShopApplications(ctx context.Context) ([]ApplicationDTO, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id FROM shop_applications ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanApps(rows, func(id string) (ApplicationDTO, error) { return a.getShopApplication(ctx, id) })
}

func (a *App) ReviewShopApplication(ctx context.Context, reviewerID, id, decision, note string) (ApplicationDTO, error) {
	d, err := a.getShopApplication(ctx, id)
	if err != nil {
		return d, err
	}
	if d.Status != "PENDING" {
		return d, apperr.StateConflict
	}
	now := a.Now()
	if decision == "REJECTED" {
		_, err = a.DB.ExecContext(ctx, `UPDATE shop_applications SET status='REJECTED', note=?, reviewed_at=?, reviewer_id=? WHERE id=?`, note, now, reviewerID, id)
		if err != nil {
			return d, err
		}
		return a.getShopApplication(ctx, id)
	}
	if decision != "APPROVED" {
		return d, apperr.Validation("decision 必须为 APPROVED 或 REJECTED")
	}
	err = persistTx(ctx, a, func(tx *sql.Tx) error {
		store, err := a.CreatePlatformStore(ctx, d.StoreName, "Asia/Shanghai")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE shop_members SET role=? WHERE store_id=? AND role=?`, domain.RoleManager, store.ID, domain.RoleOwner); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO shop_members (store_id, admin_user_id, role, created_at) VALUES (?,?,?,?)
			ON DUPLICATE KEY UPDATE role=VALUES(role)`, store.ID, d.ApplicantID, domain.RoleOwner, now); err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE shop_applications SET status='APPROVED', note=?, reviewed_at=?, reviewer_id=? WHERE id=? AND status='PENDING'`, note, now, reviewerID, id)
		return err
	})
	if err != nil {
		return d, err
	}
	return a.getShopApplication(ctx, id)
}

func (a *App) SubmitJoinRequest(ctx context.Context, userID, storeID, role string) (ApplicationDTO, error) {
	if role != domain.RoleStaff && role != domain.RoleManager && role != domain.RoleOwner {
		return ApplicationDTO{}, apperr.Validation("角色无效")
	}
	if _, err := a.GetPlatformStore(ctx, storeID); err != nil {
		return ApplicationDTO{}, err
	}
	id := a.NewID()
	now := a.Now()
	_, err := a.DB.ExecContext(ctx, `INSERT INTO shop_join_requests (id, applicant_admin_user_id, store_id, requested_role, status, created_at)
		VALUES (?,?,?,?, 'PENDING', ?)`, id, userID, storeID, role, now)
	if err != nil {
		return ApplicationDTO{}, err
	}
	return a.getJoinRequest(ctx, id)
}

func (a *App) getJoinRequest(ctx context.Context, id string) (ApplicationDTO, error) {
	var d ApplicationDTO
	var reviewed sql.NullTime
	var note sql.NullString
	var created time.Time
	err := a.DB.QueryRowContext(ctx, `SELECT id, applicant_admin_user_id, store_id, requested_role, status, note, created_at, reviewed_at FROM shop_join_requests WHERE id=?`, id).
		Scan(&d.ID, &d.ApplicantID, &d.StoreID, &d.Role, &d.Status, &note, &created, &reviewed)
	if err == sql.ErrNoRows {
		return d, apperr.NotFound
	}
	if err != nil {
		return d, err
	}
	d.Note = scanNullString(note)
	d.CreatedAt = created.UTC().Format(time.RFC3339)
	if reviewed.Valid {
		s := reviewed.Time.UTC().Format(time.RFC3339)
		d.ReviewedAt = &s
	}
	return d, nil
}

func (a *App) MyJoinRequests(ctx context.Context, userID string) ([]ApplicationDTO, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id FROM shop_join_requests WHERE applicant_admin_user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanApps(rows, func(id string) (ApplicationDTO, error) { return a.getJoinRequest(ctx, id) })
}

func (a *App) ListJoinRequests(ctx context.Context, storeID string) ([]ApplicationDTO, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id FROM shop_join_requests WHERE store_id=? ORDER BY created_at DESC`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanApps(rows, func(id string) (ApplicationDTO, error) { return a.getJoinRequest(ctx, id) })
}

func (a *App) ReviewJoinRequest(ctx context.Context, reviewer AdminPrincipal, id, decision, note string) (ApplicationDTO, error) {
	d, err := a.getJoinRequest(ctx, id)
	if err != nil {
		return d, err
	}
	if d.StoreID != reviewer.StoreID || reviewer.Role != domain.RoleOwner {
		return d, apperr.Forbidden
	}
	if d.Status != "PENDING" {
		return d, apperr.StateConflict
	}
	now := a.Now()
	if decision == "REJECTED" {
		_, err = a.DB.ExecContext(ctx, `UPDATE shop_join_requests SET status='REJECTED', note=?, reviewed_at=?, reviewer_id=? WHERE id=?`, note, now, reviewer.UserID, id)
		if err != nil {
			return d, err
		}
		return a.getJoinRequest(ctx, id)
	}
	if decision != "APPROVED" {
		return d, apperr.Validation("decision 无效")
	}
	err = persistTx(ctx, a, func(tx *sql.Tx) error {
		var existingStore sql.NullString
		_ = tx.QueryRow(`SELECT store_id FROM shop_members WHERE admin_user_id=?`, d.ApplicantID).Scan(&existingStore)
		if existingStore.Valid && existingStore.String != d.StoreID {
			return apperr.New(409, "MEMBER_CONFLICT", "该账号已绑定其他门店")
		}
		if d.Role == domain.RoleOwner {
			if _, err := tx.Exec(`UPDATE shop_members SET role=? WHERE store_id=? AND role=?`, domain.RoleManager, d.StoreID, domain.RoleOwner); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`INSERT INTO shop_members (store_id, admin_user_id, role, created_at) VALUES (?,?,?,?)
			ON DUPLICATE KEY UPDATE role=VALUES(role)`, d.StoreID, d.ApplicantID, d.Role, now); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE shop_join_requests SET status='APPROVED', note=?, reviewed_at=?, reviewer_id=? WHERE id=?`, note, now, reviewer.UserID, id)
		return err
	})
	if err != nil {
		return d, err
	}
	return a.getJoinRequest(ctx, id)
}

func persistTx(ctx context.Context, a *App, fn func(*sql.Tx) error) error {
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func scanApps(rows *sql.Rows, get func(string) (ApplicationDTO, error)) ([]ApplicationDTO, error) {
	var out []ApplicationDTO
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		d, err := get(id)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if out == nil {
		out = []ApplicationDTO{}
	}
	return out, rows.Err()
}

func isDup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Duplicate")
}
