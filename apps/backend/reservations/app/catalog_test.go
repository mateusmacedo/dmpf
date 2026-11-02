package app

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/provider"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

func TestEachRoleCatalogsOnlyTheChannelItUses(t *testing.T) {
	cfg := Config{
		OrdersTopic: "orders.events", OrdersDLQ: "orders.events.dlq",
		ReservationsTopic: "reservations.events", ReservationsDLQ: "reservations.events.dlq",
		Group: "reservations",
	}

	consumer, err := channel.NewCatalog(OrdersChannel(cfg))
	if err != nil {
		t.Fatalf("NewCatalog(OrdersChannel) = %v", err)
	}
	if _, ok := consumer[ordersDestination]; !ok || len(consumer) != 1 {
		t.Fatalf("consumer catalog = %v, want only %q", consumer, ordersDestination)
	}

	relay, err := channel.NewCatalog(ReservationsChannel(cfg))
	if err != nil {
		t.Fatalf("NewCatalog(ReservationsChannel) = %v", err)
	}
	if _, ok := relay[application.Destination]; !ok || len(relay) != 1 {
		t.Fatalf("relay catalog = %v, want only %q", relay, application.Destination)
	}
	if consumer[ordersDestination].Address == relay[application.Destination].Address {
		t.Fatal("the two roles resolved to the same topic; each channel addresses its own")
	}
}

func TestTheReservationsChannelDeclaresEveryEventTheRelayPublishes(t *testing.T) {
	cfg := Defaults(RoleRelay)
	cfg.ReservationsTopic, cfg.ReservationsDLQ, cfg.Group = "reservations.v1", "reservations.v1.dlq", "reservations-relay"
	ch := ReservationsChannel(cfg)

	if err := ch.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	want := []string{"com.company.reservations.reservation-confirmed.v1", "com.company.reservations.reservation-cancelled.v1"}
	if got := channel.WireTypes(ch); !slices.Equal(got, want) {
		t.Fatalf("WireTypes() = %v, want %v (KFK-03b)", got, want)
	}
}

func TestEveryTypeTheMapperPublishesIsOnTheReservationsChannel(t *testing.T) {
	carried := channel.WireTypes(ReservationsChannel(Defaults(RoleRelay)))

	for _, event := range []kernel.DomainEvent{domain.ReservationConfirmed{}, domain.ReservationCancelled{}} {
		mapped, err := provider.Mapper{}.Map(event)
		if err != nil {
			t.Fatalf("Map(%s) = %v, want nil", event.EventName(), err)
		}
		if !slices.Contains(carried, mapped.Type) {
			t.Errorf("Map(%s).Type = %q, outside the channel's %v: the relay would publish a type it does not declare (KFK-03b)", event.EventName(), mapped.Type, carried)
		}
	}
}
