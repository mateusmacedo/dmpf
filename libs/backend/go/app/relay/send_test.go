package relay

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	testAddress    = "dmpf.orders.integration.v1"
	testTraceState = "congo=t61rcWkgMzE"
	testTenant     = "tenant-7"
)

type publishFunc func(ctx context.Context, destination string, message []byte) error

func (f publishFunc) Publish(ctx context.Context, destination string, message []byte) error {
	return f(ctx, destination, message)
}

// categorizedError carries a broker address in its text, which the send must
// never record: only the category leaves the process.
type categorizedError struct{ category, code string }

func (e categorizedError) Error() string         { return "broker 10.0.0.9:9092 refused the record" }
func (e categorizedError) ErrorCategory() string { return e.category }
func (e categorizedError) ErrorCode() string     { return e.code }

func sendRelay(store Store, publisher Publisher) (Relay, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(baggageCopier{}),
		sdktrace.WithSpanProcessor(recorder),
	)
	relay := loopRelay(store, publisher)
	relay.Tracer = provider.Tracer("relay-test")
	relay.System = "kafka"
	relay.Address = func(destination string) string {
		if destination == "orders.integration" {
			return testAddress
		}
		return ""
	}
	return relay, recorder
}

func recordOfTenant(t *testing.T, id int64, attempt int) postgres.Claimed {
	t.Helper()

	record := recordCreatedUnder(t, id, traceparentFor(int(id), "01"))
	record.AttemptCount = attempt
	record.Metadata = fmt.Appendf(nil,
		`{"correlationid":%q,"causationid":"caus-1","traceparent":%q,"tracestate":%q,"tenantid":%q}`,
		testCorrelation, traceparentFor(int(id), "01"), testTraceState, testTenant)
	return record
}

func sends(recorder *tracetest.SpanRecorder) []sdktrace.ReadOnlySpan {
	var found []sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if span.SpanKind() == trace.SpanKindClient && strings.HasPrefix(span.Name(), "send") {
			found = append(found, span)
		}
	}
	return found
}

func onlySend(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()

	found := sends(recorder)
	if len(found) != 1 {
		t.Fatalf("%d sends among %d ended spans, want exactly one", len(found), len(recorder.Ended()))
	}
	return found[0]
}

func stringOf(t *testing.T, span sdktrace.ReadOnlySpan, key string) string {
	t.Helper()

	value, ok := attributeOf(span.Attributes(), key)
	if !ok {
		t.Fatalf("%q carries no %s: %v", span.Name(), key, span.Attributes())
	}
	return value.AsString()
}

func TestTheEnvelopeLeavesWithTheCreationContextIntact(t *testing.T) {
	record := recordOfTenant(t, 1, 1)
	var published []byte
	publisher := publishFunc(func(_ context.Context, _ string, message []byte) error {
		published = append([]byte(nil), message...)
		return nil
	})
	relay, recorder := sendRelay(newFakeStore([]postgres.Claimed{record}), publisher)

	runBriefly(t, relay)
	send := onlySend(t, recorder)

	env, err := envelope.Unmarshal(published)
	if err != nil {
		t.Fatalf("Unmarshal() = %v, want the published envelope", err)
	}
	if env.TraceParent != traceparentFor(1, "01") {
		t.Fatalf("traceparent = %q, want the creation context %q untouched", env.TraceParent, traceparentFor(1, "01"))
	}
	if env.TraceState == nil || *env.TraceState != testTraceState {
		t.Fatalf("tracestate = %v, want %q untouched", env.TraceState, testTraceState)
	}
	if strings.Contains(env.TraceParent, send.SpanContext().SpanID().String()) {
		t.Fatalf("traceparent %q carries the send %s, want the creation context", env.TraceParent, send.SpanContext().SpanID())
	}
}

func TestTheSendIsAClientChildOfTheDrainLinkedToTheCreationContext(t *testing.T) {
	record := recordOfTenant(t, 1, 1)
	relay, recorder := sendRelay(newFakeStore([]postgres.Claimed{record}), &fakePublisher{})

	runBriefly(t, relay)
	drain := onlyDrain(t, recorder)
	send := onlySend(t, recorder)

	if send.Name() != "send "+testAddress {
		t.Fatalf("name = %q, want %q", send.Name(), "send "+testAddress)
	}
	if !send.Parent().Equal(drain.SpanContext()) {
		t.Fatalf("parent = %v, want the drain %v", send.Parent(), drain.SpanContext())
	}
	links := send.Links()
	if len(links) != 1 || links[0].SpanContext.TraceID().String() != fmt.Sprintf("%032x", 0xa1) ||
		links[0].SpanContext.SpanID().String() != fmt.Sprintf("%016x", 0xb1) {
		t.Fatalf("links = %v, want one to the creation context in the metadata", links)
	}

	want := map[string]string{
		"messaging.system":                  "kafka",
		"messaging.operation.type":          "send",
		"messaging.operation.name":          "send",
		"messaging.destination.name":        testAddress,
		"messaging.message.id":              record.MessageID,
		"messaging.message.conversation_id": testCorrelation,
		"cloudevents.event_id":              record.MessageID,
		"cloudevents.event_source":          testSource,
		"cloudevents.event_type":            record.MessageType,
		tracing.KeyTenantID:                 testTenant,
		tracing.KeyOutcomeCategory:          "ok",
	}
	for key, value := range want {
		if got := stringOf(t, send, key); got != value {
			t.Fatalf("%s = %q, want %q", key, got, value)
		}
	}
	if got, _ := attributeOf(send.Attributes(), tracing.KeyOutboxAttempt); got.AsInt64() != 1 {
		t.Fatalf("%s = %v, want 1", tracing.KeyOutboxAttempt, got.String())
	}
	for _, absent := range []string{tracing.KeyCorrelationID, "error.type"} {
		if got, ok := attributeOf(send.Attributes(), absent); ok {
			t.Fatalf("send carries %s = %q, want it absent", absent, got.String())
		}
	}
	if send.Status().Code != codes.Unset {
		t.Fatalf("status = %v, want unset on a delivered message", send.Status())
	}
}

func TestASendWithoutAnAddressNeverNamesTheLogicalChannel(t *testing.T) {
	record := recordOfTenant(t, 1, 1)
	relay, recorder := sendRelay(newFakeStore([]postgres.Claimed{record}), &fakePublisher{})
	relay.Address = nil

	runBriefly(t, relay)
	send := onlySend(t, recorder)

	if send.Name() != "send" {
		t.Fatalf("name = %q, want %q", send.Name(), "send")
	}
	if got, ok := attributeOf(send.Attributes(), "messaging.destination.name"); ok {
		t.Fatalf("messaging.destination.name = %q, want it absent rather than the logical channel", got.String())
	}
}

func TestTheSendHasARequestIDOfItsOwnThatItsChildrenInherit(t *testing.T) {
	record := recordOfTenant(t, 1, 1)
	var relay Relay
	publisher := publishFunc(func(ctx context.Context, _ string, _ []byte) error {
		_, child := relay.Tracer.Start(ctx, "produce")
		child.End()
		return nil
	})
	relay, recorder := sendRelay(newFakeStore([]postgres.Claimed{record}), publisher)

	runBriefly(t, relay)
	drain := onlyDrain(t, recorder)
	send := onlySend(t, recorder)

	drainRequest := stringOf(t, drain, tracing.KeyRequestID)
	sendRequest := stringOf(t, send, tracing.KeyRequestID)
	if sendRequest == drainRequest {
		t.Fatalf("send %s = the drain's %q, want one of its own", tracing.KeyRequestID, drainRequest)
	}

	var child sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if span.Name() == "produce" {
			child = span
		}
	}
	if child == nil || !child.Parent().Equal(send.SpanContext()) {
		t.Fatalf("child = %v, want a span under the send", child)
	}
	inherited := map[string]string{
		tracing.KeyRequestID:     sendRequest,
		tracing.KeyCorrelationID: testCorrelation,
		tracing.KeyTenantID:      testTenant,
	}
	for key, value := range inherited {
		if got := stringOf(t, child, key); got != value {
			t.Fatalf("child %s = %q, want %q from the send's baggage", key, got, value)
		}
	}
}

func TestTheSendIsTheSpanThePublishCallOwns(t *testing.T) {
	record := recordOfTenant(t, 1, 1)
	var owned trace.SpanContext
	publisher := publishFunc(func(ctx context.Context, _ string, _ []byte) error {
		span, ok := tracing.OwnsSpan(ctx)
		if !ok {
			return errors.New("the publish call owns no span")
		}
		owned = span.SpanContext()
		span.SetAttributes(tracing.Attributes{}.OutcomeCategory("not_leader_for_partition").KeyValues()...)
		tracing.RecordError(span, "not_leader_for_partition")
		return errTransport
	})
	relay, recorder := sendRelay(newFakeStore([]postgres.Claimed{record}), publisher)

	runBriefly(t, relay)
	send := onlySend(t, recorder)

	if !owned.Equal(send.SpanContext()) {
		t.Fatalf("the publish call owned %v, want the send %v", owned, send.SpanContext())
	}
	for _, key := range []string{tracing.KeyOutcomeCategory, "error.type"} {
		if got := stringOf(t, send, key); got != "not_leader_for_partition" {
			t.Fatalf("%s = %q, want the publisher's category kept", key, got)
		}
	}
}

func TestAFailedSendIsMarkedAndTheRetryOpensANewOne(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"categorized failure", categorizedError{category: "network"}, "network"},
		{"unclassified failure", errTransport, semconv.ErrorTypeOther.Value.AsString()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var mu sync.Mutex
			calls := 0
			publisher := publishFunc(func(context.Context, string, []byte) error {
				mu.Lock()
				defer mu.Unlock()
				calls++
				if calls == 1 {
					return c.err
				}
				return nil
			})
			store := newFakeStore(
				[]postgres.Claimed{recordOfTenant(t, 1, 1)},
				[]postgres.Claimed{recordOfTenant(t, 1, 2)},
			)
			relay, recorder := sendRelay(store, publisher)

			runBriefly(t, relay)
			found := sends(recorder)
			if len(found) != 2 {
				t.Fatalf("%d sends, want one per attempt", len(found))
			}

			failed, retried := found[0], found[1]
			if failed.Status().Code != codes.Error || failed.Status().Description != "" {
				t.Fatalf("failed status = %+v, want Error without a description", failed.Status())
			}
			for _, key := range []string{"error.type", tracing.KeyOutcomeCategory} {
				if got := stringOf(t, failed, key); got != c.want {
					t.Fatalf("failed %s = %q, want %q", key, got, c.want)
				}
			}
			if got, _ := attributeOf(failed.Attributes(), tracing.KeyOutboxAttempt); got.AsInt64() != 1 {
				t.Fatalf("failed %s = %v, want 1", tracing.KeyOutboxAttempt, got.String())
			}

			if got, _ := attributeOf(retried.Attributes(), tracing.KeyOutboxAttempt); got.AsInt64() != 2 {
				t.Fatalf("retried %s = %v, want 2", tracing.KeyOutboxAttempt, got.String())
			}
			if retried.Status().Code != codes.Unset || stringOf(t, retried, tracing.KeyOutcomeCategory) != "ok" {
				t.Fatalf("retried status = %+v outcome = %q, want a delivered send",
					retried.Status(), stringOf(t, retried, tracing.KeyOutcomeCategory))
			}
			if retried.SpanContext().SpanID() == failed.SpanContext().SpanID() {
				t.Fatal("the retry reused the failed send, want a new one")
			}
		})
	}
}

func TestARecordThatCannotBeAssembledFailsItsSend(t *testing.T) {
	record := recordOfTenant(t, 1, 1)
	record.PayloadHash = "sha256:tampered"
	publisher := &fakePublisher{}
	relay, recorder := sendRelay(newFakeStore([]postgres.Claimed{record}), publisher)

	runBriefly(t, relay)
	send := onlySend(t, recorder)

	if publisher.delivered.Load() != 0 {
		t.Fatalf("%d publications, want none of a tampered record", publisher.delivered.Load())
	}
	if send.Status().Code != codes.Error || stringOf(t, send, "error.type") != semconv.ErrorTypeOther.Value.AsString() {
		t.Fatalf("status = %+v error.type = %q, want the send failed as _OTHER",
			send.Status(), stringOf(t, send, "error.type"))
	}
}

func TestASendOfAnUnreadableCreationContextLeavesWithoutALinkAndPublishes(t *testing.T) {
	invalid := recordCreatedUnder(t, 1, "not-a-traceparent")
	valid := recordCreatedUnder(t, 2, traceparentFor(2, "01"))
	var mu sync.Mutex
	published := map[string]int{}
	publisher := publishFunc(func(_ context.Context, _ string, message []byte) error {
		env, err := envelope.Unmarshal(message)
		if err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		published[env.ID]++
		return nil
	})
	store := newFakeStore([]postgres.Claimed{invalid, valid})
	relay, recorder := sendRelay(store, publisher)

	runBriefly(t, relay)

	byMessage := map[string]sdktrace.ReadOnlySpan{}
	for _, send := range sends(recorder) {
		byMessage[stringOf(t, send, "messaging.message.id")] = send
	}
	if len(byMessage) != 2 || byMessage[invalid.MessageID] == nil || byMessage[valid.MessageID] == nil {
		t.Fatalf("sends = %v, want one per message of the batch", byMessage)
	}
	if links := byMessage[invalid.MessageID].Links(); len(links) != 0 {
		t.Fatalf("send of %s links %v, want no link to an unreadable creation context", invalid.MessageID, links)
	}
	links := byMessage[valid.MessageID].Links()
	if len(links) != 1 || links[0].SpanContext.TraceID().String() != fmt.Sprintf("%032x", 0xa2) ||
		links[0].SpanContext.SpanID().String() != fmt.Sprintf("%016x", 0xb2) {
		t.Fatalf("send of %s links %v, want one to its creation context", valid.MessageID, links)
	}

	mu.Lock()
	defer mu.Unlock()
	if published[invalid.MessageID] != 1 || published[valid.MessageID] != 1 {
		t.Fatalf("published = %v, want each message of the batch once", published)
	}
	outcomes := map[int64]string{}
	for _, recorded := range store.recorded() {
		outcomes[recorded.id] = recorded.kind
	}
	if outcomes[invalid.ID] != "published" || outcomes[valid.ID] != "published" {
		t.Fatalf("outcomes = %v, want %d and %d published", outcomes, invalid.ID, valid.ID)
	}
}
