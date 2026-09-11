package dmpfports_test

import (
	"context"
	"errors"
	"testing"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

var errFirstFailed = errors.New("inbox_test: first failed")

type fakePending struct {
	completed bool
}

func (p *fakePending) Complete(_ context.Context, _ dmpfports.Completion) error {
	p.completed = true
	return nil
}

func (p *fakePending) Completed() bool { return p.completed }

func TestMatchCallsExactlyOneBranch(t *testing.T) {
	tests := []struct {
		name string
		r    dmpfports.Reception
	}{
		{"first", dmpfports.FirstReception(&fakePending{})},
		{"processed", dmpfports.ProcessedReception()},
		{"rejected", dmpfports.RejectedReception()},
		{"collision", dmpfports.CollisionReception()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := map[string]int{}
			err := tt.r.Match(
				func(p dmpfports.Pending) error {
					calls["first"]++
					return p.Complete(context.Background(), dmpfports.Completion{Status: dmpfports.StatusProcessed})
				},
				func() error { calls["processed"]++; return nil },
				func() error { calls["rejected"]++; return nil },
				func() error { calls["collision"]++; return nil },
			)
			if err != nil {
				t.Fatalf("Match() = %v, want nil", err)
			}
			total := calls["first"] + calls["processed"] + calls["rejected"] + calls["collision"]
			if total != 1 {
				t.Fatalf("branches called %d times, want exactly 1: %v", total, calls)
			}
			if calls[tt.name] != 1 {
				t.Fatalf("expected branch %q to run, got %v", tt.name, calls)
			}
		})
	}
}

func TestMatchPendingOnlyReachesFirst(t *testing.T) {
	tests := []struct {
		name string
		r    dmpfports.Reception
	}{
		{"processed", dmpfports.ProcessedReception()},
		{"rejected", dmpfports.RejectedReception()},
		{"collision", dmpfports.CollisionReception()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.r.Match(
				func(dmpfports.Pending) error {
					t.Fatal("first must not run outside R1")
					return nil
				},
				func() error { return nil },
				func() error { return nil },
				func() error { return nil },
			)
			if err != nil {
				t.Fatalf("Match() = %v, want nil", err)
			}
		})
	}
}

func TestMatchFirstWithoutCompleteReturnsErrPendingNotCompleted(t *testing.T) {
	r := dmpfports.FirstReception(&fakePending{})

	err := r.Match(
		func(dmpfports.Pending) error { return nil },
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
	)

	if !errors.Is(err, dmpfports.ErrPendingNotCompleted) {
		t.Fatalf("Match() = %v, want ErrPendingNotCompleted", err)
	}
}

func TestMatchFirstWithCompleteReturnsNil(t *testing.T) {
	p := &fakePending{}
	r := dmpfports.FirstReception(p)

	err := r.Match(
		func(pending dmpfports.Pending) error {
			return pending.Complete(context.Background(), dmpfports.Completion{Status: dmpfports.StatusProcessed})
		},
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
	)

	if err != nil {
		t.Fatalf("Match() = %v, want nil", err)
	}
	if !p.Completed() {
		t.Fatal("Complete was not observed by Completed()")
	}
}

func TestMatchFirstErrorPropagatesUnwrapped(t *testing.T) {
	r := dmpfports.FirstReception(&fakePending{})

	err := r.Match(
		func(dmpfports.Pending) error { return errFirstFailed },
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
	)

	if !errors.Is(err, errFirstFailed) {
		t.Fatalf("Match() = %v, want an error matching errFirstFailed", err)
	}
}

func TestMatchZeroValuePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Match on a zero-value Reception must panic")
		}
	}()

	var r dmpfports.Reception
	_ = r.Match(
		func(dmpfports.Pending) error { return nil },
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
	)
}

func TestMatchNilBranchPanics(t *testing.T) {
	tests := []struct {
		name string
		call func() error
	}{
		{"nil first", func() error {
			return dmpfports.ProcessedReception().Match(nil, func() error { return nil }, func() error { return nil }, func() error { return nil })
		}},
		{"nil processed", func() error {
			return dmpfports.ProcessedReception().Match(func(dmpfports.Pending) error { return nil }, nil, func() error { return nil }, func() error { return nil })
		}},
		{"nil rejected", func() error {
			return dmpfports.ProcessedReception().Match(func(dmpfports.Pending) error { return nil }, func() error { return nil }, nil, func() error { return nil })
		}},
		{"nil collision", func() error {
			return dmpfports.ProcessedReception().Match(func(dmpfports.Pending) error { return nil }, func() error { return nil }, func() error { return nil }, nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("Match with a nil branch must panic")
				}
			}()
			_ = tt.call()
		})
	}
}

func TestFirstReceptionNilPendingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("FirstReception(nil) must panic")
		}
	}()
	dmpfports.FirstReception(nil)
}

func TestStatusHasExactlyTwoValues(t *testing.T) {
	tests := []struct {
		status dmpfports.Status
		want   string
	}{
		{dmpfports.StatusProcessed, "processed"},
		{dmpfports.StatusRejected, "rejected"},
	}

	seen := map[dmpfports.Status]bool{}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Fatalf("String() = %q, want %q", got, tt.want)
		}
		seen[tt.status] = true
	}
	if len(seen) != 2 {
		t.Fatalf("expected exactly 2 distinct Status values, got %d", len(seen))
	}
}

func TestSentinelsAreDistinguishable(t *testing.T) {
	if errors.Is(dmpfports.ErrRegisterTimeout, dmpfports.ErrPendingNotCompleted) {
		t.Fatal("ErrRegisterTimeout must not match ErrPendingNotCompleted")
	}
}
