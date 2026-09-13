//go:build integration

package dmpfpostgres_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/tb/pg"
)

// KIT-04 over the real dependency: the same suites the in-memory realization
// passes, run against Postgres (§8.1.1). The unit-of-work and outbox tests of
// this package keep their own finer clauses; these are the shared contract.

func TestUnitOfWorkConformsToTheKit(t *testing.T) {
	pool := pg.OpenPool(t)
	v := providerkit.UnitOfWork(func() providerkit.UnitOfWorkSubject[writer] {
		pg.ResetTables(t, pool)
		n := 0
		return providerkit.UnitOfWorkSubject[writer]{
			UoW: dmpfpostgres.NewUnitOfWork(pool, bindWriter),
			Write: func(ctx context.Context, w writer) error {
				n++
				return w.write(ctx, "o-"+strconv.Itoa(n))
			},
			Kept: func() int { return kept(t, pool) },
			// A commit failure cannot be injected into pgx from outside; the
			// clause is reported as skipped, not silently absent.
			ArmCommitFailure: nil,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 1 {
		t.Fatalf("skipped = %v, want exactly the commit-failure clause", v.Skipped)
	}
	evidence.RecordVerdict(t, "provider", "postgres-unit-of-work", v)
}

func TestInboxConformsToTheKit(t *testing.T) {
	pool := pg.OpenPool(t)
	v := providerkit.Inbox(func() providerkit.InboxSubject {
		pg.ResetTables(t, pool)
		return providerkit.InboxSubject{
			Within: func(ctx context.Context, consumer string, fn func(ctx context.Context, inbox dmpfports.Inbox) error) error {
				uow := dmpfpostgres.NewUnitOfWork(pool, func(tx *dmpfpostgres.Tx) dmpfports.Inbox {
					return tx.Inbox(consumer, 2*time.Second)
				})
				return uow.Within(ctx, fn)
			},
			ReadStatus: func(consumer string, id dmpfports.MessageID) (dmpfports.Status, bool) {
				return committedStatus(t, pool, consumer, id)
			},
			Concurrent:       true,
			ConsumerMismatch: dmpfpostgres.ErrInboxConsumerMismatch,
			Rows: func() int {
				var n int
				if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM dmpf_inbox").Scan(&n); err != nil {
					t.Fatalf("inbox rows: %v", err)
				}
				return n
			},
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("Postgres serializes on the key; nothing should be skipped: %v", v.Skipped)
	}
	evidence.RecordVerdict(t, "provider", "postgres-inbox", v)
}

func TestOutboxStoreConformsToTheKit(t *testing.T) {
	pool := pg.OpenPool(t)
	v := providerkit.Outbox(func() providerkit.OutboxSubject[dmpfpostgres.Claimed] {
		pg.ResetTables(t, pool)
		fake := clock.New(dmpfports.Instant(1_000_000))
		store := dmpfpostgres.NewOutboxStore(pool, fake)
		return providerkit.OutboxSubject[dmpfpostgres.Claimed]{
			Store: store,
			Enqueue: func(n int) error {
				for i := range n {
					r := defaultRow("m-kit-" + strconv.Itoa(i))
					r.occurredAt, r.availableAt = int64(fake.Now()), int64(fake.Now())
					insert(t, context.Background(), pool, r)
				}
				return nil
			},
			ID:    func(c dmpfpostgres.Claimed) int64 { return c.ID },
			Clock: fake,
			State: func(id int64) (providerkit.RecordState, error) {
				var (
					st                       providerkit.RecordState
					lockedBy, lastError      *string
					lockedUntil, publishedAt *int64
					availableAt              int64
				)
				err := pool.QueryRow(context.Background(), `
					SELECT status, locked_by, locked_until, available_at, attempt_count, published_at, last_error
					  FROM dmpf_outbox WHERE id = $1`, id).
					Scan(&st.Status, &lockedBy, &lockedUntil, &availableAt, &st.Attempts, &publishedAt, &lastError)
				st.AvailableAt = dmpfports.Instant(availableAt)
				if lockedBy != nil {
					st.LockedBy = *lockedBy
				}
				if lastError != nil {
					st.LastError = *lastError
				}
				if lockedUntil != nil {
					st.LockedUntil = dmpfports.Instant(*lockedUntil)
				}
				if publishedAt != nil {
					st.PublishedAt = dmpfports.Instant(*publishedAt)
				}
				return st, err
			},
			Pending: func() (int64, error) {
				health, err := dmpfpostgres.OutboxSignals(context.Background(), pool, fake)
				return health.Pending, err
			},
			Purge: func(before dmpfports.Instant) (int64, error) {
				purge, err := dmpfpostgres.PurgePublished(context.Background(), pool, before)
				return purge.Count, err
			},
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped: %v", v.Skipped)
	}
	evidence.RecordVerdict(t, "provider", "postgres-outbox", v)
}

func committedStatus(t *testing.T, pool *pgxpool.Pool, consumer string, id dmpfports.MessageID) (dmpfports.Status, bool) {
	t.Helper()
	var raw string
	err := pool.QueryRow(context.Background(),
		`SELECT status FROM dmpf_inbox WHERE consumer_name = $1 AND message_id = $2`, consumer, string(id)).Scan(&raw)
	if err != nil {
		return 0, false
	}
	switch raw {
	case "processed":
		return dmpfports.StatusProcessed, true
	case "rejected":
		return dmpfports.StatusRejected, true
	default:
		return 0, true
	}
}
