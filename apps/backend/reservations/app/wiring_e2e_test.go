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
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
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
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
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
	createRelayTopics(t, brokers, cfg.Group, cfg.ReservationsTopic, cfg.ReservationsDLQ)
	enqueueForTheRelay(t, pool, application.Destination)
	spans := tracetest.NewInMemoryExporter()
	logs := &recordingExporter{}
	runtime := relayRuntime(t, spans, logs)

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- app.RunWith(ctx, cfg, runtime) }()
	send := awaitSend(t, runtime, spans, 15*time.Second)
	cancel()
	if err := <-stopped; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("RunWith(relay) = %v, want nil or the cancellation", err)
	}
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
	if scopes := scopesOf(logs, "relay draining"); len(scopes) != 1 || scopes[0] != reflect.TypeFor[app.Config]().PkgPath() {
		t.Fatalf("\"relay draining\" scopes = %v, want the import path of the app package that emits it (RF-A1)", scopes)
	}
	draining, _ := recordAttributes(logs, "relay draining")
	requireTheVocabulary(t, "relay draining", draining, map[string]string{
		string(semconv.MessagingSystemKey):          semconv.MessagingSystemKafka.Value.AsString(),
		string(semconv.MessagingDestinationNameKey): cfg.ReservationsTopic,
		"dmpf.channel.name":                         application.Destination,
	})

	if send.Name != "send "+cfg.ReservationsTopic {
		t.Fatalf("send span = %q, want %q (RF-B7)", send.Name, "send "+cfg.ReservationsTopic)
	}
	if got := destinationOf(send); got != cfg.ReservationsTopic {
		t.Fatalf("messaging.destination.name = %q, want the physical topic %q, not the channel %q (RF-B7)",
			got, cfg.ReservationsTopic, application.Destination)
	}
}

func relayRuntime(t *testing.T, spans *tracetest.InMemoryExporter, logs sdklog.Exporter) *otelboot.Runtime {
	t.Helper()
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "reservations", ServiceVersion: "dev", ServiceInstanceID: "reservations-relay-1", Role: "relay"},
		TraceExporter: spans,
		Sampling:      tracing.UniformRates(1),
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("otelboot.Start() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	return runtime
}

func createRelayTopics(t *testing.T, brokers []string, group string, topics ...string) {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatalf("kgo.NewClient() = %v, want nil", err)
	}
	t.Cleanup(client.Close)
	admin := kadm.NewClient(client)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := admin.CreateTopics(ctx, 1, 1, nil, topics...); err != nil {
		t.Fatalf("CreateTopics() = %v, want nil", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = admin.DeleteTopics(ctx, topics...)
		_, _ = admin.DeleteGroups(ctx, group)
	})
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

func awaitSend(t *testing.T, runtime *otelboot.Runtime, spans *tracetest.InMemoryExporter, timeout time.Duration) tracetest.SpanStub {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if err := runtime.ForceFlush(context.Background()); err != nil {
			t.Fatalf("ForceFlush() = %v, want nil", err)
		}
		names := []string{}
		for _, span := range spans.GetSpans() {
			if strings.HasPrefix(span.Name, "send") {
				return span
			}
			names = append(names, span.Name)
		}
		if time.Now().After(deadline) {
			t.Fatalf("no send span within %v, exported %v: the relay of runRelay does not trace through the runtime", timeout, names)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func destinationOf(span tracetest.SpanStub) string {
	for _, kv := range span.Attributes {
		if kv.Key == semconv.MessagingDestinationNameKey {
			return kv.Value.AsString()
		}
	}
	return ""
}

func scopesOf(logs *recordingExporter, body string) []string {
	logs.mu.Lock()
	defer logs.mu.Unlock()
	var scopes []string
	for _, record := range logs.records {
		if record.Body().AsString() == body {
			scopes = append(scopes, record.InstrumentationScope().Name)
		}
	}
	return scopes
}

func TestEveryRoleReportsTheConnectionsOfItsPoolThroughTheRuntime(t *testing.T) {
	brokers := strings.Split(tb.Env(t, "KAFKA_BROKERS"), ",")
	for _, role := range app.Roles {
		t.Run(string(role), func(t *testing.T) {
			appkit.OpenPool(t)
			suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
			cfg := app.Defaults(role)
			cfg.DSN = pg.DSN(t, appkit.PoolOptions.Project)
			cfg.GRPCAddr, cfg.GRPCInsecure = "127.0.0.1:0", true
			cfg.Brokers, cfg.KafkaInsecure = brokers, true
			cfg.OrdersTopic = "pool-orders-" + suffix
			cfg.OrdersDLQ = cfg.OrdersTopic + "-dlq"
			cfg.ReservationsTopic = "pool-reservations-" + suffix
			cfg.ReservationsDLQ = cfg.ReservationsTopic + "-dlq"
			cfg.Group = "pool-reservations-group-" + suffix
			if role == app.RoleConsumer {
				createRelayTopics(t, brokers, cfg.Group, cfg.OrdersTopic, cfg.OrdersDLQ)
			}
			reader := sdkmetric.NewManualReader()
			runtime := meteredRuntime(t, string(role), reader)

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			stopped := make(chan error, 1)
			go func() { stopped <- app.RunWith(ctx, cfg, runtime) }()
			awaitPooledConnection(t, reader, cfg.DSN, stopped, 15*time.Second)
			cancel()
			if err := <-stopped; err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("RunWith(%s) = %v, want nil or the cancellation", role, err)
			}
		})
	}
}

func meteredRuntime(t *testing.T, role string, reader sdkmetric.Reader) *otelboot.Runtime {
	t.Helper()
	runtime, err := otelboot.Start(context.Background(), otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "reservations", ServiceVersion: "dev", ServiceInstanceID: "reservations-" + role + "-1", Role: role},
		TraceExporter: tracetest.NewInMemoryExporter(),
		MetricReader:  reader,
	})
	if err != nil {
		t.Fatalf("otelboot.Start() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	return runtime
}

func awaitPooledConnection(t *testing.T, reader *sdkmetric.ManualReader, dsn string, stopped <-chan error, timeout time.Duration) {
	t.Helper()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig() = %v, want nil", err)
	}
	pool := config.ConnConfig.Host + ":" + strconv.Itoa(int(config.ConnConfig.Port)) + "/" + config.ConnConfig.Database
	deadline := time.Now().Add(timeout)
	for pooledConnections(t, reader, pool) < 1 {
		select {
		case err := <-stopped:
			t.Fatalf("RunWith() = %v before the runtime reported a connection of the pool %s", err, pool)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("db.client.connection.count reported no connection of the pool %s within %v: the role builds its pool without the meter provider of the runtime (RF-D6)", pool, timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func pooledConnections(t *testing.T, reader *sdkmetric.ManualReader, pool string) int64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v, want nil", err)
	}
	var connections int64
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			count, ok := m.Data.(metricdata.Sum[int64])
			if m.Name != "db.client.connection.count" || !ok {
				continue
			}
			for _, point := range count.DataPoints {
				if name, found := point.Attributes.Value(semconv.DBClientConnectionPoolNameKey); found && name.AsString() == pool {
					connections += point.Value
				}
			}
		}
	}
	return connections
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
	createRelayTopics(t, brokers, cfg.Group, cfg.OrdersTopic, cfg.OrdersDLQ)
	logs := &recordingExporter{}
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

	requireTheVocabulary(t, "consumer joining", joining, map[string]string{
		string(semconv.MessagingSystemKey):            semconv.MessagingSystemKafka.Value.AsString(),
		string(semconv.MessagingDestinationNameKey):   cfg.OrdersTopic,
		string(semconv.MessagingConsumerGroupNameKey): cfg.Group,
		"dmpf.channel.name":                           app.OrdersChannel(cfg).Name,
	})
}

func awaitRecord(t *testing.T, provider *sdklog.LoggerProvider, logs *recordingExporter, body string, stopped <-chan error, timeout time.Duration) map[string]string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if err := provider.ForceFlush(context.Background()); err != nil {
			t.Fatalf("ForceFlush() = %v, want nil", err)
		}
		if attributes, found := recordAttributes(logs, body); found {
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
