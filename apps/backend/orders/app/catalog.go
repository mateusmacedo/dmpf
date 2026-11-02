package app

import (
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

var (
	orderPlaced = channel.Event{
		Type:        "com.company.orders.order-placed",
		ContractRef: "company/orders/event/v1/order_placed.proto#OrderPlaced",
	}
	itemAdded = channel.Event{
		Type:        "com.company.orders.item-added",
		ContractRef: "company/orders/event/v1/item_added.proto#ItemAdded",
	}
)

// OrdersChannel is the channel the relay publishes to, named after the
// destination the use case authors, because the publisher resolves by it.
func OrdersChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.Destination, cfg.OrdersTopic, cfg.OrdersDLQ, cfg.Group, orderPlaced, itemAdded)
}
