package reservations

import (
	"context"
	"crypto/tls"
	"strconv"
	"time"

	ordersapp "github.com/mateusmacedo/dmpf/libs/backend/go/application/example/orders"
	reservationsapp "github.com/mateusmacedo/dmpf/libs/backend/go/application/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

const (
	orderPlacedEventType         = "com.company.orders.order-placed"
	orderPlacedContract          = "company/orders/event/v1/order_placed.proto#OrderPlaced"
	reservationConfirmedType     = "com.company.reservations.reservation-confirmed"
	reservationConfirmedContract = "company/reservations/event/v1/reservation_confirmed.proto#ReservationConfirmed"

	// inlineAttempts is the channel's retry ceiling (TRP-31); the adapter takes
	// the same value, because the two limits must agree (ADR-039).
	inlineAttempts = 2

	retentionByTime = 7 * 24 * time.Hour
)

// OrdersChannel is the channel the consumer reads, named after the destination
// the orders use case authors.
func OrdersChannel(cfg Config) channel.Channel {
	return kafkaChannel(ordersapp.Destination, cfg.OrdersTopic, cfg.OrdersDLQ, cfg.Group, orderPlacedEventType, orderPlacedContract)
}

// ReservationsChannel is the channel the relay publishes to, named after the
// destination the reservations use cases author, because the publisher resolves by it.
func ReservationsChannel(cfg Config) channel.Channel {
	return kafkaChannel(reservationsapp.Destination, cfg.ReservationsTopic, cfg.ReservationsDLQ, cfg.Group, reservationConfirmedType, reservationConfirmedContract)
}

func kafkaChannel(name, address, dlq, group, eventType, contract string) channel.Channel {
	return channel.Channel{
		Name:          name,
		Transport:     channel.Kafka,
		Address:       address,
		EventType:     eventType,
		ContractMajor: 1,
		ContractRef:   contract,
		Ordering:      channel.Ordering{Key: "partitionkey", Unit: channel.Partition},
		Redelivery: channel.KafkaWindow(channel.KafkaRetention{
			RetentionByTime: retentionByTime, RetentionBySize: -1, CleanupPolicy: "delete", InitialOffset: "earliest",
		}),
		Containment: dlq,
		Retry:       channel.Retry{Strategy: channel.InlineWithLimit, MaxAttempts: inlineAttempts},
		Group:       group,
		Partitions:  1,
		Partitioner: "default",
		KeyEncoding: "utf-8",
	}
}

// NewCatalog is the catalog of ASY-01 holding the one channel a role uses,
// validated so a missing item refuses the process at startup.
func NewCatalog(ch channel.Channel) (channel.Catalog, error) {
	catalog := channel.Catalog{ch.Name: ch}
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	return catalog, nil
}

// EventTypeOf is the wire type the channel carries: its event type at its
// contract major (PTB-03), which is what the consumer subscribes to.
func EventTypeOf(ch channel.Channel) string {
	return ch.EventType + ".v" + strconv.Itoa(ch.ContractMajor)
}

// NewKafkaConfig is the Kafka side of the process. TLS at 1.2 or above is the
// default; DMPF_KAFKA_INSECURE opts out for development and CI only.
func NewKafkaConfig(ctx context.Context, cfg Config, rt *otelboot.Runtime, catalog channel.Catalog) kafka.Config {
	kcfg := kafka.Config{
		Brokers:     cfg.Brokers,
		Catalog:     catalog,
		Sheet:       resilience.Defaults("kafka"),
		Service:     cfg.Service,
		Clock:       obsclock.System(),
		Tracer:      rt.Tracer(),
		Instruments: rt.Instruments(),
		Logger:      rt.Logger(),
	}
	if cfg.KafkaInsecure {
		rt.Logger().WarnContext(ctx, "kafka transport without TLS: DMPF_KAFKA_INSECURE is set (development and CI only)")
		kcfg.InsecureForDevelopmentOnly = true
	} else {
		kcfg.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return kcfg
}
