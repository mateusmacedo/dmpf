// comment-discipline-ok-file: godoc de package; o contrato de imutabilidade é registrado aqui por exigência da spec do KRN-03.

// Package dmpfdomain realizes in Go the UPR outcome fixed by DMPF FND-03 §2–§3
// and ADR-018: a UPR returns (Accepted[R], *Rejection), exactly one of them set,
// with the concrete *Rejection (never error) as the only refusal channel. No
// port, clock, identifier generator, context or wire type lives here.
//
// Immutability contract: Accepted and Rejection copy the sequences they hold
// (DEC-12, DEC-13); the content of R and of each DomainEvent is the aggregate's
// contract — comparable value types, without slice, map or pointer — because a
// deep copy of an arbitrary R is not realizable without reflection (FND-03 §3.5).
package dmpfdomain
