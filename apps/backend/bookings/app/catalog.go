package app

import (
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

const (
	bookingReservedEventType = "com.company.bookings.booking-reserved"
	bookingReservedContract  = "company/bookings/event/v1/booking_reserved.proto#BookingReserved"
)

// BookingsChannel is the channel the relay publishes to, named after the
// destination the use case authors, because the publisher resolves by it.
func BookingsChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.Destination, cfg.BookingsTopic, cfg.BookingsDLQ, cfg.Group,
		bookingReservedEventType, bookingReservedContract)
}

// NewCatalog is the catalog of ASY-01 with the one channel this context drains.
func NewCatalog(cfg Config) (channel.Catalog, error) {
	bookings := BookingsChannel(cfg)
	catalog := channel.Catalog{bookings.Name: bookings}
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	return catalog, nil
}
