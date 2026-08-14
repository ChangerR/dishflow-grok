package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
)

const (
	SceneDineIn = "DINE_IN"
	ScenePickup = "PICKUP"

	PickupImmediate = "IMMEDIATE"
	PickupScheduled = "SCHEDULED"

	StockUnlimited = "UNLIMITED"
	StockDaily     = "DAILY"

	RoleStaff   = "STAFF"
	RoleManager = "MANAGER"
	RoleOwner   = "OWNER"

	OrderPendingPayment  = "PENDING_PAYMENT"
	OrderPaid            = "PAID"
	OrderAccepted        = "ACCEPTED"
	OrderPreparing       = "PREPARING"
	OrderReady           = "READY"
	OrderCompleted       = "COMPLETED"
	OrderCancelled       = "CANCELLED"
	OrderCancelRequested = "CANCEL_REQUESTED"
	OrderRefunding       = "REFUNDING"
	OrderRefunded        = "REFUNDED"

	PayUnpaid        = "UNPAID"
	PayPrepayCreated = "PREPAY_CREATED"
	PaySuccess       = "SUCCESS"
	PayClosed        = "CLOSED"

	RefundNone       = ""
	RefundCreated    = "CREATED"
	RefundProcessing = "PROCESSING"
	RefundSuccess    = "SUCCESS"
	RefundAbnormal   = "ABNORMAL"
	RefundClosed     = "CLOSED"

	PurchaseDraft     = "DRAFT"
	PurchaseSubmitted = "SUBMITTED"
	PurchasePrinted   = "PRINTED"
	PurchaseCompleted = "COMPLETED"
	PurchaseVoid      = "VOID"

	PrintQueued    = "QUEUED"
	PrintSending   = "SENDING"
	PrintSubmitted = "SUBMITTED"
	PrintPrinted   = "PRINTED"
	PrintFailed    = "FAILED"
)

type CartItem struct {
	SKUID     string   `json:"sku_id"`
	OptionIDs []string `json:"option_ids"`
	Qty       int      `json:"qty"`
}

func NormalizeCart(items []CartItem) ([]CartItem, error) {
	merged := map[string]*CartItem{}
	order := make([]string, 0, len(items))
	for _, it := range items {
		if it.SKUID == "" {
			return nil, apperr.Validation("sku_id 不能为空")
		}
		if it.Qty < 0 || it.Qty > 99 {
			return nil, apperr.Validation("数量必须在 0～99")
		}
		if it.Qty == 0 {
			continue
		}
		key := CartKey(it)
		if existing, ok := merged[key]; ok {
			existing.Qty += it.Qty
			if existing.Qty > 99 {
				return nil, apperr.Validation("单行数量不能超过 99")
			}
			continue
		}
		cp := it
		cp.OptionIDs = CanonicalOptionIDs(it.OptionIDs)
		merged[key] = &cp
		order = append(order, key)
	}
	out := make([]CartItem, 0, len(order))
	for _, k := range order {
		out = append(out, *merged[k])
	}
	return out, nil
}

func CanonicalOptionIDs(ids []string) []string {
	if len(ids) == 0 {
		return []string{}
	}
	cp := append([]string{}, ids...)
	sort.Strings(cp)
	return cp
}

func CartKey(it CartItem) string {
	return it.SKUID + "|" + strings.Join(CanonicalOptionIDs(it.OptionIDs), ",")
}

func CartDigest(items []CartItem) (string, error) {
	norm, err := NormalizeCart(items)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(norm)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

type QuoteLine struct {
	SKUID          string   `json:"sku_id"`
	ProductID      string   `json:"product_id"`
	ProductName    string   `json:"product_name"`
	SKUName        string   `json:"sku_name"`
	OptionIDs      []string `json:"option_ids"`
	OptionNames    []string `json:"option_names"`
	Qty            int      `json:"qty"`
	UnitPriceCents int64    `json:"unit_price_cents"`
	PackingCents   int64    `json:"packing_fee_cents"`
	LineTotalCents int64    `json:"line_total_cents"`
}

type DiscountDetail struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Amount int64  `json:"amount_cents"`
}

type Quote struct {
	ID                string           `json:"id"`
	StoreID           string           `json:"store_id"`
	CustomerID        string           `json:"customer_id,omitempty"`
	Scene             string           `json:"scene"`
	TableID           string           `json:"table_id,omitempty"`
	TableNo           string           `json:"table_no,omitempty"`
	PickupType        string           `json:"pickup_type"`
	ScheduledFor      *time.Time       `json:"scheduled_for,omitempty"`
	PickupBusinessDay string           `json:"pickup_business_date"`
	Lines             []QuoteLine      `json:"lines"`
	GoodsCents        int64            `json:"goods_cents"`
	PackingCents      int64            `json:"packing_cents"`
	DiscountCents     int64            `json:"discount_cents"`
	PayableCents      int64            `json:"payable_cents"`
	Discounts         []DiscountDetail `json:"discounts"`
	CouponID          string           `json:"customer_coupon_id,omitempty"`
	PromotionID       string           `json:"promotion_id,omitempty"`
	CartDigest        string           `json:"cart_digest"`
	ExpiresAt         time.Time        `json:"expires_at"`
}

type Promotion struct {
	ID             string
	Name           string
	ThresholdCents int64
	DiscountCents  int64
	StartsAt       time.Time
	EndsAt         time.Time
	Enabled        bool
}

type Coupon struct {
	ID             string
	TemplateID     string
	Name           string
	MinSpendCents  int64
	DiscountCents  int64
	StartsAt       time.Time
	EndsAt         time.Time
	Enabled        bool
	Status         string
	CustomerID     string
	StoreID        string
}

func LineUnitPrice(skuPrice int64, optionPrices []int64) int64 {
	total := skuPrice
	for _, p := range optionPrices {
		total += p
	}
	return total
}

func BestPromotion(now time.Time, goodsCents int64, promos []Promotion) *Promotion {
	var best *Promotion
	for i := range promos {
		p := promos[i]
		if !p.Enabled || now.Before(p.StartsAt) || !now.Before(p.EndsAt) {
			continue
		}
		if goodsCents < p.ThresholdCents {
			continue
		}
		if best == nil || p.DiscountCents > best.DiscountCents {
			cp := p
			best = &cp
		}
	}
	return best
}

func CouponDiscount(now time.Time, goodsCents int64, coupon *Coupon) int64 {
	if coupon == nil || !coupon.Enabled || coupon.Status != "AVAILABLE" {
		return 0
	}
	if now.Before(coupon.StartsAt) || !now.Before(coupon.EndsAt) {
		return 0
	}
	if goodsCents < coupon.MinSpendCents {
		return 0
	}
	return coupon.DiscountCents
}

func ChooseDiscount(promo *Promotion, couponAmount int64) (kind string, amount int64) {
	promoAmount := int64(0)
	if promo != nil {
		promoAmount = promo.DiscountCents
	}
	if couponAmount > promoAmount {
		return "COUPON", couponAmount
	}
	if promoAmount > 0 {
		return "PROMOTION", promoAmount
	}
	return "", 0
}

func FinalizeAmounts(goods, packing, discount int64) (discountOut, payable int64) {
	if discount < 0 {
		discount = 0
	}
	subtotal := goods + packing
	if discount > subtotal {
		discount = subtotal
	}
	return discount, subtotal - discount
}

func PointsForPayment(payableCents int64, pointsPerYuan int) int {
	if payableCents <= 0 || pointsPerYuan <= 0 {
		return 0
	}
	return int(payableCents/100) * pointsPerYuan
}

func PickupNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%03d", n)
	}
	return fmt.Sprintf("%d", n)
}

func CanStaffTransition(from, to string) bool {
	switch from + "->" + to {
	case OrderPaid + "->" + OrderAccepted,
		OrderAccepted + "->" + OrderPreparing,
		OrderPreparing + "->" + OrderReady,
		OrderReady + "->" + OrderCompleted:
		return true
	default:
		return false
	}
}

func PrintCanAccept(status string) bool {
	return status == OrderPaid
}

func RoleAtLeast(role, need string) bool {
	rank := map[string]int{RoleStaff: 1, RoleManager: 2, RoleOwner: 3}
	return rank[role] >= rank[need]
}

func ValidPassword(pw string) error {
	if n := len([]byte(pw)); n < 12 || n > 72 {
		return apperr.Validation("密码长度必须为 12～72")
	}
	return nil
}
