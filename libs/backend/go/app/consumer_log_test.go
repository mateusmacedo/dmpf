package app_test

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	messageConsumed = "message consumed"
	consumerScope   = "github.com/mateusmacedo/dmpf/libs/backend/go/app"
)

var errAck = errors.New("broker unavailable")

type recordingExporter struct{ records []sdklog.Record }

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}
func (*recordingExporter) Shutdown(context.Context) error   { return nil }
func (*recordingExporter) ForceFlush(context.Context) error { return nil }

func loggedConsumer(t *testing.T, handler *fakeHandler, containment ports.Containment) (app.Consumer, *tracetest.SpanRecorder, func() []sdklog.Record) {
	t.Helper()
	exporter := &recordingExporter{}
	provider := otelboot.NewLoggerProvider(otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource:   otelboot.Resource{ServiceName: "reservations", ServiceVersion: "1.0.0", ServiceInstanceID: "reservations-1", Role: "consumer"},
	}, exporter)
	consumer, _, recorder := tracedConsumer(handler.handle, containment, sdktrace.AlwaysSample())
	consumer.LoggerProvider = provider
	return consumer, recorder, func() []sdklog.Record {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() = %v", err)
		}
		return exporter.records
	}
}

func onlyConsumed(t *testing.T, records []sdklog.Record) sdklog.Record {
	t.Helper()
	var found []sdklog.Record
	for _, record := range records {
		if record.Body().AsString() == messageConsumed {
			found = append(found, record)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d %q records among %d, want exactly one per message", len(found), messageConsumed, len(records))
	}
	if scope := found[0].InstrumentationScope().Name; scope != consumerScope {
		t.Fatalf("%q scope = %q, want the import path of the emitting package %q (RF-A1)", messageConsumed, scope, consumerScope)
	}
	return found[0]
}

func recordAttributes(record sdklog.Record) map[string]attribute.Value {
	attributes := map[string]attribute.Value{}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		attributes[string(kv.Key)] = kv.Value
		return true
	})
	return attributes
}

func requireRecord(t *testing.T, record sdklog.Record, want map[string]attribute.Value) {
	t.Helper()
	got := recordAttributes(record)
	for key, value := range want {
		if actual, ok := got[key]; !ok {
			t.Errorf("%q carries no %s, want %v", messageConsumed, key, value.String())
		} else if actual != value {
			t.Errorf("%q %s = %v, want %v", messageConsumed, key, actual.String(), value.String())
		}
	}
}

func TestOneMessageConsumedRecordCarriesTheMessagingKeysAndTheProcessContext(t *testing.T) {
	raw, _ := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}
	consumer, recorder, collect := loggedConsumer(t, handler, &fakeContainment{})

	if _, err := consumer.Consume(underACallerSpan(t, consumer.Tracer), app.Delivery{Raw: raw, Attempt: 2}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	record := onlyConsumed(t, collect())
	process := onlyProcess(t, recorder)

	if record.Severity() != log.SeverityInfo {
		t.Errorf("severity = %v, want info for an accepted consumption", record.Severity())
	}
	if record.TraceID() != process.SpanContext().TraceID() || record.SpanID() != process.SpanContext().SpanID() {
		t.Errorf("record trace = %s/%s, want the process %s/%s", record.TraceID(), record.SpanID(),
			process.SpanContext().TraceID(), process.SpanContext().SpanID())
	}
	requireRecord(t, record, map[string]attribute.Value{
		string(semconv.MessagingSystemKey):            attribute.StringValue("kafka"),
		string(semconv.MessagingOperationNameKey):     attribute.StringValue("process"),
		string(semconv.MessagingDestinationNameKey):   attribute.StringValue(testTopic),
		string(semconv.MessagingConsumerGroupNameKey): attribute.StringValue(testGroup),
		string(semconv.MessagingMessageIDKey):         attribute.StringValue("evt-1"),
		string(semconv.CloudEventsEventTypeKey):       attribute.StringValue("com.company.orders.order-placed.v1"),
		tracing.KeyInboxDisposition:                   attribute.StringValue(application.R1D1.String()),
		tracing.KeyInboxGesture:                       attribute.StringValue("ack"),
		tracing.KeyInboxAttempt:                       attribute.Int64Value(2),
		tracing.KeyOutcomeCategory:                    attribute.StringValue("ok"),
		tracing.KeyCorrelationID:                      attribute.StringValue("corr-1"),
		tracing.KeyRequestID:                          attribute.StringValue(handler.execution.RequestID()),
		tracing.KeyTenantID:                           attribute.StringValue(fixtureTenant),
	})
	got := recordAttributes(record)
	for _, key := range []string{"duration_ms", string(semconv.MessagingDestinationPartitionIDKey), string(semconv.MessagingKafkaOffsetKey)} {
		if value, ok := got[key]; ok {
			t.Errorf("%q carries %s = %v, want it absent (RF-A5)", messageConsumed, key, value.String())
		}
	}
}

func TestTheMessageConsumedLevelFollowsTheConsumerSeverity(t *testing.T) {
	transient := application.NewFailure(application.TransientDependency, true, errHandler)
	terminal := application.NewFailure(application.Unexpected, false, errHandler)

	for _, tc := range []struct {
		name        string
		disposition application.Disposition
		handleErr   error
		ackErr      error
		severity    log.Severity
		gesture     string
		category    string
	}{
		{name: "released for another attempt", disposition: application.R1D3, handleErr: transient, severity: log.SeverityInfo, gesture: "release", category: string(application.TransientDependency)},
		{name: "contained as terminal", disposition: application.R1D4, handleErr: terminal, severity: log.SeverityError, gesture: "ack", category: string(application.Unexpected)},
		{name: "acknowledgement failed", disposition: application.R1D1, ackErr: errAck, severity: log.SeverityError, gesture: "ack", category: semconv.ErrorTypeOther.Value.AsString()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := validRaw(t)
			handler := &fakeHandler{disposition: tc.disposition, err: tc.handleErr}
			consumer, _, collect := loggedConsumer(t, handler, &fakeContainment{})

			_, _ = consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{err: tc.ackErr})
			record := onlyConsumed(t, collect())

			if record.Severity() != tc.severity {
				t.Errorf("severity = %v, want %v", record.Severity(), tc.severity)
			}
			requireRecord(t, record, map[string]attribute.Value{
				tracing.KeyInboxDisposition: attribute.StringValue(tc.disposition.String()),
				tracing.KeyInboxGesture:     attribute.StringValue(tc.gesture),
				tracing.KeyOutcomeCategory:  attribute.StringValue(tc.category),
			})
		})
	}
}

func TestTheMessageConsumedLevelFollowsTheFND07OutcomeOfTheHandlersFailure(t *testing.T) {
	failure := func(category application.Category, retryable bool) error {
		return application.NewFailure(category, retryable, errHandler)
	}
	errQuarantine := errors.New("quarantine unavailable")

	for _, tc := range []struct {
		name       string
		handleErr  error
		panics     bool
		attempt    int
		quarantine error
		ackErr     error
		severity   log.Severity
		category   string
	}{
		{name: "Validation is rejected", handleErr: failure(application.Validation, false), severity: log.SeverityInfo, category: string(application.Validation)},
		{name: "DomainRejection is rejected", handleErr: failure(application.DomainRejection, false), severity: log.SeverityInfo, category: string(application.DomainRejection)},
		{name: "NotFound is rejected", handleErr: failure(application.NotFound, false), severity: log.SeverityInfo, category: string(application.NotFound)},
		{name: "Conflict is rejected", handleErr: failure(application.Conflict, false), severity: log.SeverityInfo, category: string(application.Conflict)},
		{name: "Forbidden is denied", handleErr: failure(application.Forbidden, false), severity: log.SeverityWarn, category: string(application.Forbidden)},
		{name: "Unauthenticated is denied", handleErr: failure(application.Unauthenticated, false), severity: log.SeverityWarn, category: string(application.Unauthenticated)},
		{name: "Unexpected is failed", handleErr: failure(application.Unexpected, false), severity: log.SeverityError, category: string(application.Unexpected)},
		{name: "a panic is failed as Unexpected", panics: true, severity: log.SeverityError, category: string(application.Unexpected)},
		{name: "TransientDependency exhausted is failed", handleErr: failure(application.TransientDependency, true), attempt: 3, severity: log.SeverityError, category: string(application.TransientDependency)},
		{name: "RateLimited is failed", handleErr: failure(application.RateLimited, false), severity: log.SeverityError, category: string(application.RateLimited)},
		{name: "DeadlineExceeded is failed", handleErr: failure(application.DeadlineExceeded, false), severity: log.SeverityError, category: string(application.DeadlineExceeded)},
		{name: "Cancelled is failed", handleErr: failure(application.Cancelled, false), severity: log.SeverityError, category: string(application.Cancelled)},
		{name: "an unclassified error is failed", handleErr: errHandler, severity: log.SeverityError, category: semconv.ErrorTypeOther.Value.AsString()},
		{name: "a rejection the quarantine could not keep is failed", handleErr: failure(application.DomainRejection, false), quarantine: errQuarantine, severity: log.SeverityError, category: string(application.DomainRejection)},
		{name: "a denial the broker did not confirm is failed", handleErr: failure(application.Forbidden, false), ackErr: errAck, severity: log.SeverityError, category: string(application.Forbidden)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := validRaw(t)
			handler := &fakeHandler{}
			if tc.handleErr != nil {
				handler = &fakeHandler{disposition: application.Classify(tc.handleErr), err: tc.handleErr}
			}
			consumer, _, collect := loggedConsumer(t, handler, &fakeContainment{err: tc.quarantine})
			if tc.panics {
				consumer.Handle = panickingHandler
			}
			attempt := max(tc.attempt, 1)

			_, _ = consumeLikeTheWorker(t, context.Background(), consumer, app.Delivery{Raw: raw, Attempt: attempt}, &fakeAck{err: tc.ackErr})
			record := onlyConsumed(t, collect())

			if record.Severity() != tc.severity {
				t.Errorf("severity = %v, want %v for %s (RF-A5)", record.Severity(), tc.severity, tc.category)
			}
			requireRecord(t, record, map[string]attribute.Value{
				tracing.KeyOutcomeCategory: attribute.StringValue(tc.category),
			})
		})
	}
}

func TestAMessageOutsideTheBoundaryIsLoggedUnderItsProcessWithoutAnExecution(t *testing.T) {
	raw, _ := validRaw(t)
	handler := &fakeHandler{}
	consumer, recorder, collect := loggedConsumer(t, handler, &fakeContainment{})
	consumer.Boundary = app.Boundary{Transport: app.TransportVerified, Sources: []string{"urn:dmpf:billing"}}

	if _, err := consumer.Consume(underACallerSpan(t, consumer.Tracer), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	record := onlyConsumed(t, collect())

	if record.TraceID() != onlyProcess(t, recorder).SpanContext().TraceID() {
		t.Errorf("record trace = %s, want the process's", record.TraceID())
	}
	requireRecord(t, record, map[string]attribute.Value{
		string(semconv.MessagingMessageIDKey): attribute.StringValue("evt-1"),
		tracing.KeyInboxGesture:               attribute.StringValue("ack"),
		tracing.KeyOutcomeCategory:            attribute.StringValue("ok"),
	})
	got := recordAttributes(record)
	for _, key := range []string{tracing.KeyInboxDisposition, tracing.KeyCorrelationID, tracing.KeyRequestID, tracing.KeyTenantID} {
		if value, ok := got[key]; ok {
			t.Errorf("%q carries %s = %v, want it absent without an execution (CTX-26)", messageConsumed, key, value.String())
		}
	}
}

func TestAnInvalidEnvelopeHasNoProcessAndSoNoConsumptionRecord(t *testing.T) {
	consumer, _, collect := loggedConsumer(t, &fakeHandler{}, &fakeContainment{})

	if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: []byte("not a cloudevent"), Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	for _, record := range collect() {
		if record.Body().AsString() == messageConsumed {
			t.Fatalf("an invalid envelope logged %q, want no record outside a process", messageConsumed)
		}
	}
}

type orderSpy struct{ trace *[]string }

func (s orderSpy) OnEmit(context.Context, *sdklog.Record) error {
	*s.trace = append(*s.trace, "log")
	return nil
}
func (orderSpy) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }
func (orderSpy) Shutdown(context.Context) error                         { return nil }
func (orderSpy) ForceFlush(context.Context) error                       { return nil }

func TestTheConsumptionIsLoggedAfterTheGesture(t *testing.T) {
	raw, _ := validRaw(t)
	var order []string
	handler := &fakeHandler{disposition: application.R1D1, trace: &order}
	consumer := newConsumer(handler, &fakeContainment{}, 3)
	consumer.LoggerProvider = sdklog.NewLoggerProvider(sdklog.WithProcessor(orderSpy{trace: &order}))

	if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{trace: &order}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	if len(order) != 3 || order[0] != "handle" || order[1] != "ack" || order[2] != "log" {
		t.Fatalf("order = %v, want [handle ack log]", order)
	}
}

type keyRecorder struct{ keys *[]string }

func (h keyRecorder) OnEmit(_ context.Context, record *sdklog.Record) error {
	if record.Body().AsString() == messageConsumed {
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			*h.keys = append(*h.keys, string(kv.Key))
			return true
		})
	}
	return nil
}
func (keyRecorder) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }
func (keyRecorder) Shutdown(context.Context) error                         { return nil }
func (keyRecorder) ForceFlush(context.Context) error                       { return nil }

func TestTheConsumerNeverHandsDurationPartitionOrOffsetToTheLogger(t *testing.T) {
	raw, _ := validRaw(t)
	var keys []string
	consumer, _, _ := tracedConsumer((&fakeHandler{disposition: application.R1D1}).handle, &fakeContainment{}, sdktrace.AlwaysSample())
	consumer.LoggerProvider = sdklog.NewLoggerProvider(sdklog.WithProcessor(keyRecorder{keys: &keys}))

	if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	if len(keys) == 0 {
		t.Fatalf("no %q reached the logger", messageConsumed)
	}
	for _, key := range []string{"duration_ms", string(semconv.MessagingDestinationPartitionIDKey), string(semconv.MessagingKafkaOffsetKey)} {
		if slices.Contains(keys, key) {
			t.Errorf("%q hands %s to the logger, want it only on the span (RF-A5)", messageConsumed, key)
		}
	}
}

type countingProvider struct {
	log.LoggerProvider
	loggers atomic.Int64
}

func (p *countingProvider) Logger(name string, options ...log.LoggerOption) log.Logger {
	p.loggers.Add(1)
	return p.LoggerProvider.Logger(name, options...)
}

func TestTheConsumerBuildsItsLoggerOnceAcrossMessages(t *testing.T) {
	raw, _ := validRaw(t)
	consumer := newConsumer(&fakeHandler{disposition: application.R1D1}, &fakeContainment{}, 3)
	provider := &countingProvider{LoggerProvider: sdklog.NewLoggerProvider()}
	consumer.LoggerProvider = provider

	for attempt := 1; attempt <= 3; attempt++ {
		if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: attempt}, &fakeAck{}); err != nil {
			t.Fatalf("Consume() = %v", err)
		}
	}

	if built := provider.loggers.Load(); built != 1 {
		t.Fatalf("%d loggers built for three %q records, want one", built, messageConsumed)
	}
}

type unhashableProvider struct {
	log.LoggerProvider
	scopes []string
}

func TestAProviderThatCannotKeyTheCacheStillLogsEveryMessage(t *testing.T) {
	raw, _ := validRaw(t)
	var keys []string
	consumer := newConsumer(&fakeHandler{disposition: application.R1D1}, &fakeContainment{}, 3)
	consumer.LoggerProvider = unhashableProvider{
		LoggerProvider: sdklog.NewLoggerProvider(sdklog.WithProcessor(keyRecorder{keys: &keys})),
		scopes:         []string{consumerScope},
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: attempt}, &fakeAck{}); err != nil {
			t.Fatalf("Consume() = %v", err)
		}
	}

	logged := 0
	for _, key := range keys {
		if key == string(semconv.MessagingMessageIDKey) {
			logged++
		}
	}
	if logged != 2 {
		t.Fatalf("%d %q records through a provider that is not comparable, want one per message (2)", logged, messageConsumed)
	}
}
