// comment-discipline-ok-file: godoc de package; a alocação de cada responsabilidade entre este bloco, o domínio e FND-07 é norma (FND-04 §3.2, RFC §4.1), por exigência da spec do KRN-04.

// Package ordersapp is the reference write use case of the DMPF kernel: the
// caller side of the two UPRs of the orders example, walking the nine steps of
// FND-04 §3.2. It exists as an executable subject for the unit of work and for
// the conformance vectors, not as a business module.
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
// imports the domain and the ports, never the in-memory realization next to it:
// application → provider is a forbidden cell, and the bind that joins them is
// written by the composition root.
package ordersapp
