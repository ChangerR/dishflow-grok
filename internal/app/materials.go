package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/domain"
	"github.com/changerr/dishflow-grok/internal/ids"
)

func (a *App) ListMaterials(ctx context.Context, storeID, q, cat string) ([]map[string]any, error) {
	query := `SELECT id, name, image_url, category, unit, default_qty, note, enabled, sort_order FROM materials WHERE store_id=?`
	args := []any{storeID}
	if q != "" {
		query += ` AND name LIKE ?`
		args = append(args, "%"+q+"%")
	}
	if cat != "" {
		query += ` AND category=?`
		args = append(args, cat)
	}
	query += ` ORDER BY enabled DESC, sort_order ASC, name ASC`
	rows, err := a.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, name, unit string
		var img, category, note sql.NullString
		var qty float64
		var enabled bool
		var sort int
		if err := rows.Scan(&id, &name, &img, &category, &unit, &qty, &note, &enabled, &sort); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "name": name, "image_url": scanNullString(img), "category": scanNullString(category), "unit": unit, "default_qty": qty, "note": scanNullString(note), "enabled": enabled, "sort_order": sort})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) SaveMaterial(ctx context.Context, storeID, id string, in map[string]any) (map[string]any, error) {
	name, _ := in["name"].(string)
	unit, _ := in["unit"].(string)
	if strings.TrimSpace(name) == "" || unit == "" {
		return nil, apperr.Validation("名称和单位不能为空")
	}
	now := a.Now()
	enabled := true
	if v, ok := in["enabled"].(bool); ok {
		enabled = v
	}
	qty := 1.0
	if v, ok := in["default_qty"].(float64); ok {
		qty = v
	}
	sort, _ := in["sort_order"].(float64)
	cat, _ := in["category"].(string)
	note, _ := in["note"].(string)
	img, _ := in["image_url"].(string)
	if id == "" {
		id = a.NewID()
		_, err := a.DB.ExecContext(ctx, `INSERT INTO materials (id, store_id, name, image_url, category, unit, default_qty, note, enabled, sort_order, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, storeID, name, nullS(img), nullS(cat), unit, qty, nullS(note), enabled, int(sort), now, now)
		if err != nil {
			if isDup(err) {
				return nil, apperr.Conflict
			}
			return nil, err
		}
	} else {
		_, err := a.DB.ExecContext(ctx, `UPDATE materials SET name=?, image_url=?, category=?, unit=?, default_qty=?, note=?, enabled=?, sort_order=?, updated_at=? WHERE id=? AND store_id=?`,
			name, nullS(img), nullS(cat), unit, qty, nullS(note), enabled, int(sort), now, id, storeID)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{"id": id, "name": name, "unit": unit, "enabled": enabled}, nil
}

func (a *App) DeleteMaterial(ctx context.Context, storeID, id string) error {
	_, err := a.DB.ExecContext(ctx, `UPDATE materials SET enabled=0, updated_at=? WHERE id=? AND store_id=?`, a.Now(), id, storeID)
	return err
}

func (a *App) ListPurchaseLists(ctx context.Context, storeID string) ([]map[string]any, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, list_no, business_date, title, status, total_amount_cents, version, print_count, created_at FROM purchase_lists WHERE store_id=? ORDER BY created_at DESC LIMIT 100`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, no, title, status string
		var biz time.Time
		var total sql.NullInt64
		var ver, prints int
		var created time.Time
		if err := rows.Scan(&id, &no, &biz, &title, &status, &total, &ver, &prints, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "list_no": no, "business_date": biz.Format("2006-01-02"), "title": title, "status": status, "total_amount_cents": total.Int64, "version": ver, "print_count": prints, "created_at": created.UTC().Format(time.RFC3339)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func (a *App) CreatePurchaseList(ctx context.Context, storeID, actorID, title string) (map[string]any, error) {
	biz, err := a.businessDate(ctx, storeID, nil)
	if err != nil {
		return nil, err
	}
	var n int
	_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM purchase_lists WHERE store_id=? AND business_date=?`, storeID, biz).Scan(&n)
	no := fmt.Sprintf("P%s-%03d", strings.ReplaceAll(biz, "-", ""), n+1)
	id := a.NewID()
	now := a.Now()
	if title == "" {
		title = "采购清单 " + biz
	}
	_, err = a.DB.ExecContext(ctx, `INSERT INTO purchase_lists (id, store_id, list_no, business_date, title, status, version, print_count, created_by, created_at, updated_at) VALUES (?,?,?,?,?,'DRAFT',1,0,?,?,?)`,
		id, storeID, no, biz, title, actorID, now, now)
	if err != nil {
		return nil, err
	}
	_, _ = a.DB.ExecContext(ctx, `INSERT INTO purchase_list_events (id, list_id, store_id, to_status, actor_id, created_at) VALUES (?,?,?,?,?,?)`, ids.New(), id, storeID, domain.PurchaseDraft, actorID, now)
	return a.GetPurchaseList(ctx, storeID, id)
}

func (a *App) GetPurchaseList(ctx context.Context, storeID, id string) (map[string]any, error) {
	var no, title, status, createdBy string
	var biz, created time.Time
	var total sql.NullInt64
	var ver, prints int
	err := a.DB.QueryRowContext(ctx, `SELECT list_no, business_date, title, status, total_amount_cents, version, print_count, created_by, created_at FROM purchase_lists WHERE id=? AND store_id=?`, id, storeID).
		Scan(&no, &biz, &title, &status, &total, &ver, &prints, &createdBy, &created)
	if err == sql.ErrNoRows {
		return nil, apperr.NotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.QueryContext(ctx, `SELECT id, material_id, name_snapshot, unit_snapshot, qty, note FROM purchase_list_items WHERE list_id=?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var iid, name, unit string
		var mid, note sql.NullString
		var qty float64
		if err := rows.Scan(&iid, &mid, &name, &unit, &qty, &note); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": iid, "material_id": scanNullString(mid), "name": name, "unit": unit, "qty": qty, "note": scanNullString(note)})
	}
	return map[string]any{"id": id, "list_no": no, "business_date": biz.Format("2006-01-02"), "title": title, "status": status, "total_amount_cents": nilIfNull(total), "version": ver, "print_count": prints, "items": items}, nil
}

func nilIfNull(v sql.NullInt64) any {
	if !v.Valid {
		return nil
	}
	return v.Int64
}

func (a *App) PatchPurchaseList(ctx context.Context, storeID, id string, title string, total *int64, version int) (map[string]any, error) {
	now := a.Now()
	q := `UPDATE purchase_lists SET updated_at=?, version=version+1`
	args := []any{now}
	if title != "" {
		q += `, title=?`
		args = append(args, title)
	}
	if total != nil {
		q += `, total_amount_cents=?`
		args = append(args, *total)
	}
	q += ` WHERE id=? AND store_id=? AND version=? AND status IN ('DRAFT','SUBMITTED','PRINTED')`
	args = append(args, id, storeID, version)
	res, err := a.DB.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, apperr.StateConflict
	}
	return a.GetPurchaseList(ctx, storeID, id)
}

func (a *App) AddPurchaseItem(ctx context.Context, storeID, listID, materialID string, qty float64, note string) error {
	var status string
	if err := a.DB.QueryRowContext(ctx, `SELECT status FROM purchase_lists WHERE id=? AND store_id=?`, listID, storeID).Scan(&status); err != nil {
		return apperr.NotFound
	}
	if status != domain.PurchaseDraft {
		return apperr.StateConflict
	}
	if qty <= 0 {
		return apperr.Validation("数量必须大于 0")
	}
	var name, unit string
	if err := a.DB.QueryRowContext(ctx, `SELECT name, unit FROM materials WHERE id=? AND store_id=?`, materialID, storeID).Scan(&name, &unit); err != nil {
		return apperr.NotFound
	}
	_, err := a.DB.ExecContext(ctx, `INSERT INTO purchase_list_items (id, list_id, store_id, material_id, name_snapshot, unit_snapshot, qty, note) VALUES (?,?,?,?,?,?,?,?)`,
		a.NewID(), listID, storeID, materialID, name, unit, qty, nullS(note))
	if isDup(err) {
		return apperr.Conflict
	}
	return err
}

func (a *App) PatchPurchaseItem(ctx context.Context, storeID, listID, itemID string, qty *float64, note *string) error {
	var status string
	_ = a.DB.QueryRowContext(ctx, `SELECT status FROM purchase_lists WHERE id=? AND store_id=?`, listID, storeID).Scan(&status)
	if status != domain.PurchaseDraft {
		return apperr.StateConflict
	}
	if qty != nil {
		if *qty <= 0 {
			return apperr.Validation("数量必须大于 0")
		}
		_, err := a.DB.ExecContext(ctx, `UPDATE purchase_list_items SET qty=? WHERE id=? AND list_id=?`, *qty, itemID, listID)
		return err
	}
	if note != nil {
		_, err := a.DB.ExecContext(ctx, `UPDATE purchase_list_items SET note=? WHERE id=? AND list_id=?`, *note, itemID, listID)
		return err
	}
	return nil
}

func (a *App) DeletePurchaseItem(ctx context.Context, storeID, listID, itemID string) error {
	var status string
	_ = a.DB.QueryRowContext(ctx, `SELECT status FROM purchase_lists WHERE id=? AND store_id=?`, listID, storeID).Scan(&status)
	if status != domain.PurchaseDraft {
		return apperr.StateConflict
	}
	_, err := a.DB.ExecContext(ctx, `DELETE FROM purchase_list_items WHERE id=? AND list_id=?`, itemID, listID)
	return err
}

func (a *App) TransitionPurchase(ctx context.Context, storeID, actorID, id, action string, version int, reason string, total *int64) error {
	var status string
	if err := a.DB.QueryRowContext(ctx, `SELECT status FROM purchase_lists WHERE id=? AND store_id=?`, id, storeID).Scan(&status); err != nil {
		return apperr.NotFound
	}
	to := ""
	switch action {
	case "submit":
		if status != domain.PurchaseDraft {
			return apperr.StateConflict
		}
		var n int
		_ = a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM purchase_list_items WHERE list_id=?`, id).Scan(&n)
		if n == 0 {
			return apperr.Validation("至少需要一个条目")
		}
		to = domain.PurchaseSubmitted
	case "mark-printed":
		if status != domain.PurchaseSubmitted && status != domain.PurchasePrinted {
			return apperr.StateConflict
		}
		to = domain.PurchasePrinted
	case "complete":
		if status != domain.PurchaseSubmitted && status != domain.PurchasePrinted {
			return apperr.StateConflict
		}
		if total == nil {
			var t sql.NullInt64
			_ = a.DB.QueryRowContext(ctx, `SELECT total_amount_cents FROM purchase_lists WHERE id=?`, id).Scan(&t)
			if !t.Valid {
				return apperr.Validation("完成前需回填金额")
			}
		}
		to = domain.PurchaseCompleted
	case "void":
		if status == domain.PurchaseCompleted || status == domain.PurchaseVoid {
			return apperr.StateConflict
		}
		to = domain.PurchaseVoid
	default:
		return apperr.Validation("动作无效")
	}
	now := a.Now()
	q := `UPDATE purchase_lists SET status=?, version=version+1, updated_at=?`
	args := []any{to, now}
	if action == "mark-printed" {
		q += `, print_count=print_count+1`
	}
	if total != nil {
		q += `, total_amount_cents=?`
		args = append(args, *total)
	}
	q += ` WHERE id=? AND version=?`
	args = append(args, id, version)
	res, err := a.DB.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperr.StateConflict
	}
	_, err = a.DB.ExecContext(ctx, `INSERT INTO purchase_list_events (id, list_id, store_id, from_status, to_status, actor_id, note, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		ids.New(), id, storeID, status, to, actorID, nullS(reason), now)
	return err
}
