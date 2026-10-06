package otelboot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

type recordingExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (e *recordingExporter) Shutdown(context.Context) error   { return nil }
func (e *recordingExporter) ForceFlush(context.Context) error { return nil }

func TestTheRuntimeShutdownExportsThePendingLogsUnderTheServiceResource(t *testing.T) {
	exporter := &recordingExporter{}
	config := validConfig()
	config.TraceExporter = tracetest.NewInMemoryExporter()
	config.LoggerProvider = otelboot.NewLoggerProvider(config, exporter)

	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	var record log.Record
	record.SetBody(attribute.StringValue("placed"))
	config.LoggerProvider.Logger("test").Emit(context.Background(), record)

	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
	if len(exporter.records) != 1 {
		t.Fatalf("exported %d records, want 1", len(exporter.records))
	}
	service, _ := exporter.records[0].Resource().Set().Value(semconv.ServiceNameKey)
	if service.AsString() != "orders" {
		t.Fatalf("service.name = %q, want orders", service.AsString())
	}
}

func TestTheAuditTrailReachesTheLogExporterOfTheRuntime(t *testing.T) {
	exporter := &recordingExporter{}
	config := validConfig()
	config.TraceExporter = tracetest.NewInMemoryExporter()
	config.LoggerProvider = otelboot.NewLoggerProvider(config, exporter)

	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	event := audit.Event{Subject: "user-1", Object: "order-9", Action: "read", Outcome: "allowed"}
	if err := audit.NewLogSink(runtime.LoggerProvider()).Emit(context.Background(), event); err != nil {
		t.Fatalf("Emit() = %v, want nil", err)
	}

	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
	if len(exporter.records) != 1 {
		t.Fatalf("exported %d records, want 1", len(exporter.records))
	}
	if name := exporter.records[0].EventName(); name != audit.EventName {
		t.Fatalf("event name = %q, want %q", name, audit.EventName)
	}
}

func TestTheAuditTrailReachesTheLoggerOfTheRuntimeWithoutALogPipeline(t *testing.T) {
	routes := map[string]func(*otelboot.Config){
		"in memory": func(config *otelboot.Config) { config.TraceExporter = tracetest.NewInMemoryExporter() },
		"disabled":  func(config *otelboot.Config) { config.Disabled = true },
	}
	for name, route := range routes {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			config := validConfig()
			route(&config)
			config.Logger = slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: slog.LevelWarn}))

			runtime, err := otelboot.Start(context.Background(), config)
			if err != nil {
				t.Fatalf("Start() = %v, want nil", err)
			}
			t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
			event := audit.Event{Subject: "user-1", Object: "order-9", Action: "read", Outcome: "allowed"}
			if err := audit.NewLogSink(runtime.LoggerProvider()).Emit(context.Background(), event); err != nil {
				t.Fatalf("Emit() = %v, want nil", err)
			}

			var line map[string]any
			if err := json.Unmarshal(out.Bytes(), &line); err != nil {
				t.Fatalf("the logger wrote %q, want one JSON record: %v", out.String(), err)
			}
			want := map[string]any{
				"msg":                            "audit",
				string(semconv.OTelEventNameKey): audit.EventName,
				audit.KeySubject:                 "user-1",
				audit.KeyAction:                  "read",
			}
			for key, value := range want {
				if line[key] != value {
					t.Errorf("%s = %v, want %v (record %s)", key, line[key], value, out.String())
				}
			}
		})
	}
}
