package otelboot_test

import (
	"context"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

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
