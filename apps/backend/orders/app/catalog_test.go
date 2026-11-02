package app_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

func TestTheOrdersChannelDeclaresEveryEventTheRelayPublishes(t *testing.T) {
	cfg := app.Defaults(app.RoleRelay)
	cfg.OrdersTopic, cfg.OrdersDLQ, cfg.Group = "orders.v1", "orders.v1.dlq", "orders-relay"
	ch := app.OrdersChannel(cfg)

	if err := ch.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	want := []string{"com.company.orders.order-placed.v1", "com.company.orders.item-added.v1"}
	if got := channel.WireTypes(ch); !slices.Equal(got, want) {
		t.Fatalf("WireTypes() = %v, want %v (KFK-03b)", got, want)
	}
}
