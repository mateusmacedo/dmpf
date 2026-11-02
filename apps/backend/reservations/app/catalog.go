package app

import (
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

// ordersDestination is the logical flow the orders context publishes. The
// consumer names it itself: a context never imports another context's
// blocks, and the channel name, not the package, is the contract (ASY-01).
const ordersDestination = "orders.events"

var (
	orderPlaced = channel.Event{
		Type:        "com.company.orders.order-placed",
		ContractRef: "company/orders/event/v1/order_placed.proto#OrderPlaced",
	}
	itemAdded = channel.Event{
		Type:        "com.company.orders.item-added",
		ContractRef: "company/orders/event/v1/item_added.proto#ItemAdded",
	}
	reservationConfirmed = channel.Event{
		Type:        "com.company.reservations.reservation-confirmed",
		ContractRef: "company/reservations/event/v1/reservation_confirmed.proto#ReservationConfirmed",
	}
	reservationCancelled = channel.Event{
		Type:        "com.company.reservations.reservation-cancelled",
		ContractRef: "company/reservations/event/v1/reservation_cancelled.proto#ReservationCancelled",
	}
)

// OrdersChannel is the channel the consumer reads, named after the destination
// the orders use case authors, with every event the orders relay publishes on it.
func OrdersChannel(cfg Config) channel.Channel {
	return channel.NewKafka(ordersDestination, cfg.OrdersTopic, cfg.OrdersDLQ, cfg.Group, orderPlaced, itemAdded)
}

// ReservationsChannel is the channel the relay publishes to, named after the
// destination the reservations use cases author, because the publisher resolves by it.
func ReservationsChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.Destination, cfg.ReservationsTopic, cfg.ReservationsDLQ, cfg.Group, reservationConfirmed, reservationCancelled)
}
