package domainkit

import (
	"strconv"

	dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"
)

// Subject is how the kit observes one UPR of an aggregate, everything by value
// (ORA-36): no double, no clock, no identifier generator reaches the domain.
type Subject[S, R any] struct {
	// Decide runs the UPR over the target and returns the kernel outcome.
	Decide func(S) (dmpfdomain.Accepted[R], *dmpfdomain.Rejection)
	// Response projects the domain response of the accepting branch.
	Response func(R) Fields
	// Event projects one domain event into its name and fields.
	Event func(dmpfdomain.DomainEvent) (string, Fields)
	// Snapshot projects the observable state of the target (ORA-32: only what
	// DEC-10's post-condition needs).
	Snapshot func(S) Fields
	// Clone gives ReadTwice a second target with the same state.
	Clone func(S) S
}

// eventsAccessor is the shape of a second path to the events: an aggregate
// that hands them out itself, next to the outcome, breaks the single-accessor
// rule (ORA-37, DEC-08).
type eventsAccessor interface {
	Events() []dmpfdomain.DomainEvent
}

// complete panics naming the first function the Subject left nil: an
// incomplete subject is a mistake in the test, not a property of the domain,
// and a nil dereference deep in Run would not say which field it was.
func (s Subject[S, R]) complete() {
	switch {
	case s.Decide == nil:
		panic("domainkit: Subject.Decide is nil")
	case s.Response == nil:
		panic("domainkit: Subject.Response is nil")
	case s.Event == nil:
		panic("domainkit: Subject.Event is nil")
	case s.Snapshot == nil:
		panic("domainkit: Subject.Snapshot is nil")
	}
}

// Run executes the UPR once and returns the projection, reading the outcome
// twice on the way (ORA-38): a second read that differs is a violation the
// projection carries, since no fixture can expect it.
func Run[S, R any](target S, s Subject[S, R]) Projection {
	s.complete()
	p := Projection{StateBefore: s.Snapshot(target)}
	acc, rej := s.Decide(target)
	p.StateAfter = s.Snapshot(target)

	first := project(acc.Events(), s.Event)
	second := project(acc.Events(), s.Event)
	if !sameEvents(first, second) {
		p.Violations = append(p.Violations, Diagnostic{Code: CodeSecondRead, Field: "events", Expected: describe(first), Got: describe(second)})
	}
	p.Events = first

	if rej != nil {
		p.Branch = Rejected
		p.Rejection = Rejection{Code: string(rej.Code()), Message: rej.Message(), Details: detailFields(rej.Details())}
	} else {
		p.Branch = Accepted
		p.Response = s.Response(acc.Response())
		if again := s.Response(acc.Response()); !again.equal(p.Response) {
			p.Violations = append(p.Violations, Diagnostic{Code: CodeSecondRead, Field: "response", Expected: describeFields(p.Response), Got: describeFields(again)})
		}
	}

	if a, ok := any(target).(eventsAccessor); ok {
		p.Violations = append(p.Violations, Diagnostic{
			Code: CodeSecondAccessor, Field: "events",
			Expected: "the outcome as the only accessor of the sequence",
			Got:      "target exposes Events() with " + strconv.Itoa(len(a.Events())) + " event(s)",
		})
	}
	if p.Branch == Rejected && len(p.Events) != 0 {
		p.Violations = append(p.Violations, Diagnostic{Code: CodeSecondAccessor, Field: "events", Expected: "empty sequence under Rejected", Got: describe(p.Events)})
	}
	return p
}

// ReadTwice runs the same UPR over two clones of the target and decides
// whether the two projections are observationally equal (ORA-34).
func ReadTwice[S, R any](target S, s Subject[S, R]) Verdict {
	if s.Clone == nil {
		panic("domainkit: Subject.Clone is nil")
	}
	first := Run(s.Clone(target), s)
	second := Run(s.Clone(target), s)
	v := Equal(first, second)
	for i := range v.Diagnostics {
		// Only what Equal decided is about determinism; the violations Run
		// observed (ORA-37, ORA-38) keep the rule they name.
		if v.Diagnostics[i].Code == CodeProjection {
			v.Diagnostics[i].Code = CodeDeterminism
		}
	}
	return v
}

func project(events []dmpfdomain.DomainEvent, projectEvent func(dmpfdomain.DomainEvent) (string, Fields)) []Event {
	out := make([]Event, 0, len(events))
	for _, e := range events {
		name, fields := projectEvent(e)
		out = append(out, Event{Name: name, Fields: fields})
	}
	return out
}

func sameEvents(a, b []Event) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].equal(b[i]) {
			return false
		}
	}
	return true
}

func detailFields(details []dmpfdomain.Detail) Fields {
	out := Fields{}
	for _, d := range details {
		out[d.Key] = d.Value
	}
	return out
}
