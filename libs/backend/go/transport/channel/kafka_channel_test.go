package channel_test

import (
	"reflect"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

func newKafkaUnderTest() channel.Channel {
	return channel.NewKafka("orders.events", "orders.topic", "orders.topic.dlq", "orders",
		"com.company.orders.order-placed", "company/orders/event/v1/order_placed.proto#OrderPlaced")
}

func TestNewKafkaCataloguesAValidChannel(t *testing.T) {
	ch := newKafkaUnderTest()

	if err := (channel.Catalog{ch.Name: ch}).Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	if ch.Transport != channel.Kafka {
		t.Fatalf("Transport = %v, want Kafka", ch.Transport)
	}
	if ch.Name != "orders.events" || ch.Address != "orders.topic" || ch.Containment != "orders.topic.dlq" {
		t.Fatalf("channel = %+v, want the name, address and containment it was given", ch)
	}
}

func TestNewKafkaOrdersByPartitionKey(t *testing.T) {
	ch := newKafkaUnderTest()

	if ch.Ordering.Key != "partitionkey" || ch.Ordering.Unit != channel.Partition {
		t.Fatalf("Ordering = %+v, want the partition key of the envelope", ch.Ordering)
	}
}

func TestNewKafkaRetriesInlineUpToTheSharedCeiling(t *testing.T) {
	ch := newKafkaUnderTest()

	if ch.Retry.Strategy != channel.InlineWithLimit || ch.Retry.MaxAttempts != channel.InlineAttempts {
		t.Fatalf("Retry = %+v, want inline up to InlineAttempts: the adapter takes the same ceiling (ADR-039)", ch.Retry)
	}
}

func TestNewKafkaDerivesTheRedeliveryWindowFromTheRetention(t *testing.T) {
	ch := newKafkaUnderTest()

	want := channel.KafkaWindow(channel.KafkaRetention{
		RetentionByTime: channel.RetentionByTime, RetentionBySize: -1, CleanupPolicy: "delete", InitialOffset: "earliest",
	})
	if !reflect.DeepEqual(ch.Redelivery, want) {
		t.Fatalf("Redelivery = %+v, want %+v", ch.Redelivery, want)
	}
}

func TestEventTypeOfCarriesTheContractMajor(t *testing.T) {
	ch := newKafkaUnderTest()

	if got := channel.EventTypeOf(ch); got != "com.company.orders.order-placed.v1" {
		t.Fatalf("EventTypeOf() = %q, want the event type at its contract major (PTB-03)", got)
	}
}
