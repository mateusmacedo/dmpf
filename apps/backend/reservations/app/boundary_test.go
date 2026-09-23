package app

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/app"
)

func TestOrdersBoundaryAdmitsTheConfiguredProducer(t *testing.T) {
	cfg := Defaults(RoleConsumer)

	boundary := OrdersBoundary(cfg)

	if !slices.Equal(boundary.Sources, []string{"urn:dmpf:reference-orders"}) {
		t.Fatalf("Sources = %v, want the source the orders relay stamps by default", boundary.Sources)
	}
	if boundary.Transport != kernel.TransportVerified {
		t.Fatalf("Transport = %q, want verified: a secured Kafka is the default", boundary.Transport)
	}
}

func TestOrdersBoundaryFollowsTheKafkaDevelopmentOptOut(t *testing.T) {
	cfg := Defaults(RoleConsumer)
	cfg.KafkaInsecure = true

	if got := OrdersBoundary(cfg).Transport; got != kernel.TransportDevelopmentOnly {
		t.Fatalf("Transport = %q, want development-only: the boundary cannot claim more than the transport", got)
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
		t.Fatalf("OrdersSource = %q, want the one DMPF_ORDERS_SOURCE declares", cfg.OrdersSource)
	}
}
