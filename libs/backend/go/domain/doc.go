// comment-discipline-ok-file: godoc de package; o contrato de imutabilidade é registrado aqui por exigência da spec do KRN-03.

// Package domain realizes in Go the UPR outcome fixed by DMPF FND-03 §2–§3
// and ADR-018: a UPR returns (Accepted[R], *Rejection), exactly one of them set,
// with the concrete *Rejection (never error) as the only refusal channel. No
// port, clock, identifier generator, context or wire type lives here.
//
// Immutability contract: Accepted and Rejection copy the sequences they hold
// (DEC-13), so the outcome cannot be reordered or extended after it is produced.
// The content of R and of each DomainEvent is the aggregate's contract (DEC-12):
// comparable value types whose fields hold no pointer, slice, map or function.
// The kernel cannot deep-copy an arbitrary R without reflection, and comparable
// alone does not exclude pointers — it excludes slices, maps and functions;
// absence of pointers is a structural review item of each aggregate. See ADR-032.
package domain
