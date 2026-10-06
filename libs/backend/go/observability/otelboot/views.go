package otelboot

import (
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
	"go.opentelemetry.io/otel/semconv/v1.43.0/otelconv"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
)

var sdkDurationBoundaries = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// NewMetricView filters every instrument (RF-D3): a catalogue series keeps only
// its declared labels, any other loses the keys of RF-B3, and an HTTP server
// series also loses server.address and server.port, taken from the client's Host.
func NewMetricView() sdkmetric.View {
	filters := make(map[string]attribute.Filter, len(metrics.Catalog()))
	for _, series := range metrics.Catalog() {
		keys := make([]attribute.Key, len(series.Labels))
		for i, label := range series.Labels {
			keys[i] = attribute.Key(label)
		}
		filters[series.Name] = attribute.NewAllowKeysFilter(keys...)
	}
	rfB3 := []attribute.Key{
		semconv.ClientAddressKey, semconv.NetworkPeerAddressKey, semconv.NetworkPeerPortKey,
		semconv.UserAgentOriginalKey, semconv.URLFullKey, semconv.URLPathKey,
		semconv.ExceptionMessageKey, semconv.ExceptionStacktraceKey,
	}
	forbidden := attribute.NewDenyKeysFilter(rfB3...)
	edge := attribute.NewDenyKeysFilter(append(rfB3, semconv.ServerAddressKey, semconv.ServerPortKey)...)
	for _, name := range []string{
		httpconv.ServerRequestDuration{}.Name(),
		httpconv.ServerRequestBodySize{}.Name(),
		httpconv.ServerResponseBodySize{}.Name(),
	} {
		filters[name] = edge
	}

	sdkDurations := map[string]bool{
		otelconv.SDKExporterOperationDuration{}.Name():      true,
		otelconv.SDKMetricReaderCollectionDuration{}.Name(): true,
	}

	return func(instrument sdkmetric.Instrument) (sdkmetric.Stream, bool) {
		filter, known := filters[instrument.Name]
		if !known {
			filter = forbidden
		}
		stream := sdkmetric.Stream{
			Name:            instrument.Name,
			Description:     instrument.Description,
			Unit:            instrument.Unit,
			AttributeFilter: filter,
		}
		if sdkDurations[instrument.Name] {
			stream.Aggregation = sdkmetric.AggregationExplicitBucketHistogram{Boundaries: sdkDurationBoundaries}
		}
		return stream, true
	}
}
