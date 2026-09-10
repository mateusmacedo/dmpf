package dmpfreference

import (
	"context"
	"crypto/tls"
	"strconv"
	"time"

	ordersapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/orders"
	reservationsapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/reservations"
	obsclock "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/otelboot"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	dmpfkafka "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-kafka"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
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

// OrdersChannel is the channel the api writes and the consumer reads: named
// after the destination the use case authors (the publisher resolves by it),
// with the physical address, group and containment topic from the environment.
func OrdersChannel(cfg Config) channel.Channel {
	return kafkaChannel(ordersapp.Destination, cfg.Topic, cfg.DLQ, cfg.Group, orderPlacedEventType, orderPlacedContract)
}

// ReservationsChannel is where the consumer's own outbox publishes
// ReservationConfirmed. Nothing in this process consumes it, but the relay
// drains every destination the two use cases author, and a destination
// without a channel would leave those rows failing (ErrUnknownChannel). The
// group is a declaration KFK-07 demands of every Kafka channel.
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

// NewCatalog is the catalogue of ASY-01 with the two channels of the example,
// validated so a missing item refuses the process at startup.
func NewCatalog(cfg Config) (channel.Catalog, error) {
	orders, reservations := OrdersChannel(cfg), ReservationsChannel(cfg)
	catalog := channel.Catalog{orders.Name: orders, reservations.Name: reservations}
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
// default; DMPF_KAFKA_INSECURE opts out for development and CI only, and the
// opt-out is logged so an audit finds it.
func NewKafkaConfig(ctx context.Context, cfg Config, rt *otelboot.Runtime, catalog channel.Catalog) dmpfkafka.Config {
	kcfg := dmpfkafka.Config{
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
