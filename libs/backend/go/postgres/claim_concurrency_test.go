//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// The window this test walks is the one FND-04 §5.4 leaves open on purpose:
// A's lease expires while its message is still in flight, B takes over and
// publishes, and A comes back to write. A's write has to be rejected and the
// record has to be exactly what B left (OBX-10, OBX-11).
func TestALateWriteFromAnExpiredClaimChangesNothing(t *testing.T) {
	pool := openConcurrencyPool(t)
	ctx := context.Background()
	now := ports.Instant(10_000)
	const lease = 1_000 * time.Nanosecond

	insert(t, ctx, pool, defaultRow("m-1"))

	first := postgres.NewOutboxStore(pool, fixedClock(now))
	claimedByA, err := first.Claim(ctx, claimA, 10, lease)
	if err != nil || len(claimedByA) != 1 {
		t.Fatalf("A Claim() = %v, %d rows; want nil, 1", err, len(claimedByA))
	}

	afterExpiry := now + ports.Instant(lease) + 1
	second := postgres.NewOutboxStore(pool, fixedClock(afterExpiry))
	claimedByB, err := second.Claim(ctx, claimB, 10, testLease)
	if err != nil || len(claimedByB) != 1 {
		t.Fatalf("B Claim() = %v, %d rows; want nil, 1", err, len(claimedByB))
	}
	if claimedByB[0].ID != claimedByA[0].ID {
		t.Fatalf("B claimed record %d, want the same record A held (%d)", claimedByB[0].ID, claimedByA[0].ID)
	}

	if affected, err := second.MarkPublished(ctx, claimedByB[0].ID, claimedByB[0].LockedBy); err != nil || affected != 1 {
		t.Fatalf("B MarkPublished() = %v, %d rows; want nil, 1", err, affected)
	}
	afterB := snapshot(t, ctx, pool, claimedByB[0].ID)

	late := []struct {
		name  string
		apply func() (int64, error)
	}{
		{"MarkPublished", func() (int64, error) {
			return first.MarkPublished(ctx, claimedByA[0].ID, claimedByA[0].LockedBy)
		}},
		{"Reschedule", func() (int64, error) {
			return first.Reschedule(ctx, claimedByA[0].ID, claimedByA[0].LockedBy, afterExpiry+9_999, "late transient")
		}},
		{"Fail", func() (int64, error) {
			return first.Fail(ctx, claimedByA[0].ID, claimedByA[0].LockedBy, "late failure")
		}},
	}
	for _, l := range late {
		t.Run("A writes late with "+l.name, func(t *testing.T) {
			affected, err := l.apply()
			if err != nil {
				t.Fatalf("%s = %v, want nil", l.name, err)
			}
			if affected != 0 {
				t.Fatalf("%s affected %d rows, want 0", l.name, affected)
			}
			if got := snapshot(t, ctx, pool, claimedByA[0].ID); got != afterB {
				t.Fatalf("record moved under A's late write:\nwant=%+v\n got=%+v", afterB, got)
			}
		})
	}
}

// SKIP LOCKED is what keeps two relays off each other: while A holds the row
// lock, B must step over it rather than wait for A's transaction.
func TestOverlappingClaimsSkipTheLockedRecord(t *testing.T) {
	pool := openConcurrencyPool(t)
	ctx := context.Background()
	now := ports.Instant(10_000)

	cases := []struct {
		name      string
		available int
		want      int
	}{
		{"a second record is available", 2, 1},
		{"the locked record is the only one", 1, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			truncate(t, ctx, pool)
			var ids []int64
			for i := range c.available {
				r := defaultRow(string(rune('a'+i)) + "-1")
				ids = append(ids, insert(t, ctx, pool, r))
			}

			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatalf("Begin() = %v, want nil", err)
			}
			defer func() { _ = tx.Rollback(ctx) }()

			var locked int64
			if err := tx.QueryRow(ctx, claimEligibility, int64(now), 1).Scan(&locked); err != nil {
				t.Fatalf("A's SELECT FOR UPDATE = %v, want nil", err)
			}
			if locked != ids[0] {
				t.Fatalf("A locked record %d, want %d (lowest available_at, id)", locked, ids[0])
			}

			store := postgres.NewOutboxStore(pool, fixedClock(now))
			claimed, err := store.Claim(ctx, claimB, 10, testLease)
			if err != nil {
				t.Fatalf("B Claim() = %v, want nil", err)
			}
			if len(claimed) != c.want {
				t.Fatalf("B claimed %d rows, want %d", len(claimed), c.want)
			}
			for _, got := range claimed {
				if got.ID == locked {
					t.Fatalf("B claimed the record A holds (%d)", locked)
				}
			}
		})
	}
}
