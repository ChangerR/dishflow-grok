package print_test

import (
	"strings"
	"testing"

	"github.com/changerr/dishflow-grok/internal/print"
)

func TestRenderOrderIncludesReservation(t *testing.T) {
	s := print.RenderOrder(print.Ticket{
		StoreName: "测试店", PickupNumber: "001", PickupType: "SCHEDULED",
		ScheduledFor: "2026-08-14T18:45:00+08:00", PayableCents: 2500, IsMock: true,
		Items: []print.TicketItem{{Name: "牛肉面", Qty: 1}},
	})
	for _, want := range []string{"预约自取", "001", "测试店", "【测试单】"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
}
