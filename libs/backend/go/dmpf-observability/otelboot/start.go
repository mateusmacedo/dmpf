package otelboot

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	dmpfobservability "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
)

// ErrAlreadyStarted is a second bootstrap in a process that already has one.
// The propagator and the providers are process-wide, so two runtimes would
// fight over them.
var ErrAlreadyStarted = errors.New("otelboot: the OpenTelemetry runtime is already started")

// instrumentationName names this module as the instrumentation scope, so a
// reader of the telemetry knows which library produced it.
const instrumentationName = "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability"

// running guards the process against a second bootstrap. Shutdown releases it,
// so a test — or a service that restarts its telemetry — can start again.
var running atomic.Bool

// Runtime is what the composition root holds: the tracer and meter of the
// service, the platform instruments, the logger and the single shutdown that
// closes the pipeline in order.
type Runtime struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	processor      *ClassAwareProcessor
	instruments    *metrics.Instruments
	logger         *slog.Logger

	shutdownOnce sync.Once
	shutdownErr  error
}

// Start boots the SDK. It is the only place that touches the global
// propagator and the global providers, so a service that never calls Start
// leaves the process as it found it — including when the configuration is
// refused.
func Start(ctx context.Context, config Config) (*Runtime, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if config.TraceExporter == nil {
		return nil, ErrExporterRequired
	}
	if !running.CompareAndSwap(false, true) {
		return nil, ErrAlreadyStarted
	}

	runtime, err := build(config)
	if err != nil {
		running.Store(false)
		return nil, err
	}

	otel.SetTextMapPropagator(config.Propagator)
	otel.SetTracerProvider(runtime.tracerProvider)
	otel.SetMeterProvider(runtime.meterProvider)

	runtime.announce(ctx, config.Sheets)
	return runtime, nil
}

func build(config Config) (*Runtime, error) {
	resource := sdkresource.NewWithAttributes(semconv.SchemaURL, config.ResourceAttributes()...)

	meterOptions := []sdkmetric.Option{sdkmetric.WithResource(resource)}
	if config.MetricReader != nil {
		meterOptions = append(meterOptions, sdkmetric.WithReader(config.MetricReader))
	}
	meterProvider := sdkmetric.NewMeterProvider(meterOptions...)

	meter := meterProvider.Meter(instrumentationName,
		metric.WithInstrumentationVersion(dmpfobservability.OTelVersion))
	instruments, err := metrics.New(meter)
	if err != nil {
		return nil, err
	}

	processor, err := NewClassAwareProcessor(config.TraceExporter, ProcessorOptions{
		Dropped: instruments.SpansDropped,
	})
	if err != nil {
		return nil, err
	}

	logger := config.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	return &Runtime{
		tracerProvider: sdktrace.NewTracerProvider(
			sdktrace.WithResource(resource),
			sdktrace.WithSampler(NewClassSampler(config.EffectiveSampling())),
			sdktrace.WithSpanProcessor(processor),
		),
		meterProvider: meterProvider,
		processor:     processor,
		instruments:   instruments,
		logger:        logger,
	}, nil
}

// announce records the effective sheets once, so an operator reading the
// start-up of the service sees the policy it is running under (RES-40).
func (r *Runtime) announce(ctx context.Context, sheets []resilience.Sheet) {
	for _, sheet := range sheets {
		attributes := make([]slog.Attr, 0, 11)
		attributes = append(attributes, slog.String("dependency", sheet.Dependency))
		for field, value := range sheet.Effective() {
			attributes = append(attributes, slog.String(field, value))
		}
		r.logger.LogAttrs(ctx, slog.LevelInfo, "resilience sheet in effect", attributes...)
	}
}

// Tracer is the tracer of the service.
func (r *Runtime) Tracer() trace.Tracer {
	return r.tracerProvider.Tracer(instrumentationName,
		trace.WithInstrumentationVersion(dmpfobservability.OTelVersion))
}

// Meter is the meter of the service.
func (r *Runtime) Meter() metric.Meter {
	return r.meterProvider.Meter(instrumentationName,
		metric.WithInstrumentationVersion(dmpfobservability.OTelVersion))
}

// Instruments is the platform catalogue, built once at boot.
func (r *Runtime) Instruments() *metrics.Instruments { return r.instruments }

// Logger is the logger the runtime was given. It is never nil, so a caller does
// not guard every record.
func (r *Runtime) Logger() *slog.Logger { return r.logger }

// ForceFlush exports what the processor still holds.
func (r *Runtime) ForceFlush(ctx context.Context) error { return r.processor.ForceFlush(ctx) }

// Shutdown closes the trace pipeline before the metric one and releases the
// process for a later Start. It is idempotent and reports the same result on
// every call.
func (r *Runtime) Shutdown(ctx context.Context) error {
	r.shutdownOnce.Do(func() {
		r.shutdownErr = errors.Join(
			r.tracerProvider.Shutdown(ctx),
			r.meterProvider.Shutdown(ctx),
		)
		running.Store(false)
	})
	return r.shutdownErr
}
