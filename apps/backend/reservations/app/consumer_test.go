package app

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
)

type identifiedMeters struct{ metric.MeterProvider }

func TestTheReservationsConsumerProcessesOnTheOrdersTopicAndGroup(t *testing.T) {
	cfg := Defaults(RoleConsumer)
	cfg.OrdersTopic, cfg.OrdersDLQ, cfg.Group = "dev.orders.events", "dev.orders.events.dlq", "reservations-dev"
	provider := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(t.Context()) })
	tracer := provider.Tracer("consumer-test")
	meters := &identifiedMeters{MeterProvider: metricnoop.NewMeterProvider()}
	pool, err := pgxpool.New(t.Context(), "postgres://reservations@127.0.0.1:1/reservations")
	if err != nil {
		t.Fatalf("pgxpool.New() = %v, want a lazy pool", err)
	}
	t.Cleanup(pool.Close)

	logs := sdklog.NewLoggerProvider()

	consumer := NewReservationsConsumer(pool, cfg, OrdersChannel(cfg), false, tracer, meters, logs)

	if consumer.Tracer != tracer {
		t.Fatalf("Tracer = %v, want the runtime's tracer", consumer.Tracer)
	}
	if consumer.MeterProvider != meters {
		t.Fatalf("MeterProvider = %v, want the runtime's", consumer.MeterProvider)
	}
	if consumer.LoggerProvider != logs {
		t.Fatalf("LoggerProvider = %v, want the runtime's: without it \"message consumed\" is discarded", consumer.LoggerProvider)
	}
	if consumer.System != "kafka" {
		t.Fatalf("System = %q, want kafka", consumer.System)
	}
	if want := (kernelapp.Channel{Address: "dev.orders.events", Group: "reservations-dev"}); consumer.Channel != want {
		t.Fatalf("Channel = %+v, want %+v: the physical topic and the transport's group (RF-B9)", consumer.Channel, want)
	}
}
