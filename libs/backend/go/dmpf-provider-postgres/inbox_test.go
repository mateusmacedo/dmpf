//go:build integration

package dmpfpostgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
)

func withInbox(t *testing.T, pool interface {
	Begin(context.Context) (pgx.Tx, error)
}, consumer string, fn func(ctx context.Context, inbox dmpfports.Inbox) error) error {
	t.Helper()
	ctx := context.Background()
	pgxTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v, want nil", err)
	}
	defer func() { _ = pgxTx.Rollback(ctx) }()

	tx := dmpfpostgres.NewTx(pgxTx)
	inbox := tx.Inbox(consumer, 0)

	if err := fn(ctx, inbox); err != nil {
		_ = pgxTx.Rollback(ctx)
		return err
	}
	return pgxTx.Commit(ctx)
}

func receipt(consumer, id, hash string) dmpfports.Receipt {
	return dmpfports.Receipt{
		Consumer:    consumer,
		MessageID:   dmpfports.MessageID(id),
		MessageType: "example",
		PayloadHash: hash,
		ReceivedAt:  100,
	}
}

func matchBranch(t *testing.T, r dmpfports.Reception, completeAs *dmpfports.Status) string {
	t.Helper()
	var branch string
	completion := dmpfports.Completion{Status: dmpfports.StatusProcessed, At: 200}
	if completeAs != nil {
		completion.Status = *completeAs
	}
	err := r.Match(
		func(p dmpfports.Pending) error {
			branch = "first"
			if completeAs == nil {
				return nil
			}
			return p.Complete(context.Background(), completion)
		},
		func() error { branch = "processed"; return nil },
		func() error { branch = "rejected"; return nil },
		func() error { branch = "collision"; return nil },
	)
	if err != nil && completeAs != nil {
		t.Fatalf("Match() = %v, want nil", err)
	}
	return branch
}

func statusPtr(s dmpfports.Status) *dmpfports.Status { return &s }

func TestInboxFirstReception(t *testing.T) {
	pool := openPool(t)

	err := withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		reception, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}
		if branch := matchBranch(t, reception, statusPtr(dmpfports.StatusProcessed)); branch != "first" {
			t.Fatalf("branch = %q, want %q", branch, "first")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withInbox() = %v, want nil", err)
	}

	readStatus(t, pool, "orders", "m-1", "processed")
}

func TestInboxRedeliveryProcessed(t *testing.T) {
	pool := openPool(t)

	err := withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}
		matchBranch(t, r, statusPtr(dmpfports.StatusProcessed))
		return nil
	})
	if err != nil {
		t.Fatalf("first withInbox() = %v", err)
	}

	err = withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}
		if branch := matchBranch(t, r, nil); branch != "processed" {
			t.Fatalf("branch = %q, want %q", branch, "processed")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("second withInbox() = %v", err)
	}
}

func TestInboxRedeliveryRejected(t *testing.T) {
	pool := openPool(t)

	err := withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}
		matchBranch(t, r, statusPtr(dmpfports.StatusRejected))
		return nil
	})
	if err != nil {
		t.Fatalf("first withInbox() = %v", err)
	}

	err = withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}
		if branch := matchBranch(t, r, nil); branch != "rejected" {
			t.Fatalf("branch = %q, want %q", branch, "rejected")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("second withInbox() = %v", err)
	}
}

func TestInboxCollision(t *testing.T) {
	pool := openPool(t)

	err := withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}
		matchBranch(t, r, statusPtr(dmpfports.StatusProcessed))
		return nil
	})
	if err != nil {
		t.Fatalf("first withInbox() = %v", err)
	}

	err = withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h2"))
		if err != nil {
			return err
		}
		if branch := matchBranch(t, r, nil); branch != "collision" {
			t.Fatalf("branch = %q, want %q", branch, "collision")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("second withInbox() = %v", err)
	}
}

func TestInboxFirstAgainAfterRollback(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	pgxTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	tx := dmpfpostgres.NewTx(pgxTx)
	inbox := tx.Inbox("orders", 0)
	r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
	if err != nil {
		t.Fatalf("Register() = %v", err)
	}
	matchBranch(t, r, statusPtr(dmpfports.StatusProcessed))
	_ = pgxTx.Rollback(ctx)

	err = withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}
		if branch := matchBranch(t, r, statusPtr(dmpfports.StatusProcessed)); branch != "first" {
			t.Fatalf("branch = %q, want %q", branch, "first")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withInbox() = %v", err)
	}
}

func TestInboxPresentKeyLeavesTransactionUsable(t *testing.T) {
	pool := openPool(t)

	err := withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}
		matchBranch(t, r, statusPtr(dmpfports.StatusProcessed))
		return nil
	})
	if err != nil {
		t.Fatalf("seed = %v", err)
	}

	err = withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			t.Fatalf("Register(m-1) = %v, want nil — present key is a result, not a constraint error (INB-04)", err)
		}
		matchBranch(t, r, nil)

		r, err = inbox.Register(ctx, receipt("orders", "m-2", "h2"))
		if err != nil {
			return err
		}
		if branch := matchBranch(t, r, statusPtr(dmpfports.StatusProcessed)); branch != "first" {
			t.Fatalf("branch = %q, want %q", branch, "first")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withInbox() = %v", err)
	}
}

func TestInboxDistinctConsumers(t *testing.T) {
	pool := openPool(t)

	err := withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}
		matchBranch(t, r, statusPtr(dmpfports.StatusProcessed))
		return nil
	})
	if err != nil {
		t.Fatalf("seed = %v", err)
	}

	err = withInbox(t, pool, "billing", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("billing", "m-1", "h1"))
		if err != nil {
			return err
		}
		if branch := matchBranch(t, r, statusPtr(dmpfports.StatusProcessed)); branch != "first" {
			t.Fatalf("branch = %q, want %q", branch, "first")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withInbox() = %v", err)
	}
}

func TestInboxCompleteTwice(t *testing.T) {
	pool := openPool(t)

	err := withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}

		var completionErr error
		_ = r.Match(
			func(p dmpfports.Pending) error {
				if err := p.Complete(ctx, dmpfports.Completion{Status: dmpfports.StatusProcessed, At: 200}); err != nil {
					return err
				}
				completionErr = p.Complete(ctx, dmpfports.Completion{Status: dmpfports.StatusProcessed, At: 300})
				return nil
			},
			func() error { return nil },
			func() error { return nil },
			func() error { return nil },
		)

		if !errors.Is(completionErr, dmpfpostgres.ErrAlreadyCompleted) {
			t.Fatalf("second Complete() = %v, want ErrAlreadyCompleted", completionErr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withInbox() = %v", err)
	}
}

func TestInboxConsumerRequired(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	pgxTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer func() { _ = pgxTx.Rollback(ctx) }()

	tx := dmpfpostgres.NewTx(pgxTx)
	inbox := tx.Inbox("", 0)

	_, err = inbox.Register(ctx, receipt("", "m-1", "h1"))
	if !errors.Is(err, dmpfpostgres.ErrInboxConsumerRequired) {
		t.Fatalf("Register() = %v, want ErrInboxConsumerRequired", err)
	}
}

func TestInboxConsumerMismatch(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	pgxTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer func() { _ = pgxTx.Rollback(ctx) }()

	tx := dmpfpostgres.NewTx(pgxTx)
	inbox := tx.Inbox("orders", 0)

	_, err = inbox.Register(ctx, receipt("billing", "m-1", "h1"))
	if !errors.Is(err, dmpfpostgres.ErrInboxConsumerMismatch) {
		t.Fatalf("Register() = %v, want ErrInboxConsumerMismatch", err)
	}
}

func TestInboxStatusCheckRejectsDirectUpdate(t *testing.T) {
	pool := openPool(t)

	err := withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}
		matchBranch(t, r, statusPtr(dmpfports.StatusProcessed))
		return nil
	})
	if err != nil {
		t.Fatalf("seed = %v", err)
	}

	_, err = pool.Exec(context.Background(),
		`UPDATE dmpf_inbox SET status = 'processing' WHERE consumer_name = 'orders' AND message_id = 'm-1'`)
	if err == nil {
		t.Fatal("UPDATE to 'processing' should fail — CHECK allows only processed/rejected (INB-02)")
	}
}

func TestInboxLastErrorStoredOnRejected(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	pgxTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	tx := dmpfpostgres.NewTx(pgxTx)
	inbox := tx.Inbox("orders", 0)

	r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
	if err != nil {
		t.Fatalf("Register() = %v", err)
	}
	_ = r.Match(
		func(p dmpfports.Pending) error {
			return p.Complete(ctx, dmpfports.Completion{
				Status:    dmpfports.StatusRejected,
				At:        200,
				LastError: "business rule violated",
			})
		},
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
	)
	if err := pgxTx.Commit(ctx); err != nil {
		t.Fatalf("Commit() = %v", err)
	}

	var lastError *string
	if err := pool.QueryRow(ctx,
		`SELECT last_error FROM dmpf_inbox WHERE consumer_name = 'orders' AND message_id = 'm-1'`).Scan(&lastError); err != nil {
		t.Fatalf("SELECT last_error = %v", err)
	}
	if lastError == nil || *lastError != "business rule violated" {
		t.Fatalf("last_error = %v, want 'business rule violated'", lastError)
	}
}

func TestInboxLastErrorNullOnProcessed(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	err := withInbox(t, pool, "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
		r, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
		if err != nil {
			return err
		}
		matchBranch(t, r, statusPtr(dmpfports.StatusProcessed))
		return nil
	})
	if err != nil {
		t.Fatalf("withInbox() = %v", err)
	}

	var lastError *string
	if err := pool.QueryRow(ctx,
		`SELECT last_error FROM dmpf_inbox WHERE consumer_name = 'orders' AND message_id = 'm-1'`).Scan(&lastError); err != nil {
		t.Fatalf("SELECT last_error = %v", err)
	}
	if lastError != nil {
		t.Fatalf("last_error = %v, want NULL for processed status", *lastError)
	}
}

func readStatus(t *testing.T, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, consumer, id, want string) {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM dmpf_inbox WHERE consumer_name = $1 AND message_id = $2`, consumer, id).Scan(&status); err != nil {
		t.Fatalf("readStatus() = %v, want nil", err)
	}
	if status != want {
		t.Fatalf("status = %q, want %q", status, want)
	}
}
