package app_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

func TestTheBookingsChannelDeclaresEveryEventTheRelayPublishes(t *testing.T) {
	cfg := app.Defaults(app.RoleRelay)
	cfg.BookingsTopic, cfg.BookingsDLQ, cfg.Group = "bookings.v1", "bookings.v1.dlq", "bookings-relay"
	ch := app.BookingsChannel(cfg)

	if err := ch.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	want := []string{
		"com.company.bookings.booking-reserved.v1",
		"com.company.bookings.booking-cancelled.v1",
		"com.company.bookings.resource-registered.v1",
	}
	if got := channel.WireTypes(ch); !slices.Equal(got, want) {
		t.Fatalf("WireTypes() = %v, want %v (KFK-03b)", got, want)
	}
}

func TestEveryTypeTheMapperPublishesIsOnTheBookingsChannel(t *testing.T) {
	carried := channel.WireTypes(app.BookingsChannel(app.Defaults(app.RoleRelay)))

	for _, event := range []kernel.DomainEvent{domain.BookingReserved{}, domain.BookingCancelled{}, domain.ResourceRegistered{}} {
		mapped, err := provider.Mapper{}.Map(event)
		if err != nil {
			t.Fatalf("Map(%s) = %v, want nil", event.EventName(), err)
		}
		if !slices.Contains(carried, mapped.Type) {
			t.Errorf("Map(%s).Type = %q, outside the channel's %v: the relay would publish a type it does not declare (KFK-03b)", event.EventName(), mapped.Type, carried)
		}
	}
}
