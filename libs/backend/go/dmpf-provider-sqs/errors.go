// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-06 §12, TRP-19, TRP-30, ASY-01), dentro do limite de 3 linhas.

package dmpfsqs

import (
	"errors"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/channel"
)

var (
	// ErrUnknownChannel is a destination the catalogue does not carry (ASY-01).
	ErrUnknownChannel = channel.ErrUnknownChannel

	// ErrNotSQSChannel is a catalogued channel bound to another transport.
	ErrNotSQSChannel = errors.New("dmpfsqs: channel is not bound to sqs or sns-sqs")

	// ErrIncompleteConfig is a configuration without region, catalogue or clock.
	ErrIncompleteConfig = errors.New("dmpfsqs: configuration is incomplete")

	// ErrTLSRequired is an endpoint that is not https without the explicit
	// development-only opt-out: the SigV4 signature travels in the headers.
	ErrTLSRequired = errors.New("dmpfsqs: endpoint must be https outside development")

	// ErrSinkPanicked is a sink that panicked inside Handle: the attempt ends
	// without a gesture and the visibility expires; the consumer does not crash.
	ErrSinkPanicked = errors.New("dmpfsqs: sink panicked")

	// ErrIncompleteConsumer is a consumer without channel, sink, concurrency or
	// heartbeat, or one whose deadline exceeds the visibility (SQS-13).
	ErrIncompleteConsumer = errors.New("dmpfsqs: consumer is incomplete")

	// ErrInvalidEnvelope is what Publish reports when the message does not
	// decode as the envelope of FND-05: it is not published.
	ErrInvalidEnvelope = errors.New("dmpfsqs: message is not a valid envelope")

	// ErrInvalidBody is a received body that is not the single Base64 encoding
	// of TRP-19.
	ErrInvalidBody = errors.New("dmpfsqs: body is not a single base64 encoding of the envelope")

	// ErrMessageTooLarge is a final body over the smallest limit of the path
	// (SQS-12): it is refused before any send, and the channel is redesigned
	// rather than transported by reference (SQS-12b).
	ErrMessageTooLarge = errors.New("dmpfsqs: encoded message exceeds the transport limit (SQS-12)")

	// ErrTooManyAttributes is more than ten message attributes, which the raw
	// SNS → SQS delivery drops silently (SQS-03b).
	ErrTooManyAttributes = errors.New("dmpfsqs: more than ten message attributes (SQS-03b)")

	// ErrSNSEnvelopeNotRaw is a body that is the SNS notification wrapper: the
	// subscription lacks raw message delivery, the non-conforming hop (SQS-02).
	ErrSNSEnvelopeNotRaw = errors.New("dmpfsqs: body is an SNS notification wrapper, not the envelope (SQS-02)")

	// ErrRawDeliveryRequired is a subscription the publisher refuses to feed
	// because it does not deliver raw (SQS-02).
	ErrRawDeliveryRequired = errors.New("dmpfsqs: subscription must have RawMessageDelivery enabled (SQS-02)")

	// ErrFilterOnFifoTopic is a filter policy on a FIFO topic, which turns
	// delivery into at-most-once (SQS-03c).
	ErrFilterOnFifoTopic = errors.New("dmpfsqs: filter policy on a fifo topic is refused (SQS-03c)")

	// ErrNoSubscriptions is a topic with no SQS subscription: a publication
	// there reaches no queue and no inbox, silently.
	ErrNoSubscriptions = errors.New("dmpfsqs: topic has no sqs subscription")

	// ErrOrderingMismatch is a FIFO channel without group ordering, or a
	// standard one that promises order (SQS-04).
	ErrOrderingMismatch = errors.New("dmpfsqs: ordering unit disagrees with the queue type (SQS-04)")

	// ErrInlineRetryOnSQS is a consumer that would retry in a loop: on SQS the
	// redelivery is the queue's, by visibility, never an inline loop (SQS-10, SQS-11b).
	ErrInlineRetryOnSQS = errors.New("dmpfsqs: inline retry is not a gesture of this transport (SQS-11b)")

	// ErrDeadlineOverVisibility is a processing deadline that does not fit the
	// visibility applied (SQS-13).
	ErrDeadlineOverVisibility = errors.New("dmpfsqs: processing deadline must be below the visibility (SQS-13)")

	// ErrAlreadyDisposed is a second gesture on the same receipt (TRP-27, SQS-09).
	ErrAlreadyDisposed = errors.New("dmpfsqs: message already acknowledged or released")

	// ErrInvalidContainment is a quarantine without consumer, reason or envelope.
	ErrInvalidContainment = errors.New("dmpfsqs: containment needs consumer, reason and envelope")
)
