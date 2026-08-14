package domain_test

import (
	"testing"

	"github.com/changerr/dishflow-grok/internal/domain"
)

func TestInventoryInvariantConcept(t *testing.T) {
	avail, reserved, sold := 10, 3, 4
	if avail < reserved+sold {
		t.Fatal("constraint")
	}
	if domain.CanStaffTransition(domain.OrderAccepted, domain.OrderReady) {
		t.Fatal("skip preparing forbidden")
	}
	if !domain.PrintCanAccept(domain.OrderPaid) {
		t.Fatal("print can accept paid")
	}
	if domain.PrintCanAccept(domain.OrderPreparing) {
		t.Fatal("print must not skip")
	}
}
