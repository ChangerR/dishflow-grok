package domain_test

import (
	"testing"
	"time"

	"github.com/changerr/dishflow-grok/internal/domain"
)

func TestNormalizeCartMergesOptionsOrderInsensitive(t *testing.T) {
	items, err := domain.NormalizeCart([]domain.CartItem{
		{SKUID: "s1", OptionIDs: []string{"b", "a"}, Qty: 1},
		{SKUID: "s1", OptionIDs: []string{"a", "b"}, Qty: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Qty != 3 {
		t.Fatalf("got %+v", items)
	}
}

func TestChooseDiscountPrefersLargerCouponElsePromotion(t *testing.T) {
	promo := &domain.Promotion{DiscountCents: 500}
	kind, amt := domain.ChooseDiscount(promo, 800)
	if kind != "COUPON" || amt != 800 {
		t.Fatalf("coupon should win: %s %d", kind, amt)
	}
	kind, amt = domain.ChooseDiscount(promo, 500)
	if kind != "PROMOTION" || amt != 500 {
		t.Fatalf("tie uses promotion: %s %d", kind, amt)
	}
}

func TestFinalizeAmountsNeverNegative(t *testing.T) {
	d, p := domain.FinalizeAmounts(100, 20, 500)
	if d != 120 || p != 0 {
		t.Fatalf("discount=%d payable=%d", d, p)
	}
}

func TestPointsFloorToYuan(t *testing.T) {
	if got := domain.PointsForPayment(199, 2); got != 2 {
		t.Fatalf("got %d", got)
	}
	if got := domain.PointsForPayment(99, 5); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestPickupSlotsRespectLeadAndCapacity(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, loc).UTC()
	cfg := domain.StoreHours{
		Timezone: "Asia/Shanghai", BusinessHours: "09:00-12:00",
		ScheduledEnabled: true, AdvanceDays: 7, SlotMinutes: 30,
		SlotCapacity: 2, MinLeadMinutes: 30,
	}
	slots, err := domain.GenerateSlots(cfg, "2026-08-14", now, map[string]int{
		time.Date(2026, 8, 14, 11, 0, 0, 0, loc).UTC().Format(time.RFC3339): 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	found1130, found1100 := false, false
	for _, s := range slots {
		label := s.StartsAt.In(loc).Format("15:04")
		if label == "11:30" {
			found1130 = s.Available
		}
		if label == "11:00" {
			found1100 = s.Available
			if s.Remaining != 0 {
				t.Fatalf("remaining=%d", s.Remaining)
			}
		}
		if label == "10:00" && s.Available {
			t.Fatal("10:00 should be blocked by lead time")
		}
		if label == "10:30" && s.Available {
			t.Fatal("10:30 is not strictly after cutoff")
		}
	}
	if !found1130 {
		t.Fatal("11:30 should be available")
	}
	if found1100 {
		t.Fatal("full slot must not be available")
	}
}

func TestStaffTransitions(t *testing.T) {
	if !domain.CanStaffTransition(domain.OrderPaid, domain.OrderAccepted) {
		t.Fatal("paid->accepted")
	}
	if domain.CanStaffTransition(domain.OrderPaid, domain.OrderReady) {
		t.Fatal("skip forbidden")
	}
}

func TestPickupNumberPadding(t *testing.T) {
	if domain.PickupNumber(1) != "001" || domain.PickupNumber(1000) != "1000" {
		t.Fatal("format")
	}
}
