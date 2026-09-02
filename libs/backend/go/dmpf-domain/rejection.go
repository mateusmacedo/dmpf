package dmpfdomain

import (
	"fmt"
	"regexp"
	"slices"
)

// Code is the stable domain identifier of a rejection, in the form
// "context/reason" (FND-03 §3.3, §5.3). It never carries a protocol status.
type Code string

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*/[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// Valid reports whether the code follows the "context/reason" form. Reject does
// not validate: declared codes are proven valid by the aggregate's own tests.
func (c Code) Valid() bool { return codePattern.MatchString(string(c)) }

// Detail is one structured entry of a rejection, expressed in domain terms.
type Detail struct {
	Key   string
	Value string
}

// Rejection is the typed business refusal carried by the Rejected branch
// (DEC-03, DEC-09). It is immutable after construction (DEC-12).
type Rejection struct {
	code    Code
	message string
	details []Detail
}

// Reject builds a rejection. The details slice is copied so later mutation of
// the caller's slice cannot reach the rejection (DEC-12).
func Reject(code Code, message string, details ...Detail) *Rejection {
	return &Rejection{code: code, message: message, details: slices.Clone(details)}
}

// Code is the stable "context/reason" identifier of the refusal (DEC-09).
func (r *Rejection) Code() Code { return r.code }

// Message is addressed to the domain: no technical detail, no sensitive data.
func (r *Rejection) Message() string { return r.message }

// Details returns a fresh copy, empty rather than nil when there are none.
func (r *Rejection) Details() []Detail {
	out := make([]Detail, len(r.details))
	copy(out, r.details)
	return out
}

// Error lets the application service wrap a rejection with errors.Join and
// recover it with errors.As. A UPR never returns it as error (ADR-018).
func (r *Rejection) Error() string { return fmt.Sprintf("%s: %s", r.code, r.message) }
