package dmpfreferencereservations

import (
	"testing"

	ordersapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/orders"
	reservationsapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/reservations"
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
	if _, ok := consumer[ordersapp.Destination]; !ok || len(consumer) != 1 {
		t.Fatalf("consumer catalog = %v, want only %q", consumer, ordersapp.Destination)
	}

	relay, err := NewCatalog(ReservationsChannel(cfg))
	if err != nil {
		t.Fatalf("NewCatalog(ReservationsChannel) = %v", err)
	}
	if _, ok := relay[reservationsapp.Destination]; !ok || len(relay) != 1 {
		t.Fatalf("relay catalog = %v, want only %q", relay, reservationsapp.Destination)
	}
	if consumer[ordersapp.Destination].Address == relay[reservationsapp.Destination].Address {
		t.Fatal("the two roles resolved to the same topic; each channel addresses its own")
	}
}
