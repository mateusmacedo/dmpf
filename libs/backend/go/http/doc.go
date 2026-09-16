// comment-discipline-ok-file: godoc de package; o que o bloco provider é e não é vem da matriz de blocos e da política de capabilities (RFC §6.2, §7.3), por exigência da spec do KRN-10.

// Package http realizes the external synchronous transport of the DMPF
// kernel: REST/JSON at the outer edge (FND-06 §9, ADR-024), with the client
// timeout derived from the caller's deadline (RST-01 to RST-04), retry only for
// idempotent methods, and the admission of observability per route and
// tenant.
//
// It is a provider-block unit over the standard library alone: no HTTP
// framework enters the kernel. It composes transport/deadline with the
// decorators of observability (KRN-09).
//
// What this package does not contain: the shape of the REST API — resources,
// pagination, versioning — which ADR-024 leaves outside the kernel.
package http
