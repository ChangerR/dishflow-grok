package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/clock"
	"github.com/changerr/dishflow-grok/internal/config"
	"github.com/changerr/dishflow-grok/internal/cryptoutil"
	"github.com/changerr/dishflow-grok/internal/domain"
	"github.com/changerr/dishflow-grok/internal/ids"
	"github.com/changerr/dishflow-grok/internal/media"
	"github.com/changerr/dishflow-grok/internal/wechat"
)

type App struct {
	Cfg    config.Config
	DB     *sql.DB
	Redis  *redis.Client
	Clock  clock.Clock
	Wechat wechat.Client
	Media  media.Store
}

func New(cfg config.Config, db *sql.DB, rdb *redis.Client, clk clock.Clock) *App {
	if clk == nil {
		clk = clock.System{}
	}
	return &App{
		Cfg:    cfg,
		DB:     db,
		Redis:  rdb,
		Clock:  clk,
		Wechat: wechat.New(cfg),
		Media:  media.NewLocal(cfg.UploadDir),
	}
}

func (a *App) Now() time.Time { return a.Clock.Now().UTC() }

func (a *App) NewID() string { return ids.New() }

type AdminPrincipal struct {
	UserID           string `json:"id"`
	LoginName        string `json:"login_name"`
	DisplayName      string `json:"display_name"`
	IsPlatformAdmin  bool   `json:"is_platform_admin"`
	Enabled          bool   `json:"enabled"`
	StoreID          string `json:"store_id,omitempty"`
	Role             string `json:"role,omitempty"`
	StoreName        string `json:"store_name,omitempty"`
	StoreOpen        bool   `json:"store_open,omitempty"`
	MockPayment      bool   `json:"mock_payment,omitempty"`
	MockPrint        bool   `json:"mock_print,omitempty"`
}

type CustomerPrincipal struct {
	CustomerID string
	StoreID    string
	OpenID     string
}

func (a *App) BootstrapPlatform(ctx context.Context) error {
	var n int
	if err := a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users WHERE is_platform_admin=1`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := cryptoutil.HashPassword(a.Cfg.BootstrapPassword)
	if err != nil {
		return err
	}
	now := a.Now()
	_, err = a.DB.ExecContext(ctx, `INSERT INTO admin_users (id, login_name, display_name, password_hash, is_platform_admin, enabled, created_at, updated_at)
		VALUES (?,?,?,?,1,1,?,?)`, a.NewID(), a.Cfg.BootstrapLogin, a.Cfg.BootstrapName, hash, now, now)
	return err
}

func (a *App) ClientIP(remoteAddr, xff string) string {
	if len(a.Cfg.TrustedProxies) > 0 && xff != "" {
		host, _, _ := net.SplitHostPort(strings.TrimSpace(remoteAddr))
		trusted := false
		for _, p := range a.Cfg.TrustedProxies {
			if p == host {
				trusted = true
				break
			}
		}
		if trusted {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func (a *App) Audit(ctx context.Context, storeID, actorID, actorType, action, resource, summary, requestID string) {
	_, _ = a.DB.ExecContext(ctx, `INSERT INTO audit_logs (id, store_id, actor_id, actor_type, action, resource, summary, request_id, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`, a.NewID(), nullS(storeID), nullS(actorID), actorType, action, resource, clip(summary, 512), requestID, a.Now())
}

func (a *App) Outbox(tx *sql.Tx, storeID, eventType string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO outbox (id, store_id, event_type, payload, created_at) VALUES (?,?,?,?,?)`,
		a.NewID(), storeID, eventType, string(b), a.Now())
	return err
}

func (a *App) IdempotencyGet(ctx context.Context, subject, key, reqHash string) (int, []byte, bool, error) {
	var status int
	var body string
	var storedHash string
	err := a.DB.QueryRowContext(ctx, `SELECT status_code, response_body, request_hash FROM idempotency_keys WHERE subject=? AND idem_key=?`, subject, key).
		Scan(&status, &body, &storedHash)
	if err == sql.ErrNoRows {
		return 0, nil, false, nil
	}
	if err != nil {
		return 0, nil, false, err
	}
	if storedHash != reqHash {
		return 0, nil, false, apperr.IdempotencyConflict
	}
	return status, []byte(body), true, nil
}

func (a *App) IdempotencyPut(ctx context.Context, subject, key, reqHash string, status int, body []byte) error {
	_, err := a.DB.ExecContext(ctx, `INSERT INTO idempotency_keys (id, subject, idem_key, request_hash, status_code, response_body, created_at)
		VALUES (?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE status_code=VALUES(status_code), response_body=VALUES(response_body)`,
		a.NewID(), subject, key, reqHash, status, string(body), a.Now())
	return err
}

func HashRequest(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func nullS(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func scanNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func requireRole(role string, need string) error {
	if !domain.RoleAtLeast(role, need) {
		return apperr.Forbidden
	}
	return nil
}

func ownerOnly(role string) error {
	if role != domain.RoleOwner {
		return apperr.Forbidden
	}
	return nil
}

func managerPlus(role string) error {
	return requireRole(role, domain.RoleManager)
}

func (a *App) storeHours(ctx context.Context, storeID string) (domain.StoreHours, error) {
	var h domain.StoreHours
	err := a.DB.QueryRowContext(ctx, `SELECT timezone, business_hours, scheduled_pickup_enabled, pickup_advance_days, pickup_slot_minutes, pickup_slot_capacity, pickup_min_lead_minutes, pickup_minutes
		FROM stores WHERE id=?`, storeID).Scan(&h.Timezone, &h.BusinessHours, &h.ScheduledEnabled, &h.AdvanceDays, &h.SlotMinutes, &h.SlotCapacity, &h.MinLeadMinutes, &h.PickupMinutes)
	return h, err
}
