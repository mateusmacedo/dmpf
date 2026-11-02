package app

import (
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
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
	if want := (kernelapp.Channel{Address: "dev.orders.events", Group: "reservations-dev"}); consumer.Channel.Address != want.Address || consumer.Channel.Group != want.Group {
		t.Fatalf("Channel = %+v, want %+v: the physical topic and the transport's group (RF-B9)", consumer.Channel, want)
	}
}

func TestTheReservationsConsumerAcceptsTheOrderPlacedAmongTheEventsOfTheOrdersChannel(t *testing.T) {
	cfg := Defaults(RoleConsumer)
	cfg.OrdersTopic, cfg.OrdersDLQ, cfg.Group = "dev.orders.events", "dev.orders.events.dlq", "reservations-dev"
	pool, err := pgxpool.New(t.Context(), "postgres://reservations@127.0.0.1:1/reservations")
	if err != nil {
		t.Fatalf("pgxpool.New() = %v, want a lazy pool", err)
	}
	t.Cleanup(pool.Close)

	consumer := NewReservationsConsumer(pool, cfg, OrdersChannel(cfg), false, nil, nil, nil)

	if want := []string{"com.company.orders.order-placed.v1"}; !slices.Equal(consumer.Accepts, want) {
		t.Fatalf("Accepts = %v, want %v: the handler unpacks only OrderPlaced (KFK-03c)", consumer.Accepts, want)
	}
	if want := []string{"com.company.orders.order-placed.v1", "com.company.orders.item-added.v1"}; !slices.Equal(consumer.Channel.Carries, want) {
		t.Fatalf("Carries = %v, want %v: every event the orders relay publishes (KFK-03b)", consumer.Channel.Carries, want)
	}
	if err := consumer.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil: the subscription stays within the channel", err)
	}
}

func TestTheReservationsConsumerCarriesTheWireTypesOfTheChannelItIsGiven(t *testing.T) {
	cfg := Defaults(RoleConsumer)
	cfg.OrdersTopic, cfg.OrdersDLQ, cfg.Group = "dev.orders.events", "dev.orders.events.dlq", "reservations-dev"
	pool, err := pgxpool.New(t.Context(), "postgres://reservations@127.0.0.1:1/reservations")
	if err != nil {
		t.Fatalf("pgxpool.New() = %v, want a lazy pool", err)
	}
	t.Cleanup(pool.Close)
	ch := OrdersChannel(cfg)
	ch.Events = append(slices.Clone(ch.Events), channel.Event{Type: "com.company.orders.order-cancelled", ContractRef: "company/orders/event/v1/order_cancelled.proto#OrderCancelled"})

	consumer := NewReservationsConsumer(pool, cfg, ch, false, nil, nil, nil)

	if want := channel.WireTypes(ch); !slices.Equal(consumer.Channel.Carries, want) {
		t.Fatalf("Carries = %v, want %v: the wire types of the channel the role consumes (KFK-03b)", consumer.Channel.Carries, want)
	}
}

func TestAConsumerGivenNoCarriedTypesCarriesThoseOfTheOrdersChannel(t *testing.T) {
	cfg := Defaults(RoleConsumer)
	pool, err := pgxpool.New(t.Context(), "postgres://reservations@127.0.0.1:1/reservations")
	if err != nil {
		t.Fatalf("pgxpool.New() = %v, want a lazy pool", err)
	}
	t.Cleanup(pool.Close)

	consumer := NewConsumer(pool, idclock.SystemClock{}, idclock.NewMessageIDs("reservations"), cfg.Wait, cfg.ConsumerTimeout, 3, OrdersBoundary(cfg, false), ConsumerTelemetry{})

	if want := channel.WireTypes(OrdersChannel(cfg)); !slices.Equal(consumer.Channel.Carries, want) {
		t.Fatalf("Carries = %v, want %v: the catalogue's channel when none is given", consumer.Channel.Carries, want)
	}
}
