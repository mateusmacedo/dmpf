// comment-discipline-ok-file: arquivo de declarações; cada sentinela espelha a da realização Postgres para que as duas realizações da porta tenham o mesmo contrato de erro.

package memory

import "errors"

var (
	// ErrInboxConsumerRequired mirrors dmpfpostgres.ErrInboxConsumerRequired (INB-01).
	ErrInboxConsumerRequired = errors.New("memory: inbox consumer name is required")

	// ErrInboxConsumerMismatch mirrors dmpfpostgres.ErrInboxConsumerMismatch.
	ErrInboxConsumerMismatch = errors.New("memory: receipt consumer does not match inbox consumer")

	// ErrAlreadyCompleted mirrors dmpfpostgres.ErrAlreadyCompleted.
	ErrAlreadyCompleted = errors.New("memory: pending already completed")

	// ErrInvalidCompletion mirrors dmpfpostgres.ErrInvalidCompletion (INB-02).
	ErrInvalidCompletion = errors.New("memory: completion status must be processed or rejected")
)
