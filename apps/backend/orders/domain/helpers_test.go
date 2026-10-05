package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
)

const (
	orderID = domain.OrderID("P-100")
	at      = domain.Instant(1755432000)
)

func newOpenOrder(t *testing.T, items int, limit int) *domain.Order {
	t.Helper()
	o := domain.NewOrder(orderID, limit)
	for i := range items {
		cmd := domain.AddItem{SKU: domain.SKU(string(rune('A' + i))), Quantity: 1, At: at}
		if _, rej := o.AddItem(cmd); rej != nil {
			t.Fatalf("setup: AddItem #%d rejected: %v", i, rej)
		}
	}
	return o
}

func requireUnchanged(t *testing.T, before, after domain.Snapshot) {
	t.Helper()
	if !after.Equal(before) {
		t.Fatalf("DEC-10 violated: snapshot changed after a rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}
