package channel_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

func kafkaWindow() channel.RedeliveryWindow {
	return channel.KafkaWindow(channel.KafkaRetention{
		RetentionByTime: 7 * 24 * time.Hour,
		RetentionBySize: -1,
		CleanupPolicy:   "delete",
		InitialOffset:   "earliest",
	})
}

func sqsWindow() channel.RedeliveryWindow {
	return channel.SQSWindow(5, 30*time.Second, 4*24*time.Hour)
}

func kafkaChannel() channel.Channel {
	return channel.Channel{
		Name:          "propostaAprovada",
		Transport:     channel.Kafka,
		Address:       "credito.proposta.aprovada.v1",
		EventType:     "credito.proposta.aprovada",
		ContractMajor: 1,
		ContractRef:   "credito/proposta/v1/eventos.proto#PropostaAprovada",
		Ordering:      channel.Ordering{Key: "partitionkey", Unit: channel.Partition},
		Redelivery:    kafkaWindow(),
		Containment:   "credito.proposta.aprovada.v1.dlq",
		Retry:         channel.Retry{Strategy: channel.InlineWithLimit, MaxAttempts: 5},
		Group:         "credito-analise-consumer",
		Partitions:    12,
		Partitioner:   "default",
		KeyEncoding:   "utf-8",
	}
}

func sqsChannel() channel.Channel {
	return channel.Channel{
		Name:          "reservaSolicitada",
		Transport:     channel.SQS,
		Address:       "dmpf-test.fifo",
		EventType:     "estoque.reserva.solicitada",
		ContractMajor: 1,
		ContractRef:   "estoque/reserva/v1/eventos.proto#ReservaSolicitada",
		Ordering:      channel.Ordering{Key: "partitionkey", Unit: channel.Group},
		Redelivery:    sqsWindow(),
		Containment:   "dmpf-test-dlq.fifo",
		Retry:         channel.Retry{Strategy: channel.InlineWithLimit, MaxAttempts: 2},
	}
}

func TestChannelValidateAcceptsCompleteChannels(t *testing.T) {
	for name, ch := range map[string]channel.Channel{"kafka": kafkaChannel(), "sqs": sqsChannel()} {
		t.Run(name, func(t *testing.T) {
			if err := ch.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestChannelValidateNamesTheMissingItem(t *testing.T) {
	cases := map[string]struct {
		mutate func(*channel.Channel)
		item   string
	}{
		"name":          {func(c *channel.Channel) { c.Name = "" }, "name"},
		"address":       {func(c *channel.Channel) { c.Address = "" }, "address"},
		"eventType":     {func(c *channel.Channel) { c.EventType = "" }, "eventType"},
		"contractMajor": {func(c *channel.Channel) { c.ContractMajor = 0 }, "contractMajor"},
		"contractRef":   {func(c *channel.Channel) { c.ContractRef = "" }, "contractRef"},
		"transport":     {func(c *channel.Channel) { c.Transport = "" }, "transport"},
		"redelivery":    {func(c *channel.Channel) { c.Redelivery = channel.RedeliveryWindow{} }, "redelivery"},
		"containment":   {func(c *channel.Channel) { c.Containment = "" }, "containment"},
		"retry":         {func(c *channel.Channel) { c.Retry = channel.Retry{} }, "retry"},
		"maxAttempts":   {func(c *channel.Channel) { c.Retry.MaxAttempts = 0 }, "maxAttempts"},
		"group":         {func(c *channel.Channel) { c.Group = "" }, "group"},
		"partitions":    {func(c *channel.Channel) { c.Partitions = 0 }, "partitions"},
		"partitioner":   {func(c *channel.Channel) { c.Partitioner = "" }, "partitioner"},
		"keyEncoding":   {func(c *channel.Channel) { c.KeyEncoding = "" }, "keyEncoding"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ch := kafkaChannel()
			tc.mutate(&ch)

			err := ch.Validate()
			if !errors.Is(err, channel.ErrMissingItem) {
				t.Fatalf("Validate() = %v, want ErrMissingItem", err)
			}
			if !strings.Contains(err.Error(), tc.item) {
				t.Fatalf("Validate() = %q, want it to name %q", err, tc.item)
			}
		})
	}
}

func TestChannelValidateRefusesUnknownTransport(t *testing.T) {
	ch := kafkaChannel()
	ch.Transport = "rabbitmq"

	if err := ch.Validate(); !errors.Is(err, channel.ErrUnknownTransport) {
		t.Fatalf("Validate() = %v, want ErrUnknownTransport", err)
	}
}

func TestOrderingValidate(t *testing.T) {
	cases := map[string]struct {
		ordering channel.Ordering
		wantOK   bool
	}{
		"partition with key":  {channel.Ordering{Key: "k", Unit: channel.Partition}, true},
		"group with key":      {channel.Ordering{Key: "k", Unit: channel.Group}, true},
		"none without key":    {channel.Ordering{Unit: channel.None}, true},
		"none with key":       {channel.Ordering{Key: "k", Unit: channel.None}, false},
		"partition no key":    {channel.Ordering{Unit: channel.Partition}, false},
		"unknown unit":        {channel.Ordering{Key: "k", Unit: "global"}, false},
		"zero value is empty": {channel.Ordering{}, false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.ordering.Validate()
			if tc.wantOK && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if !tc.wantOK && !errors.Is(err, channel.ErrInvalidOrdering) {
				t.Fatalf("Validate() = %v, want ErrInvalidOrdering", err)
			}
		})
	}
}

func TestChannelValidateRefusesOrderedChannelWithSeparateRetry(t *testing.T) {
	ch := kafkaChannel()
	ch.Retry.Strategy = channel.SeparateChannel

	if err := ch.Validate(); !errors.Is(err, channel.ErrOrderedChannelWithSeparateRetry) {
		t.Fatalf("Validate() = %v, want ErrOrderedChannelWithSeparateRetry", err)
	}
}

func TestChannelValidateAllowsSeparateRetryWithoutOrder(t *testing.T) {
	ch := kafkaChannel()
	ch.Ordering = channel.Ordering{Unit: channel.None}
	ch.Retry.Strategy = channel.SeparateChannel

	if err := ch.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestChannelValidateRefusesUnitTheTransportDoesNotProvide(t *testing.T) {
	t.Run("kafka by group", func(t *testing.T) {
		ch := kafkaChannel()
		ch.Ordering.Unit = channel.Group
		if err := ch.Validate(); !errors.Is(err, channel.ErrUnitTransportMismatch) {
			t.Fatalf("Validate() = %v, want ErrUnitTransportMismatch", err)
		}
	})

	t.Run("sqs by partition", func(t *testing.T) {
		ch := sqsChannel()
		ch.Ordering.Unit = channel.Partition
		if err := ch.Validate(); !errors.Is(err, channel.ErrUnitTransportMismatch) {
			t.Fatalf("Validate() = %v, want ErrUnitTransportMismatch", err)
		}
	})
}

func TestChannelValidateRefusesWindowOfAnotherTransport(t *testing.T) {
	ch := kafkaChannel()
	ch.Redelivery = sqsWindow()

	if err := ch.Validate(); !errors.Is(err, channel.ErrWindowTransportMismatch) {
		t.Fatalf("Validate() = %v, want ErrWindowTransportMismatch", err)
	}
}

func TestChannelValidateRefusesInvalidKafkaAddress(t *testing.T) {
	ch := kafkaChannel()
	ch.Address = "Credito.Proposta"

	if err := ch.Validate(); !errors.Is(err, channel.ErrInvalidAddress) {
		t.Fatalf("Validate() = %v, want ErrInvalidAddress", err)
	}
}

func TestCatalogResolve(t *testing.T) {
	catalog := channel.Catalog{"propostaAprovada": kafkaChannel()}

	t.Run("known destination", func(t *testing.T) {
		got, err := catalog.Resolve("propostaAprovada")
		if err != nil {
			t.Fatalf("Resolve() = %v, want nil", err)
		}
		if got.Address != "credito.proposta.aprovada.v1" {
			t.Fatalf("Resolve().Address = %q", got.Address)
		}
	})

	t.Run("unknown destination", func(t *testing.T) {
		_, err := catalog.Resolve("propostaRecusada")
		if !errors.Is(err, channel.ErrUnknownChannel) {
			t.Fatalf("Resolve() = %v, want ErrUnknownChannel", err)
		}
	})
}

func TestCatalogValidate(t *testing.T) {
	t.Run("valid catalogue", func(t *testing.T) {
		catalog := channel.Catalog{"propostaAprovada": kafkaChannel(), "reservaSolicitada": sqsChannel()}
		if err := catalog.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})

	t.Run("key and name disagree", func(t *testing.T) {
		catalog := channel.Catalog{"outro": kafkaChannel()}
		if err := catalog.Validate(); !errors.Is(err, channel.ErrDuplicateChannel) {
			t.Fatalf("Validate() = %v, want ErrDuplicateChannel", err)
		}
	})

	t.Run("invalid channel", func(t *testing.T) {
		ch := kafkaChannel()
		ch.Containment = ""
		catalog := channel.Catalog{ch.Name: ch}
		if err := catalog.Validate(); !errors.Is(err, channel.ErrMissingItem) {
			t.Fatalf("Validate() = %v, want ErrMissingItem", err)
		}
	})

	t.Run("dot and underscore collision", func(t *testing.T) {
		a := kafkaChannel()
		a.Name, a.Address = "a", "a.b_c.v1"
		b := kafkaChannel()
		b.Name, b.Address = "b", "a.b.c.v1"
		catalog := channel.Catalog{"a": a, "b": b}

		if err := catalog.Validate(); !errors.Is(err, channel.ErrAddressCollision) {
			t.Fatalf("Validate() = %v, want ErrAddressCollision", err)
		}
	})

	t.Run("distinct addresses do not collide", func(t *testing.T) {
		a := kafkaChannel()
		a.Name, a.Address = "a", "a.b.c.v1"
		b := kafkaChannel()
		b.Name, b.Address = "b", "a.b.c.v2"
		catalog := channel.Catalog{"a": a, "b": b}

		if err := catalog.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})
}
