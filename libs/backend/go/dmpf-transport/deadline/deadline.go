// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (GRP-04/05/16/17) que o símbolo realiza, dentro do limite de 3 linhas.

// Package deadline derives the deadline of an outgoing hop from the caller's
// deadline and the method's budget, reusing the effective-deadline term of
// dmpf-observability/resilience (RES-06) instead of a second formula.
package deadline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/resilience"
)

// DefaultSlack is the hop margin RES-07 already reserves per hop; a Budget that
// has no better number declares this one (GRP-17).
const DefaultSlack = resilience.DefaultHopSlack

var (
	// ErrNoDeadline is what Require and Outgoing report for a context that
	// carries no deadline: there is never a default (GRP-04).
	ErrNoDeadline = errors.New("deadline: context declares no deadline (GRP-04)")

	// ErrDeadlineExhausted is what Outgoing reports when the time left does not
	// cover the hop's slack, so no I/O is attempted (GRP-17, CTX-21).
	ErrDeadlineExhausted = errors.New("deadline: remaining time does not cover the hop slack (GRP-17)")
)

// Budget is the time a method may spend on one outgoing hop: the limit and
// estimate the resilience decorators read, plus the slack the hop keeps for
// its own overhead (GRP-16, GRP-17).
type Budget struct {
	Dependency        string
	Method            string
	Limit             time.Duration
	Slack             time.Duration
	EstimatedDuration time.Duration
}

// Operation is the view the resilience decorators read: a remote call whose
// declared deadline is the budget's limit.
func (b Budget) Operation() resilience.Operation {
	return resilience.Operation{
		Dependency:        b.Dependency,
		Method:            b.Method,
		Kind:              resilience.Remote,
		Deadline:          b.Limit,
		EstimatedDuration: b.EstimatedDuration,
	}
}

// Validate refuses a budget the operation would refuse, and one whose slack is
// not positive (GRP-17).
func (b Budget) Validate() error {
	if err := b.Operation().Validate(); err != nil {
		return err
	}
	if b.Slack <= 0 {
		return fmt.Errorf("deadline: %s.%s: budget declares no slack (GRP-17)", b.Dependency, b.Method)
	}
	return nil
}

// Require returns the caller's deadline or ErrNoDeadline (GRP-04).
func Require(ctx context.Context) (time.Time, error) {
	d, declared := ctx.Deadline()
	if !declared {
		return time.Time{}, ErrNoDeadline
	}
	return d, nil
}

// Outgoing is the instant the next hop must finish by: now plus the effective
// deadline of RES-06 minus the slack. It never exceeds the caller's deadline
// (GRP-05) and never extends it, whatever the limit declares (GRP-16).
func Outgoing(ctx context.Context, c clock.Clock, b Budget) (time.Time, error) {
	if err := b.Validate(); err != nil {
		return time.Time{}, err
	}
	if _, err := Require(ctx); err != nil {
		return time.Time{}, err
	}

	now := c.Now()
	remaining := resilience.EffectiveDeadline(ctx, c, b.Operation(), 0) - b.Slack
	if remaining <= 0 {
		return time.Time{}, fmt.Errorf("%w: %s.%s: %v left after a slack of %v", ErrDeadlineExhausted, b.Dependency, b.Method, remaining+b.Slack, b.Slack)
	}
	return now.Add(remaining), nil
}
