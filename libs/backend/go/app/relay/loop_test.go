package relay

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

var errTransport = errors.New("transport unavailable")

func loopRelay(store Store, publisher Publisher) Relay {
	return Relay{
		Store:       store,
		Publisher:   publisher,
		ClaimIDs:    &countingIDs{},
		Clock:       fixedClock(10_000),
		Source:      testSource,
		MaxAttempts: 3,
		Lease:       time.Minute,
		Interval:    50 * time.Millisecond,
		BatchSize:   10,
		Concurrency: 2,
		Backoff:     func(int) time.Duration { return time.Second },
	}
}

// An empty batch means there is nothing to drain: the loop waits out the scan
// interval instead of spinning on the database.
func TestAnEmptyBatchWaitsOutTheScanInterval(t *testing.T) {
	store := newFakeStore()
	relay := loopRelay(store, &fakePublisher{})

	ctx, cancel := context.WithTimeout(context.Background(), 220*time.Millisecond)
	defer cancel()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	// Four intervals fit in the window; without the wait this would be
	// thousands of round trips.
	if claims := store.claims.Load(); claims < 2 || claims > 8 {
		t.Fatalf("%d claims in 220ms with a 50ms interval, want between 2 and 8", claims)
	}
}

func TestRunNeverRunsMorePublishersThanTheConcurrencyLimit(t *testing.T) {
	batch := batchOf(t, 12)
	store := newFakeStore(batch)
	publisher := &fakePublisher{hold: 20 * time.Millisecond}
	relay := loopRelay(store, publisher)
	relay.Concurrency = 3

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	if delivered := publisher.delivered.Load(); delivered != int64(len(batch)) {
		t.Fatalf("delivered %d records, want %d", delivered, len(batch))
	}
	peak := publisher.peak.Load()
	if peak > int64(relay.Concurrency) {
		t.Fatalf("%d publishers ran at once, want at most %d", peak, relay.Concurrency)
	}
	// Without a lower bound the assertion above would also pass on a loop that
	// publishes one record at a time, which is not the behaviour under test.
	if peak < 2 {
		t.Fatalf("peak concurrency was %d: the batch was published serially", peak)
	}
}

func TestATransientFailureIsRescheduledUntilTheCeiling(t *testing.T) {
	cases := []struct {
		name     string
		attempts int
		want     string
	}{
		{"below the ceiling", 1, "rescheduled"},
		{"one short of the ceiling", 2, "rescheduled"},
		{"at the ceiling", 3, "failed"},
		{"past the ceiling", 9, "failed"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			record := publishableRecord(t)
			record.ID = 1
			record.AttemptCount = c.attempts

			store := newFakeStore([]postgres.Claimed{record})
			relay := loopRelay(store, &fakePublisher{err: errTransport})

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
			defer cancel()
			if err := relay.Run(ctx); err != nil {
				t.Fatalf("Run() = %v, want nil", err)
			}

			recorded := store.recorded()
			if len(recorded) != 1 {
				t.Fatalf("%d transitions, want 1: %+v", len(recorded), recorded)
			}
			if recorded[0].kind != c.want {
				t.Fatalf("transition = %q, want %q", recorded[0].kind, c.want)
			}
			if got := recorded[0].lastError; got != "relay: delivery failed (*errors.errorString)" {
				t.Fatalf("last_error = %q, want the sanitized form", got)
			}
		})
	}
}

// OBX-10 and OBX-11: a rejected write means another claim owns the record. The
// loop records the fact and moves on; republishing would double-deliver on
// purpose, which is exactly what the condition on locked_by exists to prevent.
func TestAReplacedClaimIsRecordedAndNeverRepublished(t *testing.T) {
	batch := batchOf(t, 3)
	store := newFakeStore(batch)
	store.affected = 0
	publisher := &fakePublisher{}
	relay := loopRelay(store, publisher)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	if delivered := publisher.delivered.Load(); delivered != int64(len(batch)) {
		t.Fatalf("published %d times, want %d: a lost claim must not be retried", delivered, len(batch))
	}
	if recorded := store.recorded(); len(recorded) != len(batch) {
		t.Fatalf("%d transitions, want %d", len(recorded), len(batch))
	}
}

func TestEveryScanClaimsUnderAFreshIdentity(t *testing.T) {
	store := newFakeStore(batchOf(t, 1), batchOf(t, 1))
	relay := loopRelay(store, &fakePublisher{})
	relay.Interval = time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	seen := make(map[string]bool, len(store.claimIDs))
	for _, id := range store.claimIDs {
		if seen[id] {
			t.Fatalf("claim identity %q was reused across scans", id)
		}
		seen[id] = true
	}
	if len(seen) < 2 {
		t.Fatalf("only %d scans ran, want at least 2", len(seen))
	}
}

func TestExponentialBackoffStaysInsideTheExpectedBand(t *testing.T) {
	const (
		base    = 100 * time.Millisecond
		ceiling = time.Second
	)

	cases := []struct {
		attempt  int
		wantWide time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, ceiling},
		{40, ceiling},
	}

	for _, c := range cases {
		floor := ExponentialBackoff(base, ceiling, func() float64 { return 0 })(c.attempt)
		roof := ExponentialBackoff(base, ceiling, func() float64 { return 1 })(c.attempt)

		if floor != c.wantWide/2 {
			t.Errorf("attempt %d with no jitter = %v, want %v", c.attempt, floor, c.wantWide/2)
		}
		if roof != c.wantWide {
			t.Errorf("attempt %d with full jitter = %v, want %v", c.attempt, roof, c.wantWide)
		}
	}
}

// Half the window is fixed and half is random: the record always makes
// progress, and two relays that failed together do not come back together.
func TestExponentialBackoffJittersWithinTheSecondHalf(t *testing.T) {
	const (
		base    = 100 * time.Millisecond
		ceiling = time.Second
	)

	for _, fraction := range []float64{0, 0.25, 0.5, 0.75, 0.99} {
		got := ExponentialBackoff(base, ceiling, func() float64 { return fraction })(3)
		if got < 200*time.Millisecond || got > 400*time.Millisecond {
			t.Errorf("jitter %.2f on attempt 3 = %v, want between 200ms and 400ms", fraction, got)
		}
	}
}

func TestExponentialBackoffRejectsNoConfiguration(t *testing.T) {
	backoff := ExponentialBackoff(0, 0, nil)

	if got := backoff(1); got != 0 {
		t.Fatalf("backoff with no base = %v, want 0", got)
	}
}

func TestRunRefusesAnIncompleteRelay(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(r *Relay)
	}{
		{"no store", func(r *Relay) { r.Store = nil }},
		{"no publisher", func(r *Relay) { r.Publisher = nil }},
		{"no claim ids", func(r *Relay) { r.ClaimIDs = nil }},
		{"no clock", func(r *Relay) { r.Clock = nil }},
		{"no source", func(r *Relay) { r.Source = "" }},
		{"no batch size", func(r *Relay) { r.BatchSize = 0 }},
		{"no lease", func(r *Relay) { r.Lease = 0 }},
		{"no concurrency", func(r *Relay) { r.Concurrency = 0 }},
		{"no interval", func(r *Relay) { r.Interval = 0 }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			relay := loopRelay(newFakeStore(), &fakePublisher{})
			c.break_(&relay)

			if err := relay.Run(context.Background()); !errors.Is(err, ErrIncompleteRelay) {
				t.Fatalf("Run() = %v, want ErrIncompleteRelay", err)
			}
		})
	}
}

func TestRescheduleWritesTheBackoffOntoAvailableAt(t *testing.T) {
	record := publishableRecord(t)
	record.ID = 1
	record.AttemptCount = 1

	store := newFakeStore([]postgres.Claimed{record})
	relay := loopRelay(store, &fakePublisher{err: errTransport})
	relay.Backoff = func(int) time.Duration { return 750 * time.Millisecond }

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	recorded := store.recorded()
	want := ports.Instant(10_000) + ports.Instant(750*time.Millisecond)
	if len(recorded) != 1 || recorded[0].availableAt != want {
		t.Fatalf("available_at = %+v, want %d", recorded, want)
	}
}

// OBX-03, ERR-20, ERR-21: last_error carries no credentials, no business data
// and no stack trace. The split is by origin — deciding by inspecting the error
// chain would let a transport error that wraps one of this block's own
// sentinels carry the transport's message through untouched.
func TestATransportFailureNeverReachesLastErrorVerbatim(t *testing.T) {
	const secret = "kafka://user:hunter2@broker.internal:9092"

	cases := []struct {
		name string
		err  error
	}{
		{"a plain transport error", errors.New(secret + " refused the record")},
		{"a transport error wrapping one of our sentinels", fmt.Errorf("%s refused the record: %w", secret, envelope.ErrModality)},
		{"a transport error wrapping an assembly sentinel", fmt.Errorf("%s refused: %w", secret, ErrPayloadHashMismatch)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			record := publishableRecord(t)
			record.ID = 1
			store := newFakeStore([]postgres.Claimed{record})
			relay := loopRelay(store, &fakePublisher{err: c.err})

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
			defer cancel()
			if err := relay.Run(ctx); err != nil {
				t.Fatalf("Run() = %v, want nil", err)
			}

			recorded := store.recorded()
			if len(recorded) != 1 {
				t.Fatalf("%d transitions, want 1: %+v", len(recorded), recorded)
			}
			if strings.Contains(recorded[0].lastError, secret) {
				t.Fatalf("last_error carries the transport's own message: %q", recorded[0].lastError)
			}
			if !strings.HasPrefix(recorded[0].lastError, "relay: delivery failed (") {
				t.Fatalf("last_error = %q, want the transport named by its type alone", recorded[0].lastError)
			}
		})
	}
}

// A failure this block raised is named in full: its text is a fixed string
// about a rule, an attribute or a digest, and dropping it would leave the row
// with no diagnosis at all.
func TestAnAssemblyFailureIsRecordedInFull(t *testing.T) {
	record := publishableRecord(t)
	record.ID = 1
	record.PayloadHash = "sha-256:not-the-frozen-bytes"

	store := newFakeStore([]postgres.Claimed{record})
	relay := loopRelay(store, &fakePublisher{})

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	recorded := store.recorded()
	if len(recorded) != 1 || recorded[0].kind != "failed" {
		t.Fatalf("transitions = %+v, want one failed", recorded)
	}
	if !strings.Contains(recorded[0].lastError, ErrPayloadHashMismatch.Error()) {
		t.Fatalf("last_error = %q, want it to name the mismatch", recorded[0].lastError)
	}
}
