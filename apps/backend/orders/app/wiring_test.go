package app_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

func TestTheAuditTrailOfTheServiceReachesTheLoggerProviderOfTheRuntime(t *testing.T) {
	obstest.AuditTrail(t, "orders", func(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool) error {
		service, err := app.NewOrdersService(pool, rt, app.Defaults(app.RoleAPI))
		if err != nil {
			return err
		}
		service.Instrumentation.Audit(ctx, ports.AuditEvent{Object: "order-9", Action: "read", Outcome: ports.OutcomeAccepted})
		return nil
	})
}

func TestTheRelayPublishesThroughKafka(t *testing.T) {
	if system := app.Defaults(app.RoleRelay).Relay.System; system != "kafka" {
		t.Fatalf("Relay.System = %q, want %q", system, "kafka")
	}
}

func TestTheRelayAddressesEachChannelByItsPhysicalTopic(t *testing.T) {
	cfg := app.Defaults(app.RoleRelay)
	cfg.OrdersTopic, cfg.OrdersDLQ, cfg.Group = "orders.v1", "orders.v1.dlq", "orders-relay"
	catalog, err := app.NewCatalog(cfg)
	if err != nil {
		t.Fatalf("NewCatalog() = %v, want nil", err)
	}

	if topic := catalog.AddressOf(application.Destination); topic != cfg.OrdersTopic {
		t.Fatalf("AddressOf(%q) = %q, want the physical topic %q (RF-B7)", application.Destination, topic, cfg.OrdersTopic)
	}
	if topic := catalog.AddressOf("uncatalogued.events"); topic != "" {
		t.Fatalf("AddressOf(uncatalogued.events) = %q, want empty: the send never names a logical channel", topic)
	}
}
