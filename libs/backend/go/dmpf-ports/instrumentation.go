// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-08, TRC-16, LOG-14, MET-08..MET-10), dentro do limite de 3 linhas.

package dmpfports

import (
	"context"
	"errors"
)

// ErrDenied is the sentinel an authorization hook wraps to declare a refusal.
// Any other authorization error is technical failure: the taxonomy of FND-07
// has no realization in the kernel, so a denial is never inferred (ERR-11).
var ErrDenied = errors.New("dmpfports: authorization denied")

// OutcomeCategory is the terminal category of an operation, and its string is
// the value of the outcome_category label of MET-08 to MET-10.
type OutcomeCategory string

const (
	// OutcomeAccepted is the accepting branch of the UPR.
	OutcomeAccepted OutcomeCategory = "accepted"
	// OutcomeRejected is the refusing branch of the UPR, which is not a failure (DEC-04).
	OutcomeRejected OutcomeCategory = "rejected"
	// OutcomeDenied is authorization refused at step 1, with nothing accessed.
	OutcomeDenied OutcomeCategory = "denied"
	// OutcomeFailed is technical failure, the only category that carries Result.Err.
	OutcomeFailed OutcomeCategory = "failed"
)

// Result closes an operation. Err is the raw technical error, populated only on
// OutcomeFailed, so the realization classifies it with its own taxonomy instead
// of receiving a category the kernel cannot define.
type Result struct {
	Outcome OutcomeCategory
	Err     error
}

// AuditEvent is the record of LOG-14 without the subject: the authenticated
// identity has no realization in the kernel, and the provider resolves it.
type AuditEvent struct {
	Object  string
	Action  string
	Outcome OutcomeCategory
	At      Instant
}

// EndOperation closes the operation opened by BeginOperation.
type EndOperation func(result Result)

// Instrumentation is the port through which a use case reports what it is doing.
// It lives here, and not in the application block, because Go satisfies an
// interface by identical signatures and not by structure: a provider that
// declared its own Result would not satisfy it, and provider → application is a
// forbidden cell. Declared above, realized below, like every other port.
type Instrumentation interface {
	BeginOperation(ctx context.Context, operation string) (context.Context, EndOperation)
	Audit(ctx context.Context, event AuditEvent)
}

// NoInstrumentation is the inert realization. It is the composition root's
// explicit choice when nothing observes, never a silent default.
func NoInstrumentation() Instrumentation { return noInstrumentation{} }

type noInstrumentation struct{}

func (noInstrumentation) BeginOperation(ctx context.Context, _ string) (context.Context, EndOperation) {
	return ctx, func(Result) {}
}

func (noInstrumentation) Audit(context.Context, AuditEvent) {}
