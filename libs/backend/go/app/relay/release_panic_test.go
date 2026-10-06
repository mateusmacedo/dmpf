package relay

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const releaseFailed = "outbox release failed"

type panickingTracer struct{ trace.Tracer }

func (panickingTracer) Start(context.Context, string, ...trace.SpanStartOption) (context.Context, trace.Span) {
	panic(relayPanicValue)
}

type panickingRelease struct{ *fakeStore }

func (panickingRelease) Reschedule(context.Context, int64, string, ports.Instant, string) (int64, error) {
	panic(relayPanicValue)
}

type refusingRelease struct {
	*fakeStore
	err error
}

func (s refusingRelease) Reschedule(context.Context, int64, string, ports.Instant, string) (int64, error) {
	return 0, s.err
}

func unsettledStore(batch []postgres.Claimed) *fakeStore {
	store := newFakeStore(batch)
	store.failMarkPublished = true
	return store
}

func TestAPanicOpeningTheDrainEndsRunWithTheErrorAndHandsTheClaimsBack(t *testing.T) {
	store := newFakeStore(batchOf(t, 2))
	publisher := &fakePublisher{}
	relay := loopRelay(store, publisher)
	relay.Tracer = panickingTracer{noop.NewTracerProvider().Tracer("")}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := relay.Run(ctx); !errors.Is(err, ErrPanicked) {
		t.Fatalf("Run() = %v, want ErrPanicked (RF-A1, ERR-23)", err)
	}
	released := map[int64]bool{}
	for _, transition := range store.recorded() {
		if transition.kind == "rescheduled" && transition.lastError == releaseReason {
			released[transition.id] = true
		}
	}
	if len(released) != 2 || publisher.delivered.Load() != 0 {
		t.Fatalf("released %v after %d deliveries, want both claims handed back before any delivery (OBX-13)", released, publisher.delivered.Load())
	}
}

func TestAPanicReleasingTheClaimsStillEndsTheDrain(t *testing.T) {
	relay, recorder := tracedRelay(panickingRelease{unsettledStore(batchOf(t, 1))}, sdktrace.AlwaysSample())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := relay.Run(ctx); !errors.Is(err, ErrPanicked) {
		t.Fatalf("Run() = %v, want ErrPanicked (RF-A1, ERR-23)", err)
	}
	onlyDrain(t, recorder)
}

func TestAReleaseTheStoreFailsIsLoggedUnderTheDrain(t *testing.T) {
	failure := categorizedError{category: "TransientDependency", code: "pg-unavailable"}
	batch := batchOf(t, 1)
	store := refusingRelease{fakeStore: unsettledStore(batch), err: failure}
	traced, recorder := tracedRelay(store, sdktrace.AlwaysSample())
	relay, collect := withLogger(t, traced)

	runBriefly(t, relay)
	logged := recordsNamed(collect(), releaseFailed)
	drain := onlyDrain(t, recorder)

	if len(logged) != 1 {
		t.Fatalf("%d %q records, want one per claim the store failed to hand back", len(logged), releaseFailed)
	}
	if logged[0].Severity() != log.SeverityWarn {
		t.Errorf("%q severity = %v, want %v", releaseFailed, logged[0].Severity(), log.SeverityWarn)
	}
	if logged[0].TraceID() != drain.SpanContext().TraceID() || logged[0].SpanID() != drain.SpanContext().SpanID() {
		t.Errorf("%q logged under %s/%s, want the drain %s/%s", releaseFailed,
			logged[0].TraceID(), logged[0].SpanID(), drain.SpanContext().TraceID(), drain.SpanContext().SpanID())
	}
	requireLogged(t, logged[0], map[string]attribute.Value{
		string(semconv.MessagingMessageIDKey): attribute.StringValue(batch[0].MessageID),
		redact.KeyErrorType:                   attribute.StringValue(failure.category),
		redact.KeyErrorCode:                   attribute.StringValue(failure.code),
	})
	requireNoErrorText(t, logged[0], "10.0.0.9")
}

func TestARejectedReleaseIsNotLogged(t *testing.T) {
	relay, collect := withLogger(t, loopRelay(refusingRelease{fakeStore: unsettledStore(batchOf(t, 1))}, &fakePublisher{}))

	runBriefly(t, relay)

	if logged := recordsNamed(collect(), releaseFailed); len(logged) != 0 {
		t.Fatalf("%d %q records for a release the claim already replaced, want none (OBX-10)", len(logged), releaseFailed)
	}
}
