package dmpfreference_test

import (
	"errors"
	"testing"

	dmpfreference "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/apps/backend/dmpf-reference"
	ordersapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/orders"
	reservationsapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/reservations"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
)

func relayConfig(t *testing.T) dmpfreference.Config {
	t.Helper()
	cfg, err := dmpfreference.FromEnv(dmpfreference.RoleConsumer, env(
		"DMPF_PG_DSN", testDSN,
		"DMPF_KAFKA_BROKERS", "localhost:9092",
		"DMPF_KAFKA_TOPIC", "orders.events.topic",
		"DMPF_KAFKA_GROUP", "reservations",
		"DMPF_KAFKA_DLQ", "orders.events.dlq",
		"DMPF_KAFKA_RESERVATIONS_TOPIC", "reservations.events.topic",
		"DMPF_KAFKA_RESERVATIONS_DLQ", "reservations.events.dlq",
	))
	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	return cfg
}

// Both use cases of the example author a destination, and the relay drains
// whatever either one enqueues: a destination without a channel would leave
// its rows failing after every attempt (ErrUnknownChannel), never published.
func TestCatalogDeclaresOneChannelPerDestinationTheUseCasesAuthor(t *testing.T) {
	cfg := relayConfig(t)

	catalog, err := dmpfreference.NewCatalog(cfg)
	if err != nil {
		t.Fatalf("NewCatalog() = %v, want nil", err)
	}
	if len(catalog) != 2 {
		t.Fatalf("catalog has %d channels, want 2 (orders and reservations)", len(catalog))
	}

	orders, err := catalog.Resolve(ordersapp.Destination)
	if err != nil {
		t.Fatalf("Resolve(%q) = %v, want the orders channel", ordersapp.Destination, err)
	}
	if orders.Address != "orders.events.topic" || orders.Group != "reservations" || orders.Containment != "orders.events.dlq" {
		t.Fatalf("orders channel = %+v, want address, group and containment from the environment", orders)
	}
	if orders.Transport != channel.Kafka || orders.Retry.Strategy != channel.InlineWithLimit || orders.Retry.MaxAttempts <= 0 {
		t.Fatalf("orders channel = %+v, want Kafka with inline retry and a positive attempt limit (TRP-31)", orders)
	}
	if want := "com.company.orders.order-placed.v1"; dmpfreference.EventTypeOf(orders) != want {
		t.Fatalf("EventTypeOf(orders) = %q, want %q", dmpfreference.EventTypeOf(orders), want)
	}

	reservations, err := catalog.Resolve(reservationsapp.Destination)
	if err != nil {
		t.Fatalf("Resolve(%q) = %v, want the reservations channel", reservationsapp.Destination, err)
	}
	if reservations.Address != "reservations.events.topic" || reservations.Containment != "reservations.events.dlq" {
		t.Fatalf("reservations channel = %+v, want its own topic and DLQ", reservations)
	}
	if want := "com.company.reservations.reservation-confirmed.v1"; dmpfreference.EventTypeOf(reservations) != want {
		t.Fatalf("EventTypeOf(reservations) = %q, want %q", dmpfreference.EventTypeOf(reservations), want)
	}
	if dmpfreference.OrdersChannel(cfg).Name != orders.Name {
		t.Fatalf("OrdersChannel() = %q, want the channel the consumer subscribes to, %q", dmpfreference.OrdersChannel(cfg).Name, orders.Name)
	}
}

func TestCatalogRefusesAChannelWithoutContainment(t *testing.T) {
	for name, strip := range map[string]func(*dmpfreference.Config){
		"orders":       func(c *dmpfreference.Config) { c.DLQ = "" },
		"reservations": func(c *dmpfreference.Config) { c.ReservationsDLQ = "" },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := relayConfig(t)
			strip(&cfg)

			_, err := dmpfreference.NewCatalog(cfg)

			if !errors.Is(err, channel.ErrMissingItem) {
				t.Fatalf("NewCatalog() = %v, want ErrMissingItem (containment is an ASY-02 item)", err)
			}
		})
	}
}
