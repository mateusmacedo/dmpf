// comment-discipline-ok-file: godoc de portas; a razão de tempo e entropia serem porta, e não domínio, é norma (RFC §6.2, ADR-014, ADR-016), não código.

package dmpfports

// Clock is the only path by which an instant reaches a caller of this kernel.
// Reading the clock is io.clock capability by RFC §6.2, so it is a port: the
// domain does not ask for the time, it receives it (ADR-014, ADR-016).
type Clock interface {
	Now() Instant
}

// IDGenerator is the only source of message identity, on the same ground as
// Clock: entropy is io.random capability by RFC §6.2.
type IDGenerator interface {
	NewMessageID() MessageID
}
