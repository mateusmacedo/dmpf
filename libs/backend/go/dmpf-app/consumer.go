// comment-discipline-ok-file: arquivo de declarações do adapter; cada godoc é contrato de API pública com referência normativa (FND-04 §6.3, INB-08, INB-10, GAR-07, GAR-08), dentro do limite de 3 linhas.

package dmpfapp

import (
	"context"
	"errors"

	dmpfapplication "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/payloadhash"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// ErrIncompleteConsumer is what Consume reports when a collaborator is missing;
// the adapter has no defaults to fall back on, because every value here is the
// caller's declaration (FND-08 catalogues them, this block only demands them).
var ErrIncompleteConsumer = errors.New("dmpfapp: consumer requires name, handler, containment and clock")

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
type Handler func(ctx context.Context, receipt dmpfports.Receipt, env envelope.Envelope) (dmpfapplication.Disposition, error)

// Consumer is the consumer adapter of FND-04 §6.3: steps 1, 2 and 7 around a
// Handler. MaxAttempts <= 0 disables the attempt limit; the value belongs to
// FND-08, the existence of the limit to GAR-08.
type Consumer struct {
	Name        string
	MaxAttempts int
	Handle      Handler
	Containment dmpfports.Containment
	Clock       dmpfports.Clock
}

// Outcome is what the adapter did with one delivery. Classified is false only
// for an invalid envelope, which never reaches the inbox and so has no
// disposition (INB-10); Contained says the message left the flow (GAR-11).
type Outcome struct {
	Disposition dmpfapplication.Disposition
	Classified  bool
	Contained   bool
	Reason      dmpfports.Reason
}

// Consume runs the sequence of §6.3 on the adapter's side and applies the
// broker effect strictly after the handler returned (INB-08). The error is the
// handler's own under D3/D4, or the broker's or quarantine's when they fail.
func (c Consumer) Consume(ctx context.Context, d Delivery, ack dmpfports.Acknowledger) (Outcome, error) {
	if c.Name == "" || c.Handle == nil || c.Containment == nil || c.Clock == nil {
		return Outcome{}, ErrIncompleteConsumer
	}

	env, err := envelope.Unmarshal(d.Raw)
	if err != nil {
		return c.contain(ctx, ack, Outcome{Reason: dmpfports.ReasonInvalidEnvelope}, dmpfports.Contained{
			Consumer: c.Name,
			Reason:   dmpfports.ReasonInvalidEnvelope,
			Envelope: d.Raw,
			Error:    err.Error(),
			At:       c.Clock.Now(),
		}, nil)
	}

	receipt := dmpfports.Receipt{
		Consumer:    c.Name,
		MessageID:   dmpfports.MessageID(env.ID),
		MessageType: env.Type,
		PayloadHash: payloadhash.Sum(env.Payload),
		ReceivedAt:  c.Clock.Now(),
	}

	disposition, handleErr := c.Handle(ctx, receipt, env)
	outcome := Outcome{Disposition: disposition, Classified: true}

	switch disposition {
	case dmpfapplication.R1D1, dmpfapplication.R1D2, dmpfapplication.R2, dmpfapplication.R3:
		return outcome, errors.Join(handleErr, ack.Ack(ctx))
	case dmpfapplication.R1D3:
		if c.MaxAttempts > 0 && d.Attempt >= c.MaxAttempts {
			return c.contain(ctx, ack, withReason(outcome, dmpfports.ReasonAttemptsExhausted), c.contained(receipt, d.Raw, dmpfports.ReasonAttemptsExhausted, disposition, handleErr), handleErr)
		}
		return outcome, errors.Join(handleErr, ack.Release(ctx))
	case dmpfapplication.R1D4:
		return c.contain(ctx, ack, withReason(outcome, dmpfports.ReasonTerminalFailure), c.contained(receipt, d.Raw, dmpfports.ReasonTerminalFailure, disposition, handleErr), handleErr)
	case dmpfapplication.R4:
		return c.contain(ctx, ack, withReason(outcome, dmpfports.ReasonCollision), c.contained(receipt, d.Raw, dmpfports.ReasonCollision, disposition, handleErr), handleErr)
	default:
		panic("dmpfapp: handler returned a disposition outside the seven of FND-04 §6.4")
	}
}

// contain quarantines first and confirms only afterwards: a message that could
// not be kept is not contained, it would be lost with a record (GAR-07), so the
// broker never hears an ack for it.
func (c Consumer) contain(ctx context.Context, ack dmpfports.Acknowledger, outcome Outcome, item dmpfports.Contained, cause error) (Outcome, error) {
	if mechanism, ok := MechanismFor(item.Reason); !ok || mechanism != MechanismQuarantine {
		return outcome, errors.Join(cause, ErrUnsupportedMechanism)
	}
	if err := c.Containment.Quarantine(ctx, item); err != nil {
		return outcome, errors.Join(cause, err)
	}
	outcome.Contained = true
	return outcome, errors.Join(cause, ack.Ack(ctx))
}

func (c Consumer) contained(receipt dmpfports.Receipt, raw []byte, reason dmpfports.Reason, disposition dmpfapplication.Disposition, cause error) dmpfports.Contained {
	return dmpfports.Contained{
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
func sanitizedError(disposition dmpfapplication.Disposition, cause error) string {
	var failure *dmpfapplication.Failure
	if errors.As(cause, &failure) {
		return string(failure.Category())
	}
	return disposition.String()
}

func withReason(outcome Outcome, reason dmpfports.Reason) Outcome {
	outcome.Reason = reason
	return outcome
}
