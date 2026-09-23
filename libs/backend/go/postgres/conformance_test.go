//go:build integration

package postgres_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
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
			UoW: postgres.NewUnitOfWork(pool, bindWriter),
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

func TestRepositoryConformsToTheKit(t *testing.T) {
	pool := pg.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[string, probe] {
		pg.ResetTables(t, pool)
		return providerkit.RepositorySubject[string, probe]{
			Within: func(ctx context.Context, fn func(ctx context.Context, repo ports.Repository[string, probe]) error) error {
				uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) ports.Repository[string, probe] {
					return probeTable.Repository(tx)
				})
				return uow.Within(ctx, fn)
			},
			Reader:           probeTable.Reader(pool),
			NewID:            func(n int) string { return "kit-" + strconv.Itoa(n) },
			NewState:         func(marker int) probe { return probe{Items: marker} },
			Marker:           func(p probe) int { return p.Items },
			TenantUnresolved: postgres.ErrTenantUnresolved,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction; nothing should be skipped: %v", v.Skipped)
	}
	evidence.RecordVerdict(t, "provider", "postgres-repository", v)
}

func TestInboxConformsToTheKit(t *testing.T) {
	pool := pg.OpenPool(t)
	v := providerkit.Inbox(func() providerkit.InboxSubject {
		pg.ResetTables(t, pool)
		return providerkit.InboxSubject{
			Within: func(ctx context.Context, consumer string, fn func(ctx context.Context, inbox ports.Inbox) error) error {
				uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) ports.Inbox {
					return tx.Inbox(consumer, 2*time.Second)
				})
				return uow.Within(ctx, fn)
			},
			ReadStatus: func(consumer string, id ports.MessageID) (ports.Status, bool) {
				return committedStatus(t, pool, consumer, id)
			},
			Concurrent:       true,
			ConsumerMismatch: postgres.ErrInboxConsumerMismatch,
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
	v := providerkit.Outbox(func() providerkit.OutboxSubject[postgres.Claimed] {
		pg.ResetTables(t, pool)
		fake := clock.New(ports.Instant(1_000_000))
		store := postgres.NewOutboxStore(pool, fake)
		return providerkit.OutboxSubject[postgres.Claimed]{
			Store: store,
			Enqueue: func(n int) error {
				for i := range n {
					r := defaultRow("m-kit-" + strconv.Itoa(i))
					r.occurredAt, r.availableAt = int64(fake.Now()), int64(fake.Now())
					insert(t, context.Background(), pool, r)
				}
				return nil
			},
			ID:    func(c postgres.Claimed) int64 { return c.ID },
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
				st.AvailableAt = ports.Instant(availableAt)
				if lockedBy != nil {
					st.LockedBy = *lockedBy
				}
				if lastError != nil {
					st.LastError = *lastError
				}
				if lockedUntil != nil {
					st.LockedUntil = ports.Instant(*lockedUntil)
				}
				if publishedAt != nil {
					st.PublishedAt = ports.Instant(*publishedAt)
				}
				return st, err
			},
			Pending: func() (int64, error) {
				health, err := postgres.OutboxSignals(context.Background(), pool, fake)
				return health.Pending, err
			},
			Purge: func(before ports.Instant) (int64, error) {
				purge, err := postgres.PurgePublished(context.Background(), pool, before)
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

func committedStatus(t *testing.T, pool *pgxpool.Pool, consumer string, id ports.MessageID) (ports.Status, bool) {
	t.Helper()
	var raw string
	err := pool.QueryRow(context.Background(),
		`SELECT status FROM dmpf_inbox WHERE consumer_name = $1 AND message_id = $2`, consumer, string(id)).Scan(&raw)
	if err != nil {
		return 0, false
	}
	switch raw {
	case "processed":
		return ports.StatusProcessed, true
	case "rejected":
		return ports.StatusRejected, true
	default:
		return 0, true
	}
}
