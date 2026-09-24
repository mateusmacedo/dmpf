// comment-discipline-ok-file: arquivo de declarações; cada sentinela espelha a da realização Postgres para que as duas realizações da porta tenham o mesmo contrato de erro.

package memory

import "errors"

var (
	// ErrTenantUnresolved reports a read or write whose carrier resolved no
	// tenant. It is a refusal, not an empty result: the scope is not optional
	// here for the same reason it is not optional in Postgres (IDN-15).
	ErrTenantUnresolved = errors.New("memory: the execution context resolved no tenant")

	// ErrInboxConsumerRequired mirrors postgres.ErrInboxConsumerRequired (INB-01).
	ErrInboxConsumerRequired = errors.New("memory: inbox consumer name is required")

	// ErrInboxConsumerMismatch mirrors postgres.ErrInboxConsumerMismatch.
	ErrInboxConsumerMismatch = errors.New("memory: receipt consumer does not match inbox consumer")

	// ErrAlreadyCompleted mirrors postgres.ErrAlreadyCompleted.
	ErrAlreadyCompleted = errors.New("memory: pending already completed")

	// ErrInvalidCompletion mirrors postgres.ErrInvalidCompletion (INB-02).
	ErrInvalidCompletion = errors.New("memory: completion status must be processed or rejected")
)
