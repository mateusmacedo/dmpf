package kafka_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type categorized struct{ category, code string }

func (e categorized) Error() string         { return "broker 10.0.0.7 refused the request" }
func (e categorized) ErrorCategory() string { return e.category }
func (e categorized) ErrorCode() string     { return e.code }

var unavailable = categorized{category: "TransientDependency", code: "broker-unavailable"}

func consumerLogs() (log.LoggerProvider, *recordingExporter) {
	exporter := &recordingExporter{}
	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(sdkresource.NewSchemaless(attribute.String(otelboot.ProcessRoleAttribute, "consumer"))),
		sdklog.WithProcessor(otelboot.NewLogProcessor(sdklog.NewSimpleProcessor(exporter), otelboot.LogPolicy{})),
	)
	return provider, exporter
}

func exported(exporter *recordingExporter, body string) (map[string]attribute.Value, bool) {
	for _, record := range exporter.snapshot() {
		if record.Body().AsString() != body {
			continue
		}
		attributes := map[string]attribute.Value{}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			attributes[string(kv.Key)] = kv.Value
			return true
		})
		return attributes, true
	}
	return nil, false
}

func awaitExported(t *testing.T, exporter *recordingExporter, body string) map[string]attribute.Value {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if attributes, found := exported(exporter, body); found {
			return attributes
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %q record crossed the processor of the consumer role", body)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func assertSurvives(t *testing.T, body string, got map[string]attribute.Value, want map[string]attribute.Value) {
	t.Helper()
	for key, value := range want {
		kept, present := got[key]
		switch {
		case !present:
			t.Errorf("%s: %s did not survive the allowlist of the consumer role (RF-A3)", body, key)
		case kept.Type() != value.Type() || kept.String() != value.String():
			t.Errorf("%s: %s = %s %s, want %s %s", body, key, kept.Type(), kept.String(), value.Type(), value.String())
		}
	}
	for key, value := range got {
		if strings.Contains(value.String(), "10.0.0.7") {
			t.Errorf("%s: %s carries the error message; only its category and code may leave (DAT-23)", body, key)
		}
	}
}

func redactedError() map[string]attribute.Value {
	return map[string]attribute.Value{
		"error.type":      attribute.StringValue(unavailable.category),
		"dmpf.error.code": attribute.StringValue(unavailable.code),
	}
}

func with(base map[string]attribute.Value, more map[string]attribute.Value) map[string]attribute.Value {
	out := map[string]attribute.Value{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range more {
		out[key] = value
	}
	return out
}

func loggedConsumer(sink kafka.Sink) (*kafka.Consumer, *recordingExporter) {
	c := newConsumer(sink)
	provider, exporter := consumerLogs()
	c.Config.LoggerProvider = provider
	return c, exporter
}

func TestAFetchErrorIsLoggedByTheMessagingKeysAndTheRedactedError(t *testing.T) {
	c, exporter := loggedConsumer(newSink())
	fake := kafka.NewFakeClient()
	fake.FeedError(topic, 3, unavailable)

	consume(t, c, fake, func() bool {
		_, found := exported(exporter, "kafka: fetch error")
		return found
	})

	body := "kafka: fetch error"
	got, _ := exported(exporter, body)
	assertSurvives(t, body, got, with(redactedError(), map[string]attribute.Value{
		"messaging.destination.name":         attribute.StringValue(topic),
		"messaging.destination.partition.id": attribute.StringValue("3"),
	}))
}

func TestASinkThatAcknowledgesWithAnErrorIsLoggedByTheMessagingKeys(t *testing.T) {
	sink := newSink()
	sink.on(0, alwaysAck)
	sink.on(1, func(ctx context.Context, _ int, ack ports.Acknowledger) error {
		_ = ack.Ack(ctx)
		return unavailable
	})
	c, exporter := loggedConsumer(sink)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 2, recordsAt(0, 1)...)

	consume(t, c, fake, func() bool { return len(fake.Commits()) >= 2 })

	body := "kafka: sink acknowledged with an error"
	assertSurvives(t, body, awaitExported(t, exporter, body), with(redactedError(), map[string]attribute.Value{
		"messaging.destination.name":         attribute.StringValue(topic),
		"messaging.destination.partition.id": attribute.StringValue("2"),
		"messaging.kafka.offset":             attribute.Int64Value(1),
		"dmpf.inbox.attempt":                 attribute.Int64Value(1),
	}))
}

func TestAStalledPartitionIsLoggedByTheMessagingKeysAndTheQueueBehind(t *testing.T) {
	sink := newSink()
	sink.on(0, func(context.Context, int, ports.Acknowledger) error { return unavailable })
	sink.on(1, alwaysAck)
	c, exporter := loggedConsumer(sink)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 1, recordsAt(0, 1)...)

	consume(t, c, fake, func() bool { return c.StalledAt(topic, 1) })

	body := "kafka: sink returned without a gesture; record left pending and partition stalled"
	got := awaitExported(t, exporter, body)
	assertSurvives(t, body, got, with(redactedError(), map[string]attribute.Value{
		"messaging.destination.name":         attribute.StringValue(topic),
		"messaging.destination.partition.id": attribute.StringValue("1"),
		"messaging.kafka.offset":             attribute.Int64Value(0),
		"dmpf.inbox.attempt":                 attribute.Int64Value(1),
	}))
	if queued, present := got["dmpf.consumer.queued_behind"]; !present || queued.Type() != attribute.INT64 {
		t.Errorf("%s: dmpf.consumer.queued_behind = %v, want the records waiting behind the stalled one", body, got["dmpf.consumer.queued_behind"])
	}
}

func TestASinkPanicIsLoggedAsUnexpectedAndNeverByItsValue(t *testing.T) {
	sink := newSink()
	sink.on(0, func(context.Context, int, ports.Acknowledger) error { panic("broker 10.0.0.7: bad envelope") })
	c, exporter := loggedConsumer(sink)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 4, recordsAt(0)...)

	consume(t, c, fake, func() bool { return c.StalledAt(topic, 4) })

	body := "kafka: sink returned without a gesture; record left pending and partition stalled"
	assertSurvives(t, body, awaitExported(t, exporter, body), map[string]attribute.Value{
		"messaging.destination.name":         attribute.StringValue(topic),
		"messaging.destination.partition.id": attribute.StringValue("4"),
		"error.type":                         attribute.StringValue("Unexpected"),
	})
}

func TestAFailedOffsetCommitIsLoggedByTheMessagingKeysAndTheRedactedError(t *testing.T) {
	sink := newSink()
	sink.on(0, alwaysAck)
	c, exporter := loggedConsumer(sink)
	fake := kafka.NewFakeClient()
	fake.CommitErr = unavailable
	fake.Feed(topic, 0, recordsAt(0)...)

	consume(t, c, fake, func() bool {
		_, found := exported(exporter, "kafka: offset commit failed")
		return found
	})

	body := "kafka: offset commit failed"
	got, _ := exported(exporter, body)
	assertSurvives(t, body, got, with(redactedError(), map[string]attribute.Value{
		"messaging.destination.name":         attribute.StringValue(topic),
		"messaging.destination.partition.id": attribute.StringValue("0"),
		"messaging.kafka.offset":             attribute.Int64Value(0),
	}))
}

func errorType(category string) map[string]attribute.Value {
	return map[string]attribute.Value{"error.type": attribute.StringValue(category)}
}

var errPastDeadline = fmt.Errorf("reserve stock at 10.0.0.7: %w", context.DeadlineExceeded)

func TestAFetchErrorOfTheBrokerIsLoggedByTheCategoryOfItsCode(t *testing.T) {
	for name, tc := range map[string]struct {
		err      error
		category string
	}{
		"leader election":      {kerr.NotLeaderForPartition, "TransientDependency"},
		"authorization denied": {kerr.TopicAuthorizationFailed, "Forbidden"},
	} {
		t.Run(name, func(t *testing.T) {
			c, exporter := loggedConsumer(newSink())
			fake := kafka.NewFakeClient()
			fake.FeedError(topic, 3, tc.err)

			consume(t, c, fake, func() bool {
				_, found := exported(exporter, "kafka: fetch error")
				return found
			})

			body := "kafka: fetch error"
			got, _ := exported(exporter, body)
			assertSurvives(t, body, got, errorType(tc.category))
		})
	}
}

func TestASinkThatAcknowledgesPastItsDeadlineIsLoggedAsDeadlineExceeded(t *testing.T) {
	sink := newSink()
	sink.on(0, func(ctx context.Context, _ int, ack ports.Acknowledger) error {
		_ = ack.Ack(ctx)
		return errPastDeadline
	})
	c, exporter := loggedConsumer(sink)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 2, recordsAt(0)...)

	consume(t, c, fake, func() bool { return len(fake.Commits()) >= 1 })

	body := "kafka: sink acknowledged with an error"
	assertSurvives(t, body, awaitExported(t, exporter, body), errorType("DeadlineExceeded"))
}

func TestAPartitionStalledByTheProcessingDeadlineIsLoggedAsDeadlineExceeded(t *testing.T) {
	sink := newSink()
	sink.on(0, func(context.Context, int, ports.Acknowledger) error { return errPastDeadline })
	c, exporter := loggedConsumer(sink)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 1, recordsAt(0)...)

	consume(t, c, fake, func() bool { return c.StalledAt(topic, 1) })

	body := "kafka: sink returned without a gesture; record left pending and partition stalled"
	assertSurvives(t, body, awaitExported(t, exporter, body), errorType("DeadlineExceeded"))
}

func TestAPartitionStalledWithoutAnErrorCarriesNoErrorType(t *testing.T) {
	sink := newSink()
	sink.on(0, func(context.Context, int, ports.Acknowledger) error { return nil })
	c, exporter := loggedConsumer(sink)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 1, recordsAt(0)...)

	consume(t, c, fake, func() bool { return c.StalledAt(topic, 1) })

	body := "kafka: sink returned without a gesture; record left pending and partition stalled"
	if category, present := awaitExported(t, exporter, body)["error.type"]; present {
		t.Errorf("%s: error.type = %s, want none: the sink returned no error", body, category.String())
	}
}

func TestAnOffsetCommitRefusedByTheBrokerIsLoggedByTheCategoryOfItsCode(t *testing.T) {
	sink := newSink()
	sink.on(0, alwaysAck)
	c, exporter := loggedConsumer(sink)
	fake := kafka.NewFakeClient()
	fake.CommitErr = kerr.GroupAuthorizationFailed
	fake.Feed(topic, 0, recordsAt(0)...)

	consume(t, c, fake, func() bool {
		_, found := exported(exporter, "kafka: offset commit failed")
		return found
	})

	body := "kafka: offset commit failed"
	got, _ := exported(exporter, body)
	assertSurvives(t, body, got, errorType("Forbidden"))
}

func TestARebalanceRecordKeepsItsMessagingKeysThroughTheProcessor(t *testing.T) {
	c, exporter := loggedConsumer(newSink())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.Assign(ctx, kafka.NewFakeClient(), map[string][]int32{topic: {0}})
	c.Revoke(ctx, map[string][]int32{topic: {0}})

	body := "partitions assigned [0]"
	got, _ := exported(exporter, body)
	assertSurvives(t, body, got, map[string]attribute.Value{
		"messaging.system":              attribute.StringValue("kafka"),
		"messaging.consumer.group.name": attribute.StringValue(c.Channel.Group),
		"messaging.destination.name":    attribute.StringValue(topic),
	})
}

func TestAContainedMessageKeepsItsKeysThroughTheProcessor(t *testing.T) {
	cfg := publishConfig()
	provider, exporter := consumerLogs()
	cfg.LoggerProvider = provider
	dlq, err := kafka.NewDLQWith(cfg, ordersChannel(), kafka.NewFakeClient())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	if err := dlq.Quarantine(context.Background(), ports.Contained{Consumer: "billing-consumer", MessageID: "evt-1", Reason: ports.ReasonAttemptsExhausted, Envelope: raw}); err != nil {
		t.Fatal(err)
	}

	body := "message contained"
	got, _ := exported(exporter, body)
	assertSurvives(t, body, got, map[string]attribute.Value{
		"messaging.message.id":    attribute.StringValue("evt-1"),
		"dmpf.containment.reason": attribute.StringValue("attempts-exhausted"),
	})
}

type countingLogs struct {
	log.LoggerProvider
	loggers atomic.Int32
}

func (p *countingLogs) Logger(name string, options ...log.LoggerOption) log.Logger {
	p.loggers.Add(1)
	return p.LoggerProvider.Logger(name, options...)
}

func TestTheConsumerAsksTheProviderForItsLoggerOnce(t *testing.T) {
	sink := newSink()
	for _, offset := range []int64{0, 1, 2} {
		sink.on(offset, func(ctx context.Context, _ int, ack ports.Acknowledger) error {
			_ = ack.Ack(ctx)
			return unavailable
		})
	}
	c := newConsumer(sink)
	provider, exporter := consumerLogs()
	counting := &countingLogs{LoggerProvider: provider}
	c.Config.LoggerProvider = counting
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0, 1)...)
	fake.Feed(topic, 1, recordsAt(2)...)

	consume(t, c, fake, func() bool { return len(exporter.snapshot()) >= 3 })

	if n := counting.loggers.Load(); n != 1 {
		t.Fatalf("the provider built %d loggers for %d records, want 1: the logger is built once, not per emission (RF-A1)", n, len(exporter.snapshot()))
	}
}

func TestTheDLQAsksTheProviderForItsLoggerOnce(t *testing.T) {
	cfg := publishConfig()
	provider, _ := consumerLogs()
	counting := &countingLogs{LoggerProvider: provider}
	cfg.LoggerProvider = counting
	dlq, err := kafka.NewDLQWith(cfg, ordersChannel(), kafka.NewFakeClient())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	built := counting.loggers.Load()
	for range 3 {
		if err := dlq.Quarantine(context.Background(), ports.Contained{Consumer: "c", MessageID: "evt-1", Reason: ports.ReasonAttemptsExhausted, Envelope: raw}); err != nil {
			t.Fatal(err)
		}
	}

	if n := counting.loggers.Load() - built; n != 0 {
		t.Fatalf("3 containments built %d loggers after the DLQ was opened, want 0: the logger is built with the DLQ (RF-A1)", n)
	}
}
