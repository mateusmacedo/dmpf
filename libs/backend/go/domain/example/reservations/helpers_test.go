package reservations_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
)

const (
	orderID = reservations.OrderID("P-100")
	at      = reservations.Instant(1755432000)
)

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

func requireUnchanged(t *testing.T, before, after reservations.Snapshot) {
	t.Helper()
	if !after.Equal(before) {
		t.Fatalf("DEC-10 violated: snapshot changed after a rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}
