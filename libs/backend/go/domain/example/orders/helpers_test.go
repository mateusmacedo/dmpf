package orders_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/orders"
)

const (
	orderID = orders.OrderID("P-100")
	at      = orders.Instant(1755432000)
)

func newOpenOrder(t *testing.T, items int, limit int) *orders.Order {
	t.Helper()
	o := orders.NewOrder(orderID, limit)
	for i := range items {
		cmd := orders.AddItem{SKU: orders.SKU(string(rune('A' + i))), Quantity: 1, At: at}
		if _, rej := o.AddItem(cmd); rej != nil {
			t.Fatalf("setup: AddItem #%d rejected: %v", i, rej)
		}
	}
	return o
}

func requireRejected[R any](t *testing.T, acc domain.Accepted[R], rej *domain.Rejection, code domain.Code) {
	t.Helper()
	if rej == nil {
		t.Fatal("expected Rejected, got Accepted")
	}
	if rej.Code() != code {
		t.Fatalf("Code() = %q, want %q", rej.Code(), code)
	}
	var zero R
	if any(acc.Response()) != any(zero) {
		t.Fatalf("Rejected must carry the zero response, got %v", acc.Response())
	}
	if n := len(acc.Events()); n != 0 {
		t.Fatalf("Rejected must carry no events, got %d", n)
	}
}

func requireAccepted[R any](t *testing.T, rej *domain.Rejection) {
	t.Helper()
	if rej != nil {
		t.Fatalf("expected Accepted, got Rejected %v", rej)
	}
}

func requireUnchanged(t *testing.T, before, after orders.Snapshot) {
	t.Helper()
	if !after.Equal(before) {
		t.Fatalf("DEC-10 violated: snapshot changed after a rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}
