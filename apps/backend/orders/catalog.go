package orders

import (
	"context"
	"crypto/tls"
	"time"

	ordersapp "github.com/mateusmacedo/dmpf/libs/backend/go/application/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

const (
	orderPlacedEventType = "com.company.orders.order-placed"
	orderPlacedContract  = "company/orders/event/v1/order_placed.proto#OrderPlaced"

	// inlineAttempts is the channel's retry ceiling (TRP-31); a consumer of the
	// channel takes the same value, because the two limits must agree (ADR-039).
	inlineAttempts = 2

	retentionByTime = 7 * 24 * time.Hour
)

// OrdersChannel is the channel the relay publishes to, named after the
// destination the use case authors, because the publisher resolves by it.
func OrdersChannel(cfg Config) channel.Channel {
	return channel.Channel{
		Name:          ordersapp.Destination,
		Transport:     channel.Kafka,
		Address:       cfg.OrdersTopic,
		EventType:     orderPlacedEventType,
		ContractMajor: 1,
		ContractRef:   orderPlacedContract,
		Ordering:      channel.Ordering{Key: "partitionkey", Unit: channel.Partition},
		Redelivery: channel.KafkaWindow(channel.KafkaRetention{
			RetentionByTime: retentionByTime, RetentionBySize: -1, CleanupPolicy: "delete", InitialOffset: "earliest",
		}),
		Containment: cfg.OrdersDLQ,
		Retry:       channel.Retry{Strategy: channel.InlineWithLimit, MaxAttempts: inlineAttempts},
		Group:       cfg.Group,
		Partitions:  1,
		Partitioner: "default",
		KeyEncoding: "utf-8",
	}
}

// NewCatalog is the catalog of ASY-01 with the one channel this context drains.
func NewCatalog(cfg Config) (channel.Catalog, error) {
	orders := OrdersChannel(cfg)
	catalog := channel.Catalog{orders.Name: orders}
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	return catalog, nil
}

// NewKafkaConfig is the Kafka side of the relay. TLS at 1.2 or above is the
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
