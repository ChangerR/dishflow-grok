package app

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/cryptoutil"
	"github.com/changerr/dishflow-grok/internal/domain"
	"github.com/changerr/dishflow-grok/internal/media"
)

type BootstrapDTO struct {
	StoreID     string `json:"store_id"`
	StoreName   string `json:"store_name"`
	BrandName   string `json:"brand_name"`
	ThemeColor  string `json:"theme_color,omitempty"`
	LogoURL     string `json:"logo_url,omitempty"`
	IsOpen      bool   `json:"is_open"`
	Announcement string `json:"announcement,omitempty"`
}

type PublicStoreDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Phone          string `json:"phone,omitempty"`
	Address        string `json:"address,omitempty"`
	BusinessHours  string `json:"business_hours"`
	IsOpen         bool   `json:"is_open"`
	Timezone       string `json:"timezone"`
	Announcement   string `json:"announcement,omitempty"`
	PickupMinutes  int    `json:"pickup_minutes"`
	ScheduledPickupEnabled bool `json:"scheduled_pickup_enabled"`
	PickupAdvanceDays int `json:"pickup_advance_days"`
	PickupSlotMinutes int `json:"pickup_slot_minutes"`
	PickupSlotCapacity int `json:"pickup_slot_capacity"`
	PickupMinLeadMinutes int `json:"pickup_min_lead_minutes"`
}

type PoliciesDTO struct {
	PrivacyPolicy  string `json:"privacy_policy,omitempty"`
	RefundPolicy   string `json:"refund_policy,omitempty"`
	Qualifications string `json:"qualifications,omitempty"`
}

func (a *App) Bootstrap(ctx context.Context, storeID string) (BootstrapDTO, error) {
	var d BootstrapDTO
	var color, logo, ann sql.NullString
	err := a.DB.QueryRowContext(ctx, `SELECT id, name, COALESCE(brand_name,name), theme_color, logo_url, is_open, announcement FROM stores WHERE id=? AND enabled=1`, storeID).
		Scan(&d.StoreID, &d.StoreName, &d.BrandName, &color, &logo, &d.IsOpen, &ann)
	if err == sql.ErrNoRows {
		return d, apperr.NotFound
	}
	d.ThemeColor = scanNullString(color)
	d.LogoURL = scanNullString(logo)
	d.Announcement = scanNullString(ann)
	d.BrandName = strings.TrimSpace(d.BrandName)
	return d, err
}

func (a *App) PublicStore(ctx context.Context, storeID string) (PublicStoreDTO, error) {
	var d PublicStoreDTO
	var phone, addr, ann sql.NullString
	err := a.DB.QueryRowContext(ctx, `SELECT id, name, phone, address, business_hours, is_open, timezone, announcement, pickup_minutes, scheduled_pickup_enabled, pickup_advance_days, pickup_slot_minutes, pickup_slot_capacity, pickup_min_lead_minutes
		FROM stores WHERE id=? AND enabled=1`, storeID).
		Scan(&d.ID, &d.Name, &phone, &addr, &d.BusinessHours, &d.IsOpen, &d.Timezone, &ann, &d.PickupMinutes, &d.ScheduledPickupEnabled, &d.PickupAdvanceDays, &d.PickupSlotMinutes, &d.PickupSlotCapacity, &d.PickupMinLeadMinutes)
	if err == sql.ErrNoRows {
		return d, apperr.NotFound
	}
	d.Phone = scanNullString(phone)
	d.Address = scanNullString(addr)
	d.Announcement = scanNullString(ann)
	return d, err
}

func (a *App) Policies(ctx context.Context, storeID string) (PoliciesDTO, error) {
	var d PoliciesDTO
	var p, r, q sql.NullString
	err := a.DB.QueryRowContext(ctx, `SELECT privacy_policy, refund_policy, qualifications FROM stores WHERE id=?`, storeID).Scan(&p, &r, &q)
	if err == sql.ErrNoRows {
		return d, apperr.NotFound
	}
	d.PrivacyPolicy = scanNullString(p)
	d.RefundPolicy = scanNullString(r)
	d.Qualifications = scanNullString(q)
	return d, err
}

type CustomerMenu struct {
	Categories []CustomerCategory `json:"categories"`
}

type CustomerCategory struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Dishes []CustomerDish `json:"dishes"`
}

type CustomerDish struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	ImageURL      string         `json:"image_url,omitempty"`
	FromPrice     int64          `json:"from_price_cents"`
	SoldOut       bool           `json:"sold_out"`
	OptionCount   int            `json:"option_count"`
	SKUs          []CustomerSKU  `json:"skus"`
	OptionGroups  []CustomerOG   `json:"option_groups"`
}

type CustomerSKU struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	PriceCents   int64  `json:"price_cents"`
	StockMode    string `json:"stock_mode"`
	Remaining    *int   `json:"remaining,omitempty"`
	Enabled      bool   `json:"enabled"`
	IsDefault    bool   `json:"is_default"`
}

type CustomerOG struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	SelectionType string        `json:"selection_type"`
	Required      bool          `json:"required"`
	MinSelect     int           `json:"min_select"`
	MaxSelect     int           `json:"max_select"`
	Items         []CustomerOI  `json:"items"`
}

type CustomerOI struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
	Enabled    bool   `json:"enabled"`
	IsDefault  bool   `json:"is_default"`
}

func (a *App) CustomerMenu(ctx context.Context, storeID string) (CustomerMenu, error) {
	biz, err := a.businessDate(ctx, storeID, nil)
	if err != nil {
		return CustomerMenu{}, err
	}
	rows, err := a.DB.QueryContext(ctx, `SELECT id, name FROM categories WHERE store_id=? AND deleted_at IS NULL AND enabled=1 ORDER BY sort_order ASC, id ASC`, storeID)
	if err != nil {
		return CustomerMenu{}, err
	}
	defer rows.Close()
	out := CustomerMenu{Categories: []CustomerCategory{}}
	for rows.Next() {
		var c CustomerCategory
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return out, err
		}
		dishes, err := a.customerDishes(ctx, storeID, c.ID, biz)
		if err != nil {
			return out, err
		}
		c.Dishes = dishes
		out.Categories = append(out.Categories, c)
	}
	return out, rows.Err()
}

func (a *App) customerDishes(ctx context.Context, storeID, catID, biz string) ([]CustomerDish, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, name, description, image_url, sold_out FROM products WHERE store_id=? AND category_id=? AND deleted_at IS NULL AND enabled=1 ORDER BY sort_order ASC, id ASC`, storeID, catID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dishes []CustomerDish
	for rows.Next() {
		var d CustomerDish
		var desc, img sql.NullString
		if err := rows.Scan(&d.ID, &d.Name, &desc, &img, &d.SoldOut); err != nil {
			return nil, err
		}
		d.Description = scanNullString(desc)
		d.ImageURL = scanNullString(img)
		if d.ImageURL != "" && !media.TrustedImageURL(d.ImageURL) {
			d.ImageURL = ""
		}
		skus, err := a.customerSKUs(ctx, storeID, d.ID, biz, d.SoldOut)
		if err != nil {
			return nil, err
		}
		sellable := 0
		var from int64 = -1
		for _, s := range skus {
			if s.Enabled && (s.Remaining == nil || *s.Remaining > 0) && !d.SoldOut {
				sellable++
				if from < 0 || s.PriceCents < from {
					from = s.PriceCents
				}
			}
		}
		if sellable == 0 {
			continue
		}
		d.SKUs = skus
		d.FromPrice = from
		ogs, err := a.customerOGs(ctx, d.ID)
		if err != nil {
			return nil, err
		}
		d.OptionGroups = ogs
		n := 0
		for _, g := range ogs {
			n += len(g.Items)
		}
		d.OptionCount = n
		dishes = append(dishes, d)
	}
	if dishes == nil {
		dishes = []CustomerDish{}
	}
	return dishes, rows.Err()
}

func (a *App) customerSKUs(ctx context.Context, storeID, productID, biz string, soldOut bool) ([]CustomerSKU, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, name, price_cents, stock_mode, daily_stock, enabled, is_default FROM skus WHERE product_id=? ORDER BY sort_order ASC, id ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CustomerSKU
	for rows.Next() {
		var s CustomerSKU
		var daily int
		if err := rows.Scan(&s.ID, &s.Name, &s.PriceCents, &s.StockMode, &daily, &s.Enabled, &s.IsDefault); err != nil {
			return nil, err
		}
		if s.StockMode == domain.StockDaily {
			rem := a.remainingStock(ctx, storeID, s.ID, biz, daily)
			if soldOut {
				z := 0
				s.Remaining = &z
			} else {
				s.Remaining = &rem
			}
		}
		out = append(out, s)
	}
	if out == nil {
		out = []CustomerSKU{}
	}
	return out, rows.Err()
}

func (a *App) remainingStock(ctx context.Context, storeID, skuID, biz string, daily int) int {
	var avail, reserved, sold int
	err := a.DB.QueryRowContext(ctx, `SELECT available_qty, reserved_qty, sold_qty FROM daily_inventory WHERE store_id=? AND sku_id=? AND business_date=?`, storeID, skuID, biz).
		Scan(&avail, &reserved, &sold)
	if err == sql.ErrNoRows {
		return daily
	}
	if err != nil {
		return 0
	}
	r := avail - reserved - sold
	if r < 0 {
		return 0
	}
	return r
}

func (a *App) customerOGs(ctx context.Context, productID string) ([]CustomerOG, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, name, selection_type, required, min_select, max_select FROM option_groups WHERE product_id=? ORDER BY sort_order ASC, id ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CustomerOG
	for rows.Next() {
		var g CustomerOG
		if err := rows.Scan(&g.ID, &g.Name, &g.SelectionType, &g.Required, &g.MinSelect, &g.MaxSelect); err != nil {
			return nil, err
		}
		items, err := a.DB.QueryContext(ctx, `SELECT id, name, price_cents, enabled, is_default FROM option_items WHERE option_group_id=? AND enabled=1 ORDER BY sort_order ASC, id ASC`, g.ID)
		if err != nil {
			return nil, err
		}
		for items.Next() {
			var it CustomerOI
			if err := items.Scan(&it.ID, &it.Name, &it.PriceCents, &it.Enabled, &it.IsDefault); err != nil {
				items.Close()
				return nil, err
			}
			g.Items = append(g.Items, it)
		}
		items.Close()
		if g.Items == nil {
			g.Items = []CustomerOI{}
		}
		out = append(out, g)
	}
	if out == nil {
		out = []CustomerOG{}
	}
	return out, rows.Err()
}

func (a *App) businessDate(ctx context.Context, storeID string, scheduled *time.Time) (string, error) {
	h, err := a.storeHours(ctx, storeID)
	if err != nil {
		return "", err
	}
	if scheduled != nil {
		loc, err := domain.LoadLocation(h.Timezone)
		if err != nil {
			return "", err
		}
		return scheduled.In(loc).Format("2006-01-02"), nil
	}
	return domain.BusinessDate(a.Now(), h.Timezone)
}

func (a *App) ResolveTable(ctx context.Context, storeID, token string) (map[string]any, error) {
	if token == "" {
		return nil, apperr.TableNotFound
	}
	var id, no string
	var enabled bool
	err := a.DB.QueryRowContext(ctx, `SELECT id, table_no, enabled FROM dining_tables WHERE store_id=? AND token_hash=?`, storeID, cryptoutil.HashToken(token)).
		Scan(&id, &no, &enabled)
	if err == sql.ErrNoRows {
		return nil, apperr.TableNotFound
	}
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, apperr.TableDisabled
	}
	return map[string]any{"id": id, "table_no": no, "enabled": enabled}, nil
}

func (a *App) PickupSlots(ctx context.Context, storeID, date string) (map[string]any, error) {
	h, err := a.storeHours(ctx, storeID)
	if err != nil {
		return nil, err
	}
	if date == "" {
		date, err = domain.BusinessDate(a.Now(), h.Timezone)
		if err != nil {
			return nil, err
		}
	}
	reserved := map[string]int{}
	rows, err := a.DB.QueryContext(ctx, `SELECT slot_start, reserved_orders FROM pickup_slot_capacity WHERE store_id=? AND DATE(slot_start)=?`, storeID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t time.Time
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		reserved[t.UTC().Format(time.RFC3339)] = n
	}
	slots, err := domain.GenerateSlots(h, date, a.Now(), reserved)
	if err != nil {
		return nil, err
	}
	loc, _ := domain.LoadLocation(h.Timezone)
	return map[string]any{
		"timezone": h.Timezone,
		"date":     date,
		"offset":   loc.String(),
		"slots":    slots,
	}, nil
}
