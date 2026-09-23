// comment-discipline-ok-file: arquivo de declarações do adapter; cada godoc é contrato de API pública com referência normativa (FND-04 §6.3, INB-08, INB-10, GAR-07, GAR-08), dentro do limite de 3 linhas.

package app

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ErrIncompleteConsumer is what Consume reports when a collaborator is missing;
// the adapter has no defaults to fall back on, because every value here is the
// caller's declaration (FND-08 catalogues them, this block only demands them).
var ErrIncompleteConsumer = errors.New("app: consumer requires name, handler, containment, clock, timeout, boundary and locale")

// Transport says on what ground the channel that delivered a message is
// trusted. A value outside the two below is an undeclared boundary.
type Transport string

const (
	// TransportVerified states that the broker verifies the producing workload
	// and the channel is under a declared administrative domain (IDN-03); the
	// proof is the platform's, this value says it was established.
	TransportVerified Transport = "verified"
	// TransportDevelopmentOnly mirrors the transport's own opt-out, never a default.
	TransportDevelopmentOnly Transport = "development-only"
)

// Boundary is the trusted boundary CTX-27 and IDN-04 require before a message
// produces a context: the transport's trust plus the producers the channel
// admits, by the envelope's source attribute (ENV-08).
type Boundary struct {
	Transport Transport
	Sources   []string
}

func (b Boundary) declared() bool {
	return (b.Transport == TransportVerified || b.Transport == TransportDevelopmentOnly) && len(b.Sources) > 0
}

func (b Boundary) admits(source string) bool { return slices.Contains(b.Sources, source) }

// ErrUnknownDisposition is what Consume reports when the handler returns a value
// outside the seven of §6.4. It is a defect, but the adapter is the border
// (ERR-23): it surfaces the error and leaves the message unconfirmed, it never
// panics a consumer loop.
var ErrUnknownDisposition = errors.New("app: handler returned a disposition outside the seven of FND-04 §6.4")

// Delivery is one message as the transport handed it over: Raw is kept byte for
// byte because quarantine must preserve what was published, never a re-marshal
// (GAR-07); Attempt is the transport's delivery count, starting at 1.
type Delivery struct {
	Raw     []byte
	Attempt int
}

// Handler is the application service of consumption behind the adapter. It
// returns the disposition of FND-04 §6.4 and, under R1×D3 or R1×D4, the
// technical error already classified (MAP-07).
type Handler func(ctx context.Context, receipt ports.Receipt, env envelope.Envelope) (application.Disposition, error)

// Consumer is the consumer adapter of FND-04 §6.3: steps 1, 2 and 7 around a
// Handler. MaxAttempts <= 0 disables the attempt limit; the value belongs to
// FND-08, the existence of the limit to GAR-08.
type Consumer struct {
	Name        string
	MaxAttempts int
	Handle      Handler
	Containment ports.Containment
	Clock       ports.Clock

	// Timeout is the time policy of this consumer, applied per attempt. CTX-28
	// makes the deadline the consumer's own, so the adapter demands one rather
	// than handing the handler an execution with no limit to declare.
	Timeout time.Duration

	Boundary Boundary

	// Locale answers the mandatory field of CTX-01 that a consumption has no
	// caller to state; it is the consumer's declaration, like Timeout.
	Locale string
}

// Outcome is what the adapter did with one delivery. Classified is false only
// for an invalid envelope, which never reaches the inbox and so has no
// disposition (INB-10); Contained says the message left the flow (GAR-11).
type Outcome struct {
	Disposition application.Disposition
	Classified  bool
	Contained   bool
	Reason      ports.Reason
}

// Consume runs the sequence of §6.3 on the adapter's side and applies the
// broker effect strictly after the handler returned (INB-08). The error is the
// handler's own under D3/D4, or the broker's or quarantine's when they fail.
func (c Consumer) Consume(ctx context.Context, d Delivery, ack ports.Acknowledger) (Outcome, error) {
	if c.Name == "" || c.Handle == nil || c.Containment == nil || c.Clock == nil || c.Timeout <= 0 || !c.Boundary.declared() || c.Locale == "" {
		return Outcome{}, ErrIncompleteConsumer
	}

	env, err := envelope.Unmarshal(d.Raw)
	if err != nil {
		return c.contain(ctx, ack, Outcome{Reason: ports.ReasonInvalidEnvelope}, ports.Contained{
			Consumer: c.Name,
			Reason:   ports.ReasonInvalidEnvelope,
			Envelope: d.Raw,
			Error:    sanitizedEnvelopeError(err),
			At:       c.Clock.Now(),
		}, nil)
	}

	if !c.Boundary.admits(env.Source) {
		return c.contain(ctx, ack, Outcome{Reason: ports.ReasonUntrustedBoundary}, ports.Contained{
			Consumer:  c.Name,
			MessageID: ports.MessageID(env.ID),
			Reason:    ports.ReasonUntrustedBoundary,
			Envelope:  d.Raw,
			Error:     "app: source outside the trusted boundary",
			At:        c.Clock.Now(),
		}, nil)
	}

	receipt := ports.Receipt{
		Consumer:    c.Name,
		MessageID:   ports.MessageID(env.ID),
		MessageType: env.Type,
		PayloadHash: payloadhash.Sum(env.Payload),
		ReceivedAt:  c.Clock.Now(),
	}

	ctx = ports.WithMessageContext(ctx, ports.MessageContext{
		CorrelationID: env.CorrelationID,
		CausationID:   env.ID,
		Traceparent:   env.TraceParent,
	})
	attempt := Attempt{RequestID: newAttemptID(), Number: d.Attempt}
	ctx = WithAttempt(ctx, attempt)

	handleCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	execution, err := c.executionOf(handleCtx, env, attempt)
	if err != nil {
		return Outcome{}, err
	}
	disposition, handleErr := c.Handle(ports.WithExecutionContext(handleCtx, execution), receipt, env)
	outcome := Outcome{Disposition: disposition, Classified: true}

	switch disposition {
	case application.R1D1, application.R1D2, application.R2, application.R3:
		return outcome, errors.Join(handleErr, ack.Ack(ctx))
	case application.R1D3:
		if c.MaxAttempts > 0 && d.Attempt >= c.MaxAttempts {
			return c.contain(ctx, ack, withReason(outcome, ports.ReasonAttemptsExhausted), c.contained(receipt, d.Raw, ports.ReasonAttemptsExhausted, disposition, handleErr), handleErr)
		}
		return outcome, errors.Join(handleErr, ack.Release(ctx))
	case application.R1D4:
		return c.contain(ctx, ack, withReason(outcome, ports.ReasonTerminalFailure), c.contained(receipt, d.Raw, ports.ReasonTerminalFailure, disposition, handleErr), handleErr)
	case application.R4:
		return c.contain(ctx, ack, withReason(outcome, ports.ReasonCollision), c.contained(receipt, d.Raw, ports.ReasonCollision, disposition, handleErr), handleErr)
	default:
		return Outcome{}, errors.Join(handleErr, ErrUnknownDisposition)
	}
}

// executionOf rebuilds the context of one consumption: correlation, causation,
// trace and tenant from the envelope (CTX-24), no subject because provenance is
// not identity (CTX-25), and request id and deadline the consumer's own (CTX-28).
func (c Consumer) executionOf(ctx context.Context, env envelope.Envelope, attempt Attempt) (ports.ExecutionContext, error) {
	deadline, _ := ctx.Deadline()
	var causation *string
	if env.CausationID != "" {
		causation = &env.CausationID
	}
	var tenant *ports.TenantID
	if env.TenantID != nil {
		value := ports.TenantID(*env.TenantID)
		tenant = &value
	}
	return ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID:     attempt.RequestID,
		CorrelationID: env.CorrelationID,
		CausationID:   causation,
		TraceContext:  env.TraceParent,
		Tenant:        tenant,
		Deadline:      ports.Instant(deadline.UnixNano()),
		Locale:        c.Locale,
	})
}

// sanitizedEnvelopeError keeps the quarantine free of transported bytes (ERR-20,
// ERR-21): the envelope package's own messages name only sentinels and attribute
// names, but ErrMalformed wraps the wire decoder's error, which is dropped.
func sanitizedEnvelopeError(err error) string {
	if errors.Is(err, envelope.ErrMalformed) {
		return envelope.ErrMalformed.Error()
	}
	return err.Error()
}

// contain quarantines first and confirms only afterwards: a message that could
// not be kept is not contained, it would be lost with a record (GAR-07), so the
// broker never hears an ack for it.
func (c Consumer) contain(ctx context.Context, ack ports.Acknowledger, outcome Outcome, item ports.Contained, cause error) (Outcome, error) {
	if mechanism, ok := MechanismFor(item.Reason); !ok || mechanism != MechanismQuarantine {
		return outcome, errors.Join(cause, ErrUnsupportedMechanism)
	}
	if err := c.Containment.Quarantine(ctx, item); err != nil {
		return outcome, errors.Join(cause, err)
	}
	outcome.Contained = true
	return outcome, errors.Join(cause, ack.Ack(ctx))
}

func (c Consumer) contained(receipt ports.Receipt, raw []byte, reason ports.Reason, disposition application.Disposition, cause error) ports.Contained {
	return ports.Contained{
		Consumer:  c.Name,
		MessageID: receipt.MessageID,
		Reason:    reason,
		Envelope:  raw,
		Error:     sanitizedError(disposition, cause),
		At:        receipt.ReceivedAt,
	}
}

// sanitizedError keeps the internal projection free of business data (ERR-20,
// ERR-21): the category when the error is classified, the disposition otherwise.
func sanitizedError(disposition application.Disposition, cause error) string {
	var failure *application.Failure
	if errors.As(cause, &failure) {
		return string(failure.Category())
	}
	return disposition.String()
}

func withReason(outcome Outcome, reason ports.Reason) Outcome {
	outcome.Reason = reason
	return outcome
}
