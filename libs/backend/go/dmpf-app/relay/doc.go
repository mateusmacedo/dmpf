// comment-discipline-ok-file: godoc de package; a fronteira entre o laço e a tecnologia é normativa (RFC §7.5, FND-04 §5.4), como no doc.go do módulo.

// Package relay drains the transactional outbox: it claims eligible records
// under a lease, publishes them outside any database transaction, and
// transitions each record only while the claim is still its own
// (FND-04 §5.4, OBX-07, OBX-10).
//
// The split with the provider is the one RFC §7.5 draws. Everything that is
// technology — the table, SKIP LOCKED, the arithmetic applied to columns —
// stays in the provider; everything that is process — the loop, concurrency,
// the lifecycle and the decision to give up — lives here.
//
// What this package does not contain: the concrete transport behind Publisher,
// which is KRN-10; OpenTelemetry instrumentation, which is KRN-09; and every
// operational value — scan interval, batch size, lease, attempt ceiling,
// concurrency limit — which the caller declares and FND-08 catalogues.
package relay
