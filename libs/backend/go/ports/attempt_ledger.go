package ports

import (
	"context"
	"errors"
)

// Attempt is one submission of a key to a partner; N counts from 1 per key and
// tenant, the platform scope the row carries (ADR-050, ADR-070). MessageID is
// the inbound message that began it, so its redelivery is told from a new one.
type Attempt struct {
	Tenant    TenantID
	Key       OutboundKey
	N         int
	MessageID MessageID
	StartedAt Instant
}

type attemptStartKind uint8

const (
	attemptStarted attemptStartKind = iota + 1
	attemptPending
	attemptDone
)

type AttemptStart struct {
	kind    attemptStartKind
	attempt Attempt
	class   ExternalClass
}

func StartedAttempt(a Attempt) AttemptStart {
	return AttemptStart{kind: attemptStarted, attempt: a}
}

// PendingAttempt reports that the key's last attempt is still begun: no row was
// written, and the call it made may still be in flight.
func PendingAttempt(a Attempt) AttemptStart {
	return AttemptStart{kind: attemptPending, attempt: a}
}

// DoneAttempt reports that the same message already finished an attempt of the
// key with a class no resend follows, whichever attempt came last: the partner
// is not called again.
func DoneAttempt(a Attempt, class ExternalClass) AttemptStart {
	return AttemptStart{kind: attemptDone, attempt: a, class: class}
}

func (s AttemptStart) Match(
	started func(Attempt) error,
	pending func(Attempt) error,
	done func(Attempt, ExternalClass) error,
) error {
	if started == nil || pending == nil || done == nil {
		panic("ports: AttemptStart.Match requires all three branches")
	}
	switch s.kind {
	case attemptStarted:
		return started(s.attempt)
	case attemptPending:
		return pending(s.attempt)
	case attemptDone:
		return done(s.attempt, s.class)
	default:
		panic("ports: AttemptStart zero value has no classification")
	}
}

// Retryable reports the two classes after which the same message submits again
// with the same key, once the decorator's budget is spent (RES-27, RES-36).
func (c ExternalClass) Retryable() bool {
	return c == ClassRetryableTransport || c == ClassRetryableUpstream
}

// AttemptOutcome closes attempt N of Key. Presumed marks a class inferred with no
// reply, the UnknownSent of the sweep or of an expired redelivery: it yields to a
// class already recorded instead of conflicting with it.
type AttemptOutcome struct {
	Key         OutboundKey
	N           int
	Class       ExternalClass
	Observation Observation
	FinishedAt  Instant
	Presumed    bool
}

type FinishResult uint8

const (
	FinishRecorded FinishResult = iota + 1
	FinishUnchanged
	FinishConflict
)

func (r FinishResult) String() string {
	switch r {
	case FinishRecorded:
		return "recorded"
	case FinishUnchanged:
		return "unchanged"
	case FinishConflict:
		return "conflict"
	default:
		return "invalid"
	}
}

// On FinishConflict, Recorded is the earlier class, kept intact; the late one
// is only recorded beside it.
type Finished struct {
	Result   FinishResult
	Recorded ExternalClass
}

type ObserveResult uint8

const (
	ObserveRecorded ObserveResult = iota + 1
	ObserveDuplicate
	ObserveConflict
)

func (r ObserveResult) String() string {
	switch r {
	case ObserveRecorded:
		return "recorded"
	case ObserveDuplicate:
		return "duplicate"
	case ObserveConflict:
		return "conflict"
	default:
		return "invalid"
	}
}

// QueriesDone counts this observation too when Result is ObserveRecorded.
type Observed struct {
	Result      ObserveResult
	N           int
	Class       ExternalClass
	QueriesDone int
}

// AttemptCursor is where a Stale page resumes: the zero value starts from the
// oldest begun attempt, so a page of attempts that cannot be closed yet is passed.
type AttemptCursor struct {
	StartedAt Instant
	Tenant    TenantID
	Key       OutboundKey
	N         int
}

func (a Attempt) Cursor() AttemptCursor {
	return AttemptCursor{StartedAt: a.StartedAt, Tenant: a.Tenant, Key: a.Key, N: a.N}
}

// AttemptLedger records each submission of an outbound key. Begin, Finish and
// Observe scope the key to the ExecutionContext's tenant; Stale crosses tenants,
// because the sweep runs before any tenant (ADR-050, ADR-070).
type AttemptLedger interface {
	Begin(ctx context.Context, key OutboundKey, message MessageID, now Instant) (AttemptStart, error)
	Finish(ctx context.Context, outcome AttemptOutcome) (Finished, error)
	Observe(ctx context.Context, observation Observation, now Instant) (Observed, error)
	// Stale lists begun attempts started at or before cutoff, the instant from
	// which Dispatch also closes one, ordered by start, tenant, key and N after.
	Stale(ctx context.Context, cutoff Instant, after AttemptCursor, limit int) ([]Attempt, error)
}

var ErrInvalidAttempt = errors.New("ports: invalid attempt ledger argument")

var ErrAttemptUnknown = errors.New("ports: attempt unknown to the ledger")

// ErrAttemptInFlight reports an observation of a key whose last attempt is still
// begun: it has no outcome to confirm or contradict yet.
var ErrAttemptInFlight = errors.New("ports: observed attempt is still in flight")
