package otlp

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/credentials"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/otelboot"
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

// check applies the transport rule of the bootstrap rather than restating it:
// plaintext telemetry stays a declared choice on both signals.
func check(config otelboot.Config) error {
	if config.Transport.Endpoint == "" {
		return ErrEndpointRequired
	}
	return config.ValidateTransport()
}
