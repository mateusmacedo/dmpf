package domainkit

import (
	"strconv"
	"strings"
)

// Code is the FND-09 §7 rule a diagnostic reproves against.
type Code string

const (
	// CodeProjection: branch, response or rejection differs from the expected (ORA-31).
	CodeProjection Code = "ORA-31"
	// CodeDeterminism: two executions over the same input diverge (ORA-34).
	CodeDeterminism Code = "ORA-34"
	// CodeSecondAccessor: the events reach the observer by a second path, or a
	// pending collection retains them after the return (ORA-37).
	CodeSecondAccessor Code = "ORA-37"
	// CodeSecondRead: the outcome or the target changed after the return (ORA-38).
	CodeSecondRead Code = "ORA-38"
)

type Diagnostic struct {
	Code     Code
	Field    string
	Expected string
	Got      string
}

func (d Diagnostic) String() string {
	return string(d.Code) + " " + d.Field + ": expected " + d.Expected + ", got " + d.Got
}

// Verdict is decidable: no diagnostic is a pass, anything else names what
// diverged and against which rule.
type Verdict struct {
	Diagnostics []Diagnostic
}

func (v Verdict) OK() bool { return len(v.Diagnostics) == 0 }

func (v Verdict) Failures() []string {
	out := make([]string, 0, len(v.Diagnostics))
	for _, d := range v.Diagnostics {
		out = append(out, d.String())
	}
	return out
}

// Equal compares a projection obtained with the one expected, field by field
// and event by event in order (ORA-34), carries over what Run observed about
// the mechanism, and under Rejected requires the target untouched (ORA-38).
// A nil expected Details, an empty Message or an empty StateAfter means the
// fixture did not declare them; a declared empty Details is compared strictly.
func Equal(got, want Projection) Verdict {
	var v Verdict
	add := func(code Code, field, expected, actual string) {
		v.Diagnostics = append(v.Diagnostics, Diagnostic{Code: code, Field: field, Expected: expected, Got: actual})
	}
	if got.Branch != want.Branch {
		add(CodeProjection, "branch", string(want.Branch), string(got.Branch))
		v.Diagnostics = append(v.Diagnostics, got.Violations...)
		return v
	}
	switch got.Branch {
	case Accepted:
		compareFields(&v, "response", got.Response, want.Response)
	case Rejected:
		if got.Rejection.Code != want.Rejection.Code {
			add(CodeProjection, "rejection.code", want.Rejection.Code, got.Rejection.Code)
		}
		if want.Rejection.Message != "" && got.Rejection.Message != want.Rejection.Message {
			add(CodeProjection, "rejection.message", want.Rejection.Message, got.Rejection.Message)
		}
		if want.Rejection.Details != nil {
			compareFields(&v, "rejection.details", got.Rejection.Details, want.Rejection.Details)
		}
		if !got.StateAfter.equal(got.StateBefore) {
			add(CodeSecondRead, "state", describeFields(got.StateBefore), describeFields(got.StateAfter))
		}
	}
	if len(got.Events) != len(want.Events) {
		add(CodeProjection, "events", describe(want.Events), describe(got.Events))
	} else {
		for i := range want.Events {
			if got.Events[i].Name != want.Events[i].Name {
				add(CodeProjection, "events["+strconv.Itoa(i)+"].name", want.Events[i].Name, got.Events[i].Name)
				continue
			}
			compareFields(&v, "events["+strconv.Itoa(i)+"]", got.Events[i].Fields, want.Events[i].Fields)
		}
	}
	if len(want.StateAfter) != 0 {
		compareFields(&v, "state_after", got.StateAfter, want.StateAfter)
	}
	v.Diagnostics = append(v.Diagnostics, got.Violations...)
	return v
}

func compareFields(v *Verdict, prefix string, got, want Fields) {
	for _, k := range want.keys() {
		if g, ok := got[k]; !ok {
			v.Diagnostics = append(v.Diagnostics, Diagnostic{Code: CodeProjection, Field: prefix + "." + k, Expected: want[k], Got: "<absent>"})
		} else if g != want[k] {
			v.Diagnostics = append(v.Diagnostics, Diagnostic{Code: CodeProjection, Field: prefix + "." + k, Expected: want[k], Got: g})
		}
	}
	for _, k := range got.keys() {
		if _, ok := want[k]; !ok {
			v.Diagnostics = append(v.Diagnostics, Diagnostic{Code: CodeProjection, Field: prefix + "." + k, Expected: "<absent>", Got: got[k]})
		}
	}
}

func describe(events []Event) string {
	if len(events) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(events))
	for _, e := range events {
		parts = append(parts, e.Name+describeFields(e.Fields))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func describeFields(f Fields) string {
	parts := make([]string, 0, len(f))
	for _, k := range f.keys() {
		parts = append(parts, k+"="+f[k])
	}
	return "{" + strings.Join(parts, " ") + "}"
}
