package domainkit_test

import (
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
)

const at = int64(1755432000)

func counterAt(total, limit int) *counter {
	return &counter{id: "c-1", total: total, limit: limit}
}

func TestRunProjectsTheAcceptingBranch(t *testing.T) {
	p := domainkit.Run(counterAt(1, 3), bumpSubject(bump{By: 2, At: at}))
	if p.Branch != domainkit.Accepted {
		t.Fatalf("branch = %s", p.Branch)
	}
	if p.Response["total"] != "3" || p.Response["counter"] != "c-1" {
		t.Fatalf("response = %v", p.Response)
	}
	if len(p.Events) != 1 || p.Events[0].Name != "counters.bumped" || p.Events[0].Fields["by"] != "2" {
		t.Fatalf("events = %+v", p.Events)
	}
	if p.StateBefore["total"] != "1" || p.StateAfter["total"] != "3" {
		t.Fatalf("state before/after = %v / %v", p.StateBefore, p.StateAfter)
	}
	if len(p.Violations) != 0 {
		t.Fatalf("violations on a conforming aggregate: %v", p.Violations)
	}
}

func TestRunProjectsTheRejectingBranchWithStateUntouched(t *testing.T) {
	p := domainkit.Run(counterAt(1, 1), bumpSubject(bump{By: 1, At: at}))
	if p.Branch != domainkit.Rejected || p.Rejection.Code != string(codeLimitExceeded) {
		t.Fatalf("projection = %+v", p)
	}
	if p.Rejection.Details["limit"] != "1" || p.Rejection.Details["attempted"] != "2" {
		t.Fatalf("details = %v", p.Rejection.Details)
	}
	if len(p.Events) != 0 {
		t.Fatalf("events under Rejected: %+v", p.Events)
	}
	if p.StateAfter["total"] != p.StateBefore["total"] {
		t.Fatalf("state changed under Rejected: %v -> %v", p.StateBefore, p.StateAfter)
	}
	v := domainkit.Equal(p, domainkit.Projection{Branch: domainkit.Rejected, Rejection: domainkit.Rejection{Code: string(codeLimitExceeded)}})
	if !v.OK() {
		t.Fatalf("a conforming rejection reproved: %v", v.Failures())
	}
}

func TestReadTwiceIsObservationallyEqual(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    domainkit.Verdict
	}{
		{"accepted", domainkit.ReadTwice(counterAt(1, 3), bumpSubject(bump{By: 2, At: at}))},
		{"rejected", domainkit.ReadTwice(counterAt(0, 3), resetSubject(reset{At: at}))},
	} {
		if !tc.v.OK() {
			t.Errorf("%s: %v", tc.name, tc.v.Failures())
		}
	}
}

func TestEqualNamesTheFirstDivergentField(t *testing.T) {
	got := domainkit.Run(counterAt(1, 3), resetSubject(reset{At: at}))
	want := domainkit.Projection{
		Branch:   domainkit.Accepted,
		Response: domainkit.Fields{"counter": "c-1"},
		Events:   []domainkit.Event{{Name: "counters.reset", Fields: domainkit.Fields{"counter": "c-1", "from": "2", "at": "1755432000"}}},
	}
	v := domainkit.Equal(got, want)
	if len(v.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want exactly one", v.Failures())
	}
	d := v.Diagnostics[0]
	if d.Code != domainkit.CodeProjection || d.Field != "events[0].from" || d.Expected != "2" || d.Got != "1" {
		t.Fatalf("diagnostic = %+v", d)
	}
	if !strings.Contains(v.Failures()[0], "ORA-31") {
		t.Fatalf("failure text lacks the rule: %s", v.Failures()[0])
	}
}

func TestEqualReprovesABranchMismatchBeforeAnythingElse(t *testing.T) {
	got := domainkit.Run(counterAt(0, 3), resetSubject(reset{At: at}))
	v := domainkit.Equal(got, domainkit.Projection{Branch: domainkit.Accepted, Response: domainkit.Fields{"counter": "c-1"}})
	if len(v.Diagnostics) != 1 || v.Diagnostics[0].Field != "branch" || v.Diagnostics[0].Got != "rejected" {
		t.Fatalf("diagnostics = %v", v.Failures())
	}
}
