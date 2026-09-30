package ports_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestIdempotencyKeyPatternIsTheFormatBothEdgesAccept(t *testing.T) {
	format := regexp.MustCompile(ports.IdempotencyKeyPattern)
	accepted := []string{"k", "k-1", "a.b_c-D9", strings.Repeat("x", 128)}
	for _, key := range accepted {
		if !format.MatchString(key) || !ports.ValidIdempotencyKey(key) {
			t.Errorf("key %q refused; want accepted", key)
		}
	}
	refused := []string{"", strings.Repeat("x", 129), "k 1", "k/1", "chave-ç", "k\n"}
	for _, key := range refused {
		if format.MatchString(key) || ports.ValidIdempotencyKey(key) {
			t.Errorf("key %q accepted; want refused", key)
		}
	}
}

func TestIdempotencyKeyCarrierRoundTripsTheKey(t *testing.T) {
	ctx := ports.WithIdempotencyKey(context.Background(), "k-42")

	key, ok := ports.IdempotencyKeyFrom(ctx)
	if !ok || key != "k-42" {
		t.Fatalf("IdempotencyKeyFrom = %q, %v; want %q, true", key, ok, "k-42")
	}
}

func TestIdempotencyKeyCarrierReportsAbsenceRatherThanAnEmptyKey(t *testing.T) {
	key, ok := ports.IdempotencyKeyFrom(context.Background())
	if ok || key != "" {
		t.Fatalf("IdempotencyKeyFrom on an empty carrier = %q, %v; want \"\", false", key, ok)
	}
}

func TestIdempotencyKeyIsNotCarriedByTheExecutionContext(t *testing.T) {
	ctx := ports.WithIdempotencyKey(context.Background(), "k-42")
	if _, ok := ports.ExecutionContextFrom(ctx); ok {
		t.Fatal("depositing the key produced an execution context; the key must travel apart from the nine CTX-01 fields")
	}
}

func TestIdempotencyOutcomeIsUnsetUntilMarked(t *testing.T) {
	ctx := ports.WithIdempotencySlot(context.Background())

	if outcome, ok := ports.IdempotencyOutcomeFrom(ctx); ok {
		t.Fatalf("fresh slot reports %v; want no outcome", outcome)
	}

	ports.MarkIdempotency(ctx, ports.IdempotencyReplayed)
	if outcome, ok := ports.IdempotencyOutcomeFrom(ctx); !ok || outcome != ports.IdempotencyReplayed {
		t.Fatalf("after marking replayed = %v, %v; want replayed, true", outcome, ok)
	}
}

func TestIdempotencyMarkReachesTheContextThatInstalledTheSlot(t *testing.T) {
	type key struct{}
	edge := ports.WithIdempotencySlot(context.Background())
	derived, cancel := context.WithCancel(context.WithValue(edge, key{}, "downstream"))
	defer cancel()

	ports.MarkIdempotency(derived, ports.IdempotencyNew)

	if outcome, ok := ports.IdempotencyOutcomeFrom(edge); !ok || outcome != ports.IdempotencyNew {
		t.Fatalf("edge reads %v, %v; want new, true: the edge reads what the use case marked downstream", outcome, ok)
	}
}

func TestIdempotencyMarkWithoutASlotIsIgnored(t *testing.T) {
	ctx := context.Background()
	ports.MarkIdempotency(ctx, ports.IdempotencyMismatch)

	if outcome, ok := ports.IdempotencyOutcomeFrom(ctx); ok {
		t.Fatalf("no slot installed, yet the outcome reads %v", outcome)
	}
}

func TestIdempotencyOutcomeNamesTheSpanValues(t *testing.T) {
	names := map[ports.IdempotencyOutcome]string{
		ports.IdempotencyNew:      "new",
		ports.IdempotencyReplayed: "replayed",
		ports.IdempotencyMismatch: "mismatch",
		ports.IdempotencyInFlight: "in_flight",
	}
	for outcome, want := range names {
		if got := outcome.String(); got != want {
			t.Errorf("%d.String() = %q; want %q", outcome, got, want)
		}
	}
	if got := ports.IdempotencyOutcome(0).String(); got != "" {
		t.Errorf("zero outcome names itself %q; want empty", got)
	}
}
