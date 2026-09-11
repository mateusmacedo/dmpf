//go:build integration

package dmpfpostgres_test

import (
	"context"
	"errors"
	"testing"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

func TestInboxSignalsAggregatesByReason(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	q := dmpfpostgres.NewQuarantine(pool)

	entries := []dmpfports.Contained{
		{Consumer: "orders", MessageID: "m-1", Reason: dmpfports.ReasonTerminalFailure, Envelope: []byte{1}, At: 100},
		{Consumer: "orders", MessageID: "m-2", Reason: dmpfports.ReasonTerminalFailure, Envelope: []byte{2}, At: 200},
		{Consumer: "orders", MessageID: "m-3", Reason: dmpfports.ReasonCollision, Envelope: []byte{3}, At: 300},
		{Consumer: "orders", MessageID: "m-4", Reason: dmpfports.ReasonAttemptsExhausted, Envelope: []byte{4}, At: 400},
		{Consumer: "orders", MessageID: "m-5", Reason: dmpfports.ReasonInvalidEnvelope, Envelope: []byte{5}, At: 500},
		{Consumer: "billing", MessageID: "m-6", Reason: dmpfports.ReasonTerminalFailure, Envelope: []byte{6}, At: 600},
	}
	for _, e := range entries {
		if err := q.Quarantine(ctx, e); err != nil {
			t.Fatalf("Quarantine(%s) = %v", e.MessageID, err)
		}
	}

	s, err := dmpfpostgres.InboxSignals(ctx, pool, "orders")
	if err != nil {
		t.Fatalf("InboxSignals() = %v, want nil", err)
	}

	if s.QuarantineDepth != 5 {
		t.Errorf("QuarantineDepth = %d, want 5", s.QuarantineDepth)
	}
	if s.TerminalFailures != 2 {
		t.Errorf("TerminalFailures = %d, want 2", s.TerminalFailures)
	}
	if s.Collisions != 1 {
		t.Errorf("Collisions = %d, want 1", s.Collisions)
	}
	if s.AttemptsExhausted != 1 {
		t.Errorf("AttemptsExhausted = %d, want 1", s.AttemptsExhausted)
	}
	if s.InvalidEnvelopes != 1 {
		t.Errorf("InvalidEnvelopes = %d, want 1", s.InvalidEnvelopes)
	}
}

func TestInboxSignalsEmptyQuarantine(t *testing.T) {
	pool := openPool(t)

	s, err := dmpfpostgres.InboxSignals(context.Background(), pool, "orders")
	if err != nil {
		t.Fatalf("InboxSignals() = %v, want nil", err)
	}
	if s.QuarantineDepth != 0 {
		t.Errorf("QuarantineDepth = %d, want 0", s.QuarantineDepth)
	}
}

func TestOutboxSignalsCountOnlyWhatTheCycleStillOwes(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	now := dmpfports.Instant(10_000)

	// Two rows are still owed, one gave up and one is done. The oldest of the
	// two owed is what lag has to reflect.
	pendingRow := defaultRow("m-pending")
	pendingRow.occurredAt, pendingRow.availableAt, pendingRow.attempts = 4_000, 4_000, 2
	insert(t, ctx, pool, pendingRow)

	publishingRow := defaultRow("m-publishing")
	publishingRow.occurredAt, publishingRow.availableAt = 6_000, 6_000
	publishingRow.status, publishingRow.attempts = "publishing", 3
	insert(t, ctx, pool, publishingRow)

	failedRow := defaultRow("m-failed")
	failedRow.occurredAt, failedRow.availableAt = 1_000, 1_000
	failedRow.status, failedRow.attempts = "failed", 9
	insert(t, ctx, pool, failedRow)

	publishedRow := defaultRow("m-published")
	publishedRow.occurredAt, publishedRow.availableAt = 2_000, 2_000
	publishedRow.status = "published"
	insert(t, ctx, pool, publishedRow)

	health, err := dmpfpostgres.OutboxSignals(ctx, pool, fixedClock(now))
	if err != nil {
		t.Fatalf("OutboxSignals() = %v, want nil", err)
	}

	want := dmpfpostgres.OutboxHealth{
		Pending:  2,
		Lag:      int64(now) - pendingRow.occurredAt,
		Attempts: 5,
		Failures: 1,
	}
	if health != want {
		t.Fatalf("OutboxSignals() = %+v, want %+v", health, want)
	}
}

func TestOutboxSignalsReportNoLagWithNothingPending(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	failedRow := defaultRow("m-failed")
	failedRow.status = "failed"
	insert(t, ctx, pool, failedRow)

	health, err := dmpfpostgres.OutboxSignals(ctx, pool, fixedClock(10_000))
	if err != nil {
		t.Fatalf("OutboxSignals() = %v, want nil", err)
	}
	if health.Pending != 0 || health.Lag != 0 {
		t.Fatalf("OutboxSignals() = %+v, want no pending and no lag", health)
	}
}

func TestOutboxSignalsRefuseAnIncompleteStore(t *testing.T) {
	if _, err := dmpfpostgres.OutboxSignals(context.Background(), nil, fixedClock(0)); !errors.Is(err, dmpfpostgres.ErrIncompleteStore) {
		t.Fatalf("OutboxSignals() with no pool = %v, want ErrIncompleteStore", err)
	}
}
