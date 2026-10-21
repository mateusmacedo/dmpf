package app_test

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	sweepPanicValue = "sweep: nil pool for postgres://app:hunter2@db"
	sweepChildEnv   = "DMPF_TEST_SWEEP_PANIC"
)

type sweepCall struct {
	cutoff ports.Instant
	after  ports.AttemptCursor
	batch  int
}

type sweepPageResult struct {
	page app.SweepPage
	err  error
}

type sweepRecorder struct {
	mu      sync.Mutex
	calls   []sweepCall
	results []sweepPageResult
	called  chan struct{}
}

func newSweepRecorder(results ...sweepPageResult) *sweepRecorder {
	return &sweepRecorder{results: results, called: make(chan struct{}, 16)}
}

func (r *sweepRecorder) sweep(_ context.Context, cutoff ports.Instant, after ports.AttemptCursor, batch int) (app.SweepPage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, sweepCall{cutoff: cutoff, after: after, batch: batch})
	var result sweepPageResult
	if len(r.results) > 0 {
		result, r.results = r.results[0], r.results[1:]
	}
	r.called <- struct{}{}
	return result.page, result.err
}

func (r *sweepRecorder) snapshot() []sweepCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]sweepCall(nil), r.calls...)
}

func runSweep(t *testing.T, cfg app.SweepConfig, fn app.SweepFunc) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.RunSweep(ctx, cfg, purgeClock(10_000), purgeLog, fn) }()
	return func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(2 * time.Second):
			t.Fatal("RunSweep did not stop after its context was cancelled")
			return nil
		}
	}
}

func awaitSweep(t *testing.T, called <-chan struct{}, why string) {
	t.Helper()
	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal(why)
	}
}

func cursorAt(n int) ports.AttemptCursor {
	return ports.AttemptCursor{StartedAt: ports.Instant(n), Tenant: "acme", Key: "k", N: 1}
}

func TestRunSweepRefusesALoopItCouldNotRunSafely(t *testing.T) {
	valid := app.SweepConfig{Interval: time.Second, Batch: 100, StaleAfter: 4_000}
	for _, tc := range []struct {
		name  string
		cfg   app.SweepConfig
		clock ports.Clock
		nolog bool
		nofn  bool
	}{
		{name: "no interval", cfg: app.SweepConfig{Batch: 100, StaleAfter: 4_000}, clock: purgeClock(1)},
		{name: "no batch", cfg: app.SweepConfig{Interval: time.Second, StaleAfter: 4_000}, clock: purgeClock(1)},
		{name: "no stale after", cfg: app.SweepConfig{Interval: time.Second, Batch: 100}, clock: purgeClock(1)},
		{name: "negative stale after", cfg: app.SweepConfig{Interval: time.Second, Batch: 100, StaleAfter: -1}, clock: purgeClock(1)},
		{name: "no clock", cfg: valid},
		{name: "no logger", cfg: valid, clock: purgeClock(1), nolog: true},
		{name: "no sweep", cfg: valid, clock: purgeClock(1), nofn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs, fn := purgeLog, app.SweepFunc(newSweepRecorder().sweep)
			if tc.nolog {
				logs = nil
			}
			if tc.nofn {
				fn = nil
			}
			if err := app.RunSweep(context.Background(), tc.cfg, tc.clock, logs, fn); !errors.Is(err, app.ErrInvalidSweepConfig) {
				t.Fatalf("RunSweep() = %v, want ErrInvalidSweepConfig: the loop would panic or never cut", err)
			}
		})
	}
}

func TestRunSweepCutsTheStaleAfterBeforeNowFromTheOldestAttempt(t *testing.T) {
	rec := newSweepRecorder()
	stop := runSweep(t, app.SweepConfig{Interval: 10 * time.Millisecond, Batch: 50, StaleAfter: 4_000}, rec.sweep)

	awaitSweep(t, rec.called, "the first cycle never ran")
	awaitSweep(t, rec.called, "the second cycle never ran")
	if err := stop(); err != nil {
		t.Fatalf("RunSweep() = %v, want nil: stopping is not a failure", err)
	}
	for _, c := range rec.snapshot() {
		if c.cutoff != 6_000 || c.batch != 50 || c.after != (ports.AttemptCursor{}) {
			t.Fatalf("sweep call = %+v, want cutoff now − stale after (6000), batch 50 and each cycle from the oldest attempt", c)
		}
	}
}

func TestAFullSweepPageIsFollowedAtOnceByTheNextFromItsCursor(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "every attempt swept"},
		{name: "a page with failures", err: errors.New("sweep_test: apply refused")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := newSweepRecorder(
				sweepPageResult{page: app.SweepPage{Seen: 50, Swept: 50, Next: cursorAt(1)}, err: tc.err},
				sweepPageResult{page: app.SweepPage{Seen: 50, Swept: 50, Next: cursorAt(2)}, err: tc.err},
				sweepPageResult{page: app.SweepPage{Seen: 3, Swept: 3, Next: cursorAt(3)}},
			)
			stop := runSweep(t, app.SweepConfig{Interval: time.Hour, Batch: 50, StaleAfter: 4_000}, rec.sweep)

			for range 3 {
				awaitSweep(t, rec.called, "a full page waited for the next interval; orphans would pile up for hours")
			}
			_ = stop()
			got := rec.snapshot()
			want := []ports.AttemptCursor{{}, cursorAt(1), cursorAt(2)}
			if len(got) != len(want) {
				t.Fatalf("calls = %+v, want three pages of one cycle", got)
			}
			for i, c := range got {
				if c.after != want[i] || c.cutoff != 6_000 {
					t.Fatalf("call %d = %+v, want after %+v under the cycle's cutoff 6000", i, c, want[i])
				}
			}
		})
	}
}

func TestAShortOrFailedPageEndsTheCycle(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result sweepPageResult
	}{
		{name: "nothing seen", result: sweepPageResult{}},
		{name: "the listing failed", result: sweepPageResult{err: errors.New("sweep_test: database away")}},
		{name: "a full page that did not advance", result: sweepPageResult{page: app.SweepPage{Seen: 50}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := newSweepRecorder(tc.result, tc.result)
			stop := runSweep(t, app.SweepConfig{Interval: 10 * time.Millisecond, Batch: 50, StaleAfter: 4_000}, rec.sweep)

			awaitSweep(t, rec.called, "the first cycle never ran")
			awaitSweep(t, rec.called, "the next interval never ran")
			if err := stop(); err != nil {
				t.Fatalf("RunSweep() = %v, want nil: a failed cycle is retried, not fatal", err)
			}
			for i, c := range rec.snapshot() {
				if c.after != (ports.AttemptCursor{}) {
					t.Fatalf("call %d = %+v, want every call to open a cycle: the page before it ended its own", i, c)
				}
			}
		})
	}
}

func panickingSweep(context.Context, ports.Instant, ports.AttemptCursor, int) (app.SweepPage, error) {
	panic(sweepPanicValue)
}

func TestAPanickingSweepEndsRunSweepWithTheError(t *testing.T) {
	err := app.RunSweep(context.Background(), app.SweepConfig{Interval: time.Hour, Batch: 50, StaleAfter: 4_000}, purgeClock(1), purgeLog, panickingSweep)

	if !errors.Is(err, app.ErrSweepPanicked) || strings.Contains(err.Error(), sweepPanicValue) {
		t.Fatalf("RunSweep() = %v, want ErrSweepPanicked without the panic value (ERR-20, ERR-23)", err)
	}
}

func TestRunningAPanickingSweep(t *testing.T) {
	if os.Getenv(sweepChildEnv) == "" {
		t.Skip("runs only as the process TestAPanickingSweepWritesNothingToTheStderrOfTheProcess starts")
	}
	err := app.RunSweep(context.Background(), app.SweepConfig{Interval: time.Hour, Batch: 50, StaleAfter: 4_000}, purgeClock(1), purgeLog, panickingSweep)
	if !errors.Is(err, app.ErrSweepPanicked) {
		t.Fatalf("RunSweep() = %v, want ErrSweepPanicked", err)
	}
}

func TestAPanickingSweepWritesNothingToTheStderrOfTheProcess(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestRunningAPanickingSweep$", "-test.count=1")
	command.Env = append(os.Environ(), sweepChildEnv+"=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr

	err := command.Run()

	if err != nil || stderr.Len() != 0 {
		t.Fatalf("process = %v, stderr = %q, stdout = %q, want exit 0 and nothing on stderr", err, stderr.String(), stdout.String())
	}
}

type staleLedger struct {
	mu       sync.Mutex
	attempts []ports.Attempt
	err      error
	calls    []sweepCall
	other    []string
	listed   chan struct{}
}

func newStaleLedger(attempts ...ports.Attempt) *staleLedger {
	return &staleLedger{attempts: attempts, listed: make(chan struct{}, 16)}
}

func (l *staleLedger) Begin(context.Context, ports.OutboundKey, ports.MessageID, ports.Instant) (ports.AttemptStart, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.other = append(l.other, "Begin")
	return ports.AttemptStart{}, errors.New("sweep_test: Begin")
}

func (l *staleLedger) Finish(context.Context, ports.AttemptOutcome) (ports.Finished, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.other = append(l.other, "Finish")
	return ports.Finished{}, errors.New("sweep_test: Finish")
}

func (l *staleLedger) Observe(context.Context, ports.Observation, ports.Instant) (ports.Observed, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.other = append(l.other, "Observe")
	return ports.Observed{}, errors.New("sweep_test: Observe")
}

func compareCursor(a, b ports.AttemptCursor) int {
	return cmp.Or(cmp.Compare(a.StartedAt, b.StartedAt), cmp.Compare(a.Tenant, b.Tenant), cmp.Compare(a.Key, b.Key), cmp.Compare(a.N, b.N))
}

func (l *staleLedger) Stale(_ context.Context, cutoff ports.Instant, after ports.AttemptCursor, limit int) ([]ports.Attempt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, sweepCall{cutoff: cutoff, after: after, batch: limit})
	defer func() { l.listed <- struct{}{} }()
	if l.err != nil {
		return nil, l.err
	}
	var page []ports.Attempt
	for _, attempt := range l.attempts {
		if attempt.StartedAt <= cutoff && (after == ports.AttemptCursor{} || compareCursor(attempt.Cursor(), after) > 0) {
			page = append(page, attempt)
		}
	}
	slices.SortFunc(page, func(a, b ports.Attempt) int { return compareCursor(a.Cursor(), b.Cursor()) })
	return page[:min(limit, len(page))], nil
}

func (l *staleLedger) snapshot() ([]sweepCall, []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]sweepCall(nil), l.calls...), append([]string(nil), l.other...)
}

type abandoned struct {
	attempt   ports.Attempt
	execution ports.ExecutionContext
	message   ports.MessageContext
	deadline  bool
}

type abandonOutcome struct {
	finished ports.Finished
	err      error
}

type abandonRecorder struct {
	mu       sync.Mutex
	outcomes map[ports.OutboundKey]abandonOutcome
	seen     []abandoned
}

func (r *abandonRecorder) abandon(ctx context.Context, attempt ports.Attempt) (ports.Finished, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	execution, _ := ports.ExecutionContextFrom(ctx)
	message, _ := ports.MessageContextFrom(ctx)
	_, deadline := ctx.Deadline()
	r.seen = append(r.seen, abandoned{attempt: attempt, execution: execution, message: message, deadline: deadline})
	outcome, ok := r.outcomes[attempt.Key]
	if !ok {
		return ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassUnknownSent}, nil
	}
	return outcome.finished, outcome.err
}

func (r *abandonRecorder) keys() []ports.OutboundKey {
	r.mu.Lock()
	defer r.mu.Unlock()
	keys := make([]ports.OutboundKey, 0, len(r.seen))
	for _, seen := range r.seen {
		keys = append(keys, seen.attempt.Key)
	}
	return keys
}

func externalSweep(ledger ports.AttemptLedger, abandon func(context.Context, ports.Attempt) (ports.Finished, error)) app.ExternalSweep {
	return app.ExternalSweep{Store: ledger, Abandon: abandon, Timeout: time.Minute, Locale: "pt-BR"}
}

func mustSweep(t *testing.T, s app.ExternalSweep) app.SweepFunc {
	t.Helper()
	fn, err := app.SweepExternalAttempts(s)
	if err != nil {
		t.Fatalf("SweepExternalAttempts() = %v, want nil", err)
	}
	return fn
}

func staleAttempt(tenant ports.TenantID, key ports.OutboundKey, n int) ports.Attempt {
	return ports.Attempt{Tenant: tenant, Key: key, N: n, MessageID: ports.MessageID("m-" + string(key)), StartedAt: 1_000}
}

func TestSweepExternalAttemptsRefusesAnIncompleteSweep(t *testing.T) {
	rec := &abandonRecorder{}
	for _, tc := range []struct {
		name   string
		mutate func(*app.ExternalSweep)
	}{
		{name: "no store", mutate: func(s *app.ExternalSweep) { s.Store = nil }},
		{name: "no abandon", mutate: func(s *app.ExternalSweep) { s.Abandon = nil }},
		{name: "no timeout", mutate: func(s *app.ExternalSweep) { s.Timeout = 0 }},
		{name: "no locale", mutate: func(s *app.ExternalSweep) { s.Locale = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := externalSweep(newStaleLedger(), rec.abandon)
			tc.mutate(&s)
			if fn, err := app.SweepExternalAttempts(s); !errors.Is(err, app.ErrInvalidSweepConfig) || fn != nil {
				t.Fatalf("SweepExternalAttempts() = (fn set %v, %v), want ErrInvalidSweepConfig and no function", fn != nil, err)
			}
		})
	}
}

func TestTheSweepAbandonsEachStaleAttemptInItsTenantAndNeverResends(t *testing.T) {
	second := staleAttempt("", "k-2", 2)
	second.StartedAt = 2_000
	ledger := newStaleLedger(staleAttempt("acme", "k-1", 1), second)
	rec := &abandonRecorder{}

	page, err := mustSweep(t, externalSweep(ledger, rec.abandon))(context.Background(), 6_000, ports.AttemptCursor{}, 50)

	if err != nil || page.Seen != 2 || page.Swept != 2 || page.Next != second.Cursor() {
		t.Fatalf("sweep = (%+v, %v), want two seen and swept, resuming after %+v", page, err, second.Cursor())
	}
	calls, other := ledger.snapshot()
	if len(calls) != 1 || calls[0] != (sweepCall{cutoff: 6_000, batch: 50}) || len(other) != 0 {
		t.Fatalf("ledger saw %+v and %v, want Stale(6000, from the oldest, 50) alone: the sweep never begins nor submits (RES-43)", calls, other)
	}
	for i, tc := range []struct {
		tenant ports.TenantID
		scoped bool
	}{
		{tenant: "acme", scoped: true},
		{tenant: "", scoped: false},
	} {
		t.Run(string(rec.seen[i].attempt.Key), func(t *testing.T) {
			got := rec.seen[i]
			tenant, scoped := got.execution.Tenant()
			causation, caused := got.execution.CausationID()
			switch {
			case tenant != tc.tenant || scoped != tc.scoped:
				t.Fatalf("tenant = (%q, %v), want (%q, %v): the attempt's own, absent when it had none (CTX-26)", tenant, scoped, tc.tenant, tc.scoped)
			case got.execution.Locale() != "pt-BR" || !got.deadline || got.execution.Deadline() <= 0:
				t.Fatalf("execution = %+v, deadline %v, want the sweep's locale and deadline (CTX-28)", got.execution, got.deadline)
			case got.execution.RequestID() == "" || got.execution.CorrelationID() == "" || got.execution.RequestID() == got.execution.CorrelationID():
				t.Fatalf("request %q, correlation %q, want two minted identifiers", got.execution.RequestID(), got.execution.CorrelationID())
			case !caused || causation != string(got.attempt.MessageID) || got.message.CausationID != causation:
				t.Fatalf("causation = (%q, %v), message %+v, want the attempt's message %q", causation, caused, got.message, got.attempt.MessageID)
			case got.message.CorrelationID != got.execution.CorrelationID():
				t.Fatalf("message correlation = %q, want the execution's %q", got.message.CorrelationID, got.execution.CorrelationID())
			}
		})
	}
	if rec.seen[0].execution.RequestID() == rec.seen[1].execution.RequestID() {
		t.Fatal("two attempts shared a request id: each closes in its own execution (CTX-15)")
	}
}

func TestAFailedAttemptDoesNotStopThePageAndStillAdvancesTheCursor(t *testing.T) {
	refused := errors.New("sweep_test: apply refused")
	last := staleAttempt("acme", "k-3", 1)
	for _, tc := range []struct {
		name      string
		ledger    *staleLedger
		outcomes  map[ports.OutboundKey]abandonOutcome
		want      app.SweepPage
		wantErr   error
		abandoned int
	}{
		{
			name:      "one of three fails",
			ledger:    newStaleLedger(staleAttempt("acme", "k-1", 1), staleAttempt("acme", "k-2", 1), last),
			outcomes:  map[ports.OutboundKey]abandonOutcome{"k-2": {err: refused}},
			want:      app.SweepPage{Seen: 3, Swept: 2, Next: last.Cursor()},
			wantErr:   refused,
			abandoned: 3,
		},
		{
			name:      "the last one fails",
			ledger:    newStaleLedger(staleAttempt("acme", "k-1", 1), last),
			outcomes:  map[ports.OutboundKey]abandonOutcome{"k-3": {err: refused}},
			want:      app.SweepPage{Seen: 2, Swept: 1, Next: last.Cursor()},
			wantErr:   refused,
			abandoned: 2,
		},
		{
			name:      "an attempt with no message mounts no context",
			ledger:    newStaleLedger(ports.Attempt{Tenant: "acme", Key: "k-1", N: 1, StartedAt: 1_000}, last),
			want:      app.SweepPage{Seen: 2, Swept: 1, Next: last.Cursor()},
			wantErr:   ports.ErrContextValueEmpty,
			abandoned: 1,
		},
		{
			name:    "stale fails",
			ledger:  &staleLedger{err: refused, listed: make(chan struct{}, 1)},
			wantErr: refused,
		},
		{
			name:   "an empty page",
			ledger: newStaleLedger(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &abandonRecorder{outcomes: tc.outcomes}

			page, err := mustSweep(t, externalSweep(tc.ledger, rec.abandon))(context.Background(), 6_000, ports.AttemptCursor{}, 50)

			if page != tc.want || !errors.Is(err, tc.wantErr) || (tc.wantErr == nil) != (err == nil) || len(rec.seen) != tc.abandoned {
				t.Fatalf("sweep = (%+v, %v) after %d abandons, want (%+v, %v) after %d", page, err, len(rec.seen), tc.want, tc.wantErr, tc.abandoned)
			}
		})
	}
}

func TestAPageOfAttemptsThatAlwaysFailDoesNotHoldBackTheOthers(t *testing.T) {
	refused := errors.New("sweep_test: apply refused")
	failing := []ports.Attempt{staleAttempt("acme", "f-1", 1), staleAttempt("acme", "f-2", 1)}
	good := []ports.Attempt{staleAttempt("acme", "g-1", 1), staleAttempt("acme", "g-2", 1)}
	ledger := newStaleLedger(append(append([]ports.Attempt(nil), failing...), good...)...)
	rec := &abandonRecorder{outcomes: map[ports.OutboundKey]abandonOutcome{"f-1": {err: refused}, "f-2": {err: refused}}}
	stop := runSweep(t, app.SweepConfig{Interval: time.Hour, Batch: 2, StaleAfter: 4_000}, mustSweep(t, externalSweep(ledger, rec.abandon)))

	for range 3 {
		awaitSweep(t, ledger.listed, "the cycle stopped at the page of failures: the attempts behind it wait an interval each")
	}
	_ = stop()
	if got := rec.keys(); !slices.Equal(got, []ports.OutboundKey{"f-1", "f-2", "g-1", "g-2"}) {
		t.Fatalf("abandoned %v, want the failing page once and the good attempts in the same cycle", got)
	}
	calls, _ := ledger.snapshot()
	want := []ports.AttemptCursor{{}, failing[1].Cursor(), good[1].Cursor()}
	for i, c := range calls {
		if c.after != want[i] {
			t.Fatalf("Stale call %d resumed after %+v, want %+v", i, c.after, want[i])
		}
	}
}

func TestACancelledSweepStopsBeforeTheNextAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	first := staleAttempt("acme", "k-1", 1)
	ledger := newStaleLedger(first, staleAttempt("acme", "k-2", 1))
	rec := &abandonRecorder{}
	abandon := func(ctx context.Context, attempt ports.Attempt) (ports.Finished, error) {
		defer cancel()
		return rec.abandon(ctx, attempt)
	}

	page, err := mustSweep(t, externalSweep(ledger, abandon))(ctx, 6_000, ports.AttemptCursor{}, 50)

	if page != (app.SweepPage{Seen: 1, Swept: 1, Next: first.Cursor()}) || !errors.Is(err, context.Canceled) || len(rec.seen) != 1 {
		t.Fatalf("sweep = (%+v, %v) after %d abandons, want one attempt swept and context.Canceled", page, err, len(rec.seen))
	}
}

type ledgerUoW struct{ ledger ports.AttemptLedger }

func (u ledgerUoW) Within(ctx context.Context, fn func(context.Context, ports.AttemptLedger) error) error {
	return fn(ctx, u.ledger)
}

type finishLedger struct {
	*staleLedger
	finished ports.Finished
}

func (l finishLedger) Finish(context.Context, ports.AttemptOutcome) (ports.Finished, error) {
	return l.finished, nil
}

func realAbandon(ledger ports.AttemptLedger, instrumentation ports.Instrumentation, steps *[]application.ExternalStep) func(context.Context, ports.Attempt) (ports.Finished, error) {
	return func(ctx context.Context, a ports.Attempt) (ports.Finished, error) {
		return application.Abandon(ctx, application.ExternalAbandon[ports.AttemptLedger]{
			UoW:             ledgerUoW{ledger: ledger},
			Ledger:          func(l ports.AttemptLedger) ports.AttemptLedger { return l },
			Attempt:         a,
			Profile:         ports.ExternalProfile{KeySemantics: ports.KeyNative, Efficacy: ports.EfficacyOnSubmit, MaxQueries: 3, KeyAlphabet: "0123456789abcdef", KeyMaxLength: 32},
			Clock:           purgeClock(10_000),
			Instrumentation: instrumentation,
			Apply: func(_ context.Context, _ ports.AttemptLedger, step application.ExternalStep) error {
				*steps = append(*steps, step)
				return nil
			},
		})
	}
}

func TestTheMountedContextIsTheOneApplicationAbandonAccepts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		tenant  ports.TenantID
		wantErr error
	}{
		{name: "an attempt of a tenant", tenant: "acme"},
		{name: "an attempt of no tenant", wantErr: application.ErrAbandonOutsideTenant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := finishLedger{staleLedger: newStaleLedger(staleAttempt(tc.tenant, "k-1", 1)), finished: ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassUnknownSent}}
			var steps []application.ExternalStep

			page, err := mustSweep(t, externalSweep(ledger, realAbandon(ledger, ports.NoInstrumentation(), &steps)))(context.Background(), 6_000, ports.AttemptCursor{}, 50)

			if !errors.Is(err, tc.wantErr) || (tc.wantErr == nil) != (err == nil) {
				t.Fatalf("sweep = (%+v, %v), want %v", page, err, tc.wantErr)
			}
			if tc.wantErr == nil && (page.Swept != 1 || len(steps) != 1 || steps[0].Class != ports.ClassUnknownSent) {
				t.Fatalf("sweep = %+v, steps %+v, want one attempt closed as UnknownSent and its step applied", page, steps)
			}
		})
	}
}

type sweptSignals struct {
	logs   []map[string]string
	bodies []string
	levels []log.Severity
	labels []map[string]string
	series map[string]int64
	spans  []map[string]string
}

func sweepWithSignals(t *testing.T, key ports.OutboundKey) sweptSignals {
	t.Helper()
	exporter := &recordingExporter{}
	reader := sdkmetric.NewManualReader()
	spans := tracetest.NewInMemoryExporter()
	config := otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource:   otelboot.Resource{ServiceName: "reservations", ServiceVersion: "1.0.0", ServiceInstanceID: "reservations-1", Role: "consumer"},
		Sampling: tracing.Rates{
			tracing.ClassWrite: 1, tracing.ClassRead: 1, tracing.ClassError: 1,
			tracing.ClassMaintenance: 1, tracing.ClassUnclassified: 1,
		},
		TraceExporter: spans,
		MetricReader:  reader,
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, exporter)
	rt, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("otelboot.Start() = %v", err)
	}
	ledger := finishLedger{staleLedger: newStaleLedger(staleAttempt("acme", key, 3)), finished: ports.Finished{Result: ports.FinishConflict, Recorded: ports.ClassAccepted}}
	var steps []application.ExternalStep
	s := externalSweep(ledger, realAbandon(ledger, usecase.New(rt, nil, nil, nil), &steps))
	s.Tracer = rt.Tracer()

	page, err := mustSweep(t, s)(context.Background(), 6_000, ports.AttemptCursor{}, 50)
	if err != nil || page.Swept != 1 {
		t.Fatalf("sweep = (%+v, %v), want one attempt swept: a conflict is a closed attempt, not a failure", page, err)
	}
	if err := rt.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	got := sweptSignals{series: map[string]int64{}}
	for _, span := range spans.GetSpans() {
		got.spans = append(got.spans, stringsOf(span.Attributes))
	}
	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}
	for _, record := range exporter.records {
		fields := map[string]string{}
		for key, value := range recordAttributes(record) {
			fields[key] = value.String()
		}
		got.logs, got.bodies, got.levels = append(got.logs, fields), append(got.bodies, record.Body().AsString()), append(got.levels, record.Severity())
	}
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			for _, set := range attributeSets(m.Data) {
				got.labels = append(got.labels, stringsOf(set.ToSlice()))
			}
			if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
				for _, point := range sum.DataPoints {
					got.series[m.Name] += point.Value
				}
			}
		}
	}
	if len(got.spans) == 0 || len(got.logs) == 0 || len(got.labels) == 0 {
		t.Fatalf("spans %d, logs %d, label sets %d: a signal left nothing to search", len(got.spans), len(got.logs), len(got.labels))
	}
	return got
}

func attributeSets(data metricdata.Aggregation) []attribute.Set {
	var sets []attribute.Set
	switch d := data.(type) {
	case metricdata.Sum[int64]:
		for _, p := range d.DataPoints {
			sets = append(sets, p.Attributes)
		}
	case metricdata.Sum[float64]:
		for _, p := range d.DataPoints {
			sets = append(sets, p.Attributes)
		}
	case metricdata.Gauge[int64]:
		for _, p := range d.DataPoints {
			sets = append(sets, p.Attributes)
		}
	case metricdata.Gauge[float64]:
		for _, p := range d.DataPoints {
			sets = append(sets, p.Attributes)
		}
	case metricdata.Histogram[float64]:
		for _, p := range d.DataPoints {
			sets = append(sets, p.Attributes)
		}
	}
	return sets
}

func stringsOf(kvs []attribute.KeyValue) map[string]string {
	fields := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		fields[string(kv.Key)] = kv.Value.String()
	}
	return fields
}

func carries(fields map[string]string, raw string) bool {
	for key, value := range fields {
		if strings.Contains(key, raw) || strings.Contains(value, raw) {
			return true
		}
	}
	return false
}

func TestTheOutboundKeyNeverLeavesTheProcessInALogALabelOrASpan(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  ports.OutboundKey
	}{
		{name: "a key in the validated format", key: "abc123def456"},
		{name: "a key outside it", key: "secret key/with spaces"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := sweepWithSignals(t, tc.key)

			conflicts := 0
			for i, body := range got.bodies {
				if body == "external attempt conflict" {
					conflicts++
					if got.levels[i] != log.SeverityWarn || got.logs[i]["dmpf.external.class"] != "accepted" || got.logs[i]["dmpf.operation"] != "external.abandon" {
						t.Fatalf("conflict record = %v at %v, want Warn with the class and the operation", got.logs[i], got.levels[i])
					}
				}
			}
			if conflicts != 1 || got.series["dmpf.external.attempt.conflicts"] != 1 {
				t.Fatalf("conflict records = %d, series = %v, want the conflict reported once in each", conflicts, got.series)
			}
			for _, group := range [][]map[string]string{got.logs, got.labels, got.spans} {
				for _, fields := range group {
					if carries(fields, string(tc.key)) {
						t.Fatalf("%v carries the outbound key %q: it never leaves the ledger", fields, tc.key)
					}
					if _, named := fields["dmpf.idempotency_key"]; named {
						t.Fatalf("%v names an idempotency key: the outbound key is not one the edge validated", fields)
					}
				}
			}
		})
	}
}
