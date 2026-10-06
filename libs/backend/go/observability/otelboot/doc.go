// Package otelboot boots the OpenTelemetry SDK for the platform: the W3C
// propagator, the resource, the per-class sampler and the batch processor of
// the SDK, behind one Start that returns a Runtime to shut down.
//
// The sampler never drops a span, and the processor exports the sampled ones:
// TRC-14 is kept by the tail sampling of the Collector (RF-E7).
package otelboot
