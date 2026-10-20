//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	tenantA = ports.TenantID("acme")
	tenantB = ports.TenantID("globex")
	keyX    = ports.OutboundKey("key-x")
	keyY    = ports.OutboundKey("key-y")
	msgA    = ports.MessageID("m-a")
	msgB    = ports.MessageID("m-b")
)

var errLedgerBoom = errors.New("attempt_ledger_test: boom")

type ledgerFixture struct {
	pool   *pgxpool.Pool
	ledger postgres.AttemptStore
	a, b   context.Context
}

func newLedgerFixture(t *testing.T) ledgerFixture {
	t.Helper()
	return ledgerFixtureOn(t, openPool(t))
}

func ledgerFixtureOn(t *testing.T, pool *pgxpool.Pool) ledgerFixture {
	t.Helper()
	return ledgerFixture{
		pool:   pool,
		ledger: postgres.NewAttemptStore(pool),
		a:      scopedTo(t, string(tenantA)),
		b:      scopedTo(t, string(tenantB)),
	}
}

func (f ledgerFixture) conflicts(t *testing.T, tenant ports.TenantID, key ports.OutboundKey) int {
	t.Helper()
	var n int
	err := f.pool.QueryRow(context.Background(),
		`SELECT coalesce(sum(jsonb_array_length(conflicts)), 0) FROM external_attempt WHERE tenant_id = $1 AND outbound_key = $2`,
		string(tenant), string(key)).Scan(&n)
	if err != nil {
		t.Fatalf("counting conflicts = %v", err)
	}
	return n
}

func (f ledgerFixture) begin(t *testing.T, ctx context.Context, key ports.OutboundKey, now ports.Instant) {
	t.Helper()
	f.beginFrom(t, ctx, key, msgA, now)
}

func (f ledgerFixture) beginFrom(t *testing.T, ctx context.Context, key ports.OutboundKey, message ports.MessageID, now ports.Instant) {
	t.Helper()
	if _, err := f.ledger.Begin(ctx, key, message, now); err != nil {
		t.Fatalf("Begin(%q, %d) = %v", key, now, err)
	}
}

func (f ledgerFixture) finish(t *testing.T, ctx context.Context, o ports.AttemptOutcome) {
	t.Helper()
	if _, err := f.ledger.Finish(ctx, o); err != nil {
		t.Fatalf("Finish(%+v) = %v", o, err)
	}
}

func (f ledgerFixture) observe(t *testing.T, ctx context.Context, o ports.Observation) {
	t.Helper()
	if _, err := f.ledger.Observe(ctx, o, 500); err != nil {
		t.Fatalf("Observe(%+v) = %v", o, err)
	}
}

type startView struct {
	branch  string
	attempt ports.Attempt
	class   ports.ExternalClass
}

func viewStart(s ports.AttemptStart) startView {
	var v startView
	_ = s.Match(
		func(a ports.Attempt) error { v = startView{"started", a, 0}; return nil },
		func(a ports.Attempt) error { v = startView{"pending", a, 0}; return nil },
		func(a ports.Attempt, c ports.ExternalClass) error { v = startView{"done", a, c}; return nil },
	)
	return v
}

func outcome(n int, class ports.ExternalClass) ports.AttemptOutcome {
	return ports.AttemptOutcome{Key: keyX, N: n, Class: class, FinishedAt: 100}
}

func presumed(n int) ports.AttemptOutcome {
	o := outcome(n, ports.ClassUnknownSent)
	o.Presumed = true
	return o
}

func observation(id string, finding ports.Finding) ports.Observation {
	return ports.Observation{Key: keyX, Finding: finding, ID: id, Provider: "partner"}
}

func TestAttemptLedgerBegin(t *testing.T) {
	tests := []struct {
		name  string
		prior func(t *testing.T, f ledgerFixture)
		want  startView
	}{
		{
			name:  "the first begin starts attempt one",
			prior: func(*testing.T, ledgerFixture) {},
			want:  startView{"started", ports.Attempt{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgA, StartedAt: 30}, 0},
		},
		{
			name:  "a begun attempt makes the next begin pending on it",
			prior: func(t *testing.T, f ledgerFixture) { f.begin(t, f.a, keyX, 10) },
			want:  startView{"pending", ports.Attempt{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgA, StartedAt: 10}, 0},
		},
		{
			name: "the same message starts again after a retryable class",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.a, keyX, 10)
				f.finish(t, f.a, outcome(1, ports.ClassRetryableTransport))
			},
			want: startView{"started", ports.Attempt{Tenant: tenantA, Key: keyX, N: 2, MessageID: msgA, StartedAt: 30}, 0},
		},
		{
			name: "the same message starts again after the upstream was unavailable",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.a, keyX, 10)
				f.finish(t, f.a, outcome(1, ports.ClassRetryableUpstream))
			},
			want: startView{"started", ports.Attempt{Tenant: tenantA, Key: keyX, N: 2, MessageID: msgA, StartedAt: 30}, 0},
		},
		{
			name: "another message starts again after a final class",
			prior: func(t *testing.T, f ledgerFixture) {
				f.beginFrom(t, f.a, keyX, msgB, 10)
				f.finish(t, f.a, outcome(1, ports.ClassFinalBusiness))
			},
			want: startView{"started", ports.Attempt{Tenant: tenantA, Key: keyX, N: 2, MessageID: msgA, StartedAt: 30}, 0},
		},
		{
			name: "the same message is done after a final class",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.a, keyX, 10)
				f.finish(t, f.a, outcome(1, ports.ClassAccepted))
			},
			want: startView{"done", ports.Attempt{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgA, StartedAt: 10}, ports.ClassAccepted},
		},
		{
			name: "the same message is done after the sweep abandoned its attempt",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.a, keyX, 10)
				f.finish(t, f.a, outcome(1, ports.ClassUnknownSent))
			},
			want: startView{"done", ports.Attempt{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgA, StartedAt: 10}, ports.ClassUnknownSent},
		},
		{
			name: "the same message is done although another message's attempt is begun after it",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.a, keyX, 10)
				f.finish(t, f.a, outcome(1, ports.ClassFinalValidation))
				f.beginFrom(t, f.a, keyX, msgB, 20)
			},
			want: startView{"done", ports.Attempt{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgA, StartedAt: 10}, ports.ClassFinalValidation},
		},
		{
			name: "the same message is done although another message finished after it",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.a, keyX, 10)
				f.finish(t, f.a, outcome(1, ports.ClassAccepted))
				f.beginFrom(t, f.a, keyX, msgB, 20)
				f.finish(t, f.a, outcome(2, ports.ClassFinalBusiness))
			},
			want: startView{"done", ports.Attempt{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgA, StartedAt: 10}, ports.ClassAccepted},
		},
		{
			name: "the same message is done on its latest final attempt",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.a, keyX, 10)
				f.finish(t, f.a, outcome(1, ports.ClassRetryableTransport))
				f.begin(t, f.a, keyX, 20)
				f.finish(t, f.a, outcome(2, ports.ClassUnknownSent))
			},
			want: startView{"done", ports.Attempt{Tenant: tenantA, Key: keyX, N: 2, MessageID: msgA, StartedAt: 20}, ports.ClassUnknownSent},
		},
		{
			name: "a begun attempt of another message is pending all the same",
			prior: func(t *testing.T, f ledgerFixture) {
				f.beginFrom(t, f.a, keyX, msgB, 10)
			},
			want: startView{"pending", ports.Attempt{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgB, StartedAt: 10}, 0},
		},
		{
			name:  "another tenant's begun attempt on the same key does not count",
			prior: func(t *testing.T, f ledgerFixture) { f.begin(t, f.b, keyX, 10) },
			want:  startView{"started", ports.Attempt{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgA, StartedAt: 30}, 0},
		},
		{
			name:  "a begun attempt on another key does not count",
			prior: func(t *testing.T, f ledgerFixture) { f.begin(t, f.a, keyY, 10) },
			want:  startView{"started", ports.Attempt{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgA, StartedAt: 30}, 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newLedgerFixture(t)
			tt.prior(t, f)

			start, err := f.ledger.Begin(f.a, keyX, msgA, 30)
			if err != nil {
				t.Fatalf("Begin() = %v", err)
			}
			if got := viewStart(start); got != tt.want {
				t.Fatalf("Begin() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAttemptLedgerBeginSerializesOnTheKey(t *testing.T) {
	const callers = 16
	f := newLedgerFixture(t)

	var (
		wg      sync.WaitGroup
		release = make(chan struct{})
		views   = make([]startView, callers)
		errs    = make([]error, callers)
	)
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-release
			start, err := f.ledger.Begin(f.a, keyX, msgA, ports.Instant(10+i))
			errs[i] = err
			if err == nil {
				views[i] = viewStart(start)
			}
		}()
	}
	close(release)
	wg.Wait()

	started := 0
	var first ports.Attempt
	for i := range callers {
		if errs[i] != nil {
			t.Fatalf("Begin() #%d = %v", i, errs[i])
		}
		if views[i].branch == "started" {
			started++
			first = views[i].attempt
		}
	}
	if started != 1 {
		t.Fatalf("%d concurrent begins started an attempt; want exactly 1", started)
	}
	for i, v := range views {
		if v.attempt != first {
			t.Fatalf("begin #%d saw %+v; want every caller to see the one started %+v", i, v.attempt, first)
		}
	}
	stale, err := f.ledger.Stale(f.a, 1_000, ports.AttemptCursor{}, callers)
	if err != nil || len(stale) != 1 {
		t.Fatalf("Stale() = %v, %v; want the single begun attempt", stale, err)
	}
}

func TestAttemptLedgerFinish(t *testing.T) {
	tests := []struct {
		name          string
		prior         func(t *testing.T, f ledgerFixture)
		outcome       ports.AttemptOutcome
		want          ports.Finished
		wantErr       error
		wantConflicts int
	}{
		{
			name:    "a begun attempt records the class",
			prior:   func(t *testing.T, f ledgerFixture) { f.begin(t, f.a, keyX, 10) },
			outcome: outcome(1, ports.ClassAccepted),
			want:    ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassAccepted},
		},
		{
			name: "the same class again is a no-op",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.a, keyX, 10)
				f.finish(t, f.a, outcome(1, ports.ClassUnknownSent))
			},
			outcome: outcome(1, ports.ClassUnknownSent),
			want:    ports.Finished{Result: ports.FinishUnchanged, Recorded: ports.ClassUnknownSent},
		},
		{
			name: "another class is a recorded conflict that keeps the first",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.a, keyX, 10)
				f.finish(t, f.a, outcome(1, ports.ClassUnknownSent))
			},
			outcome:       outcome(1, ports.ClassAccepted),
			want:          ports.Finished{Result: ports.FinishConflict, Recorded: ports.ClassUnknownSent},
			wantConflicts: 1,
		},
		{
			name: "an earlier attempt is still addressable by its number",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.a, keyX, 10)
				f.finish(t, f.a, outcome(1, ports.ClassRetryableUpstream))
				f.begin(t, f.a, keyX, 20)
			},
			outcome: outcome(1, ports.ClassRetryableUpstream),
			want:    ports.Finished{Result: ports.FinishUnchanged, Recorded: ports.ClassRetryableUpstream},
		},
		{
			name:    "a presumed class closes a begun attempt",
			prior:   func(t *testing.T, f ledgerFixture) { f.begin(t, f.a, keyX, 10) },
			outcome: presumed(1),
			want:    ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassUnknownSent},
		},
		{
			name: "a presumed class yields to the recorded one without a conflict",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.a, keyX, 10)
				f.finish(t, f.a, outcome(1, ports.ClassAccepted))
			},
			outcome: presumed(1),
			want:    ports.Finished{Result: ports.FinishUnchanged, Recorded: ports.ClassAccepted},
		},
		{
			name:    "a key never begun is unknown",
			prior:   func(*testing.T, ledgerFixture) {},
			outcome: outcome(1, ports.ClassAccepted),
			wantErr: ports.ErrAttemptUnknown,
		},
		{
			name:    "an attempt number past the last is unknown",
			prior:   func(t *testing.T, f ledgerFixture) { f.begin(t, f.a, keyX, 10) },
			outcome: outcome(2, ports.ClassAccepted),
			wantErr: ports.ErrAttemptUnknown,
		},
		{
			name:    "another tenant's attempt is unknown",
			prior:   func(t *testing.T, f ledgerFixture) { f.begin(t, f.b, keyX, 10) },
			outcome: outcome(1, ports.ClassAccepted),
			wantErr: ports.ErrAttemptUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newLedgerFixture(t)
			tt.prior(t, f)

			got, err := f.ledger.Finish(f.a, tt.outcome)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Finish() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Finish() = %+v, want %+v", got, tt.want)
			}
			if n := f.conflicts(t, tenantA, keyX); n != tt.wantConflicts {
				t.Fatalf("AttemptConflicts() = %d, want %d", n, tt.wantConflicts)
			}
		})
	}
}

func TestAttemptLedgerObserve(t *testing.T) {
	finished := func(class ports.ExternalClass, carried ports.Observation) func(*testing.T, ledgerFixture) {
		return func(t *testing.T, f ledgerFixture) {
			f.begin(t, f.a, keyX, 10)
			o := outcome(1, class)
			o.Observation = carried
			f.finish(t, f.a, o)
		}
	}
	then := func(prior func(*testing.T, ledgerFixture), seen ...ports.Observation) func(*testing.T, ledgerFixture) {
		return func(t *testing.T, f ledgerFixture) {
			prior(t, f)
			for _, o := range seen {
				f.observe(t, f.a, o)
			}
		}
	}
	unknownSent := finished(ports.ClassUnknownSent, ports.Observation{})

	tests := []struct {
		name          string
		prior         func(t *testing.T, f ledgerFixture)
		observation   ports.Observation
		want          ports.Observed
		wantErr       error
		wantConflicts int
	}{
		{
			name:        "the first observation is recorded against the class",
			prior:       unknownSent,
			observation: observation("o-1", ports.FindingPending),
			want:        ports.Observed{Result: ports.ObserveRecorded, N: 1, Class: ports.ClassUnknownSent, QueriesDone: 1},
		},
		{
			name:        "each new observation counts as one more query",
			prior:       then(unknownSent, observation("o-1", ports.FindingPending), observation("o-2", ports.FindingInconclusive)),
			observation: observation("o-3", ports.FindingApplied),
			want:        ports.Observed{Result: ports.ObserveRecorded, N: 1, Class: ports.ClassUnknownSent, QueriesDone: 3},
		},
		{
			name:        "the same observation ID is a duplicate",
			prior:       then(unknownSent, observation("o-1", ports.FindingPending)),
			observation: observation("o-1", ports.FindingPending),
			want:        ports.Observed{Result: ports.ObserveDuplicate, N: 1, Class: ports.ClassUnknownSent, QueriesDone: 1},
		},
		{
			name:        "the same terminal finding under a new ID is a duplicate",
			prior:       then(unknownSent, observation("o-1", ports.FindingApplied)),
			observation: observation("o-2", ports.FindingApplied),
			want:        ports.Observed{Result: ports.ObserveDuplicate, N: 1, Class: ports.ClassUnknownSent, QueriesDone: 1},
		},
		{
			name:          "applied after rejected is a recorded conflict",
			prior:         then(unknownSent, observation("o-1", ports.FindingRejected)),
			observation:   observation("o-2", ports.FindingApplied),
			want:          ports.Observed{Result: ports.ObserveConflict, N: 1, Class: ports.ClassUnknownSent, QueriesDone: 1},
			wantConflicts: 1,
		},
		{
			name:        "absent after a terminal finding is a duplicate, not a conflict",
			prior:       then(unknownSent, observation("o-1", ports.FindingApplied)),
			observation: observation("o-2", ports.FindingAbsent),
			want:        ports.Observed{Result: ports.ObserveDuplicate, N: 1, Class: ports.ClassUnknownSent, QueriesDone: 1},
		},
		{
			name:        "pending after a terminal finding is a duplicate, not a conflict",
			prior:       then(unknownSent, observation("o-1", ports.FindingRejected)),
			observation: observation("o-2", ports.FindingPending),
			want:        ports.Observed{Result: ports.ObserveDuplicate, N: 1, Class: ports.ClassUnknownSent, QueriesDone: 1},
		},
		{
			name:        "inconclusive after a terminal finding is a duplicate, not a conflict",
			prior:       then(unknownSent, observation("o-1", ports.FindingApplied)),
			observation: observation("o-2", ports.FindingInconclusive),
			want:        ports.Observed{Result: ports.ObserveDuplicate, N: 1, Class: ports.ClassUnknownSent, QueriesDone: 1},
		},
		{
			name:          "rejected after applied is a recorded conflict",
			prior:         then(unknownSent, observation("o-1", ports.FindingApplied)),
			observation:   observation("o-2", ports.FindingRejected),
			want:          ports.Observed{Result: ports.ObserveConflict, N: 1, Class: ports.ClassUnknownSent, QueriesDone: 1},
			wantConflicts: 1,
		},
		{
			name:          "a conflicting observation delivered again is a duplicate",
			prior:         then(unknownSent, observation("o-1", ports.FindingApplied), observation("o-2", ports.FindingRejected)),
			observation:   observation("o-2", ports.FindingRejected),
			want:          ports.Observed{Result: ports.ObserveDuplicate, N: 1, Class: ports.ClassUnknownSent, QueriesDone: 1},
			wantConflicts: 1,
		},
		{
			name:        "a non-terminal finding replaces the last one",
			prior:       then(unknownSent, observation("o-1", ports.FindingPending)),
			observation: observation("o-2", ports.FindingRejected),
			want:        ports.Observed{Result: ports.ObserveRecorded, N: 1, Class: ports.ClassUnknownSent, QueriesDone: 2},
		},
		{
			name:        "the observation Submit carried is already seen",
			prior:       finished(ports.ClassAccepted, observation("o-submit", ports.FindingPending)),
			observation: observation("o-submit", ports.FindingPending),
			want:        ports.Observed{Result: ports.ObserveDuplicate, N: 1, Class: ports.ClassAccepted},
		},
		{
			name:          "a terminal finding Submit carried is contradicted as a conflict",
			prior:         finished(ports.ClassAccepted, observation("o-submit", ports.FindingApplied)),
			observation:   observation("o-1", ports.FindingRejected),
			want:          ports.Observed{Result: ports.ObserveConflict, N: 1, Class: ports.ClassAccepted},
			wantConflicts: 1,
		},
		{
			name: "a new attempt resets the count and forgets the earlier IDs",
			prior: func(t *testing.T, f ledgerFixture) {
				unknownSent(t, f)
				f.observe(t, f.a, observation("o-1", ports.FindingAbsent))
				f.beginFrom(t, f.a, keyX, msgB, 20)
				f.finish(t, f.a, outcome(2, ports.ClassUnknownAmbiguous))
			},
			observation: observation("o-1", ports.FindingAbsent),
			want:        ports.Observed{Result: ports.ObserveRecorded, N: 2, Class: ports.ClassUnknownAmbiguous, QueriesDone: 1},
		},
		{
			name:        "a key never begun is unknown",
			prior:       func(*testing.T, ledgerFixture) {},
			observation: observation("o-1", ports.FindingApplied),
			wantErr:     ports.ErrAttemptUnknown,
		},
		{
			name: "another tenant's attempt is unknown",
			prior: func(t *testing.T, f ledgerFixture) {
				f.begin(t, f.b, keyX, 10)
				f.finish(t, f.b, outcome(1, ports.ClassUnknownSent))
			},
			observation: observation("o-1", ports.FindingApplied),
			wantErr:     ports.ErrAttemptUnknown,
		},
		{
			name:        "a last attempt still begun is in flight",
			prior:       func(t *testing.T, f ledgerFixture) { f.begin(t, f.a, keyX, 10) },
			observation: observation("o-1", ports.FindingApplied),
			wantErr:     ports.ErrAttemptInFlight,
		},
		{
			name: "an earlier finished attempt does not answer for a begun last one",
			prior: func(t *testing.T, f ledgerFixture) {
				unknownSent(t, f)
				f.beginFrom(t, f.a, keyX, msgB, 20)
			},
			observation: observation("o-1", ports.FindingApplied),
			wantErr:     ports.ErrAttemptInFlight,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newLedgerFixture(t)
			tt.prior(t, f)

			got, err := f.ledger.Observe(f.a, tt.observation, 600)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Observe() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Observe() = %+v, want %+v", got, tt.want)
			}
			if n := f.conflicts(t, tenantA, keyX); n != tt.wantConflicts {
				t.Fatalf("AttemptConflicts() = %d, want %d", n, tt.wantConflicts)
			}
		})
	}
}

func TestAttemptLedgerStale(t *testing.T) {
	seed := func(t *testing.T, f ledgerFixture) {
		f.begin(t, f.a, keyX, 30)
		f.begin(t, f.b, keyX, 10)
		f.begin(t, f.a, keyY, 20)
		f.begin(t, f.b, keyY, 20)
		f.begin(t, f.b, "key-finished", 5)
		f.finish(t, f.b, ports.AttemptOutcome{Key: "key-finished", N: 1, Class: ports.ClassAccepted, FinishedAt: 6})
		f.begin(t, f.a, "key-at-cutoff", 40)
	}
	all := []ports.Attempt{
		{Tenant: tenantB, Key: keyX, N: 1, MessageID: msgA, StartedAt: 10},
		{Tenant: tenantA, Key: keyY, N: 1, MessageID: msgA, StartedAt: 20},
		{Tenant: tenantB, Key: keyY, N: 1, MessageID: msgA, StartedAt: 20},
		{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgA, StartedAt: 30},
	}
	atCutoff := ports.Attempt{Tenant: tenantA, Key: "key-at-cutoff", N: 1, MessageID: msgA, StartedAt: 40}
	tests := []struct {
		name   string
		cutoff ports.Instant
		after  ports.AttemptCursor
		limit  int
		want   []ports.Attempt
	}{
		{"every begun attempt before the cutoff, oldest first, across tenants", 39, ports.AttemptCursor{}, 10, all},
		{"an attempt started at the cutoff is stale", 40, ports.AttemptCursor{}, 10, append(slices.Clone(all), atCutoff)},
		{"the limit keeps the oldest", 40, ports.AttemptCursor{}, 2, all[:2]},
		{"nothing started before the earliest start", 9, ports.AttemptCursor{}, 10, []ports.Attempt{}},
		{"the cursor resumes strictly after itself", 39, all[0].Cursor(), 2, all[1:3]},
		{"the cursor tells apart attempts started together", 39, all[1].Cursor(), 10, all[2:]},
		{"the cursor at the last attempt ends the list", 39, all[3].Cursor(), 10, []ports.Attempt{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newLedgerFixture(t)
			seed(t, f)

			got, err := f.ledger.Stale(context.Background(), tt.cutoff, tt.after, tt.limit)
			if err != nil {
				t.Fatalf("Stale() = %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("Stale() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAttemptLedgerRefusesInvalidArguments(t *testing.T) {
	f := newLedgerFixture(t)
	f.begin(t, f.a, keyX, 10)
	tests := []struct {
		name string
		call func() error
	}{
		{"begin with an empty key", func() error { _, err := f.ledger.Begin(f.a, "", msgA, 1); return err }},
		{"begin without a message", func() error { _, err := f.ledger.Begin(f.a, keyX, "", 1); return err }},
		{"finish with an empty key", func() error {
			_, err := f.ledger.Finish(f.a, ports.AttemptOutcome{N: 1, Class: ports.ClassAccepted})
			return err
		}},
		{"finish with attempt zero", func() error { _, err := f.ledger.Finish(f.a, outcome(0, ports.ClassAccepted)); return err }},
		{"finish without a class", func() error { _, err := f.ledger.Finish(f.a, outcome(1, 0)); return err }},
		{"finish with a class out of range", func() error {
			_, err := f.ledger.Finish(f.a, outcome(1, ports.ClassSchemaMismatch+1))
			return err
		}},
		{"finish carrying another key's observation", func() error {
			o := outcome(1, ports.ClassAccepted)
			o.Observation = ports.Observation{Key: keyY, ID: "o-1", Finding: ports.FindingApplied}
			_, err := f.ledger.Finish(f.a, o)
			return err
		}},
		{"finish carrying a finding out of range", func() error {
			o := outcome(1, ports.ClassAccepted)
			o.Observation = ports.Observation{Key: keyX, ID: "o-1", Finding: ports.FindingInconclusive + 1}
			_, err := f.ledger.Finish(f.a, o)
			return err
		}},
		{"observe with an empty key", func() error {
			_, err := f.ledger.Observe(f.a, ports.Observation{ID: "o-1", Finding: ports.FindingApplied}, 1)
			return err
		}},
		{"observe without an ID", func() error {
			_, err := f.ledger.Observe(f.a, observation("", ports.FindingApplied), 1)
			return err
		}},
		{"observe without a finding", func() error { _, err := f.ledger.Observe(f.a, observation("o-1", 0), 1); return err }},
		{"observe with a finding out of range", func() error {
			_, err := f.ledger.Observe(f.a, observation("o-1", ports.FindingInconclusive+1), 1)
			return err
		}},
		{"stale with a limit of zero", func() error { _, err := f.ledger.Stale(f.a, 100, ports.AttemptCursor{}, 0); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, ports.ErrInvalidAttempt) {
				t.Fatalf("error = %v, want %v", err, ports.ErrInvalidAttempt)
			}
		})
	}
}

func TestAttemptLedgerRequiresATenantAndAStore(t *testing.T) {
	noTenant, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r", CorrelationID: "c", TraceContext: "t", Deadline: 1, Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	noTenantCtx := ports.WithExecutionContext(context.Background(), noTenant)
	ledger := postgres.NewAttemptStore(openPool(t))
	empty := postgres.NewAttemptStore(nil)
	tests := []struct {
		name string
		call func() error
		want error
	}{
		{"begin without an execution context", func() error { _, err := ledger.Begin(context.Background(), keyX, msgA, 1); return err }, ports.ErrContextAbsent},
		{"begin without a tenant", func() error { _, err := ledger.Begin(noTenantCtx, keyX, msgA, 1); return err }, postgres.ErrTenantUnresolved},
		{"finish without a tenant", func() error { _, err := ledger.Finish(noTenantCtx, outcome(1, ports.ClassAccepted)); return err }, postgres.ErrTenantUnresolved},
		{"observe without a tenant", func() error {
			_, err := ledger.Observe(noTenantCtx, observation("o-1", ports.FindingApplied), 1)
			return err
		}, postgres.ErrTenantUnresolved},
		{"begin on a store built without one", func() error { _, err := empty.Begin(noTenantCtx, keyX, msgA, 1); return err }, postgres.ErrIncompleteAttemptStore},
		{"finish on a store built without one", func() error { _, err := empty.Finish(noTenantCtx, outcome(1, ports.ClassAccepted)); return err }, postgres.ErrIncompleteAttemptStore},
		{"observe on a store built without one", func() error {
			_, err := empty.Observe(noTenantCtx, observation("o-1", ports.FindingApplied), 1)
			return err
		}, postgres.ErrIncompleteAttemptStore},
		{"stale on a store built without one", func() error { _, err := empty.Stale(noTenantCtx, 1, ports.AttemptCursor{}, 1); return err }, postgres.ErrIncompleteAttemptStore},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestAttemptLedgerStoreRefusesACancelledContext(t *testing.T) {
	f := newLedgerFixture(t)

	cancelled, cancel := context.WithCancel(f.a)
	cancel()
	if _, err := f.ledger.Begin(cancelled, keyY, msgA, 20); !errors.Is(err, context.Canceled) {
		t.Fatalf("Begin() on a cancelled context = %v, want %v", err, context.Canceled)
	}
}

func TestAttemptLedgerInTheTransactionCommitsOrRollsBackWithIt(t *testing.T) {
	tests := []struct {
		name        string
		failWith    error
		wantErr     error
		wantStarted startView
	}{
		{
			name:        "a committed begin is seen by the next one",
			wantStarted: startView{"pending", ports.Attempt{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgA, StartedAt: 10}, 0},
		},
		{
			name:        "a callback error discards the begin",
			failWith:    errLedgerBoom,
			wantErr:     errLedgerBoom,
			wantStarted: startView{"started", ports.Attempt{Tenant: tenantA, Key: keyX, N: 1, MessageID: msgA, StartedAt: 20}, 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newLedgerFixture(t)
			uow := postgres.NewUnitOfWork(f.pool, (*postgres.Tx).AttemptLedger)

			err := uow.Within(f.a, func(ctx context.Context, ledger ports.AttemptLedger) error {
				if _, err := ledger.Begin(ctx, keyX, msgA, 10); err != nil {
					return err
				}
				return tt.failWith
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Within() = %v, want %v", err, tt.wantErr)
			}

			start, err := f.ledger.Begin(f.a, keyX, msgA, 20)
			if err != nil {
				t.Fatalf("Begin() = %v", err)
			}
			if got := viewStart(start); got != tt.wantStarted {
				t.Fatalf("Begin() after the transaction = %+v, want %+v", got, tt.wantStarted)
			}
		})
	}
}

func TestPurgeAttemptsRemovesOnlyFinishedAttemptsPastTheCutoff(t *testing.T) {
	tests := []struct {
		name      string
		before    ports.Instant
		batch     int
		wantErr   error
		wantCount int64
		wantLeft  []string
	}{
		{"a cutoff before every finish removes nothing", 10, 10, nil, 0, []string{"key-begun", "key-begun-stamped", "key-x", "key-y"}},
		{"the cutoff is exclusive", 30, 10, nil, 1, []string{"key-begun", "key-begun-stamped", "key-y"}},
		{"the batch takes the oldest finish first", 100, 1, nil, 1, []string{"key-begun", "key-begun-stamped", "key-y"}},
		{"every old finished attempt goes and the begun ones stay", 100, 10, nil, 2, []string{"key-begun", "key-begun-stamped"}},
		{"a batch of zero is refused", 100, 0, postgres.ErrPurgeBatchRequired, 0, []string{"key-begun", "key-begun-stamped", "key-x", "key-y"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := ledgerFixtureOn(t, openPoolWith(t, heapOrderOnly))
			f.begin(t, f.a, "key-begun", 1)
			f.begin(t, f.b, keyY, 3)
			f.finish(t, f.b, ports.AttemptOutcome{Key: keyY, N: 1, Class: ports.ClassFinalBusiness, FinishedAt: 30})
			f.begin(t, f.a, keyX, 2)
			f.finish(t, f.a, ports.AttemptOutcome{Key: keyX, N: 1, Class: ports.ClassAccepted, FinishedAt: 10})
			f.stampBegunFinishedAt(t, "key-begun-stamped", 1)

			purge, err := postgres.PurgeAttempts(context.Background(), f.pool, tt.before, tt.batch)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("PurgeAttempts() = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && (purge.Count != tt.wantCount || purge.Before != tt.before) {
				t.Fatalf("PurgeAttempts() = %+v, want %d rows before %d", purge, tt.wantCount, tt.before)
			}
			if left := f.keysLeft(t); !slices.Equal(left, tt.wantLeft) {
				t.Fatalf("left %v, want %v", left, tt.wantLeft)
			}
		})
	}
}

// heapOrderOnly turns off the index paths, whose finished_at order would hide a
// purge batch that does not order itself.
func heapOrderOnly(cfg *pgxpool.Config) {
	cfg.ConnConfig.RuntimeParams["enable_indexscan"] = "off"
	cfg.ConnConfig.RuntimeParams["enable_bitmapscan"] = "off"
	cfg.ConnConfig.RuntimeParams["enable_indexonlyscan"] = "off"
}

// stampBegunFinishedAt writes a row no ledger call produces, a begun attempt with
// an old finished_at, so only the state predicate keeps the purge off it.
func (f ledgerFixture) stampBegunFinishedAt(t *testing.T, key string, finishedAt int64) {
	t.Helper()
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO external_attempt (tenant_id, outbound_key, attempt, message_id, state, started_at, finished_at)
		 VALUES ($1, $2, 1, 'm-stamped', 'begun', 1, $3)`, string(tenantA), key, finishedAt)
	if err != nil {
		t.Fatalf("stamping a begun attempt = %v", err)
	}
}

func (f ledgerFixture) keysLeft(t *testing.T) []string {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `SELECT outbound_key FROM external_attempt ORDER BY outbound_key COLLATE "C"`)
	if err != nil {
		t.Fatalf("listing keys = %v", err)
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collecting keys = %v", err)
	}
	return keys
}

func TestMigrateCreatesTheAttemptLedgerSchema(t *testing.T) {
	pool := openAdminPool(t)
	ctx := context.Background()
	tests := []struct {
		name  string
		query string
		args  []any
	}{
		{"table", tableExistsQuery, []any{"external_attempt"}},
		{"primary key", constraintExistsQuery, []any{"external_attempt", "external_attempt_pkey", "PRIMARY KEY"}},
		{"state check", constraintExistsQuery, []any{"external_attempt", "external_attempt_state_check", "CHECK"}},
		{"stale index", indexExistsQuery, []any{"external_attempt_started_at_idx"}},
		{"purge index", indexExistsQuery, []any{"external_attempt_finished_at_idx"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !exists(t, ctx, pool, tt.query, tt.args...) {
				t.Fatalf("%s %v does not exist", tt.name, tt.args)
			}
		})
	}
}

type storedObservation struct {
	id, provider, externalRef *string
	sourceAt                  *int64
	finding                   *int16
}

func (f ledgerFixture) storedObservation(t *testing.T) storedObservation {
	t.Helper()
	var s storedObservation
	err := f.pool.QueryRow(context.Background(),
		`SELECT observation_id, provider, external_ref, source_at, finding FROM external_attempt
		  WHERE tenant_id = $1 AND outbound_key = $2 ORDER BY attempt DESC LIMIT 1`,
		string(tenantA), string(keyX)).Scan(&s.id, &s.provider, &s.externalRef, &s.sourceAt, &s.finding)
	if err != nil {
		t.Fatalf("reading the stored observation = %v", err)
	}
	return s
}

func (s storedObservation) String() string {
	deref := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}
	source, finding := int64(-1), int16(-1)
	if s.sourceAt != nil {
		source = *s.sourceAt
	}
	if s.finding != nil {
		finding = *s.finding
	}
	return fmt.Sprintf("%s|%s|%s|%d|%d", deref(s.id), deref(s.provider), deref(s.externalRef), source, finding)
}

func TestAttemptLedgerKeepsTheLastAcceptedObservation(t *testing.T) {
	carried := ports.Observation{Key: keyX, Finding: ports.FindingPending, ID: "o-submit", Provider: "p-1", ExternalRef: "r-1", SourceAt: 7}
	applied := ports.Observation{Key: keyX, Finding: ports.FindingApplied, ID: "o-2", Provider: "p-2", ExternalRef: "r-2", SourceAt: 9}
	rejected := ports.Observation{Key: keyX, Finding: ports.FindingRejected, ID: "o-3", Provider: "p-3", ExternalRef: "r-3", SourceAt: 11}
	tests := []struct {
		name     string
		carried  ports.Observation
		observed []ports.Observation
		want     string
	}{
		{"a finish without an observation leaves the columns empty", ports.Observation{}, nil, "<nil>|<nil>|<nil>|-1|-1"},
		{"a finish keeps what the submission carried", carried, nil, "o-submit|p-1|r-1|7|4"},
		{"a recorded observation replaces it", carried, []ports.Observation{applied}, "o-2|p-2|r-2|9|1"},
		{"a conflicting observation leaves the accepted one", carried, []ports.Observation{applied, rejected}, "o-2|p-2|r-2|9|1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newLedgerFixture(t)
			f.begin(t, f.a, keyX, 10)
			o := outcome(1, ports.ClassUnknownSent)
			o.Observation = tt.carried
			f.finish(t, f.a, o)
			for _, observation := range tt.observed {
				f.observe(t, f.a, observation)
			}

			if got := f.storedObservation(t).String(); got != tt.want {
				t.Fatalf("stored observation = %s, want %s", got, tt.want)
			}
		})
	}
}
