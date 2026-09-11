package bookingsdomain_test

import (
	"testing"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
)

const (
	bookingID  = bookingsdomain.BookingID("B-100")
	resourceID = bookingsdomain.ResourceID("R-200")
	resCode    = bookingsdomain.ResourceCode("room-101")
	at         = bookingsdomain.Instant(1755432000)
)

func newReservedBooking(t *testing.T) *bookingsdomain.Booking {
	t.Helper()
	b := bookingsdomain.NewBooking(bookingID)
	_, rej := b.Reserve(bookingsdomain.ReserveBooking{ResourceID: resourceID, Quantity: 5, At: at})
	if rej != nil {
		t.Fatalf("setup: Reserve rejected: %v", rej)
	}
	return b
}

func newRegisteredResource(t *testing.T) *bookingsdomain.Resource {
	t.Helper()
	r := bookingsdomain.NewResource(resCode)
	_, rej := r.Register(bookingsdomain.RegisterResource{Code: resCode, At: at})
	if rej != nil {
		t.Fatalf("setup: Register rejected: %v", rej)
	}
	return r
}

func requireRejected[R any](t *testing.T, acc dmpfdomain.Accepted[R], rej *dmpfdomain.Rejection, code dmpfdomain.Code) {
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

func requireAccepted[R any](t *testing.T, rej *dmpfdomain.Rejection) {
	t.Helper()
	if rej != nil {
		t.Fatalf("expected Accepted, got Rejected %v", rej)
	}
}

func requireBookingUnchanged(t *testing.T, before, after bookingsdomain.BookingSnapshot) {
	t.Helper()
	if !before.Equal(after) {
		t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}
