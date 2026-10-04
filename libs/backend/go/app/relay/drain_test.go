package relay

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func tracedRelay(store Store, sampler sdktrace.Sampler) (Relay, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithSpanProcessor(recorder),
	)
	relay := loopRelay(store, &fakePublisher{})
	relay.Tracer = provider.Tracer("relay-test")
	return relay, recorder
}

func runBriefly(t *testing.T, relay Relay) {
	t.Helper()
	runBrieflyUnder(t, context.Background(), relay)
}

func runBrieflyUnder(t *testing.T, parent context.Context, relay Relay) {
	t.Helper()

	ctx, cancel := context.WithTimeout(parent, 120*time.Millisecond)
	defer cancel()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
}

func traceparentFor(n int, flags string) string {
	return fmt.Sprintf("00-%032x-%016x-%s", 0xa0+n, 0xb0+n, flags)
}

func recordCreatedUnder(t *testing.T, id int64, traceparent string) postgres.Claimed {
	t.Helper()

	record := publishableRecord(t)
	record.ID = id
	record.MessageID = fmt.Sprintf("msg-%d", id)
	record.Metadata = fmt.Appendf(nil,
		`{"correlationid":"corr-1","causationid":"caus-1","traceparent":%q}`, traceparent)
	return record
}

func underACallerSpan() (context.Context, trace.SpanContext) {
	caller := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0xc1},
		SpanID:     trace.SpanID{0xc2},
		TraceFlags: trace.FlagsSampled,
	})
	return trace.ContextWithSpanContext(context.Background(), caller), caller
}

func onlyDrain(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()

	var drains []sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if strings.HasPrefix(span.Name(), drainName) {
			drains = append(drains, span)
		}
	}
	if len(drains) != 1 {
		t.Fatalf("%d drains among %d ended spans, want exactly one", len(drains), len(recorder.Ended()))
	}
	return drains[0]
}

func attributeOf(attributes []attribute.KeyValue, key string) (attribute.Value, bool) {
	for _, kv := range attributes {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func eventNames(span sdktrace.ReadOnlySpan) []string {
	names := make([]string, 0, len(span.Events()))
	for _, event := range span.Events() {
		names = append(names, event.Name)
	}
	return names
}

func countOf(names []string, name string) int {
	n := 0
	for _, candidate := range names {
		if candidate == name {
			n++
		}
	}
	return n
}

func TestADrainLinksEveryMessageToItsCreationContext(t *testing.T) {
	batch := []postgres.Claimed{
		recordCreatedUnder(t, 1, traceparentFor(1, "01")),
		recordCreatedUnder(t, 2, traceparentFor(2, "01")),
		recordCreatedUnder(t, 3, traceparentFor(3, "01")),
	}
	store := newFakeStore(batch)
	relay, recorder := tracedRelay(store, sdktrace.AlwaysSample())

	parent, caller := underACallerSpan()
	before := time.Now()
	runBrieflyUnder(t, parent, relay)
	drain := onlyDrain(t, recorder)

	if drain.Name() != "outbox drain orders.integration" {
		t.Fatalf("name = %q, want %q", drain.Name(), "outbox drain orders.integration")
	}
	if drain.SpanKind() != trace.SpanKindInternal {
		t.Fatalf("kind = %v, want internal", drain.SpanKind())
	}
	if drain.Parent().IsValid() || drain.SpanContext().TraceID() == caller.TraceID() {
		t.Fatalf("parent = %v in trace %s, want a new root apart from the caller's", drain.Parent(), drain.SpanContext().TraceID())
	}
	if start := drain.StartTime(); start.Before(before) || start.After(store.claimedAt[0]) {
		t.Fatalf("start = %v, want the instant the claim began, in [%v, %v]", start, before, store.claimedAt[0])
	}

	links := drain.Links()
	if len(links) != len(batch) {
		t.Fatalf("%d links, want one per message (%d)", len(links), len(batch))
	}
	for i, link := range links {
		want := batch[i]
		if got := link.SpanContext.TraceID().String(); got != fmt.Sprintf("%032x", 0xa0+want.ID) {
			t.Fatalf("link %d trace id = %s, want the one in its metadata", i, got)
		}
		if got := link.SpanContext.SpanID().String(); got != fmt.Sprintf("%016x", 0xb0+want.ID) {
			t.Fatalf("link %d span id = %s, want the one in its metadata", i, got)
		}
		if got, _ := attributeOf(link.Attributes, "messaging.message.id"); got.AsString() != want.MessageID {
			t.Fatalf("link %d messaging.message.id = %q, want %q", i, got.AsString(), want.MessageID)
		}
	}

	attributes := drain.Attributes()
	if got, _ := attributeOf(attributes, "messaging.batch.message_count"); got.AsInt64() != 3 {
		t.Fatalf("messaging.batch.message_count = %v, want 3", got.String())
	}
	if got, _ := attributeOf(attributes, tracing.KeyOutboxClaimID); got.AsString() != store.claimIDs[0] {
		t.Fatalf("%s = %q, want the claim %q", tracing.KeyOutboxClaimID, got.AsString(), store.claimIDs[0])
	}
	if got, _ := attributeOf(attributes, tracing.KeyTrafficClass); got.AsString() != string(tracing.ClassWrite) {
		t.Fatalf("%s = %q, want %q", tracing.KeyTrafficClass, got.AsString(), tracing.ClassWrite)
	}
	if names := eventNames(drain); countOf(names, tracing.EventClaimed) != 1 {
		t.Fatalf("events = %v, want one %q", names, tracing.EventClaimed)
	}
}

func TestAnInvalidCreationContextLeavesNoLinkAndAnEvent(t *testing.T) {
	cases := []struct {
		name     string
		metadata string
		outcome  string
	}{
		{"traceparent that does not parse", `{"correlationid":"corr-1","causationid":"caus-1","traceparent":"not-a-traceparent"}`, "published"},
		{"metadata that is not an object", `[]`, "failed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			invalid := recordCreatedUnder(t, 1, traceparentFor(1, "01"))
			invalid.Metadata = []byte(c.metadata)
			valid := recordCreatedUnder(t, 2, traceparentFor(2, "01"))
			store := newFakeStore([]postgres.Claimed{invalid, valid})
			relay, recorder := tracedRelay(store, sdktrace.AlwaysSample())

			runBriefly(t, relay)
			drain := onlyDrain(t, recorder)

			links := drain.Links()
			if len(links) != 1 {
				t.Fatalf("%d links, want only the valid message's", len(links))
			}
			if got, _ := attributeOf(links[0].Attributes, "messaging.message.id"); got.AsString() != valid.MessageID {
				t.Fatalf("link messaging.message.id = %q, want %q", got.AsString(), valid.MessageID)
			}
			if names := eventNames(drain); countOf(names, tracing.EventInvalidCreationContext) != 1 {
				t.Fatalf("events = %v, want one %q", names, tracing.EventInvalidCreationContext)
			}
			if got, _ := attributeOf(drain.Attributes(), "messaging.batch.message_count"); got.AsInt64() != 2 {
				t.Fatalf("messaging.batch.message_count = %v, want 2", got.String())
			}
			outcomes := map[int64]string{}
			for _, recorded := range store.recorded() {
				outcomes[recorded.id] = recorded.kind
			}
			if outcomes[invalid.ID] != c.outcome || outcomes[valid.ID] != "published" {
				t.Fatalf("outcomes = %v, want %d %s and %d published", outcomes, invalid.ID, c.outcome, valid.ID)
			}
		})
	}
}

func TestABatchOfMixedDestinationsOpensAnUnqualifiedDrain(t *testing.T) {
	first := recordCreatedUnder(t, 1, traceparentFor(1, "01"))
	second := recordCreatedUnder(t, 2, traceparentFor(2, "01"))
	second.Destination = "orders.audit"
	store := newFakeStore([]postgres.Claimed{first, second})
	relay, recorder := tracedRelay(store, sdktrace.AlwaysSample())

	runBriefly(t, relay)

	if got := onlyDrain(t, recorder).Name(); got != "outbox drain" {
		t.Fatalf("name = %q, want %q", got, "outbox drain")
	}
}

func TestAnEmptyClaimOpensNoSpan(t *testing.T) {
	store := newFakeStore()
	relay, recorder := tracedRelay(store, sdktrace.AlwaysSample())

	runBriefly(t, relay)

	if store.claims.Load() == 0 {
		t.Fatal("the loop never claimed, so the absence of a span proves nothing")
	}
	if started := recorder.Started(); len(started) != 0 {
		t.Fatalf("%d spans started on empty claims, want none", len(started))
	}
	for i, span := range store.claimSpans {
		if span.IsValid() {
			t.Fatalf("claim %d ran under span %v, want no span around the claim query", i, span)
		}
	}
}

// samplerSpy keeps what the SDK hands the sampler at the drain's creation. The
// decision on those inputs is otelboot's, proven by its own sampler tests.
type samplerSpy struct {
	mu        sync.Mutex
	seen      []sdktrace.SamplingParameters
	delegated sdktrace.Sampler
}

func (s *samplerSpy) ShouldSample(parameters sdktrace.SamplingParameters) sdktrace.SamplingResult {
	s.mu.Lock()
	s.seen = append(s.seen, parameters)
	s.mu.Unlock()
	return s.delegated.ShouldSample(parameters)
}

func (s *samplerSpy) Description() string { return "samplerSpy" }

func TestADrainHandsTheSamplerItsClassAndItsCreationContexts(t *testing.T) {
	sampled := recordCreatedUnder(t, 1, traceparentFor(1, "01"))
	unsampled := recordCreatedUnder(t, 2, traceparentFor(2, "00"))
	store := newFakeStore([]postgres.Claimed{sampled, unsampled})
	spy := &samplerSpy{delegated: sdktrace.AlwaysSample()}
	relay, _ := tracedRelay(store, spy)

	parent, _ := underACallerSpan()
	runBrieflyUnder(t, parent, relay)

	var drains []sdktrace.SamplingParameters
	for _, seen := range spy.seen {
		if strings.HasPrefix(seen.Name, drainName) {
			drains = append(drains, seen)
		}
	}
	if len(drains) != 1 {
		t.Fatalf("the sampler was asked %d times for a drain, want once", len(drains))
	}
	parameters := drains[0]
	if trace.SpanContextFromContext(parameters.ParentContext).IsValid() {
		t.Fatal("the sampler saw a parent, want a new root")
	}
	if got, _ := attributeOf(parameters.Attributes, tracing.KeyTrafficClass); got.AsString() != string(tracing.ClassWrite) {
		t.Fatalf("sampler saw %s = %q, want %q", tracing.KeyTrafficClass, got.AsString(), tracing.ClassWrite)
	}
	if len(parameters.Links) != 2 {
		t.Fatalf("sampler saw %d links, want both creation contexts", len(parameters.Links))
	}
	if !parameters.Links[0].SpanContext.IsSampled() || parameters.Links[1].SpanContext.IsSampled() {
		t.Fatalf("sampler saw links sampled = [%v %v], want the flags of each metadata [true false]",
			parameters.Links[0].SpanContext.IsSampled(), parameters.Links[1].SpanContext.IsSampled())
	}
}

func TestTheTransitionsRunUnderTheDrainWithItsOwnRequestID(t *testing.T) {
	published := recordCreatedUnder(t, 1, traceparentFor(1, "01"))
	store := newFakeStore([]postgres.Claimed{published})
	relay, recorder := tracedRelay(store, sdktrace.AlwaysSample())

	runBriefly(t, relay)
	drain := onlyDrain(t, recorder)

	requestID, _ := attributeOf(drain.Attributes(), tracing.KeyRequestID)
	if requestID.AsString() == "" {
		t.Fatalf("drain carries no %s", tracing.KeyRequestID)
	}
	if requestID.AsString() == store.claimIDs[0] {
		t.Fatalf("%s = the claim id %q, want an identity of the drain's own", tracing.KeyRequestID, store.claimIDs[0])
	}

	recorded := store.recorded()
	if len(recorded) != 1 {
		t.Fatalf("%d transitions, want 1: %+v", len(recorded), recorded)
	}
	assertUnderDrain(t, recorded[0], drain, requestID.AsString())
}

func TestTerminalRescheduleAndReleaseRunUnderTheDrain(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*fakeStore, *fakePublisher, *postgres.Claimed)
		want    string
	}{
		{"fail", func(_ *fakeStore, _ *fakePublisher, record *postgres.Claimed) {
			record.PayloadHash = "sha256:tampered"
		}, "failed"},
		{"reschedule", func(_ *fakeStore, publisher *fakePublisher, _ *postgres.Claimed) {
			publisher.err = errTransport
		}, "rescheduled"},
		{"release", func(store *fakeStore, _ *fakePublisher, _ *postgres.Claimed) {
			store.failMarkPublished = true
		}, "rescheduled"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			record := recordCreatedUnder(t, 1, traceparentFor(1, "01"))
			store := newFakeStore()
			publisher := &fakePublisher{}
			c.prepare(store, publisher, &record)
			store.batches = [][]postgres.Claimed{{record}}

			relay, recorder := tracedRelay(store, sdktrace.AlwaysSample())
			relay.Publisher = publisher
			runBriefly(t, relay)
			drain := onlyDrain(t, recorder)
			requestID, _ := attributeOf(drain.Attributes(), tracing.KeyRequestID)

			recorded := store.recorded()
			if len(recorded) != 1 || recorded[0].kind != c.want {
				t.Fatalf("transitions = %+v, want one %q", recorded, c.want)
			}
			assertUnderDrain(t, recorded[0], drain, requestID.AsString())
		})
	}
}

func assertUnderDrain(t *testing.T, recorded transition, drain sdktrace.ReadOnlySpan, requestID string) {
	t.Helper()

	if !recorded.span.Equal(drain.SpanContext()) {
		t.Fatalf("%s ran under span %v, want the drain %v", recorded.kind, recorded.span, drain.SpanContext())
	}
	if recorded.requestID != requestID {
		t.Fatalf("%s baggage %s = %q, want the drain's %q", recorded.kind, tracing.KeyRequestID, recorded.requestID, requestID)
	}
}
