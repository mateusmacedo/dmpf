package ports

import "errors"

// Fingerprint is the SHA-256 of the canonical encoding of a command (IDM-04),
// its payload hash in the inbox: the same key with another operation or payload
// is a collision (R4).
type Fingerprint [32]byte

// ErrIdempotencyMismatch is the use case's report of a command whose key the
// inbox holds for another operation or payload (R4): the command does not run.
var ErrIdempotencyMismatch = errors.New("ports: idempotency key reused with a different request")

// ErrIdempotencyInFlight is the use case's report when the inbox's wait for a
// concurrent command of the same key ends (INB-17): repeating it converges (IDM-07).
var ErrIdempotencyInFlight = errors.New("ports: idempotency key in flight")

// ErrIdempotencyKeyAbsent is the use case's refusal of a command whose carrier
// holds no key, a defense behind the edge's own check (IDM-01).
var ErrIdempotencyKeyAbsent = errors.New("ports: idempotency key absent")

// ErrIdempotencyKeyInvalid is the use case's refusal of a key outside
// IdempotencyKeyPattern, a defense behind the edge's own check (IDM-02).
var ErrIdempotencyKeyInvalid = errors.New("ports: idempotency key invalid")
