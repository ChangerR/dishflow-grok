package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/ids"
)

type StoreExport struct {
	Format     string          `json:"format"`
	Version    int             `json:"version"`
	ExportedAt string          `json:"exported_at"`
	SourceID   string          `json:"source_store_id"`
	Sections   []string        `json:"sections"`
	Data       json.RawMessage `json:"data"`
}

func (a *App) ExportStore(ctx context.Context, storeID string, sections []string) ([]byte, error) {
	if len(sections) == 0 {
		sections = []string{"basic", "menu", "miniprogram"}
	}
	payload := map[string]any{}
	for _, s := range sections {
		switch s {
		case "basic":
			st, err := a.AdminStore(ctx, storeID)
			if err != nil {
				return nil, err
			}
			payload["basic"] = st
		case "menu":
			cats, _ := a.ListCategories(ctx, storeID, false)
			dishes, _ := a.ListDishes(ctx, storeID, false)
			payload["menu"] = map[string]any{"categories": cats, "dishes": dishes}
		case "miniprogram":
			mp, _ := a.MiniprogramConfig(ctx, storeID)
			delete(mp, "appsecret_configured")
			payload["miniprogram"] = mp
		}
	}
	raw, _ := json.Marshal(payload)
	doc := StoreExport{Format: "dishflow.store-export", Version: 1, ExportedAt: a.Now().UTC().Format(time.RFC3339), SourceID: storeID, Sections: sections, Data: raw}
	return json.Marshal(doc)
}

func (a *App) ImportStore(ctx context.Context, storeID string, raw []byte, overwriteAppID bool) error {
	if len(raw) > 1500*1024 {
		return apperr.Validation("导入文件不能超过 1.5MB")
	}
	var doc StoreExport
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Format != "dishflow.store-export" || doc.Version != 1 {
		return apperr.Validation("不支持的导入文件")
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(doc.Data, &data); err != nil {
		return apperr.Validation("导入数据无效")
	}
	now := a.Now()
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		if basic, ok := data["basic"]; ok {
			var m map[string]any
			_ = json.Unmarshal(basic, &m)
			if name, _ := m["name"].(string); name != "" {
				if _, err := tx.Exec(`UPDATE stores SET name=?, phone=?, address=?, business_hours=?, announcement=?, updated_at=? WHERE id=?`,
					name, m["phone"], m["address"], m["business_hours"], m["announcement"], now, storeID); err != nil {
					return err
				}
			}
		}
		if menu, ok := data["menu"]; ok {
			var payload struct {
				Categories []CategoryDTO `json:"categories"`
				Dishes     []DishDTO     `json:"dishes"`
			}
			if err := json.Unmarshal(menu, &payload); err != nil {
				return apperr.Validation("菜单数据无效")
			}
			until := now.Add(30 * 24 * time.Hour)
			batch := ids.New()
			if _, err := tx.Exec(`UPDATE categories SET deleted_at=?, restore_until=?, delete_batch_id=?, updated_at=? WHERE store_id=? AND deleted_at IS NULL`, now, until, batch, now, storeID); err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE products SET deleted_at=?, restore_until=?, delete_batch_id=?, enabled=0, updated_at=? WHERE store_id=? AND deleted_at IS NULL`, now, until, batch, now, storeID); err != nil {
				return err
			}
			catMap := map[string]string{}
			for _, c := range payload.Categories {
				nid := ids.New()
				catMap[c.ID] = nid
				if _, err := tx.Exec(`INSERT INTO categories (id, store_id, name, enabled, sort_order, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`,
					nid, storeID, c.Name, c.Enabled, c.SortOrder, now, now); err != nil {
					return err
				}
			}
			for _, d := range payload.Dishes {
				newCat := catMap[d.CategoryID]
				if newCat == "" {
					continue
				}
				pid := ids.New()
				if _, err := tx.Exec(`INSERT INTO products (id, store_id, category_id, code, name, description, image_url, sort_order, enabled, sold_out, packing_fee_cents, created_at, updated_at)
					VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, pid, storeID, newCat, nullS(d.Code), d.Name, nullS(d.Description), nullS(d.ImageURL), d.SortOrder, d.Enabled, d.SoldOut, d.PackingFeeCents, now, now); err != nil {
					return err
				}
				for _, s := range d.SKUs {
					if _, err := tx.Exec(`INSERT INTO skus (id, store_id, product_id, name, price_cents, stock_mode, daily_stock, enabled, sort_order, is_default, created_at, updated_at)
						VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, ids.New(), storeID, pid, s.Name, s.PriceCents, s.StockMode, s.DailyStock, s.Enabled, s.SortOrder, s.IsDefault, now, now); err != nil {
						return err
					}
				}
				for _, g := range d.OptionGroups {
					gid := ids.New()
					if _, err := tx.Exec(`INSERT INTO option_groups (id, store_id, product_id, name, selection_type, required, min_select, max_select, sort_order, created_at, updated_at)
						VALUES (?,?,?,?,?,?,?,?,?,?,?)`, gid, storeID, pid, g.Name, g.SelectionType, g.Required, g.MinSelect, g.MaxSelect, g.SortOrder, now, now); err != nil {
						return err
					}
					for _, it := range g.Items {
						if _, err := tx.Exec(`INSERT INTO option_items (id, store_id, option_group_id, name, price_cents, enabled, sort_order, is_default, created_at, updated_at)
							VALUES (?,?,?,?,?,?,?,?,?,?)`, ids.New(), storeID, gid, it.Name, it.PriceCents, it.Enabled, it.SortOrder, it.IsDefault, now, now); err != nil {
							return err
						}
					}
				}
			}
		}
		if mp, ok := data["miniprogram"]; ok {
			var m map[string]any
			_ = json.Unmarshal(mp, &m)
			brand, _ := m["brand_name"].(string)
			color, _ := m["theme_color"].(string)
			logo, _ := m["logo_url"].(string)
			if _, err := tx.Exec(`UPDATE stores SET brand_name=?, theme_color=?, logo_url=?, updated_at=? WHERE id=?`, brand, color, logo, now, storeID); err != nil {
				return err
			}
			if overwriteAppID {
				appid, _ := m["wechat_appid"].(string)
				if appid != "" {
					var other string
					err := tx.QueryRow(`SELECT id FROM stores WHERE wechat_appid=? AND id<>?`, appid, storeID).Scan(&other)
					if err == nil {
						return apperr.WechatAppIDConflict
					}
					if _, err := tx.Exec(`UPDATE stores SET wechat_appid=?, updated_at=? WHERE id=?`, appid, now, storeID); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func (a *App) ReleaseExpiredHolds(ctx context.Context) error {
	rows, err := a.DB.QueryContext(ctx, `SELECT o.id, o.store_id FROM orders o JOIN payments p ON p.order_id=o.id WHERE o.status='PENDING_PAYMENT' AND p.expires_at < ?`, a.Now())
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, store string
		if err := rows.Scan(&id, &store); err != nil {
			return err
		}
		_ = a.closeUnpaid(ctx, store, id, "SYSTEM")
	}
	return rows.Err()
}

func (a *App) HandlePayNotify(ctx context.Context, pathStoreID, eventID, appid, mchid, orderID, txnID string, amount int64) error {
	var storeID string
	var payable int64
	var status string
	err := a.DB.QueryRowContext(ctx, `SELECT store_id, payable_cents, status FROM orders WHERE id=?`, orderID).Scan(&storeID, &payable, &status)
	if err != nil {
		return apperr.NotFound
	}
	if storeID != pathStoreID {
		return apperr.Forbidden
	}
	if amount != payable {
		return apperr.Validation("金额不符")
	}
	var n int
	_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM webhook_events WHERE provider_event_id=?`, eventID).Scan(&n)
	if n > 0 {
		return nil
	}
	if err := a.ConfirmPaid(ctx, storeID, orderID, txnID, false); err != nil {
		return err
	}
	_, _ = a.DB.ExecContext(ctx, `INSERT IGNORE INTO webhook_events (id, provider_event_id, store_id, kind, processed_at) VALUES (?,?,?,?,?)`,
		ids.New(), eventID, storeID, "PAY", a.Now())
	_ = appid
	_ = mchid
	return nil
}

func (a *App) HandleRefundNotify(ctx context.Context, pathStoreID, eventID, orderID, refundNo, wechatID string, amount int64) error {
	var storeID string
	if err := a.DB.QueryRowContext(ctx, `SELECT store_id FROM orders WHERE id=?`, orderID).Scan(&storeID); err != nil {
		return apperr.NotFound
	}
	if storeID != pathStoreID {
		return apperr.Forbidden
	}
	var n int
	_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM webhook_events WHERE provider_event_id=?`, eventID).Scan(&n)
	if n > 0 {
		return nil
	}
	if err := a.ConfirmRefundSuccess(ctx, storeID, orderID, wechatID); err != nil {
		return err
	}
	_, _ = a.DB.ExecContext(ctx, `INSERT IGNORE INTO webhook_events (id, provider_event_id, store_id, kind, processed_at) VALUES (?,?,?,?,?)`,
		ids.New(), eventID, storeID, "REFUND", a.Now())
	_ = refundNo
	_ = amount
	return nil
}
