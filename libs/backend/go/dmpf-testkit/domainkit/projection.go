package domainkit

import "slices"

// Fields is the abstract, stack-neutral encoding of a value (ORA-33): every
// scalar a string, nested values flattened by dotted keys. It is what the
// fixture declares and what the projection reports, so the two compare
// without any wire type in between.
type Fields map[string]string

// Branch is the discriminant of the outcome (ORA-31): exactly one of the two,
// never a third state.
type Branch string

const (
	Accepted Branch = "accepted"
	Rejected Branch = "rejected"
)

type Event struct {
	Name   string
	Fields Fields
}

// Rejection is the typed refusal as an observer sees it: the stable
// "context/reason" code, the domain message and the structured details.
type Rejection struct {
	Code    string
	Message string
	Details Fields
}

// Projection is what an observer obtains at the boundary (ORA-31, ORA-38):
// the branch, the response or the rejection, the ordered event sequence, and
// the observable state of the target before and after the call. Violations is
// what Run itself observed about the mechanism (ORA-37), which no fixture can
// declare.
type Projection struct {
	Branch      Branch
	Response    Fields
	Rejection   Rejection
	Events      []Event
	StateBefore Fields
	StateAfter  Fields
	Violations  []Diagnostic
}

func (f Fields) equal(other Fields) bool {
	if len(f) != len(other) {
		return false
	}
	for k, v := range f {
		if w, ok := other[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// Keys in a stable order, so a diagnostic names the first divergent field
// deterministically (KIT-08).
func (f Fields) keys() []string {
	out := make([]string, 0, len(f))
	for k := range f {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func (e Event) equal(other Event) bool { return e.Name == other.Name && e.Fields.equal(other.Fields) }
