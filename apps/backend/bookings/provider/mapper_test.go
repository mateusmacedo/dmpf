package provider_test

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/bookings/event/v1"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

type strayEvent struct{}

func (strayEvent) EventName() string { return "bookings.stray" }

func TestMapperMapsBookingReserved(t *testing.T) {
	t.Parallel()

	event := domain.BookingReserved{
		BookingID:  "B-100",
		ResourceID: "R-200",
		Quantity:   5,
		At:         1_755_432_000_123_456_789,
	}

	mapped, err := provider.Mapper{}.Map(event)

	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	want := &eventv1.BookingReserved{
		BookingId:  "B-100",
		ResourceId: "R-200",
		Quantity:   5,
		ReservedAt: &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789},
	}
	if !proto.Equal(mapped.Message, want) {
		t.Errorf("Map().Message = %v, want %v", mapped.Message, want)
	}
	if mapped.Type != "com.company.bookings.booking-reserved.v1" {
		t.Errorf("Map().Type = %q, want %q (PTB-03)", mapped.Type, "com.company.bookings.booking-reserved.v1")
	}
}

func TestMapperMapsBookingCancelled(t *testing.T) {
	t.Parallel()

	mapped, err := provider.Mapper{}.Map(domain.BookingCancelledEvent{BookingID: "B-100", At: 1_755_432_000_000_000_001})

	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	want := &eventv1.BookingCancelled{BookingId: "B-100", CancelledAt: &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 1}}
	if !proto.Equal(mapped.Message, want) || mapped.Type != "com.company.bookings.booking-cancelled.v1" {
		t.Errorf("Map() = %v %q, want %v com.company.bookings.booking-cancelled.v1", mapped.Message, mapped.Type, want)
	}
}

func TestMapperMapsResourceRegistered(t *testing.T) {
	t.Parallel()

	mapped, err := provider.Mapper{}.Map(domain.ResourceRegistered{Code: "R-200", At: 1_755_432_000_000_000_002})

	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	want := &eventv1.ResourceRegistered{ResourceId: "R-200", RegisteredAt: &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 2}}
	if !proto.Equal(mapped.Message, want) || mapped.Type != "com.company.bookings.resource-registered.v1" {
		t.Errorf("Map() = %v %q, want %v com.company.bookings.resource-registered.v1", mapped.Message, mapped.Type, want)
	}
}

func TestMapperReportsUnmappedEvent(t *testing.T) {
	t.Parallel()

	_, err := provider.Mapper{}.Map(strayEvent{})

	if !errors.Is(err, postgres.ErrUnmappedEvent) {
		t.Fatalf("Map() = %v, want ErrUnmappedEvent", err)
	}
}

var _ kernel.DomainEvent = strayEvent{}
