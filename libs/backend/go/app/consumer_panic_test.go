package app_test

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	handlerPanic      = "reservations: lost row of tenant acme at 10.0.0.7"
	handlerPanicLeak  = "10.0.0.7"
	panicChildEnv     = "DMPF_TEST_CONSUMER_PANIC"
	panicChildConsume = "consume"
)

func panickingHandler(context.Context, ports.Receipt, envelope.Envelope) (application.Disposition, error) {
	panic(handlerPanic)
}

// Unexpected without a declared predicate resolves to not retryable (ERR-24).
func returnedUnexpected() *fakeHandler {
	failure := application.NewFailure(application.Unexpected, false, errHandler)
	return &fakeHandler{disposition: application.Classify(failure), err: failure}
}

// The kafka and sqs workers recover what escapes the sink (ErrSinkPanicked), so
// the test does too and reads what the consumer left behind.
func consumeLikeTheWorker(t *testing.T, ctx context.Context, consumer app.Consumer, delivery app.Delivery, ack ports.Acknowledger) (outcome app.Outcome, err error) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Errorf("Consume() panicked with %v, want the handler's panic converted into Unexpected before the gesture (ERR-23)", recovered)
		}
	}()
	return consumer.Consume(ctx, delivery, ack)
}

func withoutOwnKeys(attributes map[string]attribute.Value) map[string]attribute.Value {
	rest := maps.Clone(attributes)
	delete(rest, tracing.KeyRequestID)
	delete(rest, keyPanicType)
	return rest
}

func spanAttributes(span sdktrace.ReadOnlySpan) map[string]attribute.Value {
	attributes := map[string]attribute.Value{}
	for _, kv := range span.Attributes() {
		attributes[string(kv.Key)] = kv.Value
	}
	return attributes
}

func TestAPanickingHandlerIsLoggedLikeOneThatReturnsUnexpected(t *testing.T) {
	raw, _ := validRaw(t)
	returned, _, collectReturned := loggedConsumer(t, returnedUnexpected(), &fakeContainment{})
	panicking, _, collectPanicking := loggedConsumer(t, &fakeHandler{}, &fakeContainment{})
	panicking.Handle = panickingHandler

	_, _ = returned.Consume(underACallerSpan(t, returned.Tracer), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{})
	_, _ = consumeLikeTheWorker(t, underACallerSpan(t, panicking.Tracer), panicking, app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{})
	want := onlyConsumed(t, collectReturned())
	got := onlyConsumed(t, collectPanicking())

	if got.Severity() != log.SeverityError || got.Severity() != want.Severity() {
		t.Errorf("severity = %v, want %v, the severity of a returned Unexpected", got.Severity(), want.Severity())
	}
	requireRecord(t, got, map[string]attribute.Value{
		tracing.KeyOutcomeCategory:  attribute.StringValue(string(application.Unexpected)),
		tracing.KeyInboxDisposition: attribute.StringValue(application.R1D4.String()),
		tracing.KeyInboxGesture:     attribute.StringValue("ack"),
	})
	attributes := recordAttributes(got)
	if _, ok := attributes[tracing.KeyRequestID]; !ok {
		t.Errorf("%q carries no %s, want the execution of the attempt", messageConsumed, tracing.KeyRequestID)
	}
	if gotRest, wantRest := withoutOwnKeys(attributes), withoutOwnKeys(recordAttributes(want)); !maps.Equal(gotRest, wantRest) {
		t.Errorf("%q = %v, want the record of a returned Unexpected %v", messageConsumed, gotRest, wantRest)
	}
	for key, value := range attributes {
		if strings.Contains(value.String(), handlerPanicLeak) {
			t.Errorf("%q %s = %q, want no panic value (ERR-20)", messageConsumed, key, value.String())
		}
	}
}

func TestAPanickingHandlerFailsTheProcessLikeOneThatReturnsUnexpected(t *testing.T) {
	raw, _ := validRaw(t)
	returned, _, returnedSpans := tracedConsumer(returnedUnexpected().handle, &fakeContainment{}, sdktrace.AlwaysSample())
	panicking, _, panickingSpans := tracedConsumer(panickingHandler, &fakeContainment{}, sdktrace.AlwaysSample())

	_, _ = returned.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{})
	_, _ = consumeLikeTheWorker(t, context.Background(), panicking, app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{})
	want := onlyProcess(t, returnedSpans)
	got := onlyProcess(t, panickingSpans)

	if got.Status() != (sdktrace.Status{Code: codes.Error}) {
		t.Errorf("status = %v, want Error without a description (TRC-12)", got.Status())
	}
	requireAttributes(t, got, map[string]attribute.Value{
		string(semconv.ErrorTypeKey): attribute.StringValue(string(application.Unexpected)),
		tracing.KeyOutcomeCategory:   attribute.StringValue(string(application.Unexpected)),
		tracing.KeyInboxDisposition:  attribute.StringValue(application.R1D4.String()),
		tracing.KeyInboxGesture:      attribute.StringValue("ack"),
		keyContainmentReason:         attribute.StringValue(string(ports.ReasonTerminalFailure)),
	})
	if gotRest, wantRest := withoutOwnKeys(spanAttributes(got)), withoutOwnKeys(spanAttributes(want)); !maps.Equal(gotRest, wantRest) {
		t.Errorf("%q = %v, want the process of a returned Unexpected %v", got.Name(), gotRest, wantRest)
	}
	if len(got.Events()) != len(want.Events()) {
		t.Errorf("%q events = %v, want those of a returned Unexpected %v", got.Name(), got.Events(), want.Events())
	}
}

func TestAPanickingHandlerIsContainedAndAcknowledgedLikeOneThatReturnsUnexpected(t *testing.T) {
	raw, _ := validRaw(t)
	returnedContainment, returnedAck := &fakeContainment{}, &fakeAck{}
	wantOutcome, _ := newConsumer(returnedUnexpected(), returnedContainment, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, returnedAck)
	containment, ack := &fakeContainment{}, &fakeAck{}
	consumer := newConsumer(&fakeHandler{}, containment, 3)
	consumer.Handle = panickingHandler

	outcome, err := consumeLikeTheWorker(t, context.Background(), consumer, app.Delivery{Raw: raw, Attempt: 1}, ack)

	if outcome != wantOutcome || outcome.Reason != ports.ReasonTerminalFailure {
		t.Errorf("outcome = %+v, want %+v: R1×D4 contained as a terminal failure", outcome, wantOutcome)
	}
	if !reflect.DeepEqual(containment.contained, returnedContainment.contained) {
		t.Errorf("contained = %+v, want %+v", containment.contained, returnedContainment.contained)
	}
	if ack.acks != returnedAck.acks || ack.releases != returnedAck.releases || ack.acks != 1 {
		t.Errorf("ack = %d, release = %d; want %d and %d", ack.acks, ack.releases, returnedAck.acks, returnedAck.releases)
	}
	var failure *application.Failure
	if !errors.As(err, &failure) || failure.Category() != application.Unexpected || failure.Retryable() {
		t.Errorf("Consume() = %v, want a non-retryable Unexpected (ERR-22, ERR-24)", err)
	}
	if err != nil && strings.Contains(err.Error(), handlerPanicLeak) {
		t.Errorf("Consume() = %q, want no panic value (ERR-20)", err)
	}
}

func TestAPanickingHandlerIsTimedLikeOneThatReturnsUnexpected(t *testing.T) {
	raw, _ := validRaw(t)
	returned, returnedReader := meteredConsumer(returnedUnexpected())
	panicking, reader := meteredConsumer(&fakeHandler{})
	panicking.Handle = panickingHandler

	_, _ = returned.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{})
	_, _ = consumeLikeTheWorker(t, context.Background(), panicking, app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{})
	want, got := onlyDuration(t, returnedReader), onlyDuration(t, reader)

	if value, _ := got.Attributes.Value(semconv.ErrorTypeKey); value.AsString() != string(application.Unexpected) || !got.Attributes.Equals(&want.Attributes) {
		t.Errorf("messaging.process.duration attributes = %v, want those of a returned Unexpected %v", got.Attributes.ToSlice(), want.Attributes.ToSlice())
	}
}

// The runtime writes an unrecovered panic with its stack to the stderr of the
// process (go1.27 runtime/panic.go:734, printpanics), so only a process of its own shows it.
func TestConsumingAPanickingHandler(t *testing.T) {
	if os.Getenv(panicChildEnv) != panicChildConsume {
		t.Skip("runs only when TestAPanickingHandlerWritesNothingToTheStderrOfTheProcess starts it as a process of its own")
	}
	raw, _ := validRaw(t)
	consumer, _, collect := loggedConsumer(t, &fakeHandler{}, &fakeContainment{})
	consumer.Handle = panickingHandler

	if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err == nil {
		t.Fatal("Consume() = nil, want the Unexpected of the panicking handler")
	}
	onlyConsumed(t, collect())
}

func TestAPanickingHandlerWritesNothingToTheStderrOfTheProcess(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestConsumingAPanickingHandler$", "-test.count=1", "-test.v")
	command.Env = append(os.Environ(), panicChildEnv+"="+panicChildConsume)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr

	err := command.Run()

	if err != nil || stderr.Len() != 0 || !strings.Contains(stdout.String(), "--- PASS: TestConsumingAPanickingHandler") {
		t.Fatalf("process = %v, stderr = %q, stdout = %q, want the consumption to pass with nothing on stderr: the log leaves only by OTLP (RF-A1)", err, stderr.String(), stdout.String())
	}
}
