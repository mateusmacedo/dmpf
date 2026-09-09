// comment-discipline-ok-file: godoc de package; o que o bloco app é e não é vem da matriz de blocos (RFC §7.3), por exigência da spec do KRN-07.

// Package dmpfapp realizes the app block of the DMPF kernel: the consumer
// adapter that stands between a transport delivery and the application
// service of consumption (FND-04 §6.3, steps 1, 2 and 7).
//
// It is the only block whose row in the block matrix is fully permissive
// (RFC §7.3): it imports the contract to decode the envelope and hash the
// payload, the application to invoke the use case, and the provider to bind
// the transactional ports. application → contract (cell 12) and
// provider → application (cell 26) are forbidden, which is why this adapter
// lives in its own module instead of either of them.
//
// What this block does not contain: the concrete ACK, nack or offset commit
// of a transport, and the dead-letter queue, both KRN-10; and every
// operational value — wait ceiling, attempt limit, retention — which the
// caller declares and FND-08 catalogues. The relay of the outbox (KRN-08)
// lives in the relay package of this module, and the composition root that
// wires both into real processes is apps/backend/dmpf-reference (KRN-12).
package dmpfapp
