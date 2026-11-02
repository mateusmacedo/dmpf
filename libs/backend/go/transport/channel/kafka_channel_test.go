package channel_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

var (
	orderPlaced = channel.Event{Type: "com.company.orders.order-placed", ContractRef: "company/orders/event/v1/order_placed.proto#OrderPlaced"}
	itemAdded   = channel.Event{Type: "com.company.orders.item-added", ContractRef: "company/orders/event/v1/item_added.proto#ItemAdded"}
)

func newKafkaUnderTest() channel.Channel {
	return channel.NewKafka("orders.events", "orders.topic", "orders.topic.dlq", "orders", orderPlaced, itemAdded)
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

func TestNewKafkaCarriesTheEventsItIsGiven(t *testing.T) {
	ch := newKafkaUnderTest()

	if !slices.Equal(ch.Events, []channel.Event{orderPlaced, itemAdded}) {
		t.Fatalf("Events = %+v, want the events it was given, in order (KFK-03b)", ch.Events)
	}
}

func TestNewKafkaWithoutEventsIsRefused(t *testing.T) {
	ch := channel.NewKafka("orders.events", "orders.topic", "orders.topic.dlq", "orders")

	if err := ch.Validate(); !errors.Is(err, channel.ErrMissingItem) {
		t.Fatalf("Validate() = %v, want ErrMissingItem: a channel carries at least one event (ASY-02)", err)
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

func TestWireTypesCarryTheContractMajorInDeclaredOrder(t *testing.T) {
	ch := newKafkaUnderTest()

	want := []string{"com.company.orders.order-placed.v1", "com.company.orders.item-added.v1"}
	if got := channel.WireTypes(ch); !slices.Equal(got, want) {
		t.Fatalf("WireTypes() = %q, want %q: each event type at its contract major, in declared order (PTB-03)", got, want)
	}

	ch.ContractMajor = 2
	want = []string{"com.company.orders.order-placed.v2", "com.company.orders.item-added.v2"}
	if got := channel.WireTypes(ch); !slices.Equal(got, want) {
		t.Fatalf("WireTypes() at major 2 = %q, want %q: the major is the channel's, for the whole set (KFK-03b)", got, want)
	}
}
