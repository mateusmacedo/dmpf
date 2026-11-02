package channel

import (
	"strconv"
	"time"
)

const (
	// InlineAttempts is the channel's retry ceiling (TRP-31); a consumer of the
	// channel takes the same value, because the two limits must agree (ADR-039).
	InlineAttempts = 2

	// RetentionByTime is how long the broker keeps a record available for
	// redelivery, which is what bounds the window the formula derives.
	RetentionByTime = 7 * 24 * time.Hour
)

// NewKafka catalogues a Kafka channel carrying the given events, with the
// bindings every context in this workspace declares the same way: ordering by
// partition key, redelivery derived from the retention, and inline retry up to
// the shared ceiling.
func NewKafka(name, address, dlq, group string, events ...Event) Channel {
	return Channel{
		Name:          name,
		Transport:     Kafka,
		Address:       address,
		Events:        events,
		ContractMajor: 1,
		Ordering:      Ordering{Key: "partitionkey", Unit: Partition},
		Redelivery: KafkaWindow(KafkaRetention{
			RetentionByTime: RetentionByTime, RetentionBySize: -1, CleanupPolicy: "delete", InitialOffset: "earliest",
		}),
		Containment: dlq,
		Retry:       Retry{Strategy: InlineWithLimit, MaxAttempts: InlineAttempts},
		Group:       group,
		Partitions:  1,
		Partitioner: "default",
		KeyEncoding: "utf-8",
	}
}

// WireTypes is the set of wire types the channel carries, in declared order:
// each event type at the channel's contract major (PTB-03). A consumer's
// subscription stays within it (KFK-03c).
func WireTypes(ch Channel) []string {
	major := ".v" + strconv.Itoa(ch.ContractMajor)
	types := make([]string, len(ch.Events))
	for i, event := range ch.Events {
		types[i] = event.Type + major
	}
	return types
}
