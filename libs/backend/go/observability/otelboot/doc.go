// Package otelboot boots the OpenTelemetry SDK for the platform: the W3C
// propagator, the resource, the per-class sampler and the single processor that
// owns the exporter, behind one Start that returns a Runtime to shut down.
//
// The sampler never drops a span and the processor exports every span that is
// sampled or carries an error status, which is the in-process equivalent of
// TRC-14 the factory samplers cannot give.
package otelboot
