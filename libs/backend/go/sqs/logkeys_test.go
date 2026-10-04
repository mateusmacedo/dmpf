package sqs_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/sqs"
)

type categorized struct{ category, code string }

func (e categorized) Error() string         { return "queue 10.0.0.7 refused the request" }
func (e categorized) ErrorCategory() string { return e.category }
func (e categorized) ErrorCode() string     { return e.code }

var throttled = categorized{category: "RateLimited", code: "sqs-throttled"}

type recordingExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (e *recordingExporter) Shutdown(context.Context) error   { return nil }
func (e *recordingExporter) ForceFlush(context.Context) error { return nil }

func (e *recordingExporter) snapshot() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdklog.Record(nil), e.records...)
}

func consumerLogs() (log.LoggerProvider, *recordingExporter) {
	exporter := &recordingExporter{}
	logs := sdklog.NewLoggerProvider(
		sdklog.WithResource(sdkresource.NewSchemaless(attribute.String(otelboot.ProcessRoleAttribute, "consumer"))),
		sdklog.WithProcessor(otelboot.NewLogProcessor(sdklog.NewSimpleProcessor(exporter), otelboot.LogPolicy{})),
	)
	return logs, exporter
}

func exportedAs(exporter *recordingExporter, body string) []map[string]attribute.Value {
	var out []map[string]attribute.Value
	for _, record := range exporter.snapshot() {
		if record.Body().AsString() != body {
			continue
		}
		attributes := map[string]attribute.Value{}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			attributes[string(kv.Key)] = kv.Value
			return true
		})
		out = append(out, attributes)
	}
	return out
}

func awaitExported(t *testing.T, exporter *recordingExporter, body string, n int) []map[string]attribute.Value {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if records := exportedAs(exporter, body); len(records) >= n {
			return records
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d %q records crossed the processor of the consumer role, want %d", len(exportedAs(exporter, body)), body, n)
		}
		time.Sleep(time.Millisecond)
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
		if strings.Contains(value.String(), strings.TrimSuffix(fifoURL, fifoQueue)) {
			t.Errorf("%s: %s = %s carries the queue URL; the destination is named by the queue (RF-A3)", body, key, value.String())
		}
	}
}

func queueKeys(more map[string]attribute.Value) map[string]attribute.Value {
	keys := map[string]attribute.Value{
		"messaging.system":           attribute.StringValue("aws_sqs"),
		"messaging.destination.name": attribute.StringValue(fifoQueue),
	}
	for key, value := range more {
		keys[key] = value
	}
	return keys
}

func throttledKeys(more map[string]attribute.Value) map[string]attribute.Value {
	keys := queueKeys(more)
	keys["error.type"] = attribute.StringValue(throttled.category)
	keys["dmpf.error.code"] = attribute.StringValue(throttled.code)
	return keys
}

func consumerWithLogs(c *clock.Fake, sink provider.Sink) (*provider.Consumer, *recordingExporter) {
	consumer := newConsumer(c, sink)
	logs, exporter := consumerLogs()
	consumer.Config.LoggerProvider = logs
	return consumer, exporter
}

func TestAFailingReceiveIsLoggedByTheQueueTheFailureCountAndTheRedactedError(t *testing.T) {
	c := clock.NewFake(start)
	api := provider.NewFakeSQS()
	api.ReceiveErr = throttled
	consumer, exporter := consumerWithLogs(c, newSink(nil))
	stop := run(t, consumer, api)
	defer stop()

	body := "sqs: receive failed"
	got := awaitExported(t, exporter, body, 1)[0]
	assertSurvives(t, body, got, throttledKeys(map[string]attribute.Value{
		"dmpf.consumer.consecutive_failures": attribute.Int64Value(1),
	}))
}

func TestAFailedExtensionAndTheSinkFailureAreLoggedByTheQueueAndTheRedactedError(t *testing.T) {
	c := clock.NewFake(start)
	api := provider.NewFakeSQS()
	api.ChangeErr = throttled
	sink := newSink(func(ctx context.Context, _ handled, _ ports.Acknowledger) error {
		<-ctx.Done()
		return throttled
	})
	consumer, exporter := consumerWithLogs(c, sink)
	stop := run(t, consumer, api)
	defer stop()

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-x", provider.EncodeBody(raw), "2")}})
	<-sink.seen
	awaitAlarm(t, c, 2)
	c.Advance(10 * time.Second)

	extension := "sqs: visibility extension failed; attempt cancelled"
	assertSurvives(t, extension, awaitExported(t, exporter, extension, 1)[0], throttledKeys(nil))
	failed := "sqs: sink failed"
	assertSurvives(t, failed, awaitExported(t, exporter, failed, 1)[0], throttledKeys(map[string]attribute.Value{
		"dmpf.inbox.attempt":          attribute.Int64Value(2),
		"dmpf.inbox.gesture_recorded": attribute.BoolValue(false),
	}))
}

func refusedBySDK(operation, code string) error {
	return &smithy.OperationError{ServiceID: "SQS", OperationName: operation,
		Err: &smithy.GenericAPIError{Code: code, Message: "queue 10.0.0.7 refused the request", Fault: smithy.FaultClient}}
}

func errorType(category string) map[string]attribute.Value {
	return map[string]attribute.Value{"error.type": attribute.StringValue(category)}
}

func TestAReceiveThrottledByTheSDKIsLoggedAsRateLimited(t *testing.T) {
	c := clock.NewFake(start)
	api := provider.NewFakeSQS()
	api.ReceiveErr = refusedBySDK("ReceiveMessage", "ThrottlingException")
	consumer, exporter := consumerWithLogs(c, newSink(nil))
	stop := run(t, consumer, api)
	defer stop()

	body := "sqs: receive failed"
	assertSurvives(t, body, awaitExported(t, exporter, body, 1)[0], queueKeys(errorType("RateLimited")))
}

func TestAnExtensionDeniedByTheSDKAndTheSinkItCancelsAreLoggedByTheirCategories(t *testing.T) {
	c := clock.NewFake(start)
	api := provider.NewFakeSQS()
	api.ChangeErr = refusedBySDK("ChangeMessageVisibility", "AccessDeniedException")
	sink := newSink(func(ctx context.Context, _ handled, _ ports.Acknowledger) error {
		<-ctx.Done()
		return ctx.Err()
	})
	consumer, exporter := consumerWithLogs(c, sink)
	stop := run(t, consumer, api)
	defer stop()

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-d", provider.EncodeBody(raw), "1")}})
	<-sink.seen
	awaitAlarm(t, c, 2)
	c.Advance(10 * time.Second)

	extension := "sqs: visibility extension failed; attempt cancelled"
	assertSurvives(t, extension, awaitExported(t, exporter, extension, 1)[0], queueKeys(errorType("Forbidden")))
	failed := "sqs: sink failed"
	assertSurvives(t, failed, awaitExported(t, exporter, failed, 1)[0], queueKeys(errorType("Cancelled")))
}

func TestASinkPanicIsLoggedAsUnexpectedAndNeverByItsValue(t *testing.T) {
	c := clock.NewFake(start)
	api := provider.NewFakeSQS()
	sink := newSink(func(context.Context, handled, ports.Acknowledger) error { panic("queue 10.0.0.7: bad envelope") })
	consumer, exporter := consumerWithLogs(c, sink)
	stop := run(t, consumer, api)
	defer stop()

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-p", provider.EncodeBody(raw), "1")}})

	body := "sqs: sink failed"
	assertSurvives(t, body, awaitExported(t, exporter, body, 1)[0], queueKeys(map[string]attribute.Value{
		"dmpf.inbox.attempt":          attribute.Int64Value(1),
		"dmpf.inbox.gesture_recorded": attribute.BoolValue(false),
		"error.type":                  attribute.StringValue("Unexpected"),
	}))
}

func TestASinkWithoutAGestureIsLoggedByTheQueueAndTheAttempt(t *testing.T) {
	c := clock.NewFake(start)
	api := provider.NewFakeSQS()
	sink := newSink(func(context.Context, handled, ports.Acknowledger) error { return nil })
	consumer, exporter := consumerWithLogs(c, sink)
	stop := run(t, consumer, api)
	defer stop()

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-y", provider.EncodeBody(raw), "3")}})

	body := "sqs: sink returned without a gesture; visibility left to expire (SQS-10)"
	assertSurvives(t, body, awaitExported(t, exporter, body, 1)[0], queueKeys(map[string]attribute.Value{
		"dmpf.inbox.attempt": attribute.Int64Value(3),
	}))
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
	c := clock.NewFake(start)
	api := provider.NewFakeSQS()
	api.ReceiveErr = throttled
	consumer := newConsumer(c, newSink(nil))
	logs, exporter := consumerLogs()
	counting := &countingLogs{LoggerProvider: logs}
	consumer.Config.LoggerProvider = counting
	stop := run(t, consumer, api)
	defer stop()

	awaitExported(t, exporter, "sqs: receive failed", 1)
	awaitAlarm(t, c, 1)
	c.Advance(4 * time.Second)
	awaitExported(t, exporter, "sqs: receive failed", 2)

	if n := counting.loggers.Load(); n != 1 {
		t.Fatalf("the provider built %d loggers for two failed receives, want 1: the logger is built once, not per emission (RF-A1)", n)
	}
}
