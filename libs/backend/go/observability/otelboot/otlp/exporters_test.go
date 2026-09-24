package otlp_test

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/propagation"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot/otlp"
)

func transportConfig(transport otelboot.Transport, allowInsecure bool) otelboot.Config {
	return otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{
			ServiceName:       "orders",
			ServiceVersion:    "1.4.2",
			ServiceInstanceID: "orders-7c9f",
		},
		Transport:     transport,
		AllowInsecure: allowInsecure,
	}
}

func TestAnExporterWithoutAnEndpointIsRefused(t *testing.T) {
	config := transportConfig(otelboot.Transport{Insecure: true}, true)

	if _, err := otlp.TraceExporter(context.Background(), config); !errors.Is(err, otlp.ErrEndpointRequired) {
		t.Errorf("TraceExporter() = %v, want ErrEndpointRequired", err)
	}
	if _, err := otlp.MetricReader(context.Background(), config); !errors.Is(err, otlp.ErrEndpointRequired) {
		t.Errorf("MetricReader() = %v, want ErrEndpointRequired", err)
	}
}

func TestAnInsecureExporterIsRefusedUnlessItWasAllowed(t *testing.T) {
	config := transportConfig(otelboot.Transport{Endpoint: "collector:4317", Insecure: true}, false)

	if _, err := otlp.TraceExporter(context.Background(), config); !errors.Is(err, otelboot.ErrInsecureNotAllowed) {
		t.Errorf("TraceExporter() = %v, want ErrInsecureNotAllowed", err)
	}
	if _, err := otlp.MetricReader(context.Background(), config); !errors.Is(err, otelboot.ErrInsecureNotAllowed) {
		t.Errorf("MetricReader() = %v, want ErrInsecureNotAllowed", err)
	}
}

func TestTheExportersAreBuiltForADeclaredTransport(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		transport     otelboot.Transport
		allowInsecure bool
	}{
		{"insecure, declared", otelboot.Transport{Endpoint: "collector:4317", Insecure: true}, true},
		{"tls", otelboot.Transport{Endpoint: "collector:4317", TLS: &tls.Config{MinVersion: tls.VersionTLS13}}, false},
		{"tls of the system", otelboot.Transport{Endpoint: "collector:4317"}, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			config := transportConfig(testCase.transport, testCase.allowInsecure)

			exporter, err := otlp.TraceExporter(context.Background(), config)
			if err != nil {
				t.Fatalf("TraceExporter() = %v", err)
			}
			t.Cleanup(func() { _ = exporter.Shutdown(context.Background()) })

			reader, err := otlp.MetricReader(context.Background(), config)
			if err != nil {
				t.Fatalf("MetricReader() = %v", err)
			}
			t.Cleanup(func() { _ = reader.Shutdown(context.Background()) })
		})
	}
}
