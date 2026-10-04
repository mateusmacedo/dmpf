package otelboot_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

func TestShutdownGracefullyClosesThePipelinesOfACancelledProcess(t *testing.T) {
	rt, _ := startedRuntime(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rt.ShutdownGracefully(ctx)

	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() after ShutdownGracefully() = %v, want nil: the shutdown is idempotent and already ran", err)
	}
}

func TestShutdownGracefullyReturnsWithinTheGraceWindow(t *testing.T) {
	rt, _ := startedRuntime(t, nil)

	start := time.Now()
	rt.ShutdownGracefully(context.Background())

	if elapsed := time.Since(start); elapsed > observability.ShutdownGrace {
		t.Fatalf("ShutdownGracefully() took %v, want at most the grace window of %v", elapsed, observability.ShutdownGrace)
	}
}

type refusingExporter struct {
	*tracetest.InMemoryExporter
}

func (refusingExporter) Shutdown(context.Context) error {
	return errors.New("traces export: dial tcp 10.0.0.7:4317: connect: connection refused")
}

func TestShutdownGracefullyLogsAFailureWithoutItsMessage(t *testing.T) {
	var out bytes.Buffer
	rt, _ := startedRuntime(t, func(config *otelboot.Config) {
		config.TraceExporter = refusingExporter{tracetest.NewInMemoryExporter()}
		config.Logger = slog.New(slog.NewJSONHandler(&out, nil))
	})

	rt.ShutdownGracefully(context.Background())

	if logged := out.String(); strings.Contains(logged, "10.0.0.7") || !strings.Contains(logged, `"error.type"`) {
		t.Fatalf("output = %q, want the shutdown failure reduced to error.type, without the address (RF-A3)", logged)
	}
}

func TestShutdownGracefullyExportsAFailureThroughTheLogPipelineItIsClosing(t *testing.T) {
	exporter := &recordingExporter{}
	rt, _ := startedRuntime(t, func(config *otelboot.Config) {
		config.TraceExporter = refusingExporter{tracetest.NewInMemoryExporter()}
		config.LoggerProvider = otelboot.NewLoggerProvider(*config, exporter)
	})

	rt.ShutdownGracefully(context.Background())

	for _, record := range exporter.records {
		if record.Body().AsString() != "telemetry shutdown" {
			continue
		}
		var typed, leaked bool
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			typed = typed || kv.Key == "error.type"
			leaked = leaked || strings.Contains(kv.Value.String(), "10.0.0.7")
			return true
		})
		if !typed || leaked {
			t.Fatalf("telemetry shutdown carries error.type = %t and the address = %t, want only error.type (RF-A3)", typed, leaked)
		}
		return
	}
	t.Fatalf("exported %d records without telemetry shutdown: the warn has to leave before the log pipeline closes (RF-A1)",
		len(exporter.records))
}

type closingExporter struct {
	recordingExporter
	closed atomic.Bool
}

func (e *closingExporter) Shutdown(context.Context) error {
	e.closed.Store(true)
	return nil
}

func (e *closingExporter) exported() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.records)
}

func goroutinesSettleAt(t *testing.T, baseline int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > baseline {
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutines remain over the %d before the runtime: a pipeline was left running", runtime.NumGoroutine(), baseline)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAStalledTraceExportLeavesTheLogPipelineItsShareOfTheWindow(t *testing.T) {
	before := runtime.NumGoroutine()
	traces, logs := newBlockingExporter(), &closingExporter{}
	rt, _ := startedRuntime(t, func(config *otelboot.Config) {
		config.TraceExporter = traces
		config.LoggerProvider = otelboot.NewLoggerProvider(*config, logs)
	})
	endSpan(rt, "orders.place")
	rt.LoggerFor("orders").Warn("queued before the shutdown")

	window, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()
	_ = rt.Shutdown(window)
	close(traces.release)

	if !logs.closed.Load() || logs.exported() != 1 {
		t.Fatalf("log pipeline closed = %t with %d records exported, want the queued record out and the pipeline closed while the trace export stalls (RF-A1)",
			logs.closed.Load(), logs.exported())
	}
	goroutinesSettleAt(t, before)
}
