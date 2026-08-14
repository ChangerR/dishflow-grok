package httpx

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/changerr/dishflow-grok/internal/app"
	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/media"
	"github.com/changerr/dishflow-grok/internal/wechat"
)

func (s *Server) submitShopApp(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	var body struct {
		StoreName string `json:"store_name"`
		Contact   string `json:"contact"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.SubmitShopApplication(r.Context(), p.UserID, body.StoreName, body.Contact)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) myShopApps(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	d, err := s.App.MyShopApplications(r.Context(), p.UserID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) submitJoin(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	var body struct {
		StoreID string `json:"store_id"`
		Role    string `json:"requested_role"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.SubmitJoinRequest(r.Context(), p.UserID, body.StoreID, body.Role)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) myJoins(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	d, err := s.App.MyJoinRequests(r.Context(), p.UserID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) platformStores(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	d, err := s.App.ListPlatformStores(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) createPlatformStore(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	var body struct {
		Name     string `json:"name"`
		Timezone string `json:"timezone"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.CreatePlatformStore(r.Context(), body.Name, body.Timezone)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	s.App.Audit(r.Context(), d.ID, p.UserID, "PLATFORM", "store.create", "store", d.Name, "")
	writeJSON(w, 200, d)
}
func (s *Server) patchPlatformStore(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	var body struct {
		Name    string `json:"name"`
		Enabled *bool  `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.PatchPlatformStore(r.Context(), chi.URLParam(r, "id"), body.Name, body.Enabled)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) platformUsers(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	d, err := s.App.ListAdminUsers(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) createPlatformUser(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	var body struct {
		LoginName       string `json:"login_name"`
		DisplayName     string `json:"display_name"`
		Password        string `json:"password"`
		IsPlatformAdmin bool   `json:"is_platform_admin"`
		Enabled         bool   `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !body.Enabled {
		body.Enabled = true
	}
	d, err := s.App.CreateAdminUser(r.Context(), body.LoginName, body.DisplayName, body.Password, body.IsPlatformAdmin, body.Enabled)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) patchPlatformUser(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	var body struct {
		DisplayName string `json:"display_name"`
		Enabled     *bool  `json:"enabled"`
		Platform    *bool  `json:"is_platform_admin"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.PatchAdminUser(r.Context(), chi.URLParam(r, "id"), body.DisplayName, body.Enabled, body.Platform)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) assignOwner(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	var body struct {
		StoreID string `json:"store_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.App.AssignStoreOwner(r.Context(), chi.URLParam(r, "id"), body.StoreID); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) listShopApps(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	d, err := s.App.ListShopApplications(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) reviewShopApp(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	var body struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.ReviewShopApplication(r.Context(), p.UserID, chi.URLParam(r, "id"), body.Decision, body.Note)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}

func (s *Server) board(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.Board(r.Context(), storeID, r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) adminOrders(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	q := r.URL.Query()
	d, err := s.App.AdminListOrders(r.Context(), storeID, q.Get("cursor"), strings.Split(q.Get("status"), ","), q.Get("scene"), q.Get("payment_status"), q.Get("q"), q.Get("start"), q.Get("end"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) adminOrder(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.GetOrder(r.Context(), storeID, chi.URLParam(r, "id"), "", false)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) exportOrders(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	csv, err := s.App.ExportOrdersCSV(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write([]byte(csv))
}
func (s *Server) transition(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		To      string `json:"to_status"`
		Version int    `json:"expected_version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.TransitionOrder(r.Context(), storeID, p.UserID, chi.URLParam(r, "id"), body.To, body.Version)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) storeRefund(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.App.StoreRefund(r.Context(), storeID, p.UserID, chi.URLParam(r, "id"), body.Reason); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) orderPrint(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.CloudPrintOrder(r.Context(), storeID, chi.URLParam(r, "id"), true); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) refunds(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListRefunds(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) reviewRefund(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.App.ReviewCancelRefund(r.Context(), storeID, p.UserID, chi.URLParam(r, "id"), body.Decision, body.Note); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) exceptions(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListExceptions(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) retryEx(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.RetryException(r.Context(), storeID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func rangeDates(r *http.Request) (string, string) {
	end := r.URL.Query().Get("end")
	start := r.URL.Query().Get("start")
	if end == "" {
		end = time.Now().UTC().Format("2006-01-02")
	}
	if start == "" {
		start = time.Now().UTC().AddDate(0, 0, -7).Format("2006-01-02")
	}
	return start, end
}
func (s *Server) anOverview(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	st, en := rangeDates(r)
	d, err := s.App.AnalyticsOverview(r.Context(), storeID, st, en)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) anTrends(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	st, en := rangeDates(r)
	d, err := s.App.AnalyticsTrends(r.Context(), storeID, st, en, r.URL.Query().Get("grain"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) anBreak(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	st, en := rangeDates(r)
	d, err := s.App.AnalyticsBreakdown(r.Context(), storeID, st, en)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) anCust(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.AnalyticsCustomers(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) membersList(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListCustomerMembers(r.Context(), storeID, r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) memberGet(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.GetCustomerMember(r.Context(), storeID, chi.URLParam(r, "customerId"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) pointsAdj(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Delta  int    `json:"delta"`
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.App.AdjustPoints(r.Context(), storeID, chi.URLParam(r, "customerId"), p.UserID, body.Reason, body.Delta); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) memberSettings(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.MemberSettingsGet(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) putMemberSettings(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		PointsPerYuan int    `json:"points_per_yuan"`
		Newbie        string `json:"newbie_coupon_template_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.MemberSettingsPut(r.Context(), storeID, body.PointsPerYuan, body.Newbie)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}

func (s *Server) listCats(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListCategories(r.Context(), storeID, r.URL.Query().Get("include_deleted") == "1")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) createCat(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
		Sort    int    `json:"sort_order"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.CreateCategory(r.Context(), storeID, body.Name, body.Enabled, body.Sort)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) patchCat(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Name    string `json:"name"`
		Enabled *bool  `json:"enabled"`
		Sort    *int   `json:"sort_order"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.PatchCategory(r.Context(), storeID, chi.URLParam(r, "id"), body.Name, body.Enabled, body.Sort)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) delCat(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.DeleteCategory(r.Context(), storeID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) restoreCat(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.RestoreCategory(r.Context(), storeID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) listDishes(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListDishes(r.Context(), storeID, r.URL.Query().Get("include_deleted") == "1")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) getDish(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.GetDish(r.Context(), storeID, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) createDish(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body app.DishDTO
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, r, apperr.Validation("请求体无效"))
		return
	}
	d, err := s.App.SaveDish(r.Context(), storeID, "", body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) patchDish(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body app.DishDTO
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, r, apperr.Validation("请求体无效"))
		return
	}
	d, err := s.App.SaveDish(r.Context(), storeID, chi.URLParam(r, "id"), body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) delDish(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.DeleteDish(r.Context(), storeID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) restoreDish(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.RestoreDish(r.Context(), storeID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) stockAdj(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		SKUID  string `json:"sku_id"`
		Delta  int    `json:"delta"`
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.App.AdjustStock(r.Context(), storeID, chi.URLParam(r, "id"), body.SKUID, body.Reason, p.UserID, body.Delta); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	_ = storeID
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, r, apperr.Validation("缺少文件"))
		return
	}
	defer f.Close()
	b, err := media.ReadAll(f, 2*1024*1024)
	if err != nil {
		writeErr(w, r, apperr.Validation("文件过大"))
		return
	}
	ext := ".jpg"
	name := strings.ToLower(hdr.Filename)
	if strings.HasSuffix(name, ".png") {
		ext = ".png"
	} else if strings.HasSuffix(name, ".webp") {
		ext = ".webp"
	}
	key, url, err := s.App.Media.Put("menu", ext, b)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"key": key, "url": url})
}

func (s *Server) listPromo(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListPromotions(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) createPromo(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body app.PromotionDTO
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.SavePromotion(r.Context(), storeID, "", body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) patchPromo(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body app.PromotionDTO
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.SavePromotion(r.Context(), storeID, chi.URLParam(r, "id"), body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) listTpl(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListCouponTemplates(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) createTpl(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body app.CouponTemplateDTO
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.SaveCouponTemplate(r.Context(), storeID, "", body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) patchTpl(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body app.CouponTemplateDTO
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.SaveCouponTemplate(r.Context(), storeID, chi.URLParam(r, "id"), body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) delTpl(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.DeleteCouponTemplate(r.Context(), storeID, chi.URLParam(r, "id"), p.Role); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) issueTpl(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Audience string `json:"audience"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.IssueCoupons(r.Context(), storeID, chi.URLParam(r, "id"), body.Audience)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}

func (s *Server) listTables(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListTables(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) createTable(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		TableNo string `json:"table_no"`
		Area    string `json:"area"`
		Enabled bool   `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.SaveTable(r.Context(), storeID, "", body.TableNo, body.Area, body.Enabled)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) patchTable(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		TableNo string `json:"table_no"`
		Area    string `json:"area"`
		Enabled bool   `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.SaveTable(r.Context(), storeID, chi.URLParam(r, "id"), body.TableNo, body.Area, body.Enabled)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) rotateTable(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	tok, _, err := s.App.RotateTableToken(r.Context(), storeID, chi.URLParam(r, "id"), "", "")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"token": tok})
}
func (s *Server) tableCode(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	b, ct, err := s.App.TableCode(r.Context(), storeID, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(200)
	_, _ = w.Write(b)
}

func (s *Server) getStore(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.AdminStore(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) patchStore(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	d, err := s.App.PatchStore(r.Context(), storeID, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) exportStore(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	b, err := s.App.ExportStore(r.Context(), storeID, strings.Split(r.URL.Query().Get("sections"), ","))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, _ = w.Write(b)
}
func (s *Server) importStore(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	b, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024))
	if err != nil {
		writeErr(w, r, apperr.Validation("读取失败"))
		return
	}
	overwrite := r.URL.Query().Get("overwrite_appid") == "1"
	if err := s.App.ImportStore(r.Context(), storeID, b, overwrite); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) getMP(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.MiniprogramConfig(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) patchMP(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	d, err := s.App.PatchMiniprogramConfig(r.Context(), storeID, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) getPay(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.PaymentConfigGet(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) putPay(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	d, err := s.App.PaymentConfigPut(r.Context(), storeID, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	s.App.Audit(r.Context(), storeID, p.UserID, "ADMIN", "payment.write", "payment_config", "updated secrets", "")
	writeJSON(w, 200, d)
}
func (s *Server) getMock(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.MockPaymentGet(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) putMock(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Enabled bool   `json:"enabled"`
		Confirm string `json:"confirm"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.App.MockPaymentPut(r.Context(), storeID, body.Confirm, body.Enabled); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) auditLogs(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.AuditLogs(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) shopMembers(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListMembers(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) changeRole(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Role string `json:"role"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.App.ChangeMemberRole(r.Context(), storeID, p.Role, chi.URLParam(r, "adminUserId"), body.Role); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) removeMember(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.RemoveMember(r.Context(), storeID, p.UserID, p.Role, chi.URLParam(r, "adminUserId")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) joinReqs(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListJoinRequests(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) reviewJoin(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.ReviewJoinRequest(r.Context(), p, chi.URLParam(r, "id"), body.Decision, body.Note)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}

func (s *Server) listMaterials(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListMaterials(r.Context(), storeID, r.URL.Query().Get("q"), r.URL.Query().Get("category"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) createMaterial(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	d, err := s.App.SaveMaterial(r.Context(), storeID, "", in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) patchMaterial(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	d, err := s.App.SaveMaterial(r.Context(), storeID, chi.URLParam(r, "id"), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) delMaterial(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.DeleteMaterial(r.Context(), storeID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) listPurchases(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListPurchaseLists(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) createPurchase(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Title string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.CreatePurchaseList(r.Context(), storeID, p.UserID, body.Title)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) getPurchase(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.GetPurchaseList(r.Context(), storeID, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) patchPurchase(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Title   string `json:"title"`
		Total   *int64 `json:"total_amount_cents"`
		Version int    `json:"expected_version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.PatchPurchaseList(r.Context(), storeID, chi.URLParam(r, "id"), body.Title, body.Total, body.Version)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) addPItem(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		MaterialID string  `json:"material_id"`
		Qty        float64 `json:"qty"`
		Note       string  `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.App.AddPurchaseItem(r.Context(), storeID, chi.URLParam(r, "id"), body.MaterialID, body.Qty, body.Note); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) patchPItem(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var body struct {
		Qty  *float64 `json:"qty"`
		Note *string  `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.App.PatchPurchaseItem(r.Context(), storeID, chi.URLParam(r, "id"), chi.URLParam(r, "itemID"), body.Qty, body.Note); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) delPItem(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.DeletePurchaseItem(r.Context(), storeID, chi.URLParam(r, "id"), chi.URLParam(r, "itemID")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) submitP(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	s.purchaseAction(w, r, p, storeID, "submit")
}
func (s *Server) printP(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	s.purchaseAction(w, r, p, storeID, "mark-printed")
}
func (s *Server) completeP(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	s.purchaseAction(w, r, p, storeID, "complete")
}
func (s *Server) voidP(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	s.purchaseAction(w, r, p, storeID, "void")
}
func (s *Server) purchaseAction(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID, action string) {
	var body struct {
		Version int    `json:"expected_version"`
		Reason  string `json:"reason"`
		Total   *int64 `json:"total_amount_cents"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.App.TransitionPurchase(r.Context(), storeID, p.UserID, chi.URLParam(r, "id"), action, body.Version, body.Reason, body.Total); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) cloudP(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.CloudPrintPurchase(r.Context(), storeID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) getPrintCfg(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.PrintConfigGet(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) putPrintCfg(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	d, err := s.App.PrintConfigPut(r.Context(), storeID, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) listPrinters(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListPrinters(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) createPrinter(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	d, err := s.App.SavePrinter(r.Context(), storeID, "", in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) patchPrinter(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	d, err := s.App.SavePrinter(r.Context(), storeID, chi.URLParam(r, "id"), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) delPrinter(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.DeletePrinter(r.Context(), storeID, chi.URLParam(r, "id"), p.Role); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) testPrinter(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.TestPrinter(r.Context(), storeID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) refreshPrinter(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	if err := s.App.RefreshPrinter(r.Context(), storeID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) printJobs(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal, storeID string) {
	d, err := s.App.ListPrintJobs(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}

func (s *Server) payNotify(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	ts := r.Header.Get("Wechatpay-Timestamp")
	if ts != "" && !wechat.VerifyTimestamp(ts, time.Now().UTC(), 5*time.Minute) {
		writeJSON(w, 400, map[string]any{"code": "FAIL", "message": "timestamp"})
		return
	}
	var env struct {
		ID       string `json:"id"`
		Resource struct {
			Ciphertext     string `json:"ciphertext"`
			Nonce          string `json:"nonce"`
			AssociatedData string `json:"associated_data"`
		} `json:"resource"`
		Summary struct {
			OutTradeNo    string `json:"out_trade_no"`
			TransactionID string `json:"transaction_id"`
			Amount        struct {
				Total    int64  `json:"total"`
				Currency string `json:"currency"`
			} `json:"amount"`
			Appid      string `json:"appid"`
			Mchid      string `json:"mchid"`
			TradeState string `json:"trade_state"`
		} `json:"event"`
	}
	_ = json.Unmarshal(body, &env)
	storeID := chi.URLParam(r, "storeID")
	orderID := env.Summary.OutTradeNo
	if orderID == "" {
		var alt map[string]any
		_ = json.Unmarshal(body, &alt)
		if v, ok := alt["out_trade_no"].(string); ok {
			orderID = v
		}
		if v, ok := alt["transaction_id"].(string); ok {
			env.Summary.TransactionID = v
		}
		if amt, ok := alt["amount"].(map[string]any); ok {
			if t, ok := amt["total"].(float64); ok {
				env.Summary.Amount.Total = int64(t)
			}
		}
		if v, ok := alt["id"].(string); ok {
			env.ID = v
		}
	}
	if env.ID == "" {
		env.ID = env.Summary.TransactionID
	}
	if err := s.App.HandlePayNotify(r.Context(), storeID, env.ID, env.Summary.Appid, env.Summary.Mchid, orderID, env.Summary.TransactionID, env.Summary.Amount.Total); err != nil {
		writeJSON(w, 400, map[string]any{"code": "FAIL", "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"code": "SUCCESS"})
}

func (s *Server) refundNotify(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	var env struct {
		ID           string `json:"id"`
		OutTradeNo   string `json:"out_trade_no"`
		OutRefundNo  string `json:"out_refund_no"`
		RefundID     string `json:"refund_id"`
		Amount       struct{ Refund int64 `json:"refund"` } `json:"amount"`
	}
	_ = json.Unmarshal(body, &env)
	if err := s.App.HandleRefundNotify(r.Context(), chi.URLParam(r, "storeID"), env.ID, env.OutTradeNo, env.OutRefundNo, env.RefundID, env.Amount.Refund); err != nil {
		writeJSON(w, 400, map[string]any{"code": "FAIL", "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"code": "SUCCESS"})
}
