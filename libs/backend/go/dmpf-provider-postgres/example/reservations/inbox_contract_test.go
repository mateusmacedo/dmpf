//go:build integration

// The 7 contract clauses below are duplicated from dmpf-ports/inbox_contract_test.go
// on purpose: a _test.go file is never importable, and each realization must
// prove the same properties over its own storage. An exported test kit is KRN-11's.

package reservationspg_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
)

type contractSubject struct {
	pool  *pgxpool.Pool
	pgxTx pgx.Tx
}

func newContractSubject(t *testing.T) *contractSubject {
	t.Helper()
	return &contractSubject{pool: openPool(t)}
}

func (s *contractSubject) begin(t *testing.T) {
	t.Helper()
	var err error
	s.pgxTx, err = s.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
}

func (s *contractSubject) commit(t *testing.T) {
	t.Helper()
	if err := s.pgxTx.Commit(context.Background()); err != nil {
		t.Fatalf("Commit() = %v", err)
	}
}

func (s *contractSubject) rollback() {
	_ = s.pgxTx.Rollback(context.Background())
}

func (s *contractSubject) inbox(consumer string) dmpfports.Inbox {
	return dmpfpostgres.NewTx(s.pgxTx).Inbox(consumer, 0)
}

func (s *contractSubject) readStatus(consumer string, id dmpfports.MessageID) (dmpfports.Status, bool) {
	var status string
	err := s.pool.QueryRow(context.Background(),
		`SELECT status FROM dmpf_inbox WHERE consumer_name = $1 AND message_id = $2`,
		consumer, string(id)).Scan(&status)
	if err != nil {
		return 0, false
	}
	switch status {
	case "processed":
		return dmpfports.StatusProcessed, true
	case "rejected":
		return dmpfports.StatusRejected, true
	default:
		return 0, false
	}
}

func cMatchBranch(t *testing.T, r dmpfports.Reception, completeAs *dmpfports.Status) string {
	t.Helper()
	var branch string
	err := r.Match(
		func(p dmpfports.Pending) error {
			branch = "first"
			if completeAs == nil {
				return nil
			}
			return p.Complete(context.Background(), dmpfports.Completion{Status: *completeAs})
		},
		func() error { branch = "processed"; return nil },
		func() error { branch = "rejected"; return nil },
		func() error { branch = "collision"; return nil },
	)
	if err != nil {
		t.Fatalf("Match() = %v, want nil", err)
	}
	return branch
}

func cStatusPtr(s dmpfports.Status) *dmpfports.Status { return &s }

func cRegisterAndCommit(t *testing.T, s *contractSubject, consumer string, id dmpfports.MessageID, hash string, status dmpfports.Status) {
	t.Helper()
	s.begin(t)
	r, err := s.inbox(consumer).Register(context.Background(), dmpfports.Receipt{
		Consumer: consumer, MessageID: id, MessageType: "example", PayloadHash: hash,
	})
	if err != nil {
		t.Fatalf("Register() = %v, want nil", err)
	}
	if branch := cMatchBranch(t, r, cStatusPtr(status)); branch != "first" {
		t.Fatalf("branch = %q, want %q", branch, "first")
	}
	s.commit(t)
}

func TestInboxContractOverPostgres(t *testing.T) {
	t.Run("first reception is R1", func(t *testing.T) {
		s := newContractSubject(t)
		cRegisterAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusProcessed)
		status, ok := s.readStatus("orders", "m-1")
		if !ok || status != dmpfports.StatusProcessed {
			t.Fatalf("ReadStatus() = (%v, %v), want (processed, true)", status, ok)
		}
	})

	t.Run("redelivery with the same hash after a processed commit is R2", func(t *testing.T) {
		s := newContractSubject(t)
		cRegisterAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusProcessed)
		s.begin(t)
		r, err := s.inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v", err)
		}
		if branch := cMatchBranch(t, r, nil); branch != "processed" {
			t.Fatalf("branch = %q, want %q (INB-06)", branch, "processed")
		}
		s.commit(t)
	})

	t.Run("redelivery with the same hash after a rejected commit is R3", func(t *testing.T) {
		s := newContractSubject(t)
		cRegisterAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusRejected)
		s.begin(t)
		r, err := s.inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v", err)
		}
		if branch := cMatchBranch(t, r, nil); branch != "rejected" {
			t.Fatalf("branch = %q, want %q (INB-12)", branch, "rejected")
		}
		s.commit(t)
	})

	t.Run("a divergent hash on a present key is R4", func(t *testing.T) {
		s := newContractSubject(t)
		cRegisterAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusProcessed)
		s.begin(t)
		r, err := s.inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h2",
		})
		if err != nil {
			t.Fatalf("Register() = %v", err)
		}
		if branch := cMatchBranch(t, r, nil); branch != "collision" {
			t.Fatalf("branch = %q, want %q", branch, "collision")
		}
		s.commit(t)
	})

	t.Run("first reception again after a rollback", func(t *testing.T) {
		s := newContractSubject(t)
		s.begin(t)
		r, err := s.inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v", err)
		}
		cMatchBranch(t, r, cStatusPtr(dmpfports.StatusProcessed))
		s.rollback()

		if _, ok := s.readStatus("orders", "m-1"); ok {
			t.Fatal("a rolled-back first reception must leave no committed row")
		}

		s.begin(t)
		r, err = s.inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v", err)
		}
		if branch := cMatchBranch(t, r, cStatusPtr(dmpfports.StatusProcessed)); branch != "first" {
			t.Fatalf("branch = %q, want %q", branch, "first")
		}
		s.commit(t)
	})

	t.Run("distinct consumers with the same MessageID do not dedupe", func(t *testing.T) {
		s := newContractSubject(t)
		cRegisterAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusProcessed)
		s.begin(t)
		r, err := s.inbox("billing").Register(context.Background(), dmpfports.Receipt{
			Consumer: "billing", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v", err)
		}
		if branch := cMatchBranch(t, r, cStatusPtr(dmpfports.StatusProcessed)); branch != "first" {
			t.Fatalf("branch = %q, want %q", branch, "first")
		}
		s.commit(t)
	})

	t.Run("a receipt naming another consumer is refused", func(t *testing.T) {
		s := newContractSubject(t)
		s.begin(t)
		defer s.rollback()
		_, err := s.inbox("orders").Register(context.Background(), dmpfports.Receipt{
			Consumer: "billing", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if !errors.Is(err, dmpfpostgres.ErrInboxConsumerMismatch) {
			t.Fatalf("Register() = %v, want ErrInboxConsumerMismatch — the bound consumer is the key's owner", err)
		}
	})

	t.Run("registering a present key leaves the transaction usable", func(t *testing.T) {
		s := newContractSubject(t)
		cRegisterAndCommit(t, s, "orders", "m-1", "h1", dmpfports.StatusProcessed)
		s.begin(t)
		inbox := s.inbox("orders")
		r, err := inbox.Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-1", MessageType: "example", PayloadHash: "h1",
		})
		if err != nil {
			t.Fatalf("Register() = %v, want nil (INB-04)", err)
		}
		cMatchBranch(t, r, nil)
		r, err = inbox.Register(context.Background(), dmpfports.Receipt{
			Consumer: "orders", MessageID: "m-2", MessageType: "example", PayloadHash: "h2",
		})
		if err != nil {
			t.Fatalf("subsequent Register() = %v", err)
		}
		if branch := cMatchBranch(t, r, cStatusPtr(dmpfports.StatusProcessed)); branch != "first" {
			t.Fatalf("branch = %q, want %q", branch, "first")
		}
		s.commit(t)
		if _, ok := s.readStatus("orders", "m-2"); !ok {
			t.Fatal("the second key must have committed alongside the first")
		}
	})
}
