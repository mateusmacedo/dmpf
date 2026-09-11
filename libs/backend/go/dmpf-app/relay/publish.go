package relay

import (
	"context"
	"fmt"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/envelope"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

// disposition is what happened to one record in step 3 of FND-04 §5.4.
type disposition int

const (
	dispositionPublished disposition = iota
	dispositionRescheduled
	dispositionFailed
	// dispositionClaimLost is the outcome OBX-10 defines: the write was
	// rejected because another claim owns the record now. It is not an error.
	dispositionClaimLost
	// dispositionAbandoned is a delivery the shutdown interrupted before any
	// transition was written. The claim is still live and release() returns it.
	dispositionAbandoned
)

// String names the outcome, so a failing test reads the disposition instead of
// the iota behind it.
func (d disposition) String() string {
	switch d {
	case dispositionPublished:
		return "published"
	case dispositionRescheduled:
		return "rescheduled"
	case dispositionFailed:
		return "failed"
	case dispositionClaimLost:
		return "claim lost"
	case dispositionAbandoned:
		return "abandoned"
	default:
		return "unknown disposition"
	}
}

// deliver runs steps 2 and 3 for one claimed record: assemble, serialize,
// publish outside any transaction, then transition conditionally on the claim.
// No database connection is held while Publish runs, which is OBX-07.
func (r Relay) deliver(ctx context.Context, record dmpfpostgres.Claimed) (disposition, error) {
	env, err := Assemble(record, r.Source)
	if err != nil {
		return r.terminal(ctx, record, ownError(err))
	}
	message, err := envelope.Marshal(env)
	if err != nil {
		return r.terminal(ctx, record, ownError(err))
	}

	if publishErr := r.Publisher.Publish(ctx, record.Destination, message); publishErr != nil {
		// A publisher cancelled by the shutdown did not fail on its own merits:
		// charging it a backoff would delay a record that never got its turn.
		// release() hands the claim back at once instead (OBX-13).
		if ctx.Err() != nil {
			return dispositionAbandoned, publishErr
		}
		if r.MaxAttempts > 0 && record.AttemptCount >= r.MaxAttempts {
			return r.terminal(ctx, record, transportError(publishErr))
		}
		return r.reschedule(ctx, record, transportError(publishErr))
	}

	settleCtx, cancel := r.settleContext(ctx)
	defer cancel()

	affected, err := r.Store.MarkPublished(settleCtx, record.ID, record.LockedBy)
	return settle(dispositionPublished, affected, err)
}

// settleContext detaches step 3 from the caller's cancellation. A publication
// the broker already accepted has to be recorded even while the process is
// shutting down; losing that write republishes the message for nothing. The
// deadline keeps a stuck database from holding the process open (D8).
func (r Relay) settleContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), r.shutdownGrace())
}

// terminal is outcome 3c: the record leaves the automatic cycle (OBX-06),
// either because retrying cannot repair the failure or because the attempts
// are spent. reason is already sanitized by whoever knows where it came from.
func (r Relay) terminal(ctx context.Context, record dmpfpostgres.Claimed, reason string) (disposition, error) {
	settleCtx, cancel := r.settleContext(ctx)
	defer cancel()

	affected, err := r.Store.Fail(settleCtx, record.ID, record.LockedBy, reason)
	return settle(dispositionFailed, affected, err)
}

// reschedule is outcome 3b: the backoff lands on available_at and the lease is
// released in the same commit (OBX-18).
func (r Relay) reschedule(ctx context.Context, record dmpfpostgres.Claimed, reason string) (disposition, error) {
	settleCtx, cancel := r.settleContext(ctx)
	defer cancel()

	availableAt := r.Clock.Now() + dmpfports.Instant(r.backoffFor(record.AttemptCount))
	affected, err := r.Store.Reschedule(settleCtx, record.ID, record.LockedBy, availableAt, reason)
	return settle(dispositionRescheduled, affected, err)
}

// settle turns rows affected into a disposition: zero means the claim was
// replaced while the message was in flight, and the caller records the fact
// instead of republishing (OBX-10, OBX-11).
func settle(intended disposition, affected int64, err error) (disposition, error) {
	if err != nil {
		return intended, err
	}
	if affected == 0 {
		return dispositionClaimLost, nil
	}
	return intended, nil
}

// ownError is for failures raised by this block and by the contract package.
// Their text is a fixed string naming a rule, an attribute or a digest, so it
// reaches last_error as it is.
//
// The split with transportError is by ORIGIN, not by inspecting the chain: a
// transport error that happens to wrap one of these sentinels would pass an
// errors.Is test and carry whatever the transport wrote — a broker address, a
// DSN, a token — straight into the column (OBX-03, ERR-20, ERR-21).
func ownError(err error) string { return err.Error() }

// transportError names a failure from outside by its type alone. The message
// belongs to code this block does not control and may hold anything.
func transportError(err error) string {
	return fmt.Sprintf("relay: delivery failed (%T)", err)
}

func (r Relay) backoffFor(attempt int) time.Duration {
	if r.Backoff == nil {
		return 0
	}
	return r.Backoff(attempt)
}
