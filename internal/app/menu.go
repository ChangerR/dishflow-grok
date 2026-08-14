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

type CategoryDTO struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Enabled       bool    `json:"enabled"`
	SortOrder     int     `json:"sort_order"`
	Deleted       bool    `json:"deleted"`
	RestoreUntil  *string `json:"restore_until,omitempty"`
}

type DishDTO struct {
	ID              string          `json:"id"`
	CategoryID      string          `json:"category_id"`
	Code            string          `json:"code,omitempty"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	ImageURL        string          `json:"image_url,omitempty"`
	SortOrder       int             `json:"sort_order"`
	Enabled         bool            `json:"enabled"`
	SoldOut         bool            `json:"sold_out"`
	PackingFeeCents int64           `json:"packing_fee_cents"`
	Deleted         bool            `json:"deleted"`
	RestoreUntil    *string         `json:"restore_until,omitempty"`
	SKUs            []SKUIn         `json:"skus"`
	OptionGroups    []OGIn          `json:"option_groups"`
}

type SKUIn struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
	StockMode  string `json:"stock_mode"`
	DailyStock int    `json:"daily_stock"`
	Enabled    bool   `json:"enabled"`
	SortOrder  int    `json:"sort_order"`
	IsDefault  bool   `json:"is_default"`
}

type OGIn struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name"`
	SelectionType string `json:"selection_type"`
	Required      bool   `json:"required"`
	MinSelect     int    `json:"min_select"`
	MaxSelect     int    `json:"max_select"`
	SortOrder     int    `json:"sort_order"`
	Items         []OIIn `json:"items"`
}

type OIIn struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
	Enabled    bool   `json:"enabled"`
	SortOrder  int    `json:"sort_order"`
	IsDefault  bool   `json:"is_default"`
}

func (a *App) ListCategories(ctx context.Context, storeID string, includeDeleted bool) ([]CategoryDTO, error) {
	q := `SELECT id, name, enabled, sort_order, deleted_at, restore_until, restore_closed FROM categories WHERE store_id=?`
	if !includeDeleted {
		q += ` AND deleted_at IS NULL`
	}
	q += ` ORDER BY (deleted_at IS NOT NULL), enabled DESC, sort_order ASC, id ASC`
	rows, err := a.DB.QueryContext(ctx, q, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CategoryDTO
	for rows.Next() {
		var c CategoryDTO
		var del, until sql.NullTime
		var closed bool
		if err := rows.Scan(&c.ID, &c.Name, &c.Enabled, &c.SortOrder, &del, &until, &closed); err != nil {
			return nil, err
		}
		c.Deleted = del.Valid
		if until.Valid && !closed && a.Now().Before(until.Time) {
			s := until.Time.UTC().Format(time.RFC3339)
			c.RestoreUntil = &s
		}
		out = append(out, c)
	}
	if out == nil {
		out = []CategoryDTO{}
	}
	return out, rows.Err()
}

func (a *App) CreateCategory(ctx context.Context, storeID, name string, enabled bool, sort int) (CategoryDTO, error) {
	if strings.TrimSpace(name) == "" {
		return CategoryDTO{}, apperr.Validation("分类名称不能为空")
	}
	id := a.NewID()
	now := a.Now()
	_, err := a.DB.ExecContext(ctx, `INSERT INTO categories (id, store_id, name, enabled, sort_order, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`,
		id, storeID, name, enabled, sort, now, now)
	if err != nil {
		return CategoryDTO{}, err
	}
	return CategoryDTO{ID: id, Name: name, Enabled: enabled, SortOrder: sort}, nil
}

func (a *App) PatchCategory(ctx context.Context, storeID, id, name string, enabled *bool, sort *int) (CategoryDTO, error) {
	cats, err := a.ListCategories(ctx, storeID, true)
	if err != nil {
		return CategoryDTO{}, err
	}
	var found *CategoryDTO
	for i := range cats {
		if cats[i].ID == id {
			found = &cats[i]
			break
		}
	}
	if found == nil || found.Deleted {
		return CategoryDTO{}, apperr.NotFound
	}
	if name != "" {
		found.Name = name
	}
	if enabled != nil {
		found.Enabled = *enabled
	}
	if sort != nil {
		found.SortOrder = *sort
	}
	_, err = a.DB.ExecContext(ctx, `UPDATE categories SET name=?, enabled=?, sort_order=?, updated_at=? WHERE id=? AND store_id=?`, found.Name, found.Enabled, found.SortOrder, a.Now(), id, storeID)
	return *found, err
}

func (a *App) DeleteCategory(ctx context.Context, storeID, id string) error {
	now := a.Now()
	until := now.Add(30 * 24 * time.Hour)
	batch := a.NewID()
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE categories SET deleted_at=?, restore_until=?, restore_closed=0, delete_batch_id=?, updated_at=? WHERE id=? AND store_id=? AND deleted_at IS NULL`,
			now, until, batch, now, id, storeID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return apperr.NotFound
		}
		_, err = tx.Exec(`UPDATE products SET deleted_at=?, restore_until=?, restore_closed=0, delete_batch_id=?, enabled=0, updated_at=? WHERE category_id=? AND store_id=? AND deleted_at IS NULL`,
			now, until, batch, now, id, storeID)
		return err
	})
}

func (a *App) RestoreCategory(ctx context.Context, storeID, id string) error {
	now := a.Now()
	var until sql.NullTime
	var batch sql.NullString
	var closed bool
	err := a.DB.QueryRowContext(ctx, `SELECT restore_until, restore_closed, delete_batch_id FROM categories WHERE id=? AND store_id=?`, id, storeID).
		Scan(&until, &closed, &batch)
	if err == sql.ErrNoRows {
		return apperr.NotFound
	}
	if err != nil {
		return err
	}
	if closed || !until.Valid || now.After(until.Time) {
		return apperr.New(409, "RESTORE_EXPIRED", "恢复窗口已关闭")
	}
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE categories SET deleted_at=NULL, restore_until=NULL, delete_batch_id=NULL, updated_at=? WHERE id=?`, now, id); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE products SET deleted_at=NULL, restore_until=NULL, delete_batch_id=NULL, updated_at=? WHERE store_id=? AND delete_batch_id=?`, now, storeID, batch.String)
		return err
	})
}

func (a *App) ListDishes(ctx context.Context, storeID string, includeDeleted bool) ([]DishDTO, error) {
	q := `SELECT id FROM products WHERE store_id=?`
	if !includeDeleted {
		q += ` AND deleted_at IS NULL`
	}
	q += ` ORDER BY sort_order ASC, id ASC`
	rows, err := a.DB.QueryContext(ctx, q, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DishDTO
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		d, err := a.GetDish(ctx, storeID, id)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if out == nil {
		out = []DishDTO{}
	}
	return out, rows.Err()
}

func (a *App) GetDish(ctx context.Context, storeID, id string) (DishDTO, error) {
	var d DishDTO
	var code, desc, img sql.NullString
	var del, until sql.NullTime
	var closed bool
	err := a.DB.QueryRowContext(ctx, `SELECT id, category_id, code, name, description, image_url, sort_order, enabled, sold_out, packing_fee_cents, deleted_at, restore_until, restore_closed
		FROM products WHERE id=? AND store_id=?`, id, storeID).
		Scan(&d.ID, &d.CategoryID, &code, &d.Name, &desc, &img, &d.SortOrder, &d.Enabled, &d.SoldOut, &d.PackingFeeCents, &del, &until, &closed)
	if err == sql.ErrNoRows {
		return d, apperr.NotFound
	}
	if err != nil {
		return d, err
	}
	d.Code = scanNullString(code)
	d.Description = scanNullString(desc)
	d.ImageURL = scanNullString(img)
	d.Deleted = del.Valid
	if until.Valid && !closed && a.Now().Before(until.Time) {
		s := until.Time.UTC().Format(time.RFC3339)
		d.RestoreUntil = &s
	}
	d.SKUs, _ = a.loadSKUs(ctx, id)
	d.OptionGroups, _ = a.loadOGs(ctx, id)
	return d, nil
}

func (a *App) loadSKUs(ctx context.Context, productID string) ([]SKUIn, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, name, price_cents, stock_mode, daily_stock, enabled, sort_order, is_default FROM skus WHERE product_id=? ORDER BY sort_order`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SKUIn
	for rows.Next() {
		var s SKUIn
		if err := rows.Scan(&s.ID, &s.Name, &s.PriceCents, &s.StockMode, &s.DailyStock, &s.Enabled, &s.SortOrder, &s.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if out == nil {
		out = []SKUIn{}
	}
	return out, rows.Err()
}

func (a *App) loadOGs(ctx context.Context, productID string) ([]OGIn, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, name, selection_type, required, min_select, max_select, sort_order FROM option_groups WHERE product_id=? ORDER BY sort_order`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OGIn
	for rows.Next() {
		var g OGIn
		if err := rows.Scan(&g.ID, &g.Name, &g.SelectionType, &g.Required, &g.MinSelect, &g.MaxSelect, &g.SortOrder); err != nil {
			return nil, err
		}
		items, err := a.DB.QueryContext(ctx, `SELECT id, name, price_cents, enabled, sort_order, is_default FROM option_items WHERE option_group_id=? ORDER BY sort_order`, g.ID)
		if err != nil {
			return nil, err
		}
		for items.Next() {
			var it OIIn
			if err := items.Scan(&it.ID, &it.Name, &it.PriceCents, &it.Enabled, &it.SortOrder, &it.IsDefault); err != nil {
				items.Close()
				return nil, err
			}
			g.Items = append(g.Items, it)
		}
		items.Close()
		if g.Items == nil {
			g.Items = []OIIn{}
		}
		out = append(out, g)
	}
	if out == nil {
		out = []OGIn{}
	}
	return out, rows.Err()
}

func (a *App) SaveDish(ctx context.Context, storeID, id string, in DishDTO) (DishDTO, error) {
	if strings.TrimSpace(in.Name) == "" || in.CategoryID == "" {
		return DishDTO{}, apperr.Validation("菜品名称和分类不能为空")
	}
	if in.PackingFeeCents < 0 {
		return DishDTO{}, apperr.Validation("包装费不能为负")
	}
	if in.ImageURL != "" && !media.TrustedImageURL(in.ImageURL) {
		return DishDTO{}, apperr.Validation("图片地址不安全")
	}
	if len(in.SKUs) == 0 {
		return DishDTO{}, apperr.Validation("至少需要一个 SKU")
	}
	if err := validateOptions(in.OptionGroups); err != nil {
		return DishDTO{}, err
	}
	var catDeleted sql.NullTime
	if err := a.DB.QueryRowContext(ctx, `SELECT deleted_at FROM categories WHERE id=? AND store_id=?`, in.CategoryID, storeID).Scan(&catDeleted); err != nil {
		return DishDTO{}, apperr.NotFound
	}
	now := a.Now()
	create := id == ""
	if create {
		id = a.NewID()
	}
	err := persistTx(ctx, a, func(tx *sql.Tx) error {
		if create {
			_, err := tx.Exec(`INSERT INTO products (id, store_id, category_id, code, name, description, image_url, sort_order, enabled, sold_out, packing_fee_cents, created_at, updated_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, storeID, in.CategoryID, nullS(in.Code), in.Name, nullS(in.Description), nullS(in.ImageURL), in.SortOrder, in.Enabled, in.SoldOut, in.PackingFeeCents, now, now)
			if err != nil {
				return err
			}
		} else {
			res, err := tx.Exec(`UPDATE products SET category_id=?, code=?, name=?, description=?, image_url=?, sort_order=?, enabled=?, sold_out=?, packing_fee_cents=?, updated_at=?
				WHERE id=? AND store_id=? AND deleted_at IS NULL`, in.CategoryID, nullS(in.Code), in.Name, nullS(in.Description), nullS(in.ImageURL), in.SortOrder, in.Enabled, in.SoldOut, in.PackingFeeCents, now, id, storeID)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return apperr.NotFound
			}
			if _, err := tx.Exec(`DELETE FROM option_items WHERE option_group_id IN (SELECT id FROM option_groups WHERE product_id=?)`, id); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM option_groups WHERE product_id=?`, id); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM skus WHERE product_id=?`, id); err != nil {
				return err
			}
		}
		for _, s := range in.SKUs {
			if s.PriceCents < 0 {
				return apperr.Validation("价格不能为负")
			}
			if s.StockMode == "" {
				s.StockMode = domain.StockUnlimited
			}
			sid := s.ID
			if sid == "" {
				sid = a.NewID()
			}
			if _, err := tx.Exec(`INSERT INTO skus (id, store_id, product_id, name, price_cents, stock_mode, daily_stock, enabled, sort_order, is_default, created_at, updated_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, sid, storeID, id, s.Name, s.PriceCents, s.StockMode, s.DailyStock, s.Enabled, s.SortOrder, s.IsDefault, now, now); err != nil {
				return err
			}
		}
		for _, g := range in.OptionGroups {
			gid := g.ID
			if gid == "" {
				gid = a.NewID()
			}
			if _, err := tx.Exec(`INSERT INTO option_groups (id, store_id, product_id, name, selection_type, required, min_select, max_select, sort_order, created_at, updated_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?)`, gid, storeID, id, g.Name, g.SelectionType, g.Required, g.MinSelect, g.MaxSelect, g.SortOrder, now, now); err != nil {
				return err
			}
			for _, it := range g.Items {
				iid := it.ID
				if iid == "" {
					iid = a.NewID()
				}
				if _, err := tx.Exec(`INSERT INTO option_items (id, store_id, option_group_id, name, price_cents, enabled, sort_order, is_default, created_at, updated_at)
					VALUES (?,?,?,?,?,?,?,?,?,?)`, iid, storeID, gid, it.Name, it.PriceCents, it.Enabled, it.SortOrder, it.IsDefault, now, now); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return DishDTO{}, err
	}
	return a.GetDish(ctx, storeID, id)
}

func validateOptions(groups []OGIn) error {
	for _, g := range groups {
		defaults := 0
		for _, it := range g.Items {
			if it.IsDefault {
				if !it.Enabled {
					return apperr.Validation("默认选项必须启用")
				}
				defaults++
			}
		}
		if g.SelectionType == "SINGLE" && defaults > 1 {
			return apperr.Validation("单选组最多一个默认项")
		}
		if g.SelectionType == "MULTI" && defaults > g.MaxSelect {
			return apperr.Validation("多选默认项不能超过最大选择数")
		}
	}
	return nil
}

func (a *App) DeleteDish(ctx context.Context, storeID, id string) error {
	now := a.Now()
	until := now.Add(30 * 24 * time.Hour)
	res, err := a.DB.ExecContext(ctx, `UPDATE products SET deleted_at=?, restore_until=?, restore_closed=0, enabled=0, updated_at=? WHERE id=? AND store_id=? AND deleted_at IS NULL`,
		now, until, now, id, storeID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperr.NotFound
	}
	return nil
}

func (a *App) RestoreDish(ctx context.Context, storeID, id string) error {
	var catID string
	var until sql.NullTime
	var closed bool
	err := a.DB.QueryRowContext(ctx, `SELECT category_id, restore_until, restore_closed FROM products WHERE id=? AND store_id=?`, id, storeID).
		Scan(&catID, &until, &closed)
	if err == sql.ErrNoRows {
		return apperr.NotFound
	}
	if err != nil {
		return err
	}
	if closed || !until.Valid || a.Now().After(until.Time) {
		return apperr.New(409, "RESTORE_EXPIRED", "恢复窗口已关闭")
	}
	var catDel sql.NullTime
	if err := a.DB.QueryRowContext(ctx, `SELECT deleted_at FROM categories WHERE id=?`, catID).Scan(&catDel); err != nil {
		return err
	}
	if catDel.Valid {
		return apperr.New(409, "CATEGORY_DELETED", "请先恢复所属分类")
	}
	_, err = a.DB.ExecContext(ctx, `UPDATE products SET deleted_at=NULL, restore_until=NULL, updated_at=? WHERE id=?`, a.Now(), id)
	return err
}

func (a *App) AdjustStock(ctx context.Context, storeID, dishID, skuID, reason, actorID string, delta int) error {
	if delta == 0 || strings.TrimSpace(reason) == "" {
		return apperr.Validation("调整数量不能为 0，且原因必填")
	}
	var mode string
	var daily int
	var pid string
	err := a.DB.QueryRowContext(ctx, `SELECT product_id, stock_mode, daily_stock FROM skus WHERE id=? AND store_id=?`, skuID, storeID).Scan(&pid, &mode, &daily)
	if err == sql.ErrNoRows || pid != dishID {
		return apperr.NotFound
	}
	if err != nil {
		return err
	}
	if mode != domain.StockDaily {
		return apperr.Validation("仅每日库存 SKU 可调整")
	}
	biz, err := a.businessDate(ctx, storeID, nil)
	if err != nil {
		return err
	}
	now := a.Now()
	return persistTx(ctx, a, func(tx *sql.Tx) error {
		if err := ensureInventory(tx, storeID, skuID, biz, daily); err != nil {
			return err
		}
		var avail, reserved, sold int
		if err := tx.QueryRow(`SELECT available_qty, reserved_qty, sold_qty FROM daily_inventory WHERE store_id=? AND sku_id=? AND business_date=? FOR UPDATE`, storeID, skuID, biz).
			Scan(&avail, &reserved, &sold); err != nil {
			return err
		}
		avail += delta
		if avail < reserved+sold || avail < 0 {
			return apperr.New(409, "STOCK_CONSTRAINT", "调整后可用库存不足")
		}
		if _, err := tx.Exec(`UPDATE daily_inventory SET available_qty=? WHERE store_id=? AND sku_id=? AND business_date=?`, avail, storeID, skuID, biz); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO inventory_movements (id, store_id, sku_id, business_date, delta, reason, actor_id, note, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			a.NewID(), storeID, skuID, biz, delta, "ADJUST", actorID, reason, now)
		return err
	})
}

func ensureInventory(tx *sql.Tx, storeID, skuID, biz string, daily int) error {
	_, err := tx.Exec(`INSERT IGNORE INTO daily_inventory (store_id, sku_id, business_date, available_qty, reserved_qty, sold_qty) VALUES (?,?,?,?,0,0)`,
		storeID, skuID, biz, daily)
	return err
}

func (a *App) CloseExpiredRestores(ctx context.Context) error {
	now := a.Now()
	_, err := a.DB.ExecContext(ctx, `UPDATE categories SET restore_closed=1 WHERE restore_until IS NOT NULL AND restore_until < ? AND restore_closed=0`, now)
	if err != nil {
		return err
	}
	_, err = a.DB.ExecContext(ctx, `UPDATE products SET restore_closed=1 WHERE restore_until IS NOT NULL AND restore_until < ? AND restore_closed=0`, now)
	return err
}

func (a *App) ListTables(ctx context.Context, storeID string) ([]map[string]any, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, table_no, area, enabled, mp_code_object_key FROM dining_tables WHERE store_id=? ORDER BY table_no`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, no string
		var area, key sql.NullString
		var enabled bool
		if err := rows.Scan(&id, &no, &area, &enabled, &key); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "table_no": no, "area": scanNullString(area), "enabled": enabled, "has_code": key.Valid && key.String != ""})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) SaveTable(ctx context.Context, storeID, id, no, area string, enabled bool) (map[string]any, error) {
	if strings.TrimSpace(no) == "" {
		return nil, apperr.Validation("桌号不能为空")
	}
	now := a.Now()
	if id == "" {
		id = a.NewID()
		raw, _ := cryptoutil.RandomHex(16)
		_, err := a.DB.ExecContext(ctx, `INSERT INTO dining_tables (id, store_id, table_no, area, enabled, token_hash, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
			id, storeID, no, nullS(area), enabled, cryptoutil.HashToken(raw), now, now)
		if err != nil {
			if isDup(err) {
				return nil, apperr.Conflict
			}
			return nil, err
		}
	} else {
		_, err := a.DB.ExecContext(ctx, `UPDATE dining_tables SET table_no=?, area=?, enabled=?, updated_at=? WHERE id=? AND store_id=?`, no, nullS(area), enabled, now, id, storeID)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{"id": id, "table_no": no, "area": area, "enabled": enabled}, nil
}

func (a *App) RotateTableToken(ctx context.Context, storeID, id, appid, secret string) (string, []byte, error) {
	raw, err := cryptoutil.RandomHex(16)
	if err != nil {
		return "", nil, err
	}
	scene := raw[:32]
	if len(scene) > 32 {
		scene = scene[:32]
	}
	png, err := a.Wechat.MiniProgramCode(ctx, appid, secret, scene)
	if err != nil {
		return "", nil, err
	}
	key, _, err := a.Media.Put("table-codes", ".png", png)
	if err != nil {
		return "", nil, err
	}
	_, err = a.DB.ExecContext(ctx, `UPDATE dining_tables SET token_hash=?, mp_code_object_key=?, updated_at=? WHERE id=? AND store_id=?`,
		cryptoutil.HashToken(raw), key, a.Now(), id, storeID)
	if err != nil {
		return "", nil, err
	}
	return raw, png, nil
}

func (a *App) TableCode(ctx context.Context, storeID, id string) ([]byte, string, error) {
	var key sql.NullString
	err := a.DB.QueryRowContext(ctx, `SELECT mp_code_object_key FROM dining_tables WHERE id=? AND store_id=?`, id, storeID).Scan(&key)
	if err == sql.ErrNoRows || !key.Valid {
		return nil, "", apperr.NotFound
	}
	if err != nil {
		return nil, "", err
	}
	return a.Media.Get(key.String)
}
