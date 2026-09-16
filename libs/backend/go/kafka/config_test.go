package kafka_test

import (
	"crypto/tls"
	"errors"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

var start = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func ordersChannel() channel.Channel {
	return channel.Channel{
		Name:          "orders",
		Transport:     channel.Kafka,
		Address:       "sales.order.placed.v1",
		EventType:     "sales.order.placed",
		ContractMajor: 1,
		ContractRef:   "sales/order/v1/events.proto#OrderPlaced",
		Ordering:      channel.Ordering{Key: "partitionkey", Unit: channel.Partition},
		Redelivery: channel.KafkaWindow(channel.KafkaRetention{
			RetentionByTime: 7 * 24 * time.Hour, RetentionBySize: -1, CleanupPolicy: "delete", InitialOffset: "earliest",
		}),
		Containment: "sales.order.placed.v1.dlq",
		Retry:       channel.Retry{Strategy: channel.InlineWithLimit, MaxAttempts: 3},
		Group:       "billing-consumer",
		Partitions:  3,
		Partitioner: "default",
		KeyEncoding: "utf-8",
	}
}

func sqsChannel() channel.Channel {
	return channel.Channel{
		Name: "reservations", Transport: channel.SQS, Address: "reservations.fifo",
		EventType: "stock.reservation.requested", ContractMajor: 1, ContractRef: "stock/v1/events.proto#ReservationRequested",
		Ordering:    channel.Ordering{Key: "partitionkey", Unit: channel.Group},
		Redelivery:  channel.SQSWindow(5, 30*time.Second, 4*24*time.Hour),
		Containment: "reservations-dlq.fifo", Retry: channel.Retry{Strategy: channel.InlineWithLimit, MaxAttempts: 2},
	}
}

func validConfig() kafka.Config {
	return kafka.Config{
		Brokers: []string{"localhost:9092"},
		TLS:     &tls.Config{MinVersion: tls.VersionTLS13},
		Catalog: channel.Catalog{"orders": ordersChannel(), "reservations": sqsChannel()},
		Sheet:   resilience.Defaults("kafka"),
		Service: "billing",
		Clock:   clock.NewFake(start),
	}
}

func TestConfigValidate(t *testing.T) {
	cases := map[string]struct {
		mutate func(*kafka.Config)
		want   error
	}{
		"complete":                 {func(*kafka.Config) {}, nil},
		"no brokers":               {func(c *kafka.Config) { c.Brokers = nil }, kafka.ErrIncompleteConfig},
		"no clock":                 {func(c *kafka.Config) { c.Clock = nil }, kafka.ErrIncompleteConfig},
		"no catalogue":             {func(c *kafka.Config) { c.Catalog = nil }, kafka.ErrIncompleteConfig},
		"no TLS, no opt-out":       {func(c *kafka.Config) { c.TLS = nil }, kafka.ErrTLSRequired},
		"TLS without verification": {func(c *kafka.Config) { c.TLS = &tls.Config{InsecureSkipVerify: true} }, kafka.ErrTLSTooWeak},
		"TLS below 1.2":            {func(c *kafka.Config) { c.TLS = &tls.Config{MinVersion: tls.VersionTLS10} }, kafka.ErrTLSTooWeak},
		"invalid channel": {func(c *kafka.Config) {
			ch := ordersChannel()
			ch.Retry.Strategy = channel.SeparateChannel
			c.Catalog = channel.Catalog{"orders": ch}
		}, kafka.ErrOrderedChannelWithSeparateRetry},
		"blank sheet": {func(c *kafka.Config) { c.Sheet.Breaker = resilience.Field[resilience.BreakerPolicy]{} }, resilience.ErrBlankField},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if tc.want == nil && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
	t.Run("opt-out admits no TLS", func(t *testing.T) {
		cfg := validConfig()
		cfg.TLS, cfg.InsecureForDevelopmentOnly = nil, true
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})
}

func TestConfigChannel(t *testing.T) {
	cfg := validConfig()
	if ch, err := cfg.Channel("orders"); err != nil || ch.Address != "sales.order.placed.v1" {
		t.Fatalf("Channel(orders) = %v, %v", ch.Address, err)
	}
	if _, err := cfg.Channel("payments"); !errors.Is(err, kafka.ErrUnknownChannel) {
		t.Fatalf("Channel(unknown) = %v, want ErrUnknownChannel (ASY-01)", err)
	}
	if _, err := cfg.Channel("reservations"); !errors.Is(err, kafka.ErrNotKafkaChannel) {
		t.Fatalf("Channel(sqs) = %v, want ErrNotKafkaChannel", err)
	}
}
