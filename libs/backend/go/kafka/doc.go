// comment-discipline-ok-file: godoc de package; o que o bloco provider é e não é vem da matriz de blocos e da política de capabilities (RFC §6.2, §7.3), por exigência da spec do KRN-10.

// Package kafka realizes the target asynchronous transport of the domain
// event over Kafka (FND-06 §11, ADR-025): the record key is the envelope's
// partition key and the value is the envelope byte for byte (KFK-05, TRP-13);
// the ACK is the commit of a contiguous offset after the local commit (TRP-26,
// TRP-29); retry is inline and bounded (KFK-10); and the dead-letter queue is
// published before the offset advances (KFK-12, TRP-30).
//
// It is a provider-block unit over franz-go. It realizes the ports of
// ports — Acknowledger and Containment — and the structural shape of the
// relay's Publisher, and never imports app or application.
//
// What this package does not contain: topic provisioning (TRP-41), a schema
// registry (KFK-13 to KFK-18) and the persisted binding of TRP-09/TRP-46.
package kafka
