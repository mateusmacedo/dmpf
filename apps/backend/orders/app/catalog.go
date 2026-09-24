package app

import (
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

const (
	orderPlacedEventType = "com.company.orders.order-placed"
	orderPlacedContract  = "company/orders/event/v1/order_placed.proto#OrderPlaced"
)

// OrdersChannel is the channel the relay publishes to, named after the
// destination the use case authors, because the publisher resolves by it.
func OrdersChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.Destination, cfg.OrdersTopic, cfg.OrdersDLQ, cfg.Group, orderPlacedEventType, orderPlacedContract)
}

// NewCatalog is the catalog of ASY-01 with the one channel this context drains.
func NewCatalog(cfg Config) (channel.Catalog, error) {
	orders := OrdersChannel(cfg)
	catalog := channel.Catalog{orders.Name: orders}
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	return catalog, nil
}
