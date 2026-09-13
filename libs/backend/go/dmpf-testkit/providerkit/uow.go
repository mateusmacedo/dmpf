package providerkit

import (
	"context"
	"errors"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

// UnitOfWorkSubject is what a realization gives the suite so it can observe
// Within from the outside: the unit of work, a write through the bound ports,
// and the count of writes the resource actually kept once Within returned.
type UnitOfWorkSubject[R any] struct {
	UoW   dmpfports.UnitOfWork[R]
	Write func(ctx context.Context, resources R) error
	Kept  func() int

	// ArmCommitFailure makes the next commit fail with err. A realization that
	// cannot inject one leaves it nil and the clause is reported as skipped.
	ArmCommitFailure func(err error)

	// Commits counts the commits that installed state, when the realization
	// can tell; nil leaves the count unchecked. A nil error from Within would
	// also come from a realization that rolled back and said nothing.
	Commits func() int
}

var (
	errCallbackFailed = errors.New("providerkit: callback failed")
	errCommitRefused  = errors.New("providerkit: commit refused")
)

// UnitOfWork exercises the six observable clauses of Within over any
// realization; newSubject must return a unit of work over a fresh resource on
// every call. The seventh — R is the only path by which transactional ports
// reach the callback (UOW-03, UOW-04) — is structural and proven by the
// compiler.
func UnitOfWork[R any](newSubject func() UnitOfWorkSubject[R]) Verdict {
	var v Verdict
	ctx := context.Background()

	{
		const clause = "commits every write of the single transaction it opens"
		s := newSubject()
		err := s.UoW.Within(ctx, func(ctx context.Context, res R) error {
			if err := s.Write(ctx, res); err != nil {
				return err
			}
			return s.Write(ctx, res)
		})
		if err != nil {
			v.fail(clause, "UOW-01", "Within() = %v, want nil", err)
		} else if got := s.Kept(); got != 2 {
			v.fail(clause, "UOW-02", "kept %d writes, want 2 — both must land in one transaction", got)
		} else if s.Commits != nil && s.Commits() != 1 {
			v.fail(clause, "UOW-01", "%d commits installed state, want exactly 1", s.Commits())
		}
	}

	{
		const clause = "invokes the callback exactly once"
		s := newSubject()
		calls := 0
		if err := s.UoW.Within(ctx, func(context.Context, R) error { calls++; return nil }); err != nil {
			v.fail(clause, "UOW-09", "Within() = %v, want nil", err)
		} else if calls != 1 {
			v.fail(clause, "UOW-09", "callback ran %d times, want 1 — Within never repeats it", calls)
		}
	}

	{
		const clause = "returns ctx.Err() without invoking the callback"
		s := newSubject()
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		invoked := false
		err := s.UoW.Within(cancelled, func(ctx context.Context, res R) error {
			invoked = true
			return s.Write(ctx, res)
		})
		switch {
		case !errors.Is(err, context.Canceled):
			v.fail(clause, "CTX-21", "Within() = %v, want context.Canceled", err)
		case invoked:
			v.fail(clause, "CTX-21", "the callback ran on an already cancelled context")
		case s.Kept() != 0:
			v.fail(clause, "CTX-21", "kept %d writes, want 0", s.Kept())
		}
	}

	{
		const clause = "rolls back and returns the callback error, reachable by errors.Is"
		s := newSubject()
		err := s.UoW.Within(ctx, func(ctx context.Context, res R) error {
			if err := s.Write(ctx, res); err != nil {
				return err
			}
			return errCallbackFailed
		})
		switch {
		case !errors.Is(err, errCallbackFailed):
			v.fail(clause, "UOW-06", "Within() = %v, want an error matching the callback's", err)
		case s.Kept() != 0:
			v.fail(clause, "UOW-06", "kept %d writes, want 0 — a failing callback keeps nothing", s.Kept())
		}
	}

	{
		const clause = "returns the commit error and keeps nothing"
		s := newSubject()
		if s.ArmCommitFailure == nil {
			v.skip(clause)
		} else {
			s.ArmCommitFailure(errCommitRefused)
			err := s.UoW.Within(ctx, func(ctx context.Context, res R) error { return s.Write(ctx, res) })
			switch {
			case !errors.Is(err, errCommitRefused):
				v.fail(clause, "UOW-07", "Within() = %v, want the commit error as the provider produced it", err)
			case s.Kept() != 0:
				v.fail(clause, "UOW-07", "kept %d writes, want 0 — a failed commit persists nothing", s.Kept())
			case s.Commits != nil && s.Commits() != 0:
				v.fail(clause, "UOW-07", "%d commits installed state after a failed commit, want 0", s.Commits())
			}
		}
	}

	{
		const clause = "propagates a panic and discards the transaction"
		s := newSubject()
		recovered := run(func() {
			_ = s.UoW.Within(ctx, func(ctx context.Context, res R) error {
				_ = s.Write(ctx, res)
				panic("providerkit: boom")
			})
		})
		switch {
		case recovered == nil:
			v.fail(clause, "ERR-22", "the panic did not reach the caller")
		case recovered != "providerkit: boom":
			v.fail(clause, "ERR-22", "recovered %v, want the original panic value", recovered)
		case s.Kept() != 0:
			v.fail(clause, "ERR-22", "kept %d writes, want 0 — a panicking callback keeps nothing", s.Kept())
		}
	}

	return v
}

func run(fn func()) (recovered any) {
	defer func() { recovered = recover() }()
	fn()
	return nil
}
