package dmpfports_test

import (
	"context"
	"errors"
	"testing"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// UnitOfWorkSubject is what a realization gives the contract so it can observe
// Within from the outside: the unit of work, a write through the bound ports,
// and the count of writes the resource actually kept once Within returned.
type UnitOfWorkSubject[R any] struct {
	UoW   dmpfports.UnitOfWork[R]
	Write func(resources R)
	Kept  func() int

	// ArmCommitFailure makes the next commit fail with err. A realization that
	// cannot inject one leaves it nil, and the clause is skipped out loud rather
	// than quietly absent from the contract.
	ArmCommitFailure func(err error)
}

var (
	errCallbackFailed = errors.New("contract: callback failed")
	errCommitRefused  = errors.New("contract: commit refused")
)

// RunUnitOfWorkContract exercises the six observable clauses of the Within
// contract over any realization. newSubject must return a unit of work over a
// fresh resource on every call. The seventh clause — R is the only path by which
// transactional ports reach the callback (UOW-03, UOW-04) — is structural and
// proven by the compiler, not here.
func RunUnitOfWorkContract[R any](t *testing.T, newSubject func() UnitOfWorkSubject[R]) {
	t.Helper()

	t.Run("commits every write of the single transaction it opens", func(t *testing.T) {
		s := newSubject()

		err := s.UoW.Within(context.Background(), func(_ context.Context, resources R) error {
			s.Write(resources)
			s.Write(resources)
			return nil
		})

		if err != nil {
			t.Fatalf("Within() = %v, want nil", err)
		}
		if got := s.Kept(); got != 2 {
			t.Fatalf("kept %d writes, want 2 — both must land in one transaction (UOW-01, UOW-02)", got)
		}
	})

	t.Run("invokes the callback exactly once", func(t *testing.T) {
		s := newSubject()
		calls := 0

		if err := s.UoW.Within(context.Background(), func(_ context.Context, _ R) error {
			calls++
			return nil
		}); err != nil {
			t.Fatalf("Within() = %v, want nil", err)
		}

		if calls != 1 {
			t.Fatalf("callback ran %d times, want 1 — Within never repeats it (UOW-09)", calls)
		}
	})

	t.Run("returns ctx.Err() without invoking the callback", func(t *testing.T) {
		s := newSubject()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		invoked := false

		err := s.UoW.Within(ctx, func(_ context.Context, resources R) error {
			invoked = true
			s.Write(resources)
			return nil
		})

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Within() = %v, want context.Canceled (CTX-21)", err)
		}
		if invoked {
			t.Fatal("the callback ran on an already cancelled context")
		}
		if got := s.Kept(); got != 0 {
			t.Fatalf("kept %d writes, want 0", got)
		}
	})

	t.Run("rolls back and returns the callback error unwrapped", func(t *testing.T) {
		s := newSubject()

		err := s.UoW.Within(context.Background(), func(_ context.Context, resources R) error {
			s.Write(resources)
			return errCallbackFailed
		})

		if !errors.Is(err, errCallbackFailed) {
			t.Fatalf("Within() = %v, want an error matching errCallbackFailed", err)
		}
		if got := s.Kept(); got != 0 {
			t.Fatalf("kept %d writes, want 0 — a failing callback keeps nothing", got)
		}
	})

	t.Run("returns the commit error and keeps nothing", func(t *testing.T) {
		s := newSubject()
		if s.ArmCommitFailure == nil {
			t.Skip("the realization cannot inject a commit failure; clause not exercised here")
		}
		s.ArmCommitFailure(errCommitRefused)

		err := s.UoW.Within(context.Background(), func(_ context.Context, resources R) error {
			s.Write(resources)
			return nil
		})

		if !errors.Is(err, errCommitRefused) {
			t.Fatalf("Within() = %v, want the commit error as the provider produced it", err)
		}
		if got := s.Kept(); got != 0 {
			t.Fatalf("kept %d writes, want 0 — a failed commit persists nothing", got)
		}
	})

	t.Run("propagates a panic and discards the transaction", func(t *testing.T) {
		s := newSubject()

		func() {
			defer func() {
				recovered := recover()
				if recovered == nil {
					t.Fatal("the panic did not reach the caller (ERR-22)")
				}
				if recovered != "contract: boom" {
					t.Fatalf("recovered %v, want \"contract: boom\"", recovered)
				}
			}()

			_ = s.UoW.Within(context.Background(), func(_ context.Context, resources R) error {
				s.Write(resources)
				panic("contract: boom")
			})
		}()

		if got := s.Kept(); got != 0 {
			t.Fatalf("kept %d writes, want 0 — a panicking callback keeps nothing", got)
		}
	})
}

type fakeResources struct{ tx *fakeTx }

type fakeTx struct{ writes int }

type fakeUnitOfWork struct {
	kept           int
	failNextCommit error
}

func (u *fakeUnitOfWork) Within(ctx context.Context, fn func(context.Context, fakeResources) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tx := &fakeTx{}
	if err := fn(ctx, fakeResources{tx: tx}); err != nil {
		return err
	}
	if err := u.failNextCommit; err != nil {
		u.failNextCommit = nil
		return err
	}
	u.kept += tx.writes
	return nil
}

var _ dmpfports.UnitOfWork[fakeResources] = (*fakeUnitOfWork)(nil)

func TestUnitOfWorkContract(t *testing.T) {
	RunUnitOfWorkContract(t, func() UnitOfWorkSubject[fakeResources] {
		uow := &fakeUnitOfWork{}
		return UnitOfWorkSubject[fakeResources]{
			UoW:              uow,
			Write:            func(resources fakeResources) { resources.tx.writes++ },
			Kept:             func() int { return uow.kept },
			ArmCommitFailure: func(err error) { uow.failNextCommit = err },
		}
	})
}
