package app

import (
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

var (
	bookingReserved = channel.Event{
		Type:        "com.company.bookings.booking-reserved",
		ContractRef: "company/bookings/event/v1/booking_reserved.proto#BookingReserved",
	}
	bookingCancelled = channel.Event{
		Type:        "com.company.bookings.booking-cancelled",
		ContractRef: "company/bookings/event/v1/booking_cancelled.proto#BookingCancelled",
	}
	resourceRegistered = channel.Event{
		Type:        "com.company.bookings.resource-registered",
		ContractRef: "company/bookings/event/v1/resource_registered.proto#ResourceRegistered",
	}
)

// BookingsChannel is the channel the relay publishes to, named after the
// destination the use case authors, because the publisher resolves by it.
func BookingsChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.Destination, cfg.BookingsTopic, cfg.BookingsDLQ, cfg.Group, bookingReserved, bookingCancelled, resourceRegistered)
}
