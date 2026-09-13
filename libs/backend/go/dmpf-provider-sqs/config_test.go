package dmpfsqs_test

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/resilience"
	dmpfsqs "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-sqs"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/channel"
)

var start = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

const (
	fifoURL     = "http://localhost:4566/000000000000/reservations.fifo"
	fifoDLQ     = "http://localhost:4566/000000000000/reservations-dlq.fifo"
	standardURL = "http://localhost:4566/000000000000/notifications"
	standardDLQ = "http://localhost:4566/000000000000/notifications-dlq"
	topicARN    = "arn:aws:sns:us-east-1:000000000000/orders-topic"
)

func fifoChannel() channel.Channel {
	return channel.Channel{
		Name: "reservations", Transport: channel.SQS, Address: fifoURL,
		EventType: "stock.reservation.requested", ContractMajor: 1, ContractRef: "stock/v1/events.proto#ReservationRequested",
		Ordering:    channel.Ordering{Key: "partitionkey", Unit: channel.Group},
		Redelivery:  channel.SQSWindow(2, 30*time.Second, 4*24*time.Hour),
		Containment: fifoDLQ, Retry: channel.Retry{Strategy: channel.InlineWithLimit, MaxAttempts: 2},
	}
}

func standardChannel() channel.Channel {
	return channel.Channel{
		Name: "notifications", Transport: channel.SQS, Address: standardURL,
		EventType: "crm.notification.sent", ContractMajor: 1, ContractRef: "crm/v1/events.proto#NotificationSent",
		Ordering:    channel.Ordering{Unit: channel.None},
		Redelivery:  channel.SQSWindow(3, 30*time.Second, 24*time.Hour),
		Containment: standardDLQ, Retry: channel.Retry{Strategy: channel.InlineWithLimit, MaxAttempts: 3},
	}
}

func snsChannel() channel.Channel {
	return channel.Channel{
		Name: "orders-fanout", Transport: channel.SNSSQS, Address: topicARN,
		EventType: "sales.order.placed", ContractMajor: 1, ContractRef: "sales/order/v1/events.proto#OrderPlaced",
		Ordering:    channel.Ordering{Unit: channel.None},
		Redelivery:  channel.SNSSQSWindow(time.Hour, channel.SQSWindow(3, 30*time.Second, 24*time.Hour)),
		Containment: standardDLQ, Retry: channel.Retry{Strategy: channel.InlineWithLimit, MaxAttempts: 3},
	}
}

func kafkaChannel() channel.Channel {
	return channel.Channel{
		Name: "orders", Transport: channel.Kafka, Address: "sales.order.placed.v1",
		EventType: "sales.order.placed", ContractMajor: 1, ContractRef: "sales/order/v1/events.proto#OrderPlaced",
		Ordering:    channel.Ordering{Key: "partitionkey", Unit: channel.Partition},
		Redelivery:  channel.KafkaWindow(channel.KafkaRetention{RetentionByTime: time.Hour, RetentionBySize: -1, CleanupPolicy: "delete", InitialOffset: "earliest"}),
		Containment: "sales.order.placed.v1.dlq", Retry: channel.Retry{Strategy: channel.InlineWithLimit, MaxAttempts: 3},
		Group: "g", Partitions: 3, Partitioner: "default", KeyEncoding: "utf-8",
	}
}

func validConfig() dmpfsqs.Config {
	return dmpfsqs.Config{
		AWS:     aws.Config{Region: "us-east-1"},
		Catalog: channel.Catalog{"reservations": fifoChannel(), "notifications": standardChannel(), "orders-fanout": snsChannel(), "orders": kafkaChannel()},
		Sheet:   resilience.Defaults("sqs"),
		Service: "stock",
		Clock:   clock.NewFake(start),
		Rand:    func() float64 { return 0 },
	}
}

func TestConfigValidate(t *testing.T) {
	cases := map[string]struct {
		mutate func(*dmpfsqs.Config)
		want   error
	}{
		"complete":                    {func(*dmpfsqs.Config) {}, nil},
		"no region":                   {func(c *dmpfsqs.Config) { c.AWS.Region = "" }, dmpfsqs.ErrIncompleteConfig},
		"no clock":                    {func(c *dmpfsqs.Config) { c.Clock = nil }, dmpfsqs.ErrIncompleteConfig},
		"no catalogue":                {func(c *dmpfsqs.Config) { c.Catalog = nil }, dmpfsqs.ErrIncompleteConfig},
		"plaintext endpoint":          {func(c *dmpfsqs.Config) { c.Endpoint = "http://localhost:4566" }, dmpfsqs.ErrTLSRequired},
		"plaintext endpoint, opt-out": {func(c *dmpfsqs.Config) { c.Endpoint, c.InsecureForDevelopmentOnly = "http://localhost:4566", true }, nil},
		"https endpoint":              {func(c *dmpfsqs.Config) { c.Endpoint = "https://sqs.us-east-1.amazonaws.com" }, nil},
		"blank sheet":                 {func(c *dmpfsqs.Config) { c.Sheet.Breaker = resilience.Field[resilience.BreakerPolicy]{} }, resilience.ErrBlankField},
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
}

func TestConfigChannel(t *testing.T) {
	cfg := validConfig()
	for _, name := range []string{"reservations", "notifications", "orders-fanout"} {
		if _, err := cfg.Channel(name); err != nil {
			t.Fatalf("Channel(%s) = %v, want nil", name, err)
		}
	}
	if _, err := cfg.Channel("payments"); !errors.Is(err, dmpfsqs.ErrUnknownChannel) {
		t.Fatalf("Channel(unknown) = %v, want ErrUnknownChannel (ASY-01)", err)
	}
	if _, err := cfg.Channel("orders"); !errors.Is(err, dmpfsqs.ErrNotSQSChannel) {
		t.Fatalf("Channel(kafka) = %v, want ErrNotSQSChannel", err)
	}

	t.Run("fifo without group ordering is refused", func(t *testing.T) {
		ch := fifoChannel()
		ch.Ordering = channel.Ordering{Unit: channel.None}
		cfg := validConfig()
		cfg.Catalog = channel.Catalog{ch.Name: ch}
		if _, err := cfg.Channel(ch.Name); !errors.Is(err, dmpfsqs.ErrOrderingMismatch) {
			t.Fatalf("Channel() = %v, want ErrOrderingMismatch (SQS-04)", err)
		}
	})
	t.Run("standard with group ordering is refused", func(t *testing.T) {
		ch := standardChannel()
		ch.Ordering = channel.Ordering{Key: "partitionkey", Unit: channel.Group}
		cfg := validConfig()
		cfg.Catalog = channel.Catalog{ch.Name: ch}
		if _, err := cfg.Channel(ch.Name); !errors.Is(err, dmpfsqs.ErrOrderingMismatch) {
			t.Fatalf("Channel() = %v, want ErrOrderingMismatch (SQS-04)", err)
		}
	})
}
