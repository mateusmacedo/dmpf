// comment-discipline-ok-file: godoc de package; o que o bloco port é e não é vem da matriz de blocos e da política de capabilities (RFC §6.2, §7.3), por exigência da spec do KRN-04.

// Package ports declares the port block of the DMPF kernel: the boundary
// through which an application service reaches time, message identity,
// persisted state and the outbox, expressed only in types the caller owns.
//
// Every port here is an interface the consumer declares and a provider
// realizes, so the direction of dependency is application → port → domain
// (cells 10 and 19 of RFC §7.3). This block imports the domain and nothing
// else: port → application, port → provider and port → contract are forbidden
// cells, and its whole transitive import closure is pure capability by RFC
// §6.2 — no time, no rand, no os, no net, no database/sql, no encoding, no
// third party. That is why the instant is Instant rather than time.Time, and
// why the clock is a port.
//
// What this block does not contain, and where it lives instead: the outbox
// schema, drainage and SQL are the provider's (KRN-06); mapping a domain event
// to wire format is the contract's (KRN-05); publishing and the relay are
// KRN-08; the inbox and deduplication are KRN-07; retry, budget and telemetry
// are KRN-09. There is no publishing port and no inbox port here, deliberately.
package ports
