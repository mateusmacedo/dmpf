package app

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/app"
)

func TestOrdersBoundaryAdmitsTheConfiguredProducer(t *testing.T) {
	cfg := Defaults(RoleConsumer)

	boundary := OrdersBoundary(cfg, true)

	if !slices.Equal(boundary.Sources, []string{"urn:dmpf:reference-orders"}) {
		t.Fatalf("Sources = %v, want the source the orders relay stamps by default", boundary.Sources)
	}
	if boundary.Transport != kernel.TransportVerified {
		t.Fatalf("Transport = %q, want verified: TLS with an authenticated client", boundary.Transport)
	}
}

// IDN-04: a boundary is verified only on what the transport proved, never on a
// variable saying TLS is on.
func TestOrdersBoundaryWithoutAnAuthenticatedClientIsDevelopmentOnly(t *testing.T) {
	if got := OrdersBoundary(Defaults(RoleConsumer), false).Transport; got != kernel.TransportDevelopmentOnly {
		t.Fatalf("Transport = %q, want development-only: the boundary cannot claim more than the transport proved", got)
	}
}

func TestTheAdmittedProducerIsReadFromTheEnvironment(t *testing.T) {
	env := map[string]string{
		envDSN: "postgres://x", envBrokers: "localhost:9092", envKafkaInsecure: "true",
		envOrdersTopic: "orders.events", envOrdersDLQ: "orders.events.dlq", envGroup: "reservations",
		envOrdersSource: "urn:dmpf:another-orders",
	}
	cfg, err := FromEnv(RoleConsumer, func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
	if cfg.OrdersSource != "urn:dmpf:another-orders" {
		t.Fatalf("OrdersSource = %q, want the one ORDERS_SOURCE declares", cfg.OrdersSource)
	}
}
