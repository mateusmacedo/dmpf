// Package boot assembles the telemetry of a process: the logger, the SDK error
// handler and the pipelines, in the order a composition root needs them.
//
// It is a package of its own because otelboot/otlp imports otelboot, and
// otelboot imports the root package, so neither of them can reach the
// exporters and the log handler at once.
package boot

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot/otlp"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

// Telemetry is what a process declares about itself before the pipelines
// start. Fields extracts the correlation of the call, and stays a parameter
// because each edge reads it from a different carrier.
type Telemetry struct {
	Service  string
	Version  string
	Instance string
	Endpoint string
	Insecure bool
	Class    tracing.Class
	Fields   func(ctx context.Context) logging.Fields
}

// StartTelemetry builds the logger, routes the SDK's own errors through it and
// starts the pipelines. Without an endpoint the telemetry is kept in memory,
// which is what makes a process runnable in development without a collector.
func StartTelemetry(ctx context.Context, out io.Writer, t Telemetry) (*otelboot.Runtime, error) {
	logger := slog.New(logging.NewHandler(out, logging.Config{
		Service:  t.Service,
		Version:  t.Version,
		Instance: t.Instance,
		Class:    t.Class,
		Fields:   t.Fields,
	}))

	// The SDK's default error handler writes a bare line through package log;
	// routing it through the platform handler keeps one record shape in Loki.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.WarnContext(ctx, "telemetry export failed", "error", err.Error())
	}))

	config := otelboot.Config{
		Propagator: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
		Resource: otelboot.Resource{
			ServiceName:       t.Service,
			ServiceVersion:    t.Version,
			ServiceInstanceID: t.Instance,
		},
		Logger: logger,
	}

	if t.Endpoint == "" {
		logger.WarnContext(ctx, "telemetry kept in memory (development mode): OTLP_ENDPOINT is unset")
		config.TraceExporter = tracetest.NewInMemoryExporter()
		config.MetricReader = sdkmetric.NewManualReader()
		return otelboot.Start(ctx, config)
	}

	if t.Insecure {
		logger.WarnContext(ctx, "telemetry exported without TLS: OTLP_INSECURE is set (development and CI only)", "endpoint", t.Endpoint)
	}
	config.Transport = otelboot.Transport{Endpoint: t.Endpoint, Insecure: t.Insecure}
	config.AllowInsecure = t.Insecure
	exporter, err := otlp.TraceExporter(ctx, config)
	if err != nil {
		return nil, err
	}
	reader, err := otlp.MetricReader(ctx, config)
	if err != nil {
		// WHY: the exporter above already holds a gRPC connection and its own
		// goroutines; returning without closing it leaks both on every retry.
		_ = exporter.Shutdown(ctx)
		return nil, err
	}
	config.TraceExporter, config.MetricReader = exporter, reader
	return otelboot.Start(ctx, config)
}
