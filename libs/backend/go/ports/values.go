// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-04, RFC §9.3), dentro do limite de 3 linhas.

package ports

// Instant is a point in time as nanoseconds since the Unix epoch: available_at
// anchors on occurred_at and the outbox drains ordered by it (FND-04 §4.2), so
// seconds would collide under load. It arrives through Clock (RFC §9.3).
type Instant int64

// MessageID is the globally unique identifier of a message (message_id, FND-04
// §4.1), authored by the application service before the transaction opens. The
// empty identifier is invalid.
type MessageID string

// Version is the aggregate version of optimistic locking (FND-04 §3.3). Zero
// means "not persisted yet": Save with expected == 0 creates the aggregate.
type Version uint64
