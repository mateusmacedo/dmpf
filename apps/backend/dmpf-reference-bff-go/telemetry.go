package dmpfreferencebff

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/otelboot/otlp"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/tracing"

	"github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference-bff-go/api"
	"github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference-bff-go/rpc"
)

// NewTelemetry boots the one OpenTelemetry runtime of the process; without
// DMPF_OTLP_ENDPOINT the pipeline stays in memory, the development mode.
func NewTelemetry(ctx context.Context, cfg Config, out io.Writer) (*otelboot.Runtime, error) {
	logger := slog.New(logging.NewHandler(out, logging.Config{
		Service:  cfg.Service,
		Version:  cfg.Version,
		Instance: cfg.Instance,
		Class:    tracing.ClassWrite,
		Fields:   requestFields,
	}))

	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.WarnContext(ctx, "telemetry export failed", "error", err.Error())
	}))

	config := otelboot.Config{
		Propagator: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
		Resource: otelboot.Resource{
			ServiceName:       cfg.Service,
			ServiceVersion:    cfg.Version,
			ServiceInstanceID: cfg.Instance,
		},
		Logger: logger,
	}

	if cfg.OTLPEndpoint == "" {
		logger.WarnContext(ctx, "telemetry kept in memory (development mode): DMPF_OTLP_ENDPOINT is unset")
		config.TraceExporter = tracetest.NewInMemoryExporter()
		config.MetricReader = sdkmetric.NewManualReader()
		return otelboot.Start(ctx, config)
	}

	if cfg.OTLPInsecure {
		logger.WarnContext(ctx, "telemetry exported without TLS: DMPF_OTLP_INSECURE is set (development and CI only)", "endpoint", cfg.OTLPEndpoint)
	}
	config.Transport = otelboot.Transport{Endpoint: cfg.OTLPEndpoint, Insecure: cfg.OTLPInsecure}
	config.AllowInsecure = cfg.OTLPInsecure
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

func requestFields(ctx context.Context) logging.Fields {
	fields := logging.Fields{logging.KeyTenantID: api.Tenant}
	if call, ok := rpc.CallFrom(ctx); ok {
		fields[logging.KeyCorrelationID] = call.CorrelationID
		fields[logging.KeyRequestID] = call.RequestID
	}
	return fields
}
