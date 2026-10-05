package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
)

const (
	bookingID  = domain.BookingID("B-100")
	resourceID = domain.ResourceID("R-200")
	resCode    = domain.ResourceCode("room-101")
	at         = domain.Instant(1755432000)
)

func newReservedBooking(t *testing.T) *domain.Booking {
	t.Helper()
	b := domain.NewBooking(bookingID)
	_, rej := b.Reserve(domain.ReserveBooking{ResourceID: resourceID, Quantity: 5, At: at})
	if rej != nil {
		t.Fatalf("setup: Reserve rejected: %v", rej)
	}
	return b
}

func newRegisteredResource(t *testing.T) *domain.Resource {
	t.Helper()
	r := domain.NewResource(resCode)
	_, rej := r.Register(domain.RegisterResource{Code: resCode, At: at})
	if rej != nil {
		t.Fatalf("setup: Register rejected: %v", rej)
	}
	return r
}

func requireBookingUnchanged(t *testing.T, before, after domain.BookingSnapshot) {
	t.Helper()
	if !before.Equal(after) {
		t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}
