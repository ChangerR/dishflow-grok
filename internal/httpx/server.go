package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/changerr/dishflow-grok/internal/app"
	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/domain"
	"github.com/changerr/dishflow-grok/internal/ids"
	"github.com/changerr/dishflow-grok/internal/persist"
)

type Server struct {
	App *app.App
	R   chi.Router
}

func NewServer(a *app.App) http.Handler {
	s := &Server{App: a, R: chi.NewRouter()}
	s.R.Use(middleware.RequestID)
	s.R.Use(s.withRequestID)
	s.R.Use(securityHeaders)
	s.R.Get("/health/live", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })
	s.R.Get("/health/ready", s.ready)
	s.R.Get("/media/menu/{path:*}", s.media)
	s.R.Post("/callbacks/wechat-pay/transactions/{storeID}", s.payNotify)
	s.R.Post("/callbacks/wechat-pay/refunds/{storeID}", s.refundNotify)
	s.R.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/wechat/session", s.wechatSession)
		r.Get("/storefront/bootstrap", s.withStorefront(s.bootstrap))
		r.Get("/store", s.withStorefront(s.publicStore))
		r.Get("/store/policies", s.withStorefront(s.policies))
		r.Get("/menu", s.withStorefront(s.menu))
		r.Get("/tables/resolve", s.withStorefront(s.resolveTable))
		r.Get("/pickup-slots", s.withStorefront(s.pickupSlots))
		r.Get("/coupon-offers", s.withStorefront(s.couponOffers))
		r.Post("/pricing/preview", s.withStorefront(s.preview))
		r.Post("/orders", s.withCustomer(s.createOrder))
		r.Get("/orders", s.withCustomer(s.myOrders))
		r.Get("/orders/{id}", s.withCustomer(s.myOrder))
		r.Post("/orders/{id}/prepay", s.withCustomer(s.prepay))
		r.Post("/orders/{id}/mock-payment/confirm", s.withCustomer(s.mockPay))
		r.Post("/orders/{id}/cancel", s.withCustomer(s.cancelOrder))
		r.Get("/coupons", s.withCustomer(s.myCoupons))
		r.Post("/coupon-offers/{templateID}/claim", s.withCustomer(s.claimCoupon))
		r.Get("/me", s.withCustomer(s.me))
		r.Post("/me/membership", s.withCustomer(s.joinMember))
		r.Get("/me/points", s.withCustomer(s.points))
		r.Get("/me/rewards", s.withCustomer(s.rewards))
		r.Post("/me/rewards/{templateID}/redeem", s.withCustomer(s.redeem))

		r.Post("/admin/session", s.adminLogin)
		r.Get("/admin/session", s.withAdmin(s.adminSession))
		r.Delete("/admin/session", s.adminLogout)
		r.Post("/admin/shop-applications", s.withAdmin(s.submitShopApp))
		r.Get("/admin/my/shop-applications", s.withAdmin(s.myShopApps))
		r.Post("/admin/shop-join-requests", s.withAdmin(s.submitJoin))
		r.Get("/admin/my/shop-join-requests", s.withAdmin(s.myJoins))
		r.Get("/admin/platform/stores", s.withPlatform(s.platformStores))
		r.Post("/admin/platform/stores", s.withPlatform(s.createPlatformStore))
		r.Patch("/admin/platform/stores/{id}", s.withPlatform(s.patchPlatformStore))
		r.Get("/admin/platform/users", s.withPlatform(s.platformUsers))
		r.Post("/admin/platform/users", s.withPlatform(s.createPlatformUser))
		r.Patch("/admin/platform/users/{id}", s.withPlatform(s.patchPlatformUser))
		r.Post("/admin/platform/users/{id}/assign-store-owner", s.withPlatform(s.assignOwner))
		r.Get("/admin/platform/shop-applications", s.withPlatform(s.listShopApps))
		r.Post("/admin/platform/shop-applications/{id}/review", s.withPlatform(s.reviewShopApp))

		r.Get("/admin/orders/board", s.withStore(domain.RoleStaff, s.board))
		r.Get("/admin/orders/export", s.withStore(domain.RoleManager, s.exportOrders))
		r.Get("/admin/orders", s.withStore(domain.RoleManager, s.adminOrders))
		r.Get("/admin/orders/{id}", s.withStore(domain.RoleStaff, s.adminOrder))
		r.Post("/admin/orders/{id}/transitions", s.withStore(domain.RoleStaff, s.transition))
		r.Post("/admin/orders/{id}/refunds", s.withStore(domain.RoleManager, s.storeRefund))
		r.Post("/admin/orders/{id}/cloud-print", s.withStore(domain.RoleStaff, s.orderPrint))
		r.Get("/admin/refunds", s.withStore(domain.RoleManager, s.refunds))
		r.Post("/admin/refunds/{id}/review", s.withStore(domain.RoleManager, s.reviewRefund))
		r.Get("/admin/exceptions", s.withStore(domain.RoleManager, s.exceptions))
		r.Post("/admin/exceptions/{id}/retry", s.withStore(domain.RoleManager, s.retryEx))
		r.Get("/admin/analytics/overview", s.withStore(domain.RoleManager, s.anOverview))
		r.Get("/admin/analytics/trends", s.withStore(domain.RoleManager, s.anTrends))
		r.Get("/admin/analytics/breakdown", s.withStore(domain.RoleManager, s.anBreak))
		r.Get("/admin/analytics/customers", s.withStore(domain.RoleManager, s.anCust))
		r.Get("/admin/customer-members", s.withStore(domain.RoleManager, s.membersList))
		r.Get("/admin/customer-members/{customerId}", s.withStore(domain.RoleManager, s.memberGet))
		r.Post("/admin/customer-members/{customerId}/points-adjustments", s.withStore(domain.RoleManager, s.pointsAdj))
		r.Get("/admin/member-settings", s.withStore(domain.RoleManager, s.memberSettings))
		r.Put("/admin/member-settings", s.withStore(domain.RoleManager, s.putMemberSettings))

		r.Get("/admin/categories", s.withStore(domain.RoleManager, s.listCats))
		r.Post("/admin/categories", s.withStore(domain.RoleManager, s.createCat))
		r.Patch("/admin/categories/{id}", s.withStore(domain.RoleManager, s.patchCat))
		r.Delete("/admin/categories/{id}", s.withStore(domain.RoleManager, s.delCat))
		r.Post("/admin/categories/{id}/restore", s.withStore(domain.RoleManager, s.restoreCat))
		r.Get("/admin/dishes", s.withStore(domain.RoleManager, s.listDishes))
		r.Post("/admin/dishes", s.withStore(domain.RoleManager, s.createDish))
		r.Get("/admin/dishes/{id}", s.withStore(domain.RoleManager, s.getDish))
		r.Patch("/admin/dishes/{id}", s.withStore(domain.RoleManager, s.patchDish))
		r.Delete("/admin/dishes/{id}", s.withStore(domain.RoleManager, s.delDish))
		r.Post("/admin/dishes/{id}/restore", s.withStore(domain.RoleManager, s.restoreDish))
		r.Post("/admin/dishes/{id}/stock-adjustments", s.withStore(domain.RoleManager, s.stockAdj))
		r.Post("/admin/uploads", s.withStore(domain.RoleStaff, s.upload))
		r.Get("/admin/promotions", s.withStore(domain.RoleManager, s.listPromo))
		r.Post("/admin/promotions", s.withStore(domain.RoleManager, s.createPromo))
		r.Patch("/admin/promotions/{id}", s.withStore(domain.RoleManager, s.patchPromo))
		r.Get("/admin/coupon-templates", s.withStore(domain.RoleManager, s.listTpl))
		r.Post("/admin/coupon-templates", s.withStore(domain.RoleManager, s.createTpl))
		r.Patch("/admin/coupon-templates/{id}", s.withStore(domain.RoleManager, s.patchTpl))
		r.Delete("/admin/coupon-templates/{id}", s.withStore(domain.RoleOwner, s.delTpl))
		r.Post("/admin/coupon-templates/{id}/issue", s.withStore(domain.RoleManager, s.issueTpl))
		r.Get("/admin/tables", s.withStore(domain.RoleManager, s.listTables))
		r.Post("/admin/tables", s.withStore(domain.RoleManager, s.createTable))
		r.Patch("/admin/tables/{id}", s.withStore(domain.RoleManager, s.patchTable))
		r.Post("/admin/tables/{id}/rotate-token", s.withStore(domain.RoleManager, s.rotateTable))
		r.Get("/admin/tables/{id}/miniprogram-code", s.withStore(domain.RoleManager, s.tableCode))
		r.Get("/admin/store", s.withStore(domain.RoleManager, s.getStore))
		r.Patch("/admin/store", s.withStore(domain.RoleManager, s.patchStore))
		r.Get("/admin/store/export", s.withStore(domain.RoleManager, s.exportStore))
		r.Post("/admin/store/import", s.withStore(domain.RoleOwner, s.importStore))
		r.Get("/admin/miniprogram-config", s.withStore(domain.RoleManager, s.getMP))
		r.Patch("/admin/miniprogram-config", s.withStore(domain.RoleManager, s.patchMP))
		r.Get("/admin/payment-config", s.withStore(domain.RoleOwner, s.getPay))
		r.Put("/admin/payment-config", s.withStore(domain.RoleOwner, s.putPay))
		r.Get("/admin/mock-payment", s.withStore(domain.RoleManager, s.getMock))
		r.Put("/admin/mock-payment", s.withStore(domain.RoleOwner, s.putMock))
		r.Get("/admin/audit-logs", s.withStore(domain.RoleManager, s.auditLogs))
		r.Get("/admin/members", s.withStore(domain.RoleManager, s.shopMembers))
		r.Post("/admin/members/{adminUserId}", s.withStore(domain.RoleOwner, s.changeRole))
		r.Delete("/admin/members/{adminUserId}", s.withStore(domain.RoleOwner, s.removeMember))
		r.Get("/admin/join-requests", s.withStore(domain.RoleOwner, s.joinReqs))
		r.Post("/admin/join-requests/{id}/review", s.withStore(domain.RoleOwner, s.reviewJoin))

		r.Get("/admin/materials", s.withStore(domain.RoleStaff, s.listMaterials))
		r.Post("/admin/materials", s.withStore(domain.RoleStaff, s.createMaterial))
		r.Patch("/admin/materials/{id}", s.withStore(domain.RoleStaff, s.patchMaterial))
		r.Delete("/admin/materials/{id}", s.withStore(domain.RoleManager, s.delMaterial))
		r.Get("/admin/purchase-lists", s.withStore(domain.RoleStaff, s.listPurchases))
		r.Post("/admin/purchase-lists", s.withStore(domain.RoleStaff, s.createPurchase))
		r.Get("/admin/purchase-lists/{id}", s.withStore(domain.RoleStaff, s.getPurchase))
		r.Patch("/admin/purchase-lists/{id}", s.withStore(domain.RoleStaff, s.patchPurchase))
		r.Post("/admin/purchase-lists/{id}/items", s.withStore(domain.RoleStaff, s.addPItem))
		r.Patch("/admin/purchase-lists/{id}/items/{itemID}", s.withStore(domain.RoleStaff, s.patchPItem))
		r.Delete("/admin/purchase-lists/{id}/items/{itemID}", s.withStore(domain.RoleStaff, s.delPItem))
		r.Post("/admin/purchase-lists/{id}/submit", s.withStore(domain.RoleStaff, s.submitP))
		r.Post("/admin/purchase-lists/{id}/mark-printed", s.withStore(domain.RoleStaff, s.printP))
		r.Post("/admin/purchase-lists/{id}/cloud-print", s.withStore(domain.RoleStaff, s.cloudP))
		r.Post("/admin/purchase-lists/{id}/complete", s.withStore(domain.RoleStaff, s.completeP))
		r.Post("/admin/purchase-lists/{id}/void", s.withStore(domain.RoleStaff, s.voidP))
		r.Get("/admin/print/config", s.withStore(domain.RoleManager, s.getPrintCfg))
		r.Put("/admin/print/config", s.withStore(domain.RoleManager, s.putPrintCfg))
		r.Get("/admin/print/printers", s.withStore(domain.RoleManager, s.listPrinters))
		r.Post("/admin/print/printers", s.withStore(domain.RoleManager, s.createPrinter))
		r.Patch("/admin/print/printers/{id}", s.withStore(domain.RoleManager, s.patchPrinter))
		r.Delete("/admin/print/printers/{id}", s.withStore(domain.RoleOwner, s.delPrinter))
		r.Post("/admin/print/printers/{id}/test", s.withStore(domain.RoleManager, s.testPrinter))
		r.Post("/admin/print/printers/{id}/refresh", s.withStore(domain.RoleManager, s.refreshPrinter))
		r.Get("/admin/print/jobs", s.withStore(domain.RoleManager, s.printJobs))
	})
	if a.Cfg.AdminDist != "" {
		s.R.Handle("/*", spa(a.Cfg.AdminDist))
	}
	return s.R
}

func spa(dir string) http.Handler {
	fs := http.Dir(dir)
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}

type ctxKey int

const (
	ctxAdmin ctxKey = iota
	ctxCustomer
	ctxStore
	ctxRID
)

func (s *Server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" || len(id) > 64 {
			id = middleware.GetReqID(r.Context())
			if id == "" {
				id = ids.New()
			}
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxRID, id)))
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		rid, _ := r.Context().Value(ctxRID).(string)
		writeJSON(w, ae.Status, map[string]any{"code": ae.Code, "message": ae.Message, "request_id": rid, "details": ae.Details})
		return
	}
	log.Printf("internal: %v", err)
	rid, _ := r.Context().Value(ctxRID).(string)
	writeJSON(w, 500, map[string]any{"code": "INTERNAL_ERROR", "message": "服务暂时不可用", "request_id": rid})
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return apperr.Validation("请求体无效")
	}
	return nil
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.App.DB.PingContext(r.Context()); err != nil {
		writeJSON(w, 503, map[string]any{"code": "NOT_READY", "message": "mysql"})
		return
	}
	if err := persist.PingRedis(r.Context(), s.App.Redis); err != nil {
		writeJSON(w, 503, map[string]any{"code": "NOT_READY", "message": "redis"})
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok"})
}

func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(chi.URLParam(r, "path"), "/")
	b, ct, err := s.App.Media.Get(key)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(200)
	_, _ = w.Write(b)
}

func (s *Server) wechatSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	tok, cid, sid, err := s.App.WechatLogin(r.Context(), r.Header.Get("X-Wechat-Appid"), body.Code)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"token": tok, "customer_id": cid, "store_id": sid})
}

func (s *Server) withStorefront(h func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sid, err := s.App.StoreByAppID(r.Context(), r.Header.Get("X-Wechat-Appid"))
		if err != nil {
			writeErr(w, r, err)
			return
		}
		h(w, r, sid)
	}
}

func (s *Server) withCustomer(h func(http.ResponseWriter, *http.Request, app.CustomerPrincipal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		p, err := s.App.CustomerFromToken(r.Context(), r.Header.Get("X-Wechat-Appid"), auth)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		h(w, r, p)
	}
}

func (s *Server) withAdmin(h func(http.ResponseWriter, *http.Request, app.AdminPrincipal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie(s.App.CookieName())
		tok := ""
		if c != nil {
			tok = c.Value
		}
		p, err := s.App.AdminFromToken(r.Context(), tok)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		h(w, r, p)
	}
}

func (s *Server) withPlatform(h func(http.ResponseWriter, *http.Request, app.AdminPrincipal)) http.HandlerFunc {
	return s.withAdmin(func(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
		if err := s.App.RequirePlatform(p); err != nil {
			writeErr(w, r, err)
			return
		}
		h(w, r, p)
	})
}

func (s *Server) withStore(need string, h func(http.ResponseWriter, *http.Request, app.AdminPrincipal, string)) http.HandlerFunc {
	return s.withAdmin(func(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
		storeID := r.Header.Get("X-Store-Id")
		if storeID == "" {
			storeID = p.StoreID
		}
		p2, err := s.App.RequireStoreRole(p, storeID, need)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		h(w, r, p2, storeID)
	})
}

func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request, storeID string) {
	d, err := s.App.Bootstrap(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) publicStore(w http.ResponseWriter, r *http.Request, storeID string) {
	d, err := s.App.PublicStore(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) policies(w http.ResponseWriter, r *http.Request, storeID string) {
	d, err := s.App.Policies(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) menu(w http.ResponseWriter, r *http.Request, storeID string) {
	d, err := s.App.CustomerMenu(r.Context(), storeID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) resolveTable(w http.ResponseWriter, r *http.Request, storeID string) {
	d, err := s.App.ResolveTable(r.Context(), storeID, r.URL.Query().Get("token"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) pickupSlots(w http.ResponseWriter, r *http.Request, storeID string) {
	d, err := s.App.PickupSlots(r.Context(), storeID, r.URL.Query().Get("date"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) couponOffers(w http.ResponseWriter, r *http.Request, storeID string) {
	cid := ""
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if auth != "" {
		if p, err := s.App.CustomerFromToken(r.Context(), r.Header.Get("X-Wechat-Appid"), auth); err == nil {
			cid = p.CustomerID
		}
	}
	d, err := s.App.PublicCouponOffers(r.Context(), storeID, cid)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) preview(w http.ResponseWriter, r *http.Request, storeID string) {
	var req app.PreviewReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, apperr.Validation("请求体无效"))
		return
	}
	cid := ""
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if auth != "" {
		if p, err := s.App.CustomerFromToken(r.Context(), r.Header.Get("X-Wechat-Appid"), auth); err == nil {
			cid = p.CustomerID
		}
	}
	d, err := s.App.Preview(r.Context(), storeID, cid, req)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}

func (s *Server) createOrder(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	var body struct {
		QuoteToken string            `json:"quote_token"`
		Items      []domain.CartItem `json:"items"`
		Remark     string            `json:"remark"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, r, apperr.Validation("请求体无效"))
		return
	}
	d, err := s.App.CreateOrder(r.Context(), p.StoreID, p.CustomerID, body.QuoteToken, body.Items)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) myOrders(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	d, err := s.App.ListCustomerOrders(r.Context(), p.StoreID, p.CustomerID, r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) myOrder(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	d, err := s.App.GetOrder(r.Context(), p.StoreID, chi.URLParam(r, "id"), p.CustomerID, true)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) prepay(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	d, err := s.App.Prepay(r.Context(), p.StoreID, p.CustomerID, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) mockPay(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	if err := s.App.ConfirmMockPayment(r.Context(), p.StoreID, p.CustomerID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	d, err := s.App.CancelCustomerOrder(r.Context(), p.StoreID, p.CustomerID, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) myCoupons(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	d, err := s.App.MyCoupons(r.Context(), p.StoreID, p.CustomerID, r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) claimCoupon(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	d, err := s.App.ClaimCoupon(r.Context(), p.StoreID, p.CustomerID, chi.URLParam(r, "templateID"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) me(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	d, err := s.App.Me(r.Context(), p.StoreID, p.CustomerID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) joinMember(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	var body struct {
		PhoneCode string `json:"phone_code"`
		Agreed    bool   `json:"agreed"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d, err := s.App.JoinMembership(r.Context(), p.StoreID, p.CustomerID, body.PhoneCode, body.Agreed)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) points(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	d, err := s.App.PointsLedger(r.Context(), p.StoreID, p.CustomerID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) rewards(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	d, err := s.App.Rewards(r.Context(), p.StoreID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": d})
}
func (s *Server) redeem(w http.ResponseWriter, r *http.Request, p app.CustomerPrincipal) {
	if err := s.App.RedeemReward(r.Context(), p.StoreID, p.CustomerID, chi.URLParam(r, "templateID")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: s.App.CookieName(), Value: token, Path: "/", HttpOnly: true,
		Secure: s.App.Cfg.CookieSecure, SameSite: http.SameSiteLaxMode, Domain: s.App.Cfg.CookieDomain,
		Expires: time.Now().Add(s.App.Cfg.AbsoluteSession),
	})
}

func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Login    string `json:"login_name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, r, apperr.Validation("请求体无效"))
		return
	}
	ip := s.App.ClientIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"))
	tok, p, err := s.App.LoginAdmin(r.Context(), body.Login, body.Password, ip)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	s.setCookie(w, tok)
	writeJSON(w, 200, p)
}
func (s *Server) adminLogout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie(s.App.CookieName())
	tok := ""
	if c != nil {
		tok = c.Value
	}
	_ = s.App.LogoutAdmin(r.Context(), tok)
	http.SetCookie(w, &http.Cookie{Name: s.App.CookieName(), Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) adminSession(w http.ResponseWriter, r *http.Request, p app.AdminPrincipal) {
	writeJSON(w, 200, p)
}
