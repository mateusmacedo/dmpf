package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

const (
	orderID = domain.OrderID("P-100")
	at      = kernel.Instant(1755432000)
)

func requireUnchanged(t *testing.T, before, after domain.Snapshot) {
	t.Helper()
	if !after.Equal(before) {
		t.Fatalf("DEC-10 violated: snapshot changed after a rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}
