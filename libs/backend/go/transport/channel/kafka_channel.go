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

// NewKafka catalogues a Kafka channel with the bindings every context in this
// workspace declares the same way: ordering by partition key, redelivery
// derived from the retention, and inline retry up to the shared ceiling.
func NewKafka(name, address, dlq, group, eventType, contract string) Channel {
	return Channel{
		Name:          name,
		Transport:     Kafka,
		Address:       address,
		EventType:     eventType,
		ContractMajor: 1,
		ContractRef:   contract,
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

// EventTypeOf is the wire type the channel carries: its event type at its
// contract major (PTB-03), which is what the consumer subscribes to.
func EventTypeOf(ch Channel) string {
	return ch.EventType + ".v" + strconv.Itoa(ch.ContractMajor)
}
