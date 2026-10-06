package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var errBoom = errors.New("boom")

func TestEachFailureOfACommandCarriesTheUseCasePrefix(t *testing.T) {
	reserve := func(t *testing.T, h *harness) error {
		_, err := h.service.ReserveBooking(withExecution(t, context.Background()), application.ReserveBooking{BookingID: testBookingID, ResourceID: testResourceID, Quantity: 1})
		return err
	}
	cancel := func(t *testing.T, h *harness) error {
		_, err := h.service.CancelBooking(withExecution(t, context.Background()), application.CancelBooking{BookingID: testBookingID})
		return err
	}
	register := func(t *testing.T, h *harness) error {
		_, err := h.service.RegisterResource(withExecution(t, context.Background()), application.RegisterResource{Code: testResCode})
		return err
	}
	for _, tc := range []struct {
		name    string
		options []option
		seeded  bool
		run     func(*testing.T, *harness) error
		wantIs  error
		wantMsg string
	}{
		{"ReserveBooking, load", []option{withLoadError(errBoom)}, false, reserve, errBoom, "application: reserve B-100: boom"},
		{"ReserveBooking, already exists", nil, true, reserve, ports.ErrAlreadyExists, "application: reserve B-100: ports: aggregate already exists"},
		{"ReserveBooking, save", []option{withSaveError(errBoom)}, false, reserve, errBoom, "application: reserve B-100: boom"},
		{"ReserveBooking, enqueue", []option{withEnqueueError(errBoom)}, false, reserve, errBoom, "application: reserve B-100: enqueue: boom"},
		{"CancelBooking, absent", nil, false, cancel, ports.ErrNotFound, "application: cancel B-100: ports: aggregate not found"},
		{"CancelBooking, load", []option{withLoadError(errBoom)}, true, cancel, errBoom, "application: cancel B-100: boom"},
		{"CancelBooking, save", []option{withSaveError(errBoom)}, true, cancel, errBoom, "application: cancel B-100: boom"},
		{"CancelBooking, enqueue", []option{withEnqueueError(errBoom)}, true, cancel, errBoom, "application: cancel B-100: enqueue: boom"},
		{"RegisterResource, load", []option{withLoadError(errBoom)}, false, register, errBoom, "application: register room-101: boom"},
		{"RegisterResource, save", []option{withSaveError(errBoom)}, false, register, errBoom, "application: register room-101: boom"},
		{"RegisterResource, enqueue", []option{withEnqueueError(errBoom)}, false, register, errBoom, "application: register room-101: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			if tc.seeded {
				h.seedBooking(t, domain.BookingSnapshot{ID: testBookingID, ResourceID: testResourceID, Quantity: 1, Status: domain.Reserved}, 1)
			}

			err := tc.run(t, h)

			if !errors.Is(err, tc.wantIs) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q matching %v", err, tc.wantMsg, tc.wantIs)
			}
		})
	}
}
