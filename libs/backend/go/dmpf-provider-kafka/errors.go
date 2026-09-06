// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-06 §6.2, §11; ASY-01; GAR-07), dentro do limite de 3 linhas.

package dmpfkafka

import (
	"errors"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
)

var (
	// ErrUnknownChannel is a destination the catalogue does not carry: the
	// provider does not operate it (ASY-01).
	ErrUnknownChannel = channel.ErrUnknownChannel

	// ErrOrderedChannelWithSeparateRetry is the refusal of TRP-32 and KFK-11,
	// surfaced here so a consumer of this package sees it by name.
	ErrOrderedChannelWithSeparateRetry = channel.ErrOrderedChannelWithSeparateRetry

	// ErrNotKafkaChannel is a catalogued channel bound to another transport.
	ErrNotKafkaChannel = errors.New("dmpfkafka: channel is not bound to kafka")

	// ErrTLSRequired is a client with neither TLS nor the explicit
	// development-only opt-out.
	ErrTLSRequired = errors.New("dmpfkafka: TLS is required outside development")

	// ErrTLSTooWeak is a TLS configuration that skips peer verification or
	// admits a protocol version below 1.2.
	ErrTLSTooWeak = errors.New("dmpfkafka: TLS must verify the peer and require at least TLS 1.2")

	// ErrSinkPanicked is a sink that panicked inside Handle: the attempt fails
	// without a gesture and the partition stalls; the consumer does not crash.
	ErrSinkPanicked = errors.New("dmpfkafka: sink panicked")

	// ErrIncompleteConfig is a configuration without brokers, catalogue or clock.
	ErrIncompleteConfig = errors.New("dmpfkafka: configuration is incomplete")

	// ErrIncompleteConsumer is a consumer without channel, sink, attempt limit,
	// or with a processing deadline that does not fit the rebalance timeout
	// (KFK-19).
	ErrIncompleteConsumer = errors.New("dmpfkafka: consumer is incomplete")

	// ErrAlreadyDisposed is a second gesture on the same delivery: Ack and
	// Release are terminal and exclusive (TRP-26, TRP-27).
	ErrAlreadyDisposed = errors.New("dmpfkafka: delivery already acknowledged or released")

	// ErrInvalidContainment is a quarantine without consumer, reason or
	// envelope: nothing to preserve (GAR-07).
	ErrInvalidContainment = errors.New("dmpfkafka: containment needs consumer, reason and envelope")
)
