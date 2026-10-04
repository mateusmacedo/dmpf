//go:build integration

package app_test

import (
	"bytes"
	"context"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/appkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestTheQuarantineOfTheWiringLogsThroughTheRuntimeAndNotTheDefaultLogger(t *testing.T) {
	ctx := context.Background()
	logs := &recordingExporter{}
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "reservations", ServiceVersion: "dev", ServiceInstanceID: "reservations-consumer-1"},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)
	runtime, err := otelboot.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	var defaults lockedBuffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&defaults, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	pool := appkit.OpenPool(t)
	cfg := app.Defaults(app.RoleConsumer)
	scope := reflect.TypeFor[postgres.Tx]().PkgPath()

	consumer := app.NewReservationsConsumer(pool, cfg, app.OrdersChannel(cfg), false, runtime.Tracer(), runtime.MeterProvider(), runtime.LoggerProvider())
	err = consumer.Containment.Quarantine(ctx, ports.Contained{Consumer: app.ConsumerName, MessageID: "evt-contained",
		Reason: ports.ReasonTerminalFailure, Envelope: []byte("raw envelope"), At: 1})
	if err != nil {
		t.Fatalf("Quarantine() = %v, want nil", err)
	}
	if err := runtime.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}

	var contained []string
	logs.mu.Lock()
	for _, record := range logs.records {
		if record.Body().AsString() == "message contained" {
			contained = append(contained, record.InstrumentationScope().Name)
		}
	}
	logs.mu.Unlock()
	if len(contained) != 1 || contained[0] != scope {
		t.Fatalf("runtime \"message contained\" scopes = %v, want exactly [%s]", contained, scope)
	}
	if out := defaults.String(); strings.Contains(out, "message contained") {
		t.Fatalf("slog.Default received %q, want nothing: the quarantine logs through the runtime", out)
	}
}
