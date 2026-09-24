// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-06 §6.2, §11; ASY-01; GAR-07), dentro do limite de 3 linhas.

package kafka

import (
	"errors"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

var (
	// ErrUnknownChannel is a destination the catalogue does not carry: the
	// provider does not operate it (ASY-01).
	ErrUnknownChannel = channel.ErrUnknownChannel

	// ErrOrderedChannelWithSeparateRetry is the refusal of TRP-32 and KFK-11,
	// surfaced here so a consumer of this package sees it by name.
	ErrOrderedChannelWithSeparateRetry = channel.ErrOrderedChannelWithSeparateRetry

	// ErrNotKafkaChannel is a catalogued channel bound to another transport.
	ErrNotKafkaChannel = errors.New("kafka: channel is not bound to kafka")

	// ErrTLSRequired is a client with neither TLS nor the explicit
	// development-only opt-out.
	ErrTLSRequired = errors.New("kafka: TLS is required outside development")

	// ErrTLSTooWeak is a TLS configuration that skips peer verification or
	// admits a protocol version below 1.2.
	ErrTLSTooWeak = errors.New("kafka: TLS must verify the peer and require at least TLS 1.2")

	// ErrSinkPanicked is a sink that panicked inside Handle: the attempt fails
	// without a gesture and the partition stalls; the consumer does not crash.
	ErrSinkPanicked = errors.New("kafka: sink panicked")

	// ErrClientAuthRequired is a TLS client that does not authenticate itself:
	// TLS verifies the broker, not who produces, which IDN-04 needs.
	ErrClientAuthRequired = errors.New("kafka: a TLS client must authenticate with SASL or a client certificate (IDN-04)")

	// ErrSASLMechanism is a SASL declaration outside the mechanisms this
	// provider speaks; PLAIN is not one, because it sends the secret as is.
	ErrSASLMechanism = errors.New("kafka: SASL mechanism must be SCRAM-SHA-256 or SCRAM-SHA-512")

	// ErrSASLCredentials is a SASL declaration without the principal or its secret.
	ErrSASLCredentials = errors.New("kafka: SASL requires a username and a password")

	// ErrIncompleteConfig is a configuration without brokers, catalogue or clock.
	ErrIncompleteConfig = errors.New("kafka: configuration is incomplete")

	// ErrIncompleteConsumer is a consumer without channel, sink, attempt limit,
	// or with a processing deadline that does not fit the rebalance timeout
	// (KFK-19).
	ErrIncompleteConsumer = errors.New("kafka: consumer is incomplete")

	// ErrAlreadyDisposed is a second gesture on the same delivery: Ack and
	// Release are terminal and exclusive (TRP-26, TRP-27).
	ErrAlreadyDisposed = errors.New("kafka: delivery already acknowledged or released")

	// ErrInvalidContainment is a quarantine without consumer, reason or
	// envelope: nothing to preserve (GAR-07).
	ErrInvalidContainment = errors.New("kafka: containment needs consumer, reason and envelope")
)
