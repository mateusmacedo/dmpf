package dmpfdomain

import "slices"

// Empty is the explicitly empty response of a decision that has nothing useful
// to answer (DEC-05): a response is never absent.
type Empty struct{}

// Accepted is the accepting branch of a UPR outcome: the domain response and the
// ordered, closed sequence of events it produced (DEC-05..DEC-08, DEC-13).
// The zero value is what a UPR returns alongside a non-nil *Rejection.
type Accepted[R any] struct {
	response R
	events   []DomainEvent
}

// Accept builds the accepting branch. The event sequence is copied, so the
// caller's slice cannot reorder or extend the outcome afterwards (DEC-13).
func Accept[R any](response R, events ...DomainEvent) Accepted[R] {
	return Accepted[R]{response: response, events: slices.Clone(events)}
}

// Response is the explicit domain response of the accepting branch (DEC-05).
func (a Accepted[R]) Response() R { return a.response }

// Events returns a fresh copy of the sequence, empty rather than nil when the
// decision produced no event (DEC-06, DEC-11).
func (a Accepted[R]) Events() []DomainEvent {
	out := make([]DomainEvent, len(a.events))
	copy(out, a.events)
	return out
}
