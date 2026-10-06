package tb_test

import (
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

type placed struct{ Order string }

func (p placed) EventName() string { return "orders.placed" }

type cancelled struct{ Order string }

func (c cancelled) EventName() string { return "orders.cancelled" }

const closed domain.Code = "orders/order/closed"

func TestRequireRejectedPassesARefusalWithTheZeroResponseAndNoEvents(t *testing.T) {
	s := &spy{TB: t}

	tb.RequireRejected(s, domain.Accepted[placed]{}, domain.Reject(closed, "closed"), closed)

	if s.fatal != "" {
		t.Fatalf("a well-formed refusal failed: %q", s.fatal)
	}
}

func TestRequireRejectedNamesWhatTheRefusalGotWrong(t *testing.T) {
	cases := map[string]struct {
		acc  domain.Accepted[placed]
		rej  *domain.Rejection
		want string
	}{
		"accepted instead":  {domain.Accept(placed{Order: "o-1"}), nil, "expected Rejected"},
		"another code":      {domain.Accepted[placed]{}, domain.Reject("orders/order/full", "full"), `"orders/order/full"`},
		"a response":        {domain.Accept(placed{Order: "o-1"}), domain.Reject(closed, "closed"), "zero response"},
		"events on refusal": {domain.Accept(placed{}, placed{Order: "o-1"}), domain.Reject(closed, "closed"), "no events"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s := &spy{TB: t}

			tb.RequireRejected(s, c.acc, c.rej, closed)

			if !strings.Contains(s.fatal, c.want) {
				t.Fatalf("fatal = %q, want it to mention %q", s.fatal, c.want)
			}
		})
	}
}

func TestRequireAcceptedFailsOnARefusal(t *testing.T) {
	s := &spy{TB: t}
	tb.RequireAccepted(s, nil)
	if s.fatal != "" {
		t.Fatalf("an acceptance failed: %q", s.fatal)
	}

	tb.RequireAccepted(s, domain.Reject(closed, "closed"))
	if !strings.Contains(s.fatal, "expected Accepted") {
		t.Fatalf("fatal = %q, want the refusal reported", s.fatal)
	}
}

func TestRequireSameEventsComparesNameBeforeValue(t *testing.T) {
	same := []domain.DomainEvent{placed{Order: "o-1"}}
	cases := map[string]struct {
		b    []domain.DomainEvent
		want string
	}{
		"same sequence":   {[]domain.DomainEvent{placed{Order: "o-1"}}, ""},
		"another length":  {nil, "len(a)=1, len(b)=0"},
		"another event":   {[]domain.DomainEvent{cancelled{Order: "o-1"}}, "name"},
		"another payload": {[]domain.DomainEvent{placed{Order: "o-2"}}, "value"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s := &spy{TB: t}

			tb.RequireSameEvents(s, same, c.b)

			if c.want == "" && s.fatal != "" || c.want != "" && !strings.Contains(s.fatal, c.want) {
				t.Fatalf("fatal = %q, want %q", s.fatal, c.want)
			}
		})
	}
}

func TestRunProjectionRunsEachCaseUnderItsNameAndMergesTheVerdicts(t *testing.T) {
	f := tb.ProjectionFixture{Cases: []tb.ProjectionCase{{Name: "first"}, {Name: "second"}}}
	var ran []string

	decided := tb.RunProjection(t, f, func(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
		ran = append(ran, t.Name())
		return c.Expected.Projection(), domainkit.Verdict{}
	})

	want := []string{t.Name() + "/first", t.Name() + "/second"}
	if strings.Join(ran, ",") != strings.Join(want, ",") {
		t.Fatalf("ran %v, want %v", ran, want)
	}
	if !decided.OK() {
		t.Fatalf("decided = %v, want a pass when every case matches", decided.Failures())
	}
}
