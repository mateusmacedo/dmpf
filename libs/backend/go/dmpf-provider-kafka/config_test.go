package dmpfkafka_test

import (
	"crypto/tls"
	"errors"
	"testing"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	dmpfkafka "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-kafka"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
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

func validConfig() dmpfkafka.Config {
	return dmpfkafka.Config{
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
		mutate func(*dmpfkafka.Config)
		want   error
	}{
		"complete":                 {func(*dmpfkafka.Config) {}, nil},
		"no brokers":               {func(c *dmpfkafka.Config) { c.Brokers = nil }, dmpfkafka.ErrIncompleteConfig},
		"no clock":                 {func(c *dmpfkafka.Config) { c.Clock = nil }, dmpfkafka.ErrIncompleteConfig},
		"no catalogue":             {func(c *dmpfkafka.Config) { c.Catalog = nil }, dmpfkafka.ErrIncompleteConfig},
		"no TLS, no opt-out":       {func(c *dmpfkafka.Config) { c.TLS = nil }, dmpfkafka.ErrTLSRequired},
		"TLS without verification": {func(c *dmpfkafka.Config) { c.TLS = &tls.Config{InsecureSkipVerify: true} }, dmpfkafka.ErrTLSTooWeak},
		"TLS below 1.2":            {func(c *dmpfkafka.Config) { c.TLS = &tls.Config{MinVersion: tls.VersionTLS10} }, dmpfkafka.ErrTLSTooWeak},
		"invalid channel": {func(c *dmpfkafka.Config) {
			ch := ordersChannel()
			ch.Retry.Strategy = channel.SeparateChannel
			c.Catalog = channel.Catalog{"orders": ch}
		}, dmpfkafka.ErrOrderedChannelWithSeparateRetry},
		"blank sheet": {func(c *dmpfkafka.Config) { c.Sheet.Breaker = resilience.Field[resilience.BreakerPolicy]{} }, resilience.ErrBlankField},
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
	if _, err := cfg.Channel("payments"); !errors.Is(err, dmpfkafka.ErrUnknownChannel) {
		t.Fatalf("Channel(unknown) = %v, want ErrUnknownChannel (ASY-01)", err)
	}
	if _, err := cfg.Channel("reservations"); !errors.Is(err, dmpfkafka.ErrNotKafkaChannel) {
		t.Fatalf("Channel(sqs) = %v, want ErrNotKafkaChannel", err)
	}
}
