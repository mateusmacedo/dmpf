// comment-discipline-ok-file: godoc de package; o que o bloco provider é e não é vem da matriz de blocos e da política de capabilities (RFC §6.2, §7.3), por exigência da spec do KRN-10.

// Package sqs realizes the normalized asynchronous transport over SNS and
// SQS (FND-06 §12, ADR-025): the envelope travels in the textual body encoded
// exactly once (TRP-19); FIFO derives the message group from the partition key
// and standard promises no order (SQS-04, SQS-05); the delete follows the local
// commit by receipt handle (SQS-09); visibility is extended explicitly under the
// twelve-hour ceiling (SQS-08, SQS-08b); and the SNS → SQS hop requires raw
// message delivery.
//
// It is a provider-block unit over aws-sdk-go-v2. It realizes the ports of
// ports — Acknowledger and Containment — and the structural shape of the
// relay's Publisher, and never imports app or application.
//
// What this package does not contain: queue and topic provisioning, and the
// persisted binding of TRP-09/TRP-46.
package sqs
