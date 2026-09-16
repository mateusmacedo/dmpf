// comment-discipline-ok-file: godoc de package; o que o bloco provider é e não é vem da matriz de blocos e da política de capabilities (RFC §6.2, §7.3), por exigência da spec do KRN-10.

// Package transport holds the transport primitives the four DMPF transport
// providers share (FND-06, FND-08): the per-method deadline budget (deadline),
// the channel catalogue with its redelivery-window formulas (channel), the
// attempt metadata carried beside the envelope (attempt), the admission by
// route and tenant (admission) and the observability positions (observe).
//
// It is a provider-block unit with no I/O: it never opens a connection, names
// a broker or reads a clock of its own. Its only external dependency is the
// OpenTelemetry API, through which observe fills the three observability
// positions RES-23 requires. Each provider — gRPC, HTTP, Kafka, SQS/SNS —
// composes these primitives with the decorators of observability (KRN-09)
// and realizes the ports of ports (KRN-04) against a concrete transport.
//
// What this package does not contain: the concrete address of any channel,
// which lives only in the configuration of the provider that operates it
// (TRP-07); the AsyncAPI document itself, of which channel is the in-process
// contract; and any retry or timeout formula of its own — those are KRN-09's.
package transport
