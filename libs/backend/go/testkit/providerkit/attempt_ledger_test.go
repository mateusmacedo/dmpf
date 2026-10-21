package providerkit_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const (
	clauseBeginSerializes = "concurrent begins on one key start exactly one attempt"
	clauseBegunNotPurged  = "a purge removes finished attempts past the cutoff and never a begun one"
	clauseObserveCap      = "an observation past the cap is saturated and records nothing"
	clauseTerminalApplies = "a terminal finding on a full row with none terminal yet is applied, not saturated"
	clauseFinishConflicts = "a contradicting finish past the cap of conflicts is saturated and keeps the class"
	clauseFinishObserved  = "a contradicting finish on a row full of observations is saturated"
)

func memoryAttemptLedger() providerkit.AttemptLedgerSubject {
	return providerkit.AttemptLedgerSubject{Ledger: memory.NewAttemptStore(memory.New())}
}

func TestMemoryAttemptLedgerConforms(t *testing.T) {
	v := providerkit.AttemptLedger(memoryAttemptLedger)
	tb.Require(t, v)
	if want := []string{clauseBegunNotPurged}; !slices.Equal(v.Skipped, want) {
		t.Fatalf("skipped = %v, want %v: memory keeps no retention", v.Skipped, want)
	}
}

type alwaysStarting struct{ ports.AttemptLedger }

func (l alwaysStarting) Begin(ctx context.Context, key ports.OutboundKey, message ports.MessageID, now ports.Instant) (ports.AttemptStart, error) {
	start, err := l.AttemptLedger.Begin(ctx, key, message, now)
	if err != nil {
		return start, err
	}
	asStarted := func(a ports.Attempt) error { start = ports.StartedAttempt(a); return nil }
	err = start.Match(asStarted, asStarted, func(ports.Attempt, ports.ExternalClass) error { return nil })
	return start, err
}

type uncapped struct{ ports.AttemptLedger }

func (l uncapped) Observe(ctx context.Context, o ports.Observation, now ports.Instant) (ports.Observed, error) {
	got, err := l.AttemptLedger.Observe(ctx, o, now)
	if got.Result == ports.ObserveSaturated {
		got.Result = ports.ObserveRecorded
	}
	return got, err
}

func (l uncapped) Finish(ctx context.Context, o ports.AttemptOutcome) (ports.Finished, error) {
	got, err := l.AttemptLedger.Finish(ctx, o)
	if got.Result == ports.FinishSaturated {
		got.Result = ports.FinishConflict
	}
	return got, err
}

type saturatingEveryNewID struct {
	ports.AttemptLedger
	mu   sync.Mutex
	seen map[string]bool
	last ports.Observed
}

func (l *saturatingEveryNewID) Observe(ctx context.Context, o ports.Observation, now ports.Instant) (ports.Observed, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.seen[o.ID] && len(l.seen) >= ports.MaxAttemptEntries {
		saturated := l.last
		saturated.Result = ports.ObserveSaturated
		return saturated, nil
	}
	got, err := l.AttemptLedger.Observe(ctx, o, now)
	if err == nil && got.Result != ports.ObserveSaturated {
		l.seen[o.ID], l.last = true, got
	}
	return got, err
}

type resettable struct{ ports.AttemptLedger }

func purgingEverything() providerkit.AttemptLedgerSubject {
	ledger := &resettable{AttemptLedger: memory.NewAttemptStore(memory.New())}
	return providerkit.AttemptLedgerSubject{
		Ledger: ledger,
		Purge: func(ports.Instant) (int64, error) {
			ledger.AttemptLedger = memory.NewAttemptStore(memory.New())
			return 2, nil
		},
	}
}

func purgingTheBegunToo() providerkit.AttemptLedgerSubject {
	ledger := &resettable{AttemptLedger: memory.NewAttemptStore(memory.New())}
	return providerkit.AttemptLedgerSubject{
		Ledger: ledger,
		Purge: func(before ports.Instant) (int64, error) {
			if before <= 10 {
				return 0, nil
			}
			ledger.AttemptLedger = memory.NewAttemptStore(memory.New())
			return 1, nil
		},
	}
}

func TestAttemptLedgerRejectsAPurgeOfTheBegunOnItsOwnAssertion(t *testing.T) {
	v := providerkit.AttemptLedger(purgingTheBegunToo)
	var purge []providerkit.Diagnostic
	for _, d := range v.Diagnostics {
		if d.Clause == clauseBegunNotPurged {
			purge = append(purge, d)
		}
	}
	if len(purge) != 1 || !strings.HasPrefix(purge[0].Detail, "Begin after the purge = started") {
		t.Fatalf("purge diagnostics = %v, want only the begun assertion to fail", purge)
	}
	if got := failedClauses(v); !slices.Equal(got, []string{clauseBegunNotPurged}) {
		t.Fatalf("failed clauses = %v, want only %q", got, clauseBegunNotPurged)
	}
}

func TestAttemptLedgerRejectsTheNegativeVectors(t *testing.T) {
	tests := []struct {
		name    string
		subject func() providerkit.AttemptLedgerSubject
		clauses []string
	}{
		{
			name: "a ledger that starts every begin does not serialize on the key",
			subject: func() providerkit.AttemptLedgerSubject {
				return providerkit.AttemptLedgerSubject{Ledger: alwaysStarting{memory.NewAttemptStore(memory.New())}}
			},
			clauses: []string{clauseBeginSerializes},
		},
		{
			name:    "a purge that takes the begun attempt too",
			subject: purgingEverything,
			clauses: []string{clauseBegunNotPurged},
		},
		{
			name: "a ledger that never reports saturation",
			subject: func() providerkit.AttemptLedgerSubject {
				return providerkit.AttemptLedgerSubject{Ledger: uncapped{memory.NewAttemptStore(memory.New())}}
			},
			clauses: []string{clauseObserveCap, clauseTerminalApplies, clauseFinishConflicts, clauseFinishObserved},
		},
		{
			name: "a ledger that saturates every new ID on a full row, terminal or not",
			subject: func() providerkit.AttemptLedgerSubject {
				return providerkit.AttemptLedgerSubject{Ledger: &saturatingEveryNewID{AttemptLedger: memory.NewAttemptStore(memory.New()), seen: map[string]bool{}}}
			},
			clauses: []string{clauseTerminalApplies},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := providerkit.AttemptLedger(tt.subject)
			if got := failedClauses(v); !slices.Equal(got, tt.clauses) {
				t.Fatalf("failed clauses = %v, want %v (%v)", got, tt.clauses, v.Failures())
			}
		})
	}
}

func failedClauses(v providerkit.Verdict) []string {
	var clauses []string
	for _, d := range v.Diagnostics {
		if !slices.Contains(clauses, d.Clause) {
			clauses = append(clauses, d.Clause)
		}
	}
	return clauses
}
