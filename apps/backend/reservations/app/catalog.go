package app

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
)

const (
	// ordersDestination is the logical flow the orders context publishes. The
	// consumer names it itself: a context never imports another context's
	// blocks, and the channel name, not the package, is the contract (ASY-01).
	ordersDestination = "orders.events"

	orderPlacedEventType         = "com.company.orders.order-placed"
	orderPlacedContract          = "company/orders/event/v1/order_placed.proto#OrderPlaced"
	reservationConfirmedType     = "com.company.reservations.reservation-confirmed"
	reservationConfirmedContract = "company/reservations/event/v1/reservation_confirmed.proto#ReservationConfirmed"
)

// OrdersChannel is the channel the consumer reads, named after the destination
// the orders use case authors.
func OrdersChannel(cfg Config) channel.Channel {
	return channel.NewKafka(ordersDestination, cfg.OrdersTopic, cfg.OrdersDLQ, cfg.Group, orderPlacedEventType, orderPlacedContract)
}

// ReservationsChannel is the channel the relay publishes to, named after the
// destination the reservations use cases author, because the publisher resolves by it.
func ReservationsChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.Destination, cfg.ReservationsTopic, cfg.ReservationsDLQ, cfg.Group, reservationConfirmedType, reservationConfirmedContract)
}

// NewCatalog is the catalog of ASY-01 holding the one channel a role uses,
// validated so a missing item refuses the process at startup.
func NewCatalog(ch channel.Channel) (channel.Catalog, error) {
	catalog := channel.Catalog{ch.Name: ch}
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	return catalog, nil
}
