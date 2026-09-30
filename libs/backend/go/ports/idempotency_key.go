package ports

import (
	"context"
	"regexp"
	"sync"
)

// IdempotencyKeyPattern is the one format the REST header and the gRPC
// metadata accept (IDM-02); the OpenAPI contracts publish the same expression.
const IdempotencyKeyPattern = `^[A-Za-z0-9._-]{1,128}$`

var idempotencyKeyFormat = regexp.MustCompile(IdempotencyKeyPattern)

func ValidIdempotencyKey(key string) bool {
	return idempotencyKeyFormat.MatchString(key)
}

type idempotencyKeyKey struct{}

// WithIdempotencyKey deposits the command's key on the request carrier, apart
// from the ExecutionContext: CTX-01 fixes nine fields that cross boundaries, and
// the key never does (IDM-10).
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, idempotencyKeyKey{}, key)
}

// IdempotencyKeyFrom reads the key the edge deposited; ok is false when the
// carrier holds none, so absence is never mistaken for an empty key.
func IdempotencyKeyFrom(ctx context.Context) (string, bool) {
	key, ok := ctx.Value(idempotencyKeyKey{}).(string)
	return key, ok
}

// IdempotencyOutcome is how a command's claim ended, the value the span and the
// replay header report; the zero value means no claim was made.
type IdempotencyOutcome uint8

const (
	IdempotencyNew IdempotencyOutcome = iota + 1
	IdempotencyReplayed
	IdempotencyMismatch
	IdempotencyInFlight
)

func (o IdempotencyOutcome) String() string {
	switch o {
	case IdempotencyNew:
		return "new"
	case IdempotencyReplayed:
		return "replayed"
	case IdempotencyMismatch:
		return "mismatch"
	case IdempotencyInFlight:
		return "in_flight"
	default:
		return ""
	}
}

type idempotencySlotKey struct{}

type idempotencySlot struct {
	mu      sync.Mutex
	outcome IdempotencyOutcome
}

// WithIdempotencySlot installs the mutable slot the edge reads after the
// handler returns: the use case marks it downstream, on a derived context.
func WithIdempotencySlot(ctx context.Context) context.Context {
	return context.WithValue(ctx, idempotencySlotKey{}, &idempotencySlot{})
}

// MarkIdempotency records outcome on the installed slot; without a slot it does
// nothing, so a use case runs the same under an edge that does not read it.
func MarkIdempotency(ctx context.Context, outcome IdempotencyOutcome) {
	slot, ok := ctx.Value(idempotencySlotKey{}).(*idempotencySlot)
	if !ok {
		return
	}
	slot.mu.Lock()
	slot.outcome = outcome
	slot.mu.Unlock()
}

func IdempotencyOutcomeFrom(ctx context.Context) (IdempotencyOutcome, bool) {
	slot, ok := ctx.Value(idempotencySlotKey{}).(*idempotencySlot)
	if !ok {
		return 0, false
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return slot.outcome, slot.outcome != 0
}
