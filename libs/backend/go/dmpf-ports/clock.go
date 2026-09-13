// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (RFC §6.2, ADR-014, ADR-016), dentro do limite de 3 linhas.

package dmpfports

// Clock is the only path by which an instant reaches a caller: reading the
// clock is io.clock capability (RFC §6.2), so it is a port and never the
// domain, which receives the time instead of asking for it (ADR-014, ADR-016).
type Clock interface {
	Now() Instant
}

// IDGenerator is the only source of message identity, on the same ground as
// Clock: entropy is io.random capability (RFC §6.2).
type IDGenerator interface {
	NewMessageID() MessageID
}
