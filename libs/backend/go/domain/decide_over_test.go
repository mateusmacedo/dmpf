package domain_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type tally struct {
	count int
	tags  []string
}

func (t *tally) clone() tally {
	return tally{count: t.count, tags: slices.Clone(t.tags)}
}

func (t *tally) shallow() tally { return *t }

type added struct{ Count int }

func (added) EventName() string { return "tally.added" }

func TestDecideOverLeavesTheTargetUntouchedOnRejection(t *testing.T) {
	target := &tally{count: 1, tags: []string{"a"}}

	acc, rej := domain.DecideOver(target, (*tally).clone, func(next *tally) (domain.Accepted[int], *domain.Rejection) {
		next.count = 99
		next.tags[0] = "mutated"
		next.tags = append(next.tags, "extra")
		return domain.Refuse[int]("kernel/tally/full", "tally is full")
	})

	if rej == nil || rej.Code() != "kernel/tally/full" {
		t.Fatalf("rejection = %v, want kernel/tally/full", rej)
	}
	if acc.Response() != 0 || len(acc.Events()) != 0 {
		t.Fatalf("accepted = (%d, %v), want the zero value", acc.Response(), acc.Events())
	}
	if target.count != 1 || !slices.Equal(target.tags, []string{"a"}) {
		t.Fatalf("target = %+v, want it untouched", *target)
	}
}

func TestDecideOverCommitsTheCopyOnAcceptance(t *testing.T) {
	target := &tally{count: 1, tags: []string{"a"}}

	acc, rej := domain.DecideOver(target, (*tally).clone, func(next *tally) (domain.Accepted[int], *domain.Rejection) {
		next.count++
		next.tags = append(next.tags, "b")
		return domain.Accept(next.count, added{Count: next.count}), nil
	})

	if rej != nil {
		t.Fatalf("rejection = %v, want nil", rej)
	}
	if acc.Response() != 2 || !slices.Equal(acc.Events(), []domain.DomainEvent{added{Count: 2}}) {
		t.Fatalf("accepted = (%d, %v), want (2, [added{2}])", acc.Response(), acc.Events())
	}
	if target.count != 2 || !slices.Equal(target.tags, []string{"a", "b"}) {
		t.Fatalf("target = %+v, want the decided copy", *target)
	}
}

func TestDecideOverHandsADistinctCopyAndCopiesOnce(t *testing.T) {
	target := &tally{count: 1}
	copies := 0
	copyOf := func(t *tally) tally {
		copies++
		return t.clone()
	}

	_, _ = domain.DecideOver(target, copyOf, func(next *tally) (domain.Accepted[int], *domain.Rejection) {
		if next == target {
			t.Fatal("decide received the target itself, want a copy")
		}
		return domain.Accept(0), nil
	})

	if copies != 1 {
		t.Fatalf("copyOf called %d times, want 1", copies)
	}
}

func TestDecideOverWithAShallowCopyLeaksSharedStateOnRejection(t *testing.T) {
	target := &tally{tags: []string{"a"}}

	_, _ = domain.DecideOver(target, (*tally).shallow, func(next *tally) (domain.Accepted[int], *domain.Rejection) {
		next.tags[0] = "leaked"
		return domain.Refuse[int]("kernel/tally/full", "tally is full")
	})

	if target.tags[0] != "leaked" {
		t.Fatalf("tags[0] = %q, want the leak a shallow copyOf allows", target.tags[0])
	}
}

func TestRefuseReturnsTheZeroAcceptedAndACopiedRejection(t *testing.T) {
	details := []domain.Detail{{Key: "limit", Value: "3"}}

	acc, rej := domain.Refuse[string]("kernel/tally/full", "tally is full", details...)
	details[0].Value = "mutated"

	if acc.Response() != "" || len(acc.Events()) != 0 {
		t.Fatalf("accepted = (%q, %v), want the zero value", acc.Response(), acc.Events())
	}
	if rej.Code() != "kernel/tally/full" || rej.Message() != "tally is full" {
		t.Fatalf("rejection = (%s, %q), want (kernel/tally/full, tally is full)", rej.Code(), rej.Message())
	}
	if got := rej.Details(); !slices.Equal(got, []domain.Detail{{Key: "limit", Value: "3"}}) {
		t.Fatalf("details = %v, want the values at the call", got)
	}
}
