// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (BLK-04, OBX-01, FND-04 §4.1), dentro do limite de 3 linhas.

package postgres

import "errors"

var (
	// ErrEmptyMessageID is what Enqueue reports when the application service
	// did not author the identifier. The empty identifier is invalid by
	// ports.MessageID's own contract, and the outbox is where it is caught.
	ErrEmptyMessageID = errors.New("postgres: empty message id")

	// ErrInvalidDestination is what Enqueue reports for a destination that
	// names a physical target instead of a logical flow (BLK-04).
	ErrInvalidDestination = errors.New("postgres: destination must be a logical flow name")

	// ErrUnmappedEvent is what a mapper reports for a domain event with no
	// registered contract. An event nobody can put on the wire never reaches
	// the outbox: the transaction rolls back instead.
	ErrUnmappedEvent = errors.New("postgres: no contract registered for the domain event")

	// ErrEmptyMapping is a defect in the mapper, not in the event: it returned
	// success with no Message. Reported rather than dereferenced, so the fault
	// names its owner instead of surfacing as a nil pointer inside the provider.
	ErrEmptyMapping = errors.New("postgres: mapper returned no contract message")

	// ErrDuplicateMessage is what Enqueue reports when the unique constraint on
	// message_id rejects the row. It always wraps the driver error, so
	// errors.As still reaches *pgconn.PgError and its SQLSTATE (OBX-01).
	ErrDuplicateMessage = errors.New("postgres: message id already enqueued")

	// ErrInboxConsumerRequired is what Register reports when the consumer name
	// bound to the inbox is empty (INB-01).
	ErrInboxConsumerRequired = errors.New("postgres: inbox consumer name is required")

	// ErrInboxConsumerMismatch is what Register reports when Receipt.Consumer
	// differs from the consumer bound to the inbox at Tx.Inbox time.
	ErrInboxConsumerMismatch = errors.New("postgres: receipt consumer does not match inbox consumer")

	// ErrAlreadyCompleted is what Pending.Complete reports when called more
	// than once on the same first reception.
	ErrAlreadyCompleted = errors.New("postgres: pending already completed")

	// ErrInvalidCompletion is what Pending.Complete reports for a Status outside
	// processed and rejected (INB-02), named here rather than surfacing as the
	// schema's CHECK violation.
	ErrInvalidCompletion = errors.New("postgres: completion status must be processed or rejected")

	// ErrInvalidContainment is what Quarantine reports when a required field
	// of Contained is empty (GAR-07).
	ErrInvalidContainment = errors.New("postgres: invalid containment: consumer, reason and envelope are required")

	// ErrIncompleteStore is what OutboxStore reports when it was built without
	// a pool or without a clock, rather than dereferencing either.
	ErrIncompleteStore = errors.New("postgres: outbox store requires a pool and a clock")

	// ErrClaimIdentityRequired is what the claim and the three transitions
	// report for an empty identity: OBX-08 makes it the identity of one
	// acquisition, and OBX-10 conditions every later write on it.
	ErrClaimIdentityRequired = errors.New("postgres: claim identity is required")

	// ErrInvalidBatchSize is what Claim reports for a non-positive limit; the
	// value belongs to the caller (FND-08), its positivity to the statement.
	ErrInvalidBatchSize = errors.New("postgres: claim batch size must be positive")

	// ErrInvalidLease is what Claim reports for a non-positive lease, which
	// would be born expired and hand the record to the next claim at once.
	ErrInvalidLease = errors.New("postgres: claim lease must be positive")
)
