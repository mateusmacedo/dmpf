package relay

import (
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Telemetry is what the relay takes from the runtime of the process. It is
// declared here so the production closure of app never imports the OTel SDK;
// *otelboot.Runtime satisfies it.
type Telemetry interface {
	Tracer() trace.Tracer
	MeterProvider() metric.MeterProvider
	LoggerProvider() log.LoggerProvider
}

// Instrument binds c to the telemetry of the runtime and to the address of each
// destination. System stays as the context declared it, so the relay remains
// agnostic of the transport.
func Instrument(c Config, rt Telemetry, address func(destination string) string) Config {
	c.Tracer = rt.Tracer()
	c.MeterProvider = rt.MeterProvider()
	c.LoggerProvider = rt.LoggerProvider()
	c.Address = address
	return c
}
