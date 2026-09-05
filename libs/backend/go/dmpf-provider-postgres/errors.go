// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (BLK-04, OBX-01, FND-04 §4.1), dentro do limite de 3 linhas.

package dmpfpostgres

import "errors"

var (
	// ErrEmptyMessageID is what Enqueue reports when the application service
	// did not author the identifier. The empty identifier is invalid by
	// dmpfports.MessageID's own contract, and the outbox is where it is caught.
	ErrEmptyMessageID = errors.New("dmpfpostgres: empty message id")

	// ErrInvalidDestination is what Enqueue reports for a destination that
	// names a physical target instead of a logical flow (BLK-04).
	ErrInvalidDestination = errors.New("dmpfpostgres: destination must be a logical flow name")

	// ErrUnmappedEvent is what a mapper reports for a domain event with no
	// registered contract. An event nobody can put on the wire never reaches
	// the outbox: the transaction rolls back instead.
	ErrUnmappedEvent = errors.New("dmpfpostgres: no contract registered for the domain event")

	// ErrEmptyMapping is a defect in the mapper, not in the event: it returned
	// success with no Message. Reported rather than dereferenced, so the fault
	// names its owner instead of surfacing as a nil pointer inside the provider.
	ErrEmptyMapping = errors.New("dmpfpostgres: mapper returned no contract message")

	// ErrDuplicateMessage is what Enqueue reports when the unique constraint on
	// message_id rejects the row. It always wraps the driver error, so
	// errors.As still reaches *pgconn.PgError and its SQLSTATE (OBX-01).
	ErrDuplicateMessage = errors.New("dmpfpostgres: message id already enqueued")

	// ErrInboxConsumerRequired is what Register reports when the consumer name
	// bound to the inbox is empty (INB-01).
	ErrInboxConsumerRequired = errors.New("dmpfpostgres: inbox consumer name is required")

	// ErrInboxConsumerMismatch is what Register reports when Receipt.Consumer
	// differs from the consumer bound to the inbox at Tx.Inbox time.
	ErrInboxConsumerMismatch = errors.New("dmpfpostgres: receipt consumer does not match inbox consumer")

	// ErrAlreadyCompleted is what Pending.Complete reports when called more
	// than once on the same first reception.
	ErrAlreadyCompleted = errors.New("dmpfpostgres: pending already completed")

	// ErrInvalidContainment is what Quarantine reports when a required field
	// of Contained is empty (GAR-07).
	ErrInvalidContainment = errors.New("dmpfpostgres: invalid containment: consumer, reason and envelope are required")
)
