// comment-discipline-ok-file: godoc de package; o que esta realização prova e o que NÃO prova é exigência da spec da porta de unit of work, e precisa ficar declarado junto da realização.

// Package memory realizes the unit of work and the ports in memory, as the
// provider unit kernel/provider-memory. It closes a use case end to end
// without a database, over whatever aggregates a Table names; postgres is the
// counterpart that runs it against a real one (KRN-06).
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
// the caller. Real isolation is postgres's (KRN-06).
//
// Store keeps two mutexes so that this serialization does not become a trap:
// txMu is held for the whole callback and is what makes one transaction exclude
// another, while dataMu is held per read or write. A Table.Reader load from
// inside a callback therefore returns the committed state — what a reader
// outside the transaction would see — instead of deadlocking.
//
// This module is a provider, whose capability budget the verifier leaves
// unrestricted (rule/capability.go:51), because realizing a port needs the I/O
// the blocks above cannot have.
package memory
