package domainkit_test

import (
	"strconv"
	"testing"

	dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/domainkit"
)

// The negative vectors of ORA-39: fixture domains that break one prohibition
// of FND-03 §3.4 each, and must be reproved by the verdict.

type counted struct{ Name string }

func (e counted) EventName() string { return e.Name }

// leaky keeps the events it produced in a second accessor next to the outcome
// (row 8 of §3.4) and retains the refused attempt after Rejected (row 9).
type leaky struct {
	n       int
	pending []dmpfdomain.DomainEvent
}

func (l *leaky) Events() []dmpfdomain.DomainEvent { return l.pending }

func (l *leaky) Bump(limit int) (dmpfdomain.Accepted[int], *dmpfdomain.Rejection) {
	ev := counted{Name: "bumped"}
	l.pending = append(l.pending, ev)
	if l.n >= limit {
		return dmpfdomain.Accepted[int]{}, dmpfdomain.Reject("test/limit", "limit")
	}
	l.n++
	return dmpfdomain.Accept(l.n, ev), nil
}

// mutating changes the target and then refuses (row 5 of §3.4, DEC-10).
type mutating struct{ n int }

func (m *mutating) Bump() (dmpfdomain.Accepted[int], *dmpfdomain.Rejection) {
	m.n++
	return dmpfdomain.Accepted[int]{}, dmpfdomain.Reject("test/refused", "refused")
}

func countSubject[S any](decide func(S) (dmpfdomain.Accepted[int], *dmpfdomain.Rejection), snapshot func(S) domainkit.Fields, clone func(S) S) domainkit.Subject[S, int] {
	return domainkit.Subject[S, int]{
		Decide:   decide,
		Response: func(n int) domainkit.Fields { return domainkit.Fields{"n": strconv.Itoa(n)} },
		Event:    func(e dmpfdomain.DomainEvent) (string, domainkit.Fields) { return e.EventName(), domainkit.Fields{} },
		Snapshot: snapshot,
		Clone:    clone,
	}
}

func TestASecondAccessorOfEventsIsReproved(t *testing.T) {
	s := countSubject(
		func(l *leaky) (dmpfdomain.Accepted[int], *dmpfdomain.Rejection) { return l.Bump(10) },
		func(l *leaky) domainkit.Fields { return domainkit.Fields{"n": strconv.Itoa(l.n)} },
		func(l *leaky) *leaky { return &leaky{n: l.n} },
	)
	p := domainkit.Run(&leaky{}, s)
	v := domainkit.Equal(p, domainkit.Projection{Branch: domainkit.Accepted, Response: domainkit.Fields{"n": "1"}, Events: []domainkit.Event{{Name: "bumped", Fields: domainkit.Fields{}}}})
	if v.OK() {
		t.Fatal("a domain with a second accessor of events passed")
	}
	if v.Diagnostics[0].Code != domainkit.CodeSecondAccessor {
		t.Fatalf("diagnostic = %+v, want ORA-37", v.Diagnostics[0])
	}
}

func TestAPendingCollectionAfterRejectedIsReproved(t *testing.T) {
	s := countSubject(
		func(l *leaky) (dmpfdomain.Accepted[int], *dmpfdomain.Rejection) { return l.Bump(0) },
		func(l *leaky) domainkit.Fields { return domainkit.Fields{"n": strconv.Itoa(l.n)} },
		func(l *leaky) *leaky { return &leaky{n: l.n} },
	)
	p := domainkit.Run(&leaky{}, s)
	if p.Branch != domainkit.Rejected {
		t.Fatalf("branch = %s", p.Branch)
	}
	var codes []domainkit.Code
	for _, d := range p.Violations {
		codes = append(codes, d.Code)
	}
	if len(codes) == 0 || codes[0] != domainkit.CodeSecondAccessor {
		t.Fatalf("violations = %v, want ORA-37 for the pending collection", p.Violations)
	}
}

func TestAStateMutatedUnderRejectedIsReproved(t *testing.T) {
	s := countSubject(
		func(m *mutating) (dmpfdomain.Accepted[int], *dmpfdomain.Rejection) { return m.Bump() },
		func(m *mutating) domainkit.Fields { return domainkit.Fields{"n": strconv.Itoa(m.n)} },
		func(m *mutating) *mutating { return &mutating{n: m.n} },
	)
	v := domainkit.Equal(domainkit.Run(&mutating{}, s), domainkit.Projection{Branch: domainkit.Rejected, Rejection: domainkit.Rejection{Code: "test/refused"}})
	if v.OK() {
		t.Fatal("a domain that mutates under Rejected passed")
	}
	d := v.Diagnostics[0]
	if d.Code != domainkit.CodeSecondRead || d.Field != "state" || d.Expected != "{n=0}" || d.Got != "{n=1}" {
		t.Fatalf("diagnostic = %+v, want ORA-38 on state {n=0} → {n=1}", d)
	}
}
