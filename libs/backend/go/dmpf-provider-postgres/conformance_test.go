//go:build integration

package dmpfpostgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/providerkit"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb/pg"
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
				return w.write(ctx, "o-"+string(rune('0'+n)))
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
}

func TestInboxConformsToTheKit(t *testing.T) {
	pool := pg.OpenPool(t)
	v := providerkit.Inbox(func() providerkit.InboxSubject {
		pg.ResetTables(t, pool)
		return providerkit.InboxSubject{
			Within: func(consumer string, fn func(ctx context.Context, inbox dmpfports.Inbox) error) error {
				uow := dmpfpostgres.NewUnitOfWork(pool, func(tx *dmpfpostgres.Tx) dmpfports.Inbox {
					return tx.Inbox(consumer, 2*time.Second)
				})
				return uow.Within(context.Background(), fn)
			},
			ReadStatus: func(consumer string, id dmpfports.MessageID) (dmpfports.Status, bool) {
				return committedStatus(t, pool, consumer, id)
			},
			Concurrent: true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("Postgres serializes on the key; nothing should be skipped: %v", v.Skipped)
	}
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
					r := defaultRow("m-kit-" + string(rune('a'+i)))
					r.occurredAt, r.availableAt = int64(fake.Now()), int64(fake.Now())
					insert(t, context.Background(), pool, r)
				}
				return nil
			},
			ID:    func(c dmpfpostgres.Claimed) int64 { return c.ID },
			Clock: fake,
			Status: func(id int64) (string, string, error) {
				var status string
				var lockedBy *string
				err := pool.QueryRow(context.Background(), `SELECT status, locked_by FROM dmpf_outbox WHERE id = $1`, id).Scan(&status, &lockedBy)
				if lockedBy == nil {
					return status, "", err
				}
				return status, *lockedBy, err
			},
			Pending: func() (int64, error) {
				health, err := dmpfpostgres.OutboxSignals(context.Background(), pool, fake)
				return health.Pending, err
			},
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped: %v", v.Skipped)
	}
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
