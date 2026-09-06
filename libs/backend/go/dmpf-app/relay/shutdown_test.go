package relay

import (
	"context"
	"errors"
	"testing"
	"time"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
)

// stallingPublisher blocks until the caller's context is cancelled, which is
// how a test puts records in flight across a shutdown.
type stallingPublisher struct {
	entered chan struct{}
	once    bool
}

func (p *stallingPublisher) Publish(ctx context.Context, _ string, _ []byte) error {
	if !p.once {
		p.once = true
		close(p.entered)
	}
	<-ctx.Done()
	return ctx.Err()
}

func shutdownRelay(store Store, publisher Publisher) Relay {
	relay := loopRelay(store, publisher)
	relay.ShutdownGrace = 2 * time.Second
	return relay
}

// OBX-13: shutdown must not leave a claim alive. A record still in flight when
// the loop stops is handed back to the pool at once, rather than waiting out a
// lease nobody is using.
func TestShutdownReleasesTheClaimsStillInFlight(t *testing.T) {
	record := publishableRecord(t)
	record.ID = 1
	store := newFakeStore([]dmpfpostgres.Claimed{record})
	store.failOnCancelledContext = true
	publisher := &stallingPublisher{entered: make(chan struct{})}
	relay := shutdownRelay(store, publisher)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- relay.Run(ctx) }()

	<-publisher.entered
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	recorded := store.recorded()
	if len(recorded) != 1 {
		t.Fatalf("%d transitions, want 1 (the release): %+v", len(recorded), recorded)
	}
	if recorded[0].kind != "rescheduled" {
		t.Fatalf("transition = %q, want rescheduled", recorded[0].kind)
	}
	store.mu.Lock()
	claimed := store.claimIDs[0]
	store.mu.Unlock()
	if recorded[0].claimID != claimed {
		t.Fatalf("released under claim %q, want %q", recorded[0].claimID, claimed)
	}
	if want := dmpfports.Instant(10_000); recorded[0].availableAt != want {
		t.Fatalf("available_at = %d, want %d (right now, not a backoff)", recorded[0].availableAt, want)
	}
}

// The release runs on a cleanup context of its own: the context that ended the
// loop is already cancelled, and reusing it would fail every write the
// shutdown depends on (the same reason uow.go rolls back WithoutCancel).
func TestShutdownWritesOnACleanupContextOfItsOwn(t *testing.T) {
	record := publishableRecord(t)
	record.ID = 1
	store := newFakeStore([]dmpfpostgres.Claimed{record})
	store.failOnCancelledContext = true
	publisher := &stallingPublisher{entered: make(chan struct{})}
	relay := shutdownRelay(store, publisher)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- relay.Run(ctx) }()

	<-publisher.entered
	cancel()
	<-done

	if store.cancelledWrites.Load() != 0 {
		t.Fatalf("%d writes were attempted on the cancelled context", store.cancelledWrites.Load())
	}
	if len(store.recorded()) != 1 {
		t.Fatal("the release never reached the store")
	}
}

// A record that already reached a terminal transition is not released again:
// the claim is spent, and rescheduling it would resurrect a published message.
func TestShutdownLeavesFinishedRecordsAlone(t *testing.T) {
	batch := batchOf(t, 3)
	store := newFakeStore(batch)
	publisher := &fakePublisher{}
	relay := shutdownRelay(store, publisher)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	for _, got := range store.recorded() {
		if got.kind != "published" {
			t.Fatalf("transition = %q, want published only: a finished record was released", got.kind)
		}
	}
	if len(store.recorded()) != len(batch) {
		t.Fatalf("%d transitions, want %d", len(store.recorded()), len(batch))
	}
}

func TestShutdownStopsClaiming(t *testing.T) {
	store := newFakeStore()
	relay := shutdownRelay(store, &fakePublisher{})
	relay.Interval = time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- relay.Run(ctx) }()

	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done

	settled := store.claims.Load()
	time.Sleep(30 * time.Millisecond)
	if after := store.claims.Load(); after != settled {
		t.Fatalf("%d claims after the loop returned, want none past %d", after, settled)
	}
}

// Shutting down has to beat the lease: if it took longer, the record would be
// reclaimable anyway and the graceful release would have bought nothing.
func TestShutdownFinishesWellInsideTheLease(t *testing.T) {
	record := publishableRecord(t)
	record.ID = 1
	store := newFakeStore([]dmpfpostgres.Claimed{record})
	store.failOnCancelledContext = true
	publisher := &stallingPublisher{entered: make(chan struct{})}
	relay := shutdownRelay(store, publisher)
	relay.Lease = 5 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- relay.Run(ctx) }()

	<-publisher.entered
	start := time.Now()
	cancel()
	<-done

	if elapsed := time.Since(start); elapsed >= relay.Lease {
		t.Fatalf("shutdown took %v, want well under the %v lease", elapsed, relay.Lease)
	}
}

func TestSignalsReportTheOutboxHealth(t *testing.T) {
	store := newFakeStore()
	store.health = dmpfpostgres.OutboxHealth{Pending: 7, Lag: 1_500, Attempts: 12, Failures: 2}
	relay := shutdownRelay(store, &fakePublisher{})

	got, err := relay.Signals(context.Background())
	if err != nil {
		t.Fatalf("Signals() = %v, want nil", err)
	}
	if got != store.health {
		t.Fatalf("Signals() = %+v, want %+v", got, store.health)
	}
}

func TestSignalsRefuseAnIncompleteRelay(t *testing.T) {
	var relay Relay

	if _, err := relay.Signals(context.Background()); err != ErrIncompleteRelay {
		t.Fatalf("Signals() = %v, want ErrIncompleteRelay", err)
	}
}

var errTransitionLost = errors.New("the transition never landed")

// The release belongs to the scan, not to the process: a claim nobody finished
// goes back to the pool at the end of its own scan. Holding it until shutdown
// would keep the record locked for the rest of the lease and let the set of
// held records grow without bound in a process that runs for weeks.
func TestAnUnfinishedClaimIsReleasedAtTheEndOfItsOwnScan(t *testing.T) {
	first := batchOf(t, 1)
	second := batchOf(t, 1)
	second[0].ID = 2

	store := newFakeStore(first, second)
	store.failMarkPublished = true
	relay := shutdownRelay(store, &fakePublisher{})
	relay.Interval = time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	recorded := store.recorded()
	if len(recorded) != 2 {
		t.Fatalf("%d releases, want exactly 2 — one per scan, none carried over: %+v", len(recorded), recorded)
	}

	released := map[int64]int{}
	for _, got := range recorded {
		if got.kind != "rescheduled" {
			t.Fatalf("transition = %q, want rescheduled", got.kind)
		}
		released[got.id]++
	}
	for id, times := range released {
		if times != 1 {
			t.Fatalf("record %d was released %d times, want once", id, times)
		}
	}
}

// A released claim did not fail. Writing a reason of its own would overwrite
// the error that explains why the delivery did not land, which is the only
// diagnosis the row carries.
func TestReleasingAClaimDoesNotOverwriteTheRecordedError(t *testing.T) {
	store := newFakeStore(batchOf(t, 1))
	store.failMarkPublished = true
	relay := shutdownRelay(store, &fakePublisher{})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	recorded := store.recorded()
	if len(recorded) == 0 {
		t.Fatal("the release never reached the store")
	}
	if recorded[0].lastError != "" {
		t.Fatalf("release wrote last_error = %q, want it left alone", recorded[0].lastError)
	}
}
