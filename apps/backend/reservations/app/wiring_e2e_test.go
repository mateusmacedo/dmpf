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
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

func TestTheRelayRoleSendsUnderThePhysicalTopicOfTheChannel(t *testing.T) {
	brokers := strings.Split(tb.Env(t, "KAFKA_BROKERS"), ",")
	pool := appkit.OpenPool(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	cfg := app.Defaults(app.RoleRelay)
	cfg.DSN = pg.DSN(t, appkit.PoolOptions.Project)
	cfg.Brokers, cfg.KafkaInsecure = brokers, true
	cfg.ReservationsTopic = "wiring-reservations-" + suffix
	cfg.ReservationsDLQ = cfg.ReservationsTopic + "-dlq"
	cfg.Group = "wiring-reservations-group-" + suffix
	cfg.Relay.Interval = 10 * time.Millisecond
	dist.CreateTopics(t, brokers, cfg.Group, cfg.ReservationsTopic, cfg.ReservationsDLQ)
	enqueueForTheRelay(t, pool, application.Destination)
	obstest.RelayWiring{
		Service: "reservations",
		Scope:   reflect.TypeFor[app.Config]().PkgPath(),
		Topic:   cfg.ReservationsTopic,
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
		"m-wiring-1", "com.company.reservations.reservation-confirmed.v1",
		"type.googleapis.com/company.reservations.event.v1.ReservationConfirmed", "reservation", "r-wiring-1",
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
			cfg.ReservationsTopic = "pool-reservations-" + suffix
			cfg.ReservationsDLQ = cfg.ReservationsTopic + "-dlq"
			cfg.Group = "pool-reservations-group-" + suffix
			if role == app.RoleConsumer {
				dist.CreateTopics(t, brokers, cfg.Group, cfg.OrdersTopic, cfg.OrdersDLQ)
			}
			reader := sdkmetric.NewManualReader()
			runtime := obstest.MeteredRuntime(t, "reservations", string(role), reader)

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

func TestTheConsumerJoiningRecordNamesItsChannelInTheVocabulary(t *testing.T) {
	brokers := strings.Split(tb.Env(t, "KAFKA_BROKERS"), ",")
	appkit.OpenPool(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	cfg := app.Defaults(app.RoleConsumer)
	cfg.DSN = pg.DSN(t, appkit.PoolOptions.Project)
	cfg.Brokers, cfg.KafkaInsecure = brokers, true
	cfg.OrdersTopic = "joining-orders-" + suffix
	cfg.OrdersDLQ = cfg.OrdersTopic + "-dlq"
	cfg.Group = "joining-reservations-group-" + suffix
	dist.CreateTopics(t, brokers, cfg.Group, cfg.OrdersTopic, cfg.OrdersDLQ)
	logs := &obstest.MemoryLogs{}
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "reservations", ServiceVersion: "dev", ServiceInstanceID: "reservations-consumer-1", Role: "consumer"},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	provider := otelboot.NewLoggerProvider(config, logs)
	config.LoggerProvider = provider
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("otelboot.Start() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stopped := make(chan error, 1)
	go func() { stopped <- app.RunWith(ctx, cfg, runtime) }()
	joining := awaitRecord(t, provider, logs, "consumer joining", stopped, 15*time.Second)
	cancel()
	if err := <-stopped; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("RunWith(consumer) = %v, want nil or the cancellation", err)
	}

	obstest.RequireTheVocabulary(t, "consumer joining", joining, map[string]string{
		string(semconv.MessagingSystemKey):            semconv.MessagingSystemKafka.Value.AsString(),
		string(semconv.MessagingDestinationNameKey):   cfg.OrdersTopic,
		string(semconv.MessagingConsumerGroupNameKey): cfg.Group,
		"dmpf.channel.name":                           app.OrdersChannel(cfg).Name,
	})
}

func TestADiscardedTypeDoesNotStallTheOrderPlacedBehindItInThePartition(t *testing.T) {
	brokers := strings.Split(tb.Env(t, "KAFKA_BROKERS"), ",")
	pool := appkit.OpenPool(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	cfg := app.Defaults(app.RoleConsumer)
	cfg.DSN = pg.DSN(t, appkit.PoolOptions.Project)
	cfg.Brokers, cfg.KafkaInsecure = brokers, true
	cfg.OrdersTopic = "discard-orders-" + suffix
	cfg.OrdersDLQ = cfg.OrdersTopic + "-dlq"
	cfg.Group = "discard-reservations-group-" + suffix
	cfg.OrdersSource = appkit.OrdersSource
	dist.CreateTopics(t, brokers, cfg.Group, cfg.OrdersTopic, cfg.OrdersDLQ)
	reader := sdkmetric.NewManualReader()
	runtime := obstest.MeteredRuntime(t, "reservations", string(app.RoleConsumer), reader)
	const order = "o-partition"
	publishToOrders(t, runtime, cfg, rawItemAdded(t, "evt-item-added", order), appkit.RawOrderPlaced(t, "evt-order-placed", order, 2))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stopped := make(chan error, 1)
	go func() { stopped <- app.RunWith(ctx, cfg, runtime) }()
	dist.WaitFor(t, "the reservation of the order-placed behind the discarded item-added", 30*time.Second, func() bool {
		select {
		case err := <-stopped:
			t.Fatalf("RunWith(consumer) = %v before the reservation", err)
		default:
		}
		return reservationsOf(t, pool, order) == 1
	})
	discarded := discardedOf(t, reader, "com.company.orders.item-added.v1")
	cancel()
	if err := <-stopped; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("RunWith(consumer) = %v, want nil or the cancellation", err)
	}

	if discarded != 1 {
		t.Fatalf("%s = %d, want the item-added discarded once by the kernel (KFK-03c)", metrics.ConsumerDiscardedTotal, discarded)
	}
}

func publishToOrders(t *testing.T, runtime *otelboot.Runtime, cfg app.Config, messages ...[]byte) {
	t.Helper()
	ch := app.OrdersChannel(cfg)
	catalog, err := channel.NewCatalog(ch)
	if err != nil {
		t.Fatalf("NewCatalog() = %v, want nil", err)
	}
	config, err := kafka.NewConfig(context.Background(), runtime, catalog, cfg.Brokers, cfg.KafkaInsecure, cfg.KafkaAuth)
	if err != nil {
		t.Fatalf("kafka.NewConfig() = %v, want nil", err)
	}
	publisher, err := kafka.NewPublisher(config, nil)
	if err != nil {
		t.Fatalf("kafka.NewPublisher() = %v, want nil", err)
	}
	defer publisher.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, message := range messages {
		if err := publisher.Publish(ctx, ch.Name, message); err != nil {
			t.Fatalf("Publish() = %v, want nil", err)
		}
	}
}

func reservationsOf(t *testing.T, pool *pgxpool.Pool, order string) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM reservations WHERE order_id = $1", order).Scan(&n); err != nil {
		t.Fatalf("reservations of %s: %v", order, err)
	}
	return n
}

func discardedOf(t *testing.T, reader *sdkmetric.ManualReader, eventType string) int64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v, want nil", err)
	}
	var total int64
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			if m.Name != metrics.ConsumerDiscardedTotal || !ok {
				continue
			}
			for _, point := range sum.DataPoints {
				if value, found := point.Attributes.Value(semconv.CloudEventsEventTypeKey); found && value.AsString() == eventType {
					total += point.Value
				}
			}
		}
	}
	return total
}

func awaitRecord(t *testing.T, provider *sdklog.LoggerProvider, logs *obstest.MemoryLogs, body string, stopped <-chan error, timeout time.Duration) map[string]string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if err := provider.ForceFlush(context.Background()); err != nil {
			t.Fatalf("ForceFlush() = %v, want nil", err)
		}
		if attributes, found := obstest.RecordAttributes(logs, body); found {
			return attributes
		}
		select {
		case err := <-stopped:
			t.Fatalf("RunWith() = %v before %q", err, body)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %q record within %v", body, timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
