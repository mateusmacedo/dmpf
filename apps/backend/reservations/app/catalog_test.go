package app

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
)

func TestEachRoleCatalogsOnlyTheChannelItUses(t *testing.T) {
	cfg := Config{
		OrdersTopic: "orders.events", OrdersDLQ: "orders.events.dlq",
		ReservationsTopic: "reservations.events", ReservationsDLQ: "reservations.events.dlq",
		Group: "reservations",
	}

	consumer, err := NewCatalog(OrdersChannel(cfg))
	if err != nil {
		t.Fatalf("NewCatalog(OrdersChannel) = %v", err)
	}
	if _, ok := consumer[ordersDestination]; !ok || len(consumer) != 1 {
		t.Fatalf("consumer catalog = %v, want only %q", consumer, ordersDestination)
	}

	relay, err := NewCatalog(ReservationsChannel(cfg))
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
