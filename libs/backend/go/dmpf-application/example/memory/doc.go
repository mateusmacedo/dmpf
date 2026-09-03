// comment-discipline-ok-file: godoc de package; o que esta realização prova e o que NÃO prova é exigência da spec do KRN-04, e a divergência entre o gate local e o verificador precisa ficar declarada.

// Package memory realizes the unit of work and the ports in memory, as the
// provider unit dmpf-kernel/example-memory. It closes the reference use case
// end to end without a database, and KRN-06 replaces it with Postgres.
//
// What it proves: one transaction per Within over one resource, the callback
// invoked exactly once, commit applying business state and outbox together,
// rollback on error and on panic, an injectable commit failure, a cancelled
// context never opening a transaction, and snapshots that share no backing
// array with the store.
//
// What it does not prove: isolation and serialization conflict between
// concurrent transactions. Within holds one mutex for the whole callback, so
// transactions are serialized rather than isolated, and Load and Save of the
// same Tx always see the same state — a version conflict has to be injected by
// the caller. Real isolation belongs to KRN-06.
//
// One consequence of the mutex: calling Store.Reader() from inside Within on
// the same store deadlocks. The reference use case reads outside the unit of
// work (UOW-11), which is exactly the intended shape.
//
// This subpackage is a provider, whose capability budget the verifier leaves
// unrestricted (capability.go:51), but the local depguard rule matches
// **/*-application/** and therefore reaches it under the stricter application
// budget. The divergence has no practical effect here, because every import is
// pure capability; the verifier remains the authoritative gate.
package memory
