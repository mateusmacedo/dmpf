package otelboot_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func exportedThroughTheBoot(t *testing.T, role string, emit func(ctx context.Context, logger log.Logger)) []map[string]attribute.Value {
	t.Helper()
	exporter := &recordingExporter{}
	config := validConfig()
	config.Resource.Role = role
	provider := otelboot.NewLoggerProvider(config, exporter)

	emit(context.Background(), provider.Logger("test"))
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}
	return attributesOfRecords(exporter.records)
}

func attributesOfRecords(records []sdklog.Record) []map[string]attribute.Value {
	all := make([]map[string]attribute.Value, 0, len(records))
	for _, record := range records {
		attributes := map[string]attribute.Value{}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			attributes[string(kv.Key)] = kv.Value
			return true
		})
		all = append(all, attributes)
	}
	return all
}

func onlyRecord(t *testing.T, records []map[string]attribute.Value) map[string]attribute.Value {
	t.Helper()
	if len(records) != 1 {
		t.Fatalf("exported %d records, want 1", len(records))
	}
	return records[0]
}

func emitWith(attributes ...attribute.KeyValue) func(context.Context, log.Logger) {
	return func(ctx context.Context, logger log.Logger) {
		var record log.Record
		record.SetSeverity(log.SeverityInfo)
		record.SetBody(attribute.StringValue("http request"))
		record.AddAttributes(attributes...)
		logger.Emit(ctx, record)
	}
}

func TestAnAttributeOutsideTheAllowlistOfTheRoleIsDropped(t *testing.T) {
	record := onlyRecord(t, exportedThroughTheBoot(t, "api", emitWith(
		semconv.HTTPRoute("/orders/{id}"),
		attribute.String(tracing.KeyOutcomeCategory, "accepted"),
		attribute.String(redact.KeyErrorType, "timeout"),
		attribute.String("payload", `{"card":"4111"}`),
		attribute.String("service", "orders"),
		semconv.MessagingDestinationName("orders.placed"),
	)))

	for _, kept := range []string{string(semconv.HTTPRouteKey), tracing.KeyOutcomeCategory, redact.KeyErrorType} {
		if _, present := record[kept]; !present {
			t.Errorf("record lost %s, which the api role declares (LOG-05)", kept)
		}
	}
	for _, dropped := range []string{"payload", "service", string(semconv.MessagingDestinationNameKey)} {
		if value, present := record[dropped]; present {
			t.Errorf("record %s = %v, want absent: the api role does not declare it (LOG-05)", dropped, value.String())
		}
	}
}

func TestEachRoleDeclaresItsOwnProtocolKeys(t *testing.T) {
	for role, want := range map[string]struct{ kept, dropped attribute.Key }{
		"consumer": {kept: semconv.MessagingConsumerGroupNameKey, dropped: semconv.HTTPRouteKey},
		"relay":    {kept: semconv.MessagingDestinationNameKey, dropped: semconv.RPCMethodKey},
		"api":      {kept: semconv.RPCMethodKey, dropped: semconv.MessagingConsumerGroupNameKey},
	} {
		t.Run(role, func(t *testing.T) {
			record := onlyRecord(t, exportedThroughTheBoot(t, role, emitWith(
				want.kept.String("kept"), want.dropped.String("dropped"),
			)))
			if _, present := record[string(want.kept)]; !present {
				t.Errorf("the %s role lost %s", role, want.kept)
			}
			if _, present := record[string(want.dropped)]; present {
				t.Errorf("the %s role kept %s, which it does not declare", role, want.dropped)
			}
		})
	}
}

func TestTheReadinessAndPartitionKeysAreKeptByTheRolesThatEmitThem(t *testing.T) {
	readiness := []attribute.KeyValue{semconv.ServerAddress("0.0.0.0"), semconv.ServerPort(9090)}
	partition := []attribute.KeyValue{semconv.MessagingDestinationPartitionID("3"), semconv.MessagingKafkaOffset(42)}
	for role, want := range map[string]struct{ kept, dropped []attribute.KeyValue }{
		"api":      {kept: readiness, dropped: partition},
		"consumer": {kept: partition, dropped: readiness},
		"relay":    {kept: partition, dropped: readiness},
	} {
		t.Run(role, func(t *testing.T) {
			record := onlyRecord(t, exportedThroughTheBoot(t, role, emitWith(append(want.kept, want.dropped...)...)))
			for _, kv := range want.kept {
				if _, present := record[string(kv.Key)]; !present {
					t.Errorf("the %s role lost %s (RF-A3)", role, kv.Key)
				}
			}
			for _, kv := range want.dropped {
				if _, present := record[string(kv.Key)]; present {
					t.Errorf("the %s role kept %s, which it does not declare", role, kv.Key)
				}
			}
		})
	}
}

func TestAnUndeclaredRoleKeepsTheProtocolKeysOfEveryRoleAndNothingElse(t *testing.T) {
	record := onlyRecord(t, exportedThroughTheBoot(t, "", emitWith(
		semconv.HTTPRoute("/orders/{id}"),
		semconv.MessagingDestinationName("orders.placed"),
		attribute.String("topic", "orders.placed"),
	)))

	for _, kept := range []attribute.Key{semconv.HTTPRouteKey, semconv.MessagingDestinationNameKey} {
		if _, present := record[string(kept)]; !present {
			t.Errorf("record lost %s", kept)
		}
	}
	if _, present := record["topic"]; present {
		t.Error("record kept topic, which no role declares")
	}
}

func TestASecretKeyLeavesRedacted(t *testing.T) {
	record := onlyRecord(t, exportedThroughTheBoot(t, "api", emitWith(
		attribute.String("dmpf.config.database_password", "hunter2"),
		attribute.Map("dmpf.config", attribute.String("oidc_client_secret", "s3cr3t"), attribute.String("database_host", "db")),
		attribute.String(tracing.KeyRequestID, "req-1"),
	)))

	if got := record["dmpf.config.database_password"].AsString(); got != redact.Placeholder {
		t.Errorf("dmpf.config.database_password = %q, want %q: a secret is never logged (LOG-09), and its key stays (LOG-07)", got, redact.Placeholder)
	}
	nested := map[string]string{}
	for _, kv := range record["dmpf.config"].AsMap() {
		nested[string(kv.Key)] = kv.Value.AsString()
	}
	if nested["oidc_client_secret"] != redact.Placeholder {
		t.Errorf("dmpf.config.oidc_client_secret = %q, want %q inside a group too (LOG-09)", nested["oidc_client_secret"], redact.Placeholder)
	}
	if nested["database_host"] != "db" {
		t.Errorf("dmpf.config.database_host = %q, want db: only the secret is redacted", nested["database_host"])
	}
	if got := record[tracing.KeyRequestID].AsString(); got != "req-1" {
		t.Errorf("%s = %q, want req-1", tracing.KeyRequestID, got)
	}
}

func TestEveryRecordRedactsTheSameSecretKeyAndKeepsTheSameOrdinaryOne(t *testing.T) {
	emit := emitWith(attribute.String("dmpf.config.database_password", "hunter2"), attribute.String("dmpf.config.database_host", "db"))
	records := exportedThroughTheBoot(t, "api", func(ctx context.Context, logger log.Logger) {
		emit(ctx, logger)
		emit(ctx, logger)
	})

	if len(records) != 2 {
		t.Fatalf("exported %d records, want 2", len(records))
	}
	for i, record := range records {
		if got := record["dmpf.config.database_password"].AsString(); got != redact.Placeholder {
			t.Errorf("record %d: dmpf.config.database_password = %q, want %q on every record (LOG-09)", i, got, redact.Placeholder)
		}
		if got := record["dmpf.config.database_host"].AsString(); got != "db" {
			t.Errorf("record %d: dmpf.config.database_host = %q, want db on every record", i, got)
		}
	}
}

func TestAnAttachedErrorLeavesNoExceptionAttribute(t *testing.T) {
	cause := errors.New("dial tcp 10.0.0.7:5432: password authentication failed for user orders")

	records := exportedThroughTheBoot(t, "api", func(ctx context.Context, logger log.Logger) {
		var record log.Record
		record.SetSeverity(log.SeverityError)
		record.SetBody(attribute.StringValue("not ready"))
		record.SetErr(cause)
		logger.Emit(ctx, record)
	})

	exporter := &recordingExporter{}
	config := validConfig()
	provider := otelboot.NewLoggerProvider(config, exporter)
	slog.New(otelslog.NewHandler("test", otelslog.WithLoggerProvider(provider))).
		ErrorContext(context.Background(), "not ready", slog.Any("err", cause))
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	for path, record := range map[string]map[string]attribute.Value{
		"Record.SetErr": onlyRecord(t, records),
		"slog.Any(err)": onlyRecord(t, attributesOfRecords(exporter.records)),
	} {
		for _, key := range []attribute.Key{semconv.ExceptionMessageKey, semconv.ExceptionTypeKey, semconv.ExceptionStacktraceKey} {
			if value, present := record[string(key)]; present {
				t.Errorf("%s: record %s = %q, want absent: the message carries what redaction keeps out (DAT-23)", path, key, value.String())
			}
		}
		if value, present := record["err"]; present {
			t.Errorf("%s: record err = %q, want absent", path, value.String())
		}
	}
}

func insideAnUnsampledTrace() context.Context {
	return trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1}, SpanID: trace.SpanID{1},
	}))
}

func sampledThroughTheProcessor(t *testing.T, policy otelboot.LogPolicy, ctx context.Context, severity log.Severity) int {
	t.Helper()
	exporter := &recordingExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(
		otelboot.NewLogProcessor(sdklog.NewSimpleProcessor(exporter), policy),
	))
	var record log.Record
	record.SetSeverity(severity)
	record.SetBody(attribute.StringValue("order read"))
	provider.Logger("test").Emit(ctx, record)
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}
	return len(exporter.records)
}

func TestADebugRecordIsSampledByTheClassOfTheProcess(t *testing.T) {
	for draw, want := range map[float64]int{0.5: 0, 0.005: 1} {
		policy := otelboot.LogPolicy{Class: tracing.ClassRead, Rates: tracing.DefaultRates(), Rand: func() float64 { return draw }}
		if got := sampledThroughTheProcessor(t, policy, insideAnUnsampledTrace(), log.SeverityDebug); got != want {
			t.Errorf("a draw of %v against the read rate of 0.01 exported %d records, want %d (LOG-12)", draw, got, want)
		}
	}
}

func TestEveryRecordBelowErrorIsSampledByTheClass(t *testing.T) {
	policy := otelboot.LogPolicy{Class: tracing.ClassRead, Rates: tracing.DefaultRates(), Rand: func() float64 { return 0.5 }}
	for _, severity := range []log.Severity{log.SeverityInfo, log.SeverityWarn, log.SeverityWarn4} {
		if got := sampledThroughTheProcessor(t, policy, insideAnUnsampledTrace(), severity); got != 0 {
			t.Errorf("a %v record exported %d records against the read rate of 0.01, want 0: only an error escapes the sampling (LOG-12)", severity, got)
		}
	}
}

func TestAnErrorRecordIsNeverSampledAway(t *testing.T) {
	policy := otelboot.LogPolicy{Class: tracing.ClassRead, Rates: tracing.DefaultRates(), Rand: func() float64 { return 0.99 }}
	if got := sampledThroughTheProcessor(t, policy, insideAnUnsampledTrace(), log.SeverityError); got != 1 {
		t.Fatalf("exported %d records, want 1: an error is never sampled (LOG-12)", got)
	}
}

func TestARecordOfASampledTraceIsKeptWhateverTheDraw(t *testing.T) {
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })
	ctx, span := tracer.Tracer("test").Start(context.Background(), "orders.find")
	defer span.End()

	policy := otelboot.LogPolicy{Class: tracing.ClassRead, Rates: tracing.DefaultRates(), Rand: func() float64 { return 0.99 }}
	if got := sampledThroughTheProcessor(t, policy, ctx, log.SeverityDebug); got != 1 {
		t.Fatalf("exported %d records, want 1: a sampled trace keeps the lines that explain it (LOG-12)", got)
	}
}

func TestAnAuditRecordIsNeverSampledAway(t *testing.T) {
	policy := otelboot.LogPolicy{Class: tracing.ClassRead, Rates: tracing.Rates{tracing.ClassRead: 0}, Rand: func() float64 { return 0 }}
	exporter := &recordingExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(
		otelboot.NewLogProcessor(sdklog.NewSimpleProcessor(exporter), policy),
	))
	for _, eventName := range []string{"", audit.EventName} {
		var record log.Record
		record.SetSeverity(log.SeverityInfo)
		record.SetEventName(eventName)
		record.SetBody(attribute.StringValue("order read"))
		provider.Logger("test").Emit(insideAnUnsampledTrace(), record)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	if len(exporter.records) != 1 || exporter.records[0].EventName() != audit.EventName {
		names := make([]string, len(exporter.records))
		for i, record := range exporter.records {
			names[i] = record.EventName()
		}
		t.Fatalf("exported event names %q against a read rate of 0, want only %q: the plain Info record is sampled away and no audit record is (LOG-13)", names, audit.EventName)
	}
}

func TestTheLoggerProviderSamplesByTheClassAndTheDrawOfItsConfiguration(t *testing.T) {
	exporter := &recordingExporter{}
	config := validConfig()
	config.Class = tracing.ClassRead
	config.Rand = func() float64 { return 0.5 }
	provider := otelboot.NewLoggerProvider(config, exporter)

	for ctx, body := range map[context.Context]string{
		insideAnUnsampledTrace(): "order read",
		context.Background():     "process configured",
	} {
		var record log.Record
		record.SetSeverity(log.SeverityInfo)
		record.SetBody(attribute.StringValue(body))
		provider.Logger("test").Emit(ctx, record)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	if len(exporter.records) != 1 || exporter.records[0].Body().AsString() != "process configured" {
		bodies := make([]string, len(exporter.records))
		for i, record := range exporter.records {
			bodies[i] = record.Body().AsString()
		}
		t.Fatalf("exported %q, want only the record outside a trace: a draw of 0.5 is above the read rate of 0.01 (LOG-12)", bodies)
	}
}
