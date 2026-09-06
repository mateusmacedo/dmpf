//go:build integration

package dmpfpostgres_test

import (
	"context"
	"testing"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
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
