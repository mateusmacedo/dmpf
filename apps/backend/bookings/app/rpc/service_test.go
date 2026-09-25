package rpc_test

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	servicev1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/bookings/service/v1"
)

func reserve(t *testing.T, h *harness, booking string) *servicev1.ReserveBookingResponse {
	t.Helper()
	var resp servicev1.ReserveBookingResponse
	if err := h.invoke(withTenant(t), "ReserveBooking", &servicev1.ReserveBookingRequest{BookingId: booking, ResourceId: "room-1", Quantity: 2}, &resp); err != nil {
		t.Fatalf("ReserveBooking(%s) = %v", booking, err)
	}
	return &resp
}

func TestReserveBookingAnswersTheReservedBooking(t *testing.T) {
	h := newHarness(t, nil)

	resp := reserve(t, h, "b-1")

	if got := resp.GetReserved().GetBookingId(); got != "b-1" {
		t.Fatalf("Reserved.BookingId = %q, want b-1 (response %v)", got, resp)
	}
	var found servicev1.FindBookingResponse
	if err := h.invoke(withTenant(t), "FindBooking", &servicev1.FindBookingRequest{BookingId: "b-1"}, &found); err != nil {
		t.Fatalf("FindBooking() = %v", err)
	}
	booking := found.GetBooking()
	if booking.GetResourceId() != "room-1" || booking.GetQuantity() != 2 || booking.GetStatus() != servicev1.BookingStatus_BOOKING_STATUS_RESERVED {
		t.Fatalf("FindBooking() = %v, want room-1, 2, RESERVED", booking)
	}
	if booking.GetReservedAt() != int64(occurred) {
		t.Fatalf("ReservedAt = %d, want the domain instant %d in nanoseconds (D1)", booking.GetReservedAt(), occurred)
	}
}

func TestReservingTheSameBookingTwiceIsAConflict(t *testing.T) {
	h := newHarness(t, nil)
	reserve(t, h, "b-1")

	var resp servicev1.ReserveBookingResponse
	err := h.invoke(withTenant(t), "ReserveBooking", &servicev1.ReserveBookingRequest{BookingId: "b-1", ResourceId: "room-1", Quantity: 1}, &resp)

	if status.Code(err) != codes.Aborted {
		t.Fatalf("second ReserveBooking() = %v, want Aborted: the booking already exists", err)
	}
}

func TestCancelBookingAnswersTheCancelledBookingAndThenRejects(t *testing.T) {
	h := newHarness(t, nil)
	reserve(t, h, "b-1")

	var first, second servicev1.CancelBookingResponse
	if err := h.invoke(withTenant(t), "CancelBooking", &servicev1.CancelBookingRequest{BookingId: "b-1"}, &first); err != nil {
		t.Fatalf("CancelBooking() = %v", err)
	}
	if first.GetCancelled().GetBookingId() != "b-1" {
		t.Fatalf("first CancelBooking() = %v, want Cancelled b-1", &first)
	}
	if err := h.invoke(withTenant(t), "CancelBooking", &servicev1.CancelBookingRequest{BookingId: "b-1"}, &second); err != nil {
		t.Fatalf("second CancelBooking() = %v, want the refusal in the body", err)
	}
	if second.GetRejection().GetCode() == "" {
		t.Fatalf("second CancelBooking() = %v, want a domain rejection", &second)
	}
}

func TestCancellingAnUnknownBookingIsNotFound(t *testing.T) {
	h := newHarness(t, nil)

	var resp servicev1.CancelBookingResponse
	err := h.invoke(withTenant(t), "CancelBooking", &servicev1.CancelBookingRequest{BookingId: "b-none"}, &resp)

	if status.Code(err) != codes.NotFound {
		t.Fatalf("CancelBooking(unknown) = %v, want NotFound", err)
	}
}

func TestRegisterResourceAnswersTheRegisteredResource(t *testing.T) {
	h := newHarness(t, nil)

	var resp servicev1.RegisterResourceResponse
	if err := h.invoke(withTenant(t), "RegisterResource", &servicev1.RegisterResourceRequest{ResourceId: "room-1"}, &resp); err != nil {
		t.Fatalf("RegisterResource() = %v", err)
	}
	if resp.GetRegistered().GetResourceId() != "room-1" {
		t.Fatalf("RegisterResource() = %v, want Registered room-1", &resp)
	}
}

func TestFindBookingsByResourceMapsEveryBooking(t *testing.T) {
	h := newHarness(t, byResource{"room-1": {
		{ID: "b-1", ResourceID: "room-1", Quantity: 1, Status: domain.Reserved, ReservedAt: domain.Instant(occurred)},
		{ID: "b-2", ResourceID: "room-1", Quantity: 3, Status: domain.Cancelled, ReservedAt: domain.Instant(occurred)},
	}})

	var resp servicev1.FindBookingsByResourceResponse
	if err := h.invoke(withTenant(t), "FindBookingsByResource", &servicev1.FindBookingsByResourceRequest{ResourceId: "room-1"}, &resp); err != nil {
		t.Fatalf("FindBookingsByResource() = %v", err)
	}
	bookings := resp.GetBookings()
	if len(bookings) != 2 || bookings[0].GetBookingId() != "b-1" || bookings[1].GetStatus() != servicev1.BookingStatus_BOOKING_STATUS_CANCELLED {
		t.Fatalf("FindBookingsByResource() = %v, want b-1 and a cancelled b-2", bookings)
	}
}

func TestMalformedRequestsAreRefusedBeforeTheUseCase(t *testing.T) {
	h := newHarness(t, nil)
	long := string(make([]byte, 129))

	calls := []struct {
		name string
		call func() error
	}{
		{"booking id with a space", func() error {
			var r servicev1.ReserveBookingResponse
			return h.invoke(withTenant(t), "ReserveBooking", &servicev1.ReserveBookingRequest{BookingId: "b 1", ResourceId: "room-1", Quantity: 1}, &r)
		}},
		{"quantity zero", func() error {
			var r servicev1.ReserveBookingResponse
			return h.invoke(withTenant(t), "ReserveBooking", &servicev1.ReserveBookingRequest{BookingId: "b-1", ResourceId: "room-1", Quantity: 0}, &r)
		}},
		{"quantity above the ceiling", func() error {
			var r servicev1.ReserveBookingResponse
			return h.invoke(withTenant(t), "ReserveBooking", &servicev1.ReserveBookingRequest{BookingId: "b-1", ResourceId: "room-1", Quantity: 101}, &r)
		}},
		{"empty resource to register", func() error {
			var r servicev1.RegisterResourceResponse
			return h.invoke(withTenant(t), "RegisterResource", &servicev1.RegisterResourceRequest{ResourceId: ""}, &r)
		}},
		{"resource id too long", func() error {
			var r servicev1.FindBookingsByResourceResponse
			return h.invoke(withTenant(t), "FindBookingsByResource", &servicev1.FindBookingsByResourceRequest{ResourceId: long}, &r)
		}},
		{"empty booking to find", func() error {
			var r servicev1.FindBookingResponse
			return h.invoke(withTenant(t), "FindBooking", &servicev1.FindBookingRequest{BookingId: ""}, &r)
		}},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("call = %v, want InvalidArgument", err)
			}
		})
	}
	if len(h.store.Entries()) != 0 {
		t.Fatalf("outbox holds %d entries, want none: no malformed call reached the use case", len(h.store.Entries()))
	}
}
