// comment-discipline-ok-file: godoc de package; a alocação de cada responsabilidade entre este bloco, o domínio e FND-07 é norma (FND-04 §3.2, RFC §4.1), por exigência da spec do KRN-04.

// Package application is the application block of the orders bounded context:
// the caller side of the two UPRs of the Order aggregate, walking the nine
// steps of FND-04 §3.2. It is the reference write use case of the kernel
// topology (ADR-044), and its unit and bounded context are stated in
// dmpf-units.json (ADR-012).
//
// Each command produces at most one event, so identity is resolved once with
// events = 1 before the transaction opens. Under acceptance, Save and Enqueue
// run in the same transaction, in that order, so a version conflict stops the
// sequence before any outbox record exists. Under refusal, neither is called,
// the commit still happens and the caller gets the typed rejection with a nil
// error (UOW-05, UOW-06, DEC-04). FindOrder reads outside the unit of work and
// never touches the outbox (UOW-11).
//
// What this package does not do: validate the shape of the input (Quantity <= 0
// or an empty SKU), which RFC §4.1 gives to the app block; decide anything,
// which is the aggregate's; or retry, which is the use case's own policy and
// KRN-09's subject — Within never repeats the callback (UOW-09, UOW-10). It
// imports the domain and the ports, never the in-memory realization in
// libs/backend/go/memory: application → provider is a forbidden cell, and the
// bind that joins them is written by the composition root.
package application
