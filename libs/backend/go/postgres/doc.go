// comment-discipline-ok-file: godoc de package; o que o bloco provider é e não é vem da matriz de blocos e da política de capabilities (RFC §6.2, §7.3), por exigência da spec do KRN-06.

// Package postgres realizes the provider block of the DMPF kernel over
// PostgreSQL: the UnitOfWork[R], Repository[ID, S] and Outbox ports that
// ports and application fixed in domain types (KRN-04), backed by a
// single pgx.Tx per Within call.
//
// It imports the domain (cell 25), the port (cell 28) and the contract (cell
// 30) — application → provider and provider → application are forbidden cells
// of RFC §7.3, so no file here imports application, and the reverse
// import never happens either. Its capability budget is permissive, because
// realizing a port needs the I/O the blocks above cannot have (ADR-034).
//
// What this block does not contain: publishing, claim, lease and SKIP LOCKED,
// which are the relay's (KRN-08); the inbox and deduplication, KRN-07; the
// wire codec and the payload hash formula, which it only calls, KRN-05. The
// mapping from a domain event to its Protobuf contract happens here, at the
// moment of the write, never at read time (BLK-03, ADR-021).
package postgres
