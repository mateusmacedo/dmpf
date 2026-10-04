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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
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
	cfg.BookingsTopic = "wiring-bookings-" + suffix
	cfg.BookingsDLQ = cfg.BookingsTopic + "-dlq"
	cfg.Group = "wiring-bookings-group-" + suffix
	cfg.Relay.Interval = 10 * time.Millisecond
	createRelayTopics(t, brokers, cfg.Group, cfg.BookingsTopic, cfg.BookingsDLQ)
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
		string(semconv.MessagingDestinationNameKey): cfg.BookingsTopic,
		"dmpf.channel.name":                         application.Destination,
	})

	if send.Name != "send "+cfg.BookingsTopic {
		t.Fatalf("send span = %q, want %q (RF-B7)", send.Name, "send "+cfg.BookingsTopic)
	}
	if got := destinationOf(send); got != cfg.BookingsTopic {
		t.Fatalf("messaging.destination.name = %q, want the physical topic %q, not the channel %q (RF-B7)",
			got, cfg.BookingsTopic, application.Destination)
	}
}

func relayRuntime(t *testing.T, spans *tracetest.InMemoryExporter, logs sdklog.Exporter) *otelboot.Runtime {
	t.Helper()
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "bookings", ServiceVersion: "dev", ServiceInstanceID: "bookings-relay-1", Role: "relay"},
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
		"m-wiring-1", "com.company.bookings.booking-reserved.v1",
		"type.googleapis.com/company.bookings.event.v1.BookingReserved", "booking", "b-wiring-1",
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

func recordAttributes(logs *recordingExporter, body string) (map[string]string, bool) {
	logs.mu.Lock()
	defer logs.mu.Unlock()
	for _, record := range logs.records {
		if record.Body().AsString() != body {
			continue
		}
		attributes := map[string]string{}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			attributes[string(kv.Key)] = kv.Value.String()
			return true
		})
		return attributes, true
	}
	return nil, false
}

func requireTheVocabulary(t *testing.T, body string, attributes map[string]string, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if attributes[key] != value {
			t.Errorf("%q %s = %q, want %q surviving the processor (RF-A3): %v", body, key, attributes[key], value, attributes)
		}
	}
	for _, key := range []string{tracing.KeyCorrelationID, tracing.KeyRequestID, tracing.KeyTenantID} {
		if value, present := attributes[key]; present {
			t.Errorf("%q carries %s = %q, want no key of an execution on a start record (RF-A4)", body, key, value)
		}
	}
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
			cfg.BookingsTopic = "pool-bookings-" + suffix
			cfg.BookingsDLQ = cfg.BookingsTopic + "-dlq"
			cfg.Group = "pool-bookings-group-" + suffix
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
		Resource:      otelboot.Resource{ServiceName: "bookings", ServiceVersion: "dev", ServiceInstanceID: "bookings-" + role + "-1", Role: role},
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
