// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-04 §6.2, §6.4, INB-*), dentro do limite de 3 linhas.

package ports

import (
	"context"
	"errors"
)

// Status is the terminal state of an inbox registration: exactly two values,
// no third (INB-02, §6.1).
type Status uint8

const (
	StatusProcessed Status = iota + 1
	StatusRejected
)

func (s Status) String() string {
	switch s {
	case StatusProcessed:
		return "processed"
	case StatusRejected:
		return "rejected"
	default:
		return "invalid"
	}
}

// Receipt is what an application service hands to Register before opening the
// transaction: the identity of one inbound message (INB-01, §6.2).
type Receipt struct {
	Consumer    string
	MessageID   MessageID
	MessageType string
	PayloadHash string
	ReceivedAt  Instant
}

// Completion is what Pending.Complete fixes as the terminal status of a first
// reception, written only inside R1 (§6.1, §6.4).
type Completion struct {
	Status    Status
	At        Instant
	LastError string
}

// Pending is the write half of a first reception, reachable only inside
// Reception.Match's first branch (INB-05, §6.2).
type Pending interface {
	// Complete fixes the terminal status; only reachable under R1 (§6.4).
	Complete(ctx context.Context, c Completion) error

	// Completed reports whether Complete has been called, so Match can enforce
	// INB-05 when first returns without calling it.
	Completed() bool
}

type receptionKind uint8

const (
	receptionFirst receptionKind = iota + 1
	receptionProcessed
	receptionRejected
	receptionCollision
)

// Reception is Register's outcome, one of axis 1's four values (R1-R4, §6.4),
// opaque so a caller can only inspect it through Match.
type Reception struct {
	kind    receptionKind
	pending Pending
}

// FirstReception is R1: the key was absent. p == nil is a programming defect,
// not a runtime condition (INB-05).
func FirstReception(p Pending) Reception {
	if p == nil {
		panic("ports: FirstReception requires a non-nil Pending")
	}
	return Reception{kind: receptionFirst, pending: p}
}

// ProcessedReception is R2: same hash, stored status processed (§6.4).
func ProcessedReception() Reception { return Reception{kind: receptionProcessed} }

// RejectedReception is R3: same hash, stored status rejected (§6.4, INB-12).
func RejectedReception() Reception { return Reception{kind: receptionRejected} }

// CollisionReception is R4: same key, different hash (§6.4).
func CollisionReception() Reception { return Reception{kind: receptionCollision} }

// Match calls exactly one branch for r's classification (INB-11); pending only
// reaches first. If first returns nil without Completing, Match returns
// ErrPendingNotCompleted instead of nil (INB-05).
func (r Reception) Match(
	first func(Pending) error,
	processed func() error,
	rejected func() error,
	collision func() error,
) error {
	if first == nil || processed == nil || rejected == nil || collision == nil {
		panic("ports: Reception.Match requires all four branches")
	}
	switch r.kind {
	case receptionFirst:
		if err := first(r.pending); err != nil {
			return err
		}
		if !r.pending.Completed() {
			return ErrPendingNotCompleted
		}
		return nil
	case receptionProcessed:
		return processed()
	case receptionRejected:
		return rejected()
	case receptionCollision:
		return collision()
	default:
		panic("ports: Reception zero value has no classification")
	}
}

// Inbox is the deduplication boundary of §6.2: Register never signals a
// constraint error (INB-04), serializing on the key under commit (INB-06, INB-18).
type Inbox interface {
	Register(ctx context.Context, r Receipt) (Reception, error)
}

// ErrRegisterTimeout is Register's report when the wait for a concurrent key
// exceeds its ceiling: a transient failure, mapped to R1×D3 (INB-17).
var ErrRegisterTimeout = errors.New("ports: inbox register wait exceeded")

// ErrPendingNotCompleted is Match's report when first returns nil without
// calling Complete: registering without concluding is a defect (INB-05).
var ErrPendingNotCompleted = errors.New("ports: first reception left without completion")
