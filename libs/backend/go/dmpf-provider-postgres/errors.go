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

	// ErrDuplicateMessage is what Enqueue reports when the unique constraint on
	// message_id rejects the row. It always wraps the driver error, so
	// errors.As still reaches *pgconn.PgError and its SQLSTATE (OBX-01).
	ErrDuplicateMessage = errors.New("dmpfpostgres: message id already enqueued")
)
