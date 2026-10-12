//go:build integration

package app_test

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

func TestTheRelayRoleSendsUnderThePhysicalTopicOfTheChannel(t *testing.T) {
	brokers := strings.Split(tb.Env(t, "KAFKA_BROKERS"), ",")
	pool := appkit.OpenPool(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	cfg := app.Defaults(app.RoleRelay)
	cfg.DSN = pg.DSN(t, appkit.PoolOptions.Project)
	cfg.Brokers, cfg.KafkaInsecure = brokers, true
	cfg.OrdersTopic = "wiring-orders-" + suffix
	cfg.OrdersDLQ = cfg.OrdersTopic + "-dlq"
	cfg.Group = "wiring-orders-group-" + suffix
	cfg.Relay.Interval = 10 * time.Millisecond
	dist.CreateTopics(t, brokers, cfg.Group, cfg.OrdersTopic, cfg.OrdersDLQ)
	enqueueForTheRelay(t, pool, application.Destination)
	obstest.RelayWiring{
		Service: "orders",
		Scope:   reflect.TypeFor[app.Config]().PkgPath(),
		Topic:   cfg.OrdersTopic,
		Channel: application.Destination,
		Run:     func(ctx context.Context, rt *otelboot.Runtime) error { return app.RunWith(ctx, cfg, rt) },
	}.Prove(t)
}

func enqueueForTheRelay(t *testing.T, pool *pgxpool.Pool, destination string) {
	t.Helper()
	payload := []byte("wiring")
	occurred := time.Now().UnixNano()
	_, err := pool.Exec(context.Background(), `
INSERT INTO outbox (
	message_id, message_type, schema_version, aggregate_type, aggregate_id, aggregate_version,
	partition_key, destination, payload, payload_hash, metadata, occurred_at, available_at
) VALUES ($1, $2, $3, $4, $5, 1, $5, $6, $7, $8, $9::jsonb, $10, $10)`,
		"m-wiring-1", "com.company.orders.order-placed.v1",
		"type.googleapis.com/company.orders.event.v1.OrderPlaced", "order", "o-wiring-1",
		destination, payload, payloadhash.Sum(payload),
		`{"correlationid":"corr-wiring","causationid":"caus-wiring","traceparent":"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"}`,
		occurred)
	if err != nil {
		t.Fatalf("enqueue = %v, want nil", err)
	}
}

func TestEveryRoleReportsTheConnectionsOfItsPoolThroughTheRuntime(t *testing.T) {
	brokers := strings.Split(tb.Env(t, "KAFKA_BROKERS"), ",")
	for _, role := range app.Roles {
		t.Run(string(role), func(t *testing.T) {
			appkit.OpenPool(t)
			suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
			cfg := app.Defaults(role)
			cfg.DSN = pg.DSN(t, appkit.PoolOptions.Project)
			cfg.API.GRPCAddr, cfg.API.GRPCInsecure = "127.0.0.1:0", true
			cfg.Brokers, cfg.KafkaInsecure = brokers, true
			cfg.OrdersTopic = "pool-orders-" + suffix
			cfg.OrdersDLQ = cfg.OrdersTopic + "-dlq"
			cfg.Group = "pool-orders-group-" + suffix
			reader := sdkmetric.NewManualReader()
			runtime := obstest.MeteredRuntime(t, "orders", string(role), reader)

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			stopped := make(chan error, 1)
			go func() { stopped <- app.RunWith(ctx, cfg, runtime) }()
			obstest.AwaitPooledConnection(t, reader, cfg.DSN, stopped, 15*time.Second)
			cancel()
			if err := <-stopped; err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("RunWith(%s) = %v, want nil or the cancellation", role, err)
			}
		})
	}
}
