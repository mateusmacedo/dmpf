package ports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var errFirstFailed = errors.New("inbox_test: first failed")

type fakePending struct {
	completed bool
}

func (p *fakePending) Complete(_ context.Context, _ ports.Completion) error {
	p.completed = true
	return nil
}

func (p *fakePending) Completed() bool { return p.completed }

func TestMatchCallsExactlyOneBranch(t *testing.T) {
	tests := []struct {
		name string
		r    ports.Reception
	}{
		{"first", ports.FirstReception(&fakePending{})},
		{"processed", ports.ProcessedReception()},
		{"rejected", ports.RejectedReception()},
		{"collision", ports.CollisionReception()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := map[string]int{}
			err := tt.r.Match(
				func(p ports.Pending) error {
					calls["first"]++
					return p.Complete(context.Background(), ports.Completion{Status: ports.StatusProcessed})
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
		r    ports.Reception
	}{
		{"processed", ports.ProcessedReception()},
		{"rejected", ports.RejectedReception()},
		{"collision", ports.CollisionReception()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.r.Match(
				func(ports.Pending) error {
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
	r := ports.FirstReception(&fakePending{})

	err := r.Match(
		func(ports.Pending) error { return nil },
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
	)

	if !errors.Is(err, ports.ErrPendingNotCompleted) {
		t.Fatalf("Match() = %v, want ErrPendingNotCompleted", err)
	}
}

func TestMatchFirstWithCompleteReturnsNil(t *testing.T) {
	p := &fakePending{}
	r := ports.FirstReception(p)

	err := r.Match(
		func(pending ports.Pending) error {
			return pending.Complete(context.Background(), ports.Completion{Status: ports.StatusProcessed})
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
	r := ports.FirstReception(&fakePending{})

	err := r.Match(
		func(ports.Pending) error { return errFirstFailed },
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

	var r ports.Reception
	_ = r.Match(
		func(ports.Pending) error { return nil },
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
			return ports.ProcessedReception().Match(nil, func() error { return nil }, func() error { return nil }, func() error { return nil })
		}},
		{"nil processed", func() error {
			return ports.ProcessedReception().Match(func(ports.Pending) error { return nil }, nil, func() error { return nil }, func() error { return nil })
		}},
		{"nil rejected", func() error {
			return ports.ProcessedReception().Match(func(ports.Pending) error { return nil }, func() error { return nil }, nil, func() error { return nil })
		}},
		{"nil collision", func() error {
			return ports.ProcessedReception().Match(func(ports.Pending) error { return nil }, func() error { return nil }, func() error { return nil }, nil)
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
	ports.FirstReception(nil)
}

func TestStatusHasExactlyTwoValues(t *testing.T) {
	tests := []struct {
		status ports.Status
		want   string
	}{
		{ports.StatusProcessed, "processed"},
		{ports.StatusRejected, "rejected"},
	}

	seen := map[ports.Status]bool{}
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
	if errors.Is(ports.ErrRegisterTimeout, ports.ErrPendingNotCompleted) {
		t.Fatal("ErrRegisterTimeout must not match ErrPendingNotCompleted")
	}
}
