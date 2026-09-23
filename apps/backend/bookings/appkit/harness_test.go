//go:build integration

package appkit_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
)

const (
	resourceUnderTest = domain.ResourceCode("room-appkit")
	bookingUnderTest  = domain.BookingID("b-appkit-1")
)

func harness(t *testing.T) appkit.Harness {
	t.Helper()
	return appkit.NewBookings(t, clock.New(ports.Instant(1)), &ids.Sequence{Prefix: "m-"})
}

func registerResource(t *testing.T, h appkit.Harness) domain.ResourceID {
	t.Helper()
	registered, err := h.Service.RegisterResource(withExecution(t, context.Background()),
		application.Register{Code: resourceUnderTest})
	if err != nil {
		t.Fatalf("RegisterResource() = %v, want nil", err)
	}
	if rejection, refused := registered.Rejection(); refused {
		t.Fatalf("RegisterResource() was rejected with %v, want accepted", rejection)
	}
	return domain.ResourceID(resourceUnderTest)
}

func TestTheUseCaseLeavesOneOutboxRecordPerDecision(t *testing.T) {
	h := harness(t)
	resource := registerResource(t, h)

	reserved, err := h.Service.ReserveBooking(withExecution(t, context.Background()),
		application.Reserve{BookingID: bookingUnderTest, ResourceID: resource, Quantity: 2})

	if err != nil {
		t.Fatalf("ReserveBooking() = %v, want nil", err)
	}
	if rejection, refused := reserved.Rejection(); refused {
		t.Fatalf("ReserveBooking() was rejected with %v, want accepted", rejection)
	}
	enqueued := h.Outbox(t)
	if len(enqueued) == 0 {
		t.Fatal("outbox is empty: the event goes to the outbox in the same transaction as the state")
	}
	for i, record := range enqueued {
		if record.Destination != application.Destination {
			t.Fatalf("record %d goes to %q, want %q", i, record.Destination, application.Destination)
		}
		if record.Status != "pending" {
			t.Fatalf("record %d is %q, want pending: nothing drained it yet", i, record.Status)
		}
	}
}

func TestARejectedDecisionLeavesNothingBehind(t *testing.T) {
	h := harness(t)
	resource := registerResource(t, h)
	before := len(h.Outbox(t))

	// A quantity outside the range is the rejection the aggregate declares
	// (CodeQuantityOutOfRange), and a rejection never reaches the outbox.
	reserved, err := h.Service.ReserveBooking(withExecution(t, context.Background()),
		application.Reserve{BookingID: bookingUnderTest, ResourceID: resource, Quantity: 0})

	if err != nil {
		t.Fatalf("ReserveBooking() = %v, want the rejection on the business channel, not an error", err)
	}
	if _, refused := reserved.Rejection(); !refused {
		t.Fatal("ReserveBooking() accepted a quantity of zero")
	}
	if after := len(h.Outbox(t)); after != before {
		t.Fatalf("outbox went from %d to %d records after a rejection, want no change: the transaction never committed", before, after)
	}
}
