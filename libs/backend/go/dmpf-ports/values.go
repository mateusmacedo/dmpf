// comment-discipline-ok-file: godoc de valores de fronteira; a resolução (ns), a semântica do zero e a vedação ao relógio vêm da norma (FND-04, RFC §9.3), não do código.

package dmpfports

const nanosPerSecond = 1_000_000_000

// Instant is a point in time as nanoseconds since the Unix epoch. Nanoseconds
// because available_at anchors on occurred_at and the outbox drains ordered by
// it (FND-04 §4.2); seconds would collide under load. The block never reads a
// clock: the instant arrives through Clock (RFC §9.3, ADR-016).
type Instant int64

// Unix truncates the instant to whole seconds, dropping the sub-second part. It
// exists for domains whose own time value is in seconds, such as orders.Instant.
func (i Instant) Unix() int64 { return int64(i) / nanosPerSecond }

// MessageID is the globally unique identifier of a message (message_id, FND-04
// §4.1), authored by the application service before the transaction opens. The
// empty identifier is invalid.
type MessageID string

// Version is the aggregate version of optimistic locking (FND-04 §3.3). Zero
// means "not persisted yet": Save with expected == 0 creates the aggregate.
type Version uint64
