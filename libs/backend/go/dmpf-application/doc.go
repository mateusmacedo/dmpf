// comment-discipline-ok-file: godoc de package; o que o bloco application é e não é vem da matriz de blocos e da política de capabilities (RFC §6.2, §7.3), por exigência da spec do KRN-04.

// Package dmpfapplication declares the application block of the DMPF kernel:
// the caller side of a UPR. It owns the outcome that separates the business
// channel from the technical one, the identity resolved before the transaction
// opens, and the authorization hook of step 1.
//
// It imports the domain and the ports, never a provider: application → domain,
// application → application and application → port are permitted cells of RFC
// §7.3, while application → provider and application → contract are forbidden.
// Its capability budget is pure plus observability (capability.go:47), so no
// time, no rand, no os, no net, no database/sql, no encoding and no third
// party. The instant and the message identifier arrive as port values.
//
// What this block does not contain: the transaction itself, which a provider
// realizes (example/memory here, Postgres in KRN-06); the wire format, which is
// KRN-05; publishing and the relay, KRN-08; the inbox, KRN-07; retry and
// telemetry, KRN-09. Input shape validation is the app block's (RFC §4.1), and
// the real authorization, the execution context and the error taxonomy are
// FND-07's — AuthorizeFunc is only the hook they will fill.
package dmpfapplication
