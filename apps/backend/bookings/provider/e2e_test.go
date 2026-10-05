//go:build integration

package provider_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	testids "github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const (
	e2eBookingID  = domain.BookingID("e2e-b-001")
	e2eResourceID = domain.ResourceID("e2e-r-001")
	e2eOccurred   = ports.Instant(1_755_432_000)
)

func TestReserveBookingEndToEnd(t *testing.T) {
	h := appkit.NewBookings(t, clock.New(e2eOccurred), &testids.Sequence{Prefix: "m-"})
	pool, service := h.Pool, h.Service
	ctx := context.Background()

	outcome, err := service.ReserveBooking(withExecution(t, ctx), application.ReserveBooking{
		BookingID:  e2eBookingID,
		ResourceID: e2eResourceID,
		Quantity:   3,
	})
	if err != nil {
		t.Fatalf("ReserveBooking() = %v, want nil", err)
	}
	if _, refused := outcome.Rejection(); refused {
		t.Fatal("ReserveBooking() was rejected, want accepted")
	}

	t.Run("the booking is persisted", func(t *testing.T) {
		snap, version, err := provider.NewBookingReader(postgres.NewReadPool(pool)).Load(withExecution(t, ctx), e2eBookingID)
		if err != nil {
			t.Fatalf("Load() = %v", err)
		}
		if version != 1 {
			t.Fatalf("version = %d, want 1", version)
		}
		if snap.Quantity != 3 || snap.ResourceID != e2eResourceID || snap.Status != domain.Reserved {
			t.Fatalf("snapshot = %+v", snap)
		}
	})

	t.Run("the outbox row lands at version 1", func(t *testing.T) {
		want := pg.Enqueued{
			MessageID:        "m-000001",
			MessageType:      "com.company.bookings.booking-reserved.v1",
			SchemaVersion:    "type.googleapis.com/company.bookings.event.v1.BookingReserved",
			AggregateVersion: 1,
			Destination:      "bookings.events",
			Status:           "pending",
		}
		if outbox := h.Outbox(t); len(outbox) != 1 || outbox[0] != want {
			t.Fatalf("outbox = %+v, want [%+v]", outbox, want)
		}
	})

	t.Run("cancel commits without outbox row", func(t *testing.T) {
		before := pg.Counts(t, pool, "outbox")

		cancelOutcome, err := service.CancelBooking(withExecution(t, ctx), application.CancelBooking{
			BookingID: e2eBookingID,
		})
		if err != nil {
			t.Fatalf("CancelBooking() = %v, want nil", err)
		}
		if _, refused := cancelOutcome.Rejection(); refused {
			t.Fatal("CancelBooking() was rejected, want accepted")
		}

		snap, version, err := provider.NewBookingReader(postgres.NewReadPool(pool)).Load(withExecution(t, ctx), e2eBookingID)
		if err != nil {
			t.Fatalf("Load() = %v", err)
		}
		if version != 2 {
			t.Fatalf("version = %d, want 2", version)
		}
		if snap.Status != domain.Cancelled {
			t.Fatalf("Status = %v, want Cancelled", snap.Status)
		}

		if after := pg.Counts(t, pool, "outbox"); after["outbox"] != before["outbox"]+1 {
			t.Fatalf("outbox count on cancel: %d→%d, want one Cancelled in the same transaction", before["outbox"], after["outbox"])
		}
	})
}
