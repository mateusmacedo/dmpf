// comment-discipline-ok-file: godoc de package; o que o bloco provider é e não é vem da matriz de blocos e da política de capabilities (RFC §6.2, §7.3), por exigência da spec do KRN-10.

// Package dmpfgrpc realizes the internal synchronous transport of the DMPF
// kernel over gRPC (FND-06 §10, ADR-024): a deadline on every outgoing call,
// propagated as remaining duration and never restarted (GRP-04 to GRP-07,
// GRP-16 to GRP-18); retry only for methods declared idempotent (GRP-08 to
// GRP-11); TLS mandatory outside tests (GRP-15); health per service; and the
// admission of dmpf-observability at the server boundary.
//
// It is a provider-block unit. It composes dmpf-transport/deadline with the
// decorators of dmpf-observability (KRN-09) and never imports dmpf-app or
// dmpf-application (cells 26 and 27 of RFC §7.3).
//
// What this package does not contain: Connect or transcoding, the REST
// surface (ADR-024 leaves it out), and any service definition of its own —
// the health service is the only protocol it ships.
package dmpfgrpc
