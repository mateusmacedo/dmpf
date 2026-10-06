package otelboot_test

import (
	"context"
	"log/slog"
	"slices"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	emitterScope = "github.com/mateusmacedo/dmpf/apps/backend/orders/app/rpc"
	libraryScope = "github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
)

func startWithLogs(t *testing.T, level slog.Leveler) (*otelboot.Runtime, *recordingExporter) {
	t.Helper()
	exporter := &recordingExporter{}
	config := validConfig()
	config.TraceExporter = tracetest.NewInMemoryExporter()
	config.LoggerProvider = otelboot.NewLoggerProvider(config, exporter)
	config.LogLevel = level
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	return runtime, exporter
}

func shutdownAndCollect(t *testing.T, runtime *otelboot.Runtime, exporter *recordingExporter) []sdklog.Record {
	t.Helper()
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
	return exporter.records
}

func onlyExported(t *testing.T, records []sdklog.Record) sdklog.Record {
	t.Helper()
	if len(records) != 1 {
		t.Fatalf("exported %d records, want 1", len(records))
	}
	return records[0]
}

func TestTheRuntimeLoggerWritesTheLogsDataModelWithoutPlatformAttributes(t *testing.T) {
	runtime, exporter := startWithLogs(t, slog.LevelInfo)

	runtime.LoggerFor(emitterScope).InfoContext(context.Background(), "order placed")

	record := onlyExported(t, shutdownAndCollect(t, runtime, exporter))
	if record.Body().AsString() != "order placed" || record.Severity() != log.SeverityInfo || record.Timestamp().IsZero() {
		t.Fatalf("body = %q, severity = %v, timestamp = %v; want the message as Body, info and a Timestamp",
			record.Body().AsString(), record.Severity(), record.Timestamp())
	}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		switch string(kv.Key) {
		case "time", "level", "msg", "service", "version", "instance", "trace_id", "span_id":
			t.Errorf("record carries %s as an attribute; the Logs Data Model holds it (RF-A2)", kv.Key)
		}
		return true
	})
	service, _ := record.Resource().Set().Value(semconv.ServiceNameKey)
	if service.AsString() != "orders" {
		t.Fatalf("service.name = %q on the Resource, want orders", service.AsString())
	}
}

func TestTheScopeOfARecordIsThePackageThatEmitsIt(t *testing.T) {
	runtime, exporter := startWithLogs(t, slog.LevelInfo)

	runtime.LoggerFor(emitterScope).InfoContext(context.Background(), "order placed")
	logging.NewLogger(runtime.LoggerProvider(), libraryScope).InfoContext(context.Background(), "published")

	records := shutdownAndCollect(t, runtime, exporter)
	if len(records) != 2 {
		t.Fatalf("exported %d records, want 2", len(records))
	}
	if got := records[0].InstrumentationScope().Name; got != emitterScope {
		t.Errorf("scope = %q, want the import path of the emitter %q", got, emitterScope)
	}
	if got := records[1].InstrumentationScope().Name; got != libraryScope {
		t.Errorf("scope of a library logger = %q, want its own import path %q, never the service name", got, libraryScope)
	}
}

func TestALibraryLoggerOverTheRuntimeProviderFollowsTheLogLevel(t *testing.T) {
	for level, want := range map[slog.Level][]string{
		slog.LevelInfo:  {"published"},
		slog.LevelDebug: {"detail", "published"},
	} {
		t.Run(level.String(), func(t *testing.T) {
			runtime, exporter := startWithLogs(t, level)
			logger := logging.NewLogger(runtime.LoggerProvider(), libraryScope)

			logger.DebugContext(context.Background(), "detail")
			logger.InfoContext(context.Background(), "published")

			var bodies []string
			for _, record := range shutdownAndCollect(t, runtime, exporter) {
				bodies = append(bodies, record.Body().AsString())
			}
			if !slices.Equal(bodies, want) {
				t.Fatalf("exported %q under LOG_LEVEL=%s, want %q", bodies, level, want)
			}
		})
	}
}

func TestARecordOfTheLogsAPIBelowTheLogLevelIsDropped(t *testing.T) {
	runtime, exporter := startWithLogs(t, slog.LevelInfo)
	var record log.Record
	record.SetSeverity(log.SeverityDebug)
	record.SetBody(attribute.StringValue("detail"))

	runtime.LoggerProvider().Logger(libraryScope).Emit(context.Background(), record)

	if records := shutdownAndCollect(t, runtime, exporter); len(records) != 0 {
		t.Fatalf("exported %d records, want the debug record dropped under LOG_LEVEL=info", len(records))
	}
}

func TestTheAuditTrailIsNeverDroppedByTheLogLevel(t *testing.T) {
	runtime, exporter := startWithLogs(t, slog.LevelError)
	param := log.EnabledParameters{Severity: log.SeverityInfo, EventName: audit.EventName}
	if !runtime.LoggerProvider().Logger(audit.Scope).Enabled(context.Background(), param) {
		t.Error("Enabled(audit) = false under LOG_LEVEL=error, want the trail always enabled (LOG-13)")
	}

	event := audit.Event{Subject: "user-1", Object: "order-9", Action: "read", Outcome: "allowed"}
	if err := audit.NewLogSink(runtime.LoggerProvider()).Emit(context.Background(), event); err != nil {
		t.Fatalf("Emit() = %v, want nil", err)
	}

	record := onlyExported(t, shutdownAndCollect(t, runtime, exporter))
	if record.EventName() != audit.EventName {
		t.Fatalf("event name = %q, want %q", record.EventName(), audit.EventName)
	}
}

func TestARecordCarriesTheTraceOfTheActiveSpan(t *testing.T) {
	runtime, exporter := startWithLogs(t, slog.LevelInfo)
	ctx, span := runtime.Tracer().Start(context.Background(), "orders.place")

	runtime.LoggerFor(emitterScope).InfoContext(ctx, "order placed")
	span.End()

	record := onlyExported(t, shutdownAndCollect(t, runtime, exporter))
	active := span.SpanContext()
	if record.TraceID() != active.TraceID() || record.SpanID() != active.SpanID() || record.TraceFlags() != active.TraceFlags() {
		t.Fatalf("record trace = %v/%v/%v, want the active span %v/%v/%v",
			record.TraceID(), record.SpanID(), record.TraceFlags(), active.TraceID(), active.SpanID(), active.TraceFlags())
	}
}

func TestTheLogLevelDropsWhatIsBelowIt(t *testing.T) {
	runtime, exporter := startWithLogs(t, slog.LevelInfo)
	logger := runtime.LoggerFor(emitterScope)

	logger.DebugContext(context.Background(), "detail")
	logger.InfoContext(context.Background(), "order placed")

	record := onlyExported(t, shutdownAndCollect(t, runtime, exporter))
	if record.Body().AsString() != "order placed" {
		t.Fatalf("exported %q, want only the info record under LOG_LEVEL=info", record.Body().AsString())
	}
}

func TestTheSeverityNumberIsTheOneOfTheOutcomeTable(t *testing.T) {
	runtime, exporter := startWithLogs(t, slog.LevelDebug)
	logger := runtime.LoggerFor(emitterScope)

	logger.Log(context.Background(), logging.Severity(logging.Server, ports.OutcomeDenied), "denied")
	logger.Log(context.Background(), logging.Severity(logging.Client, ports.OutcomeAccepted), "called")

	records := shutdownAndCollect(t, runtime, exporter)
	if len(records) != 2 {
		t.Fatalf("exported %d records, want 2", len(records))
	}
	if records[0].Severity() != log.SeverityWarn || records[1].Severity() != log.SeverityDebug {
		t.Fatalf("severities = %v, %v; want warn for a denied call and debug for an accepted client call",
			records[0].Severity(), records[1].Severity())
	}
}

func TestTheLogLevelSurvivesAttributesAndGroups(t *testing.T) {
	runtime, exporter := startWithLogs(t, slog.LevelInfo)
	logger := runtime.LoggerFor(emitterScope)

	logger.With("order", "o-1").DebugContext(context.Background(), "detail")
	logger.WithGroup("order").DebugContext(context.Background(), "detail")
	logger.With("order", "o-1").InfoContext(context.Background(), "order placed")

	record := onlyExported(t, shutdownAndCollect(t, runtime, exporter))
	if record.Body().AsString() != "order placed" {
		t.Fatalf("exported %q, want the debug records dropped after With and WithGroup", record.Body().AsString())
	}
}
