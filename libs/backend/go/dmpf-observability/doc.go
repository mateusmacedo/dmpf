// Package dmpfobservability declares the provider block that realizes resilience
// and observability for the DMPF kernel: OpenTelemetry bootstrap, the retry
// evaluator, the resilience decorators, the metric catalogue, tracing, logging,
// audit, redaction and the realization of the application's instrumentation
// hook.
//
// It never imports dmpf-application: the block matrix forbids provider →
// application as much as application → provider, so the hook is satisfied
// structurally and the composition root binds the two. Being a provider, its
// capability budget is unrestricted, which is exactly why the blocks above it
// stay pure.
package dmpfobservability
