// Package otlp builds the OTLP/gRPC exporters that carry spans and metrics to
// the OpenTelemetry Collector, which is the platform's destination (ADR-035).
// It is separate from otelboot so the bootstrap stays testable without a
// network.
package otlp
