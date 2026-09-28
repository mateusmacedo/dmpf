package otlp

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/credentials"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

// ErrEndpointRequired is a transport with nowhere to send. The address of the
// collector is configuration of the deployment, never a literal in the code.
var ErrEndpointRequired = errors.New("otlp: the transport declares no endpoint")

// TraceExporter builds the OTLP/gRPC span exporter of the configuration. The
// caller hands it to otelboot.Config.TraceExporter, so the single processor
// stays the only owner of it.
func TraceExporter(ctx context.Context, config otelboot.Config) (sdktrace.SpanExporter, error) {
	if err := check(config); err != nil {
		return nil, err
	}

	options := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(config.Transport.Endpoint)}
	switch {
	case config.Transport.Insecure:
		options = append(options, otlptracegrpc.WithInsecure())
	case config.Transport.TLS != nil:
		options = append(options, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(config.Transport.TLS)))
	}

	return otlptracegrpc.New(ctx, options...)
}

// MetricReader builds the OTLP/gRPC metric exporter and the periodic reader
// that drives it, which is what otelboot.Config.MetricReader takes.
func MetricReader(ctx context.Context, config otelboot.Config, options ...sdkmetric.PeriodicReaderOption) (sdkmetric.Reader, error) {
	if err := check(config); err != nil {
		return nil, err
	}

	grpcOptions := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(config.Transport.Endpoint)}
	switch {
	case config.Transport.Insecure:
		grpcOptions = append(grpcOptions, otlpmetricgrpc.WithInsecure())
	case config.Transport.TLS != nil:
		grpcOptions = append(grpcOptions, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(config.Transport.TLS)))
	}

	exporter, err := otlpmetricgrpc.New(ctx, grpcOptions...)
	if err != nil {
		return nil, err
	}
	return sdkmetric.NewPeriodicReader(exporter, options...), nil
}

// LogExporter builds the OTLP/gRPC log exporter, which otelboot.NewLoggerProvider
// takes.
func LogExporter(ctx context.Context, config otelboot.Config) (sdklog.Exporter, error) {
	if err := check(config); err != nil {
		return nil, err
	}

	options := []otlploggrpc.Option{otlploggrpc.WithEndpoint(config.Transport.Endpoint)}
	switch {
	case config.Transport.Insecure:
		options = append(options, otlploggrpc.WithInsecure())
	case config.Transport.TLS != nil:
		options = append(options, otlploggrpc.WithTLSCredentials(credentials.NewTLS(config.Transport.TLS)))
	}

	return otlploggrpc.New(ctx, options...)
}

// check applies the transport rule of the bootstrap rather than restating it:
// plaintext telemetry stays a declared choice on both signals.
func check(config otelboot.Config) error {
	if config.Transport.Endpoint == "" {
		return ErrEndpointRequired
	}
	return config.ValidateTransport()
}
