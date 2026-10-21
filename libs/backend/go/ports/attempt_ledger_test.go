package ports_test

import (
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var errBranch = errors.New("attempt_ledger_test: branch failed")

func TestAttemptStartMatchCallsOnlyItsBranchWithTheAttempt(t *testing.T) {
	attempt := ports.Attempt{Tenant: "t-1", Key: "k-1", N: 2, MessageID: "m-1", StartedAt: 40}
	tests := []struct {
		name      string
		start     ports.AttemptStart
		want      string
		wantClass ports.ExternalClass
	}{
		{"started", ports.StartedAttempt(attempt), "started", 0},
		{"pending", ports.PendingAttempt(attempt), "pending", 0},
		{"done", ports.DoneAttempt(attempt, ports.ClassFinalBusiness), "done", ports.ClassFinalBusiness},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called []string
			var got ports.Attempt
			var class ports.ExternalClass
			err := tt.start.Match(
				func(a ports.Attempt) error { called = append(called, "started"); got = a; return nil },
				func(a ports.Attempt) error { called = append(called, "pending"); got = a; return nil },
				func(a ports.Attempt, c ports.ExternalClass) error {
					called = append(called, "done")
					got, class = a, c
					return nil
				},
			)
			if err != nil {
				t.Fatalf("Match() = %v, want nil", err)
			}
			if len(called) != 1 || called[0] != tt.want {
				t.Fatalf("branches called = %v, want only %q", called, tt.want)
			}
			if got != attempt || class != tt.wantClass {
				t.Fatalf("branch received %+v, %v; want %+v, %v", got, class, attempt, tt.wantClass)
			}
		})
	}
}

func TestAttemptStartMatchReturnsTheBranchError(t *testing.T) {
	tests := []struct {
		name  string
		start ports.AttemptStart
	}{
		{"started", ports.StartedAttempt(ports.Attempt{Key: "k", N: 1})},
		{"pending", ports.PendingAttempt(ports.Attempt{Key: "k", N: 1})},
		{"done", ports.DoneAttempt(ports.Attempt{Key: "k", N: 1}, ports.ClassAccepted)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fail := func(ports.Attempt) error { return errBranch }
			failDone := func(ports.Attempt, ports.ExternalClass) error { return errBranch }
			if err := tt.start.Match(fail, fail, failDone); !errors.Is(err, errBranch) {
				t.Fatalf("Match() = %v, want %v", err, errBranch)
			}
		})
	}
}

func TestAttemptStartMatchPanicsOnADefect(t *testing.T) {
	branch := func(ports.Attempt) error { return nil }
	doneBranch := func(ports.Attempt, ports.ExternalClass) error { return nil }
	tests := []struct {
		name    string
		start   ports.AttemptStart
		started func(ports.Attempt) error
		pending func(ports.Attempt) error
		done    func(ports.Attempt, ports.ExternalClass) error
	}{
		{"nil started", ports.StartedAttempt(ports.Attempt{}), nil, branch, doneBranch},
		{"nil pending", ports.StartedAttempt(ports.Attempt{}), branch, nil, doneBranch},
		{"nil done", ports.StartedAttempt(ports.Attempt{}), branch, branch, nil},
		{"zero value", ports.AttemptStart{}, branch, branch, doneBranch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("Match() did not panic")
				}
			}()
			_ = tt.start.Match(tt.started, tt.pending, tt.done)
		})
	}
}

func TestOnlyTheTwoRetryableClassesAreRetryable(t *testing.T) {
	for class := ports.ExternalClass(0); class <= ports.ClassSchemaMismatch+1; class++ {
		t.Run(class.String(), func(t *testing.T) {
			want := class == ports.ClassRetryableTransport || class == ports.ClassRetryableUpstream
			if got := class.Retryable(); got != want {
				t.Fatalf("Retryable() = %v, want %v", got, want)
			}
		})
	}
}

func TestLedgerResultsNameEveryValueAndRefuseTheZero(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"finish zero", ports.FinishResult(0).String(), "invalid"},
		{"finish recorded", ports.FinishRecorded.String(), "recorded"},
		{"finish unchanged", ports.FinishUnchanged.String(), "unchanged"},
		{"finish conflict", ports.FinishConflict.String(), "conflict"},
		{"finish saturated", ports.FinishSaturated.String(), "saturated"},
		{"finish out of range", ports.FinishResult(9).String(), "invalid"},
		{"observe zero", ports.ObserveResult(0).String(), "invalid"},
		{"observe recorded", ports.ObserveRecorded.String(), "recorded"},
		{"observe duplicate", ports.ObserveDuplicate.String(), "duplicate"},
		{"observe conflict", ports.ObserveConflict.String(), "conflict"},
		{"observe saturated", ports.ObserveSaturated.String(), "saturated"},
		{"observe out of range", ports.ObserveResult(9).String(), "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("String() = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestLedgerSentinelsAreDistinct(t *testing.T) {
	sentinels := []error{ports.ErrInvalidAttempt, ports.ErrAttemptUnknown, ports.ErrAttemptInFlight}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Fatalf("%v matches %v; want distinct sentinels", a, b)
			}
		}
	}
}

func TestAnAttemptIsTheCursorAfterItself(t *testing.T) {
	tests := []struct {
		name    string
		attempt ports.Attempt
		want    ports.AttemptCursor
	}{
		{"every field of the order", ports.Attempt{Tenant: "t-1", Key: "k-1", N: 3, MessageID: "m-1", StartedAt: 40},
			ports.AttemptCursor{StartedAt: 40, Tenant: "t-1", Key: "k-1", N: 3}},
		{"the zero attempt is the zero cursor", ports.Attempt{}, ports.AttemptCursor{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.attempt.Cursor(); got != tt.want {
				t.Fatalf("Cursor() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
