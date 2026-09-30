package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// deadlineMargin is what the wait leaves of the deadline for the command
// itself: waiting up to the last instant would hand it a context already spent.
const deadlineMargin = ports.Instant(100_000_000)

// ErrIncompleteCommand is RunIdempotent's refusal of a command it cannot
// fingerprint: without a Fingerprint and a Digest, R4 goes undetected.
var ErrIncompleteCommand = errors.New("application: idempotent command incomplete")

// IdempotencyPolicy is fixed by the composition root, in nanoseconds because this
// block imports no time: the wait for a concurrent command (IDM-07), the retention
// of its entry (IDM-09) and the SHA-256 the inbox compares (IDM-04).
type IdempotencyPolicy struct {
	Wait      int64
	Retention int64
	Digest    func(canonical []byte) ports.Fingerprint
}

// IdempotentCommand is one command received through the inbox of its context:
// Run is called only on a first reception, and returns the outcome that R2 or
// R3 replays.
type IdempotentCommand[R any] struct {
	Inbox       ports.Inbox
	Consumer    string
	Operation   string
	Fingerprint *Fingerprint
	Now         ports.Instant
	Policy      IdempotencyPolicy
	Codec       OutcomeCodec[R]
	Run         func() (Outcome[R], error)
}

// RunIdempotent is called inside Within, before any other statement (IDM-05).
// replayed reports that the stored outcome came back and Run never executed, so
// the caller emits no audit for an effect that did not happen again.
func RunIdempotent[R any](ctx context.Context, cmd IdempotentCommand[R]) (outcome Outcome[R], replayed bool, err error) {
	key, ok := ports.IdempotencyKeyFrom(ctx)
	if !ok || key == "" {
		return outcome, false, ports.ErrIdempotencyKeyAbsent
	}
	if !ports.ValidIdempotencyKey(key) {
		return outcome, false, ports.ErrIdempotencyKeyInvalid
	}
	execution, err := ports.RequireExecutionContext(ctx)
	if err != nil {
		return outcome, false, err
	}
	if cmd.Fingerprint == nil || cmd.Policy.Digest == nil {
		return outcome, false, fmt.Errorf("%w: %s lacks a fingerprint or a digest", ErrIncompleteCommand, cmd.Operation)
	}
	fingerprint := cmd.Policy.Digest(cmd.Fingerprint.under(cmd.Operation))

	reception, err := cmd.Inbox.Register(ctx, ports.Receipt{
		Consumer:    cmd.Consumer,
		MessageID:   ports.MessageID(key),
		MessageType: cmd.Operation,
		PayloadHash: fmt.Sprintf("%x", fingerprint[:]),
		ReceivedAt:  cmd.Now,
		WaitUntil:   waitUntil(cmd.Now, cmd.Policy.Wait, execution.Deadline()),
		ExpiresAt:   cmd.Now + ports.Instant(cmd.Policy.Retention),
	})
	if errors.Is(err, ports.ErrRegisterTimeout) {
		ports.MarkIdempotency(ctx, ports.IdempotencyInFlight)
		return outcome, false, fmt.Errorf("%w: %w", ports.ErrIdempotencyInFlight, err)
	}
	if err != nil {
		return outcome, false, err
	}

	replay := func() error {
		outcome, err = DecodeOutcome(cmd.Codec, reception.Stored())
		replayed = err == nil
		return err
	}
	err = reception.Match(
		func(pending ports.Pending) error {
			ran, err := cmd.Run()
			if err != nil {
				return err
			}
			status := ports.StatusProcessed
			if _, rejected := ran.Rejection(); rejected {
				status = ports.StatusRejected
			}
			if err := pending.Complete(ctx, ports.Completion{Status: status, At: cmd.Now, Outcome: EncodeOutcome(cmd.Codec, ran)}); err != nil {
				return err
			}
			outcome = ran
			return nil
		},
		replay,
		replay,
		func() error { return ports.ErrIdempotencyMismatch },
	)
	switch {
	case errors.Is(err, ports.ErrIdempotencyMismatch):
		ports.MarkIdempotency(ctx, ports.IdempotencyMismatch)
		return Outcome[R]{}, false, err
	case err != nil:
		return Outcome[R]{}, false, err
	case replayed:
		ports.MarkIdempotency(ctx, ports.IdempotencyReplayed)
	default:
		ports.MarkIdempotency(ctx, ports.IdempotencyNew)
	}
	return outcome, replayed, nil
}

func waitUntil(now ports.Instant, wait int64, deadline ports.Instant) ports.Instant {
	return max(min(now+ports.Instant(wait), deadline-deadlineMargin), now)
}
