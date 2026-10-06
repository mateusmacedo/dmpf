package otelboot

import (
	"errors"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var ErrExporterRequired = errors.New("otelboot: the processor requires a span exporter")

// newSpanProcessor is the batch processor of the SDK, tuned by OTEL_BSP_*
// (RF-E6). It exports only the sampled spans: TRC-14 is kept by the tail
// sampling of the Collector (RF-E7).
func newSpanProcessor(exporter sdktrace.SpanExporter) sdktrace.SpanProcessor {
	return sdktrace.NewBatchSpanProcessor(exporter)
}
