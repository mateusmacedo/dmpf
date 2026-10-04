package otelboot

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/contrib/processors/baggagecopy"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

// ErrAlreadyStarted is a second bootstrap in a process that already has one.
// The propagator and the providers are process-wide, so two runtimes would
// fight over them.
var ErrAlreadyStarted = errors.New("otelboot: the OpenTelemetry runtime is already started")

// instrumentationName names this module as the instrumentation scope, so a
// reader of the telemetry knows which library produced it.
const instrumentationName = "github.com/mateusmacedo/dmpf/libs/backend/go/observability"

// running guards the process against a second bootstrap. Shutdown releases it,
// so a test — or a service that restarts its telemetry — can start again.
var running atomic.Bool

// Runtime is what the composition root holds: the tracer and meter of the
// service, the platform instruments, the logger provider and the single
// shutdown that closes the pipeline in order.
type Runtime struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	logs           log.LoggerProvider
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
	if config.TraceExporter == nil && !config.Disabled {
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

	runtime.announce(ctx, config.Sheets)
	return runtime, nil
}

// resourceOf keeps what was detected even when the environment is malformed:
// Validate refuses that configuration before a provider is built from it.
func resourceOf(config Config) *sdkresource.Resource {
	resource, _ := config.resource()
	return resource
}

// NewLoggerProvider batches the log records of the process to exporter, under
// the same resource as its traces and metrics. It is built before Start
// because the logger of the process exists before the pipelines do.
func NewLoggerProvider(config Config, exporter sdklog.Exporter) *sdklog.LoggerProvider {
	return sdklog.NewLoggerProvider(
		sdklog.WithResource(resourceOf(config)),
		logAttributeValueLengthLimit(),
		sdklog.WithProcessor(baggagecopy.NewLogProcessor(executionMembers)),
		sdklog.WithProcessor(NewLogProcessor(sdklog.NewBatchProcessor(exporter), LogPolicy{Class: config.Class, Rand: config.Rand})),
	)
}

func build(config Config) (*Runtime, error) {
	if config.Disabled {
		return disabled(config)
	}
	resource := resourceOf(config)

	meterOptions := []sdkmetric.Option{sdkmetric.WithResource(resource), sdkmetric.WithView(NewMetricView())}
	if config.MetricReader != nil {
		meterOptions = append(meterOptions, sdkmetric.WithReader(config.MetricReader))
	}
	meterProvider := sdkmetric.NewMeterProvider(meterOptions...)

	// Anything that fails from here on has to put the meter provider down. A
	// periodic reader — which is what the OTLP metric pipeline is — runs a
	// collection goroutine of its own (sdk/metric/periodic_reader.go:135), and a
	// build that just returned an error would leave it collecting for the life
	// of the process, against a runtime the caller never received.
	abort := func(cause error) (*Runtime, error) {
		return nil, errors.Join(cause, meterProvider.Shutdown(context.Background()))
	}

	meter := meterProvider.Meter(instrumentationName,
		metric.WithInstrumentationVersion(observability.OTelVersion))
	instruments, err := metrics.New(meter)
	if err != nil {
		return abort(err)
	}

	// WHY: the SDK binds its own observability instruments to the global meter
	// provider when the batch processor is built (sdk/trace/internal/observ/
	// batch_span_processor.go:55), so the provider has to be global by then.
	otel.SetMeterProvider(meterProvider)
	processor := newSpanProcessor(NewPrivacyExporter(config.TraceExporter))

	logger := config.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	var logs log.LoggerProvider = handlerProvider{handler: logger.Handler()}
	if config.LoggerProvider != nil {
		logs = Leveled(config.LoggerProvider, config.LogLevel)
	}

	return &Runtime{
		tracerProvider: sdktrace.NewTracerProvider(
			sdktrace.WithResource(resource),
			spanLimits(),
			sdktrace.WithSampler(NewClassSampler(config.EffectiveSampling())),
			sdktrace.WithSpanProcessor(baggagecopy.NewSpanProcessor(executionMembers)),
			sdktrace.WithSpanProcessor(processor),
		),
		meterProvider:  meterProvider,
		loggerProvider: config.LoggerProvider,
		logs:           logs,
		instruments:    instruments,
		logger:         logger,
	}, nil
}

func disabled(config Config) (*Runtime, error) {
	meterProvider := sdkmetric.NewMeterProvider()
	instruments, err := metrics.New(meterProvider.Meter(instrumentationName))
	if err != nil {
		return nil, errors.Join(err, meterProvider.Shutdown(context.Background()))
	}
	otel.SetMeterProvider(meterProvider)
	logger := config.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Runtime{
		tracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample())),
		meterProvider:  meterProvider,
		logs:           handlerProvider{handler: logger.Handler()},
		instruments:    instruments,
		logger:         logger,
	}, nil
}

// announce records the effective sheets once, so an operator reading the
// start-up of the service sees the policy it is running under (RES-40).
func (r *Runtime) announce(ctx context.Context, sheets []resilience.Sheet) {
	for _, sheet := range sheets {
		attributes := make([]slog.Attr, 0, 11)
		attributes = append(attributes, slog.String(tracing.KeyDependency, sheet.Dependency))
		for field, value := range sheet.Effective() {
			attributes = append(attributes, slog.String(SheetAttributePrefix+field, value))
		}
		r.LoggerFor(reflect.TypeFor[Runtime]().PkgPath()).LogAttrs(ctx, slog.LevelInfo, "resilience sheet in effect", attributes...)
	}
}

// Tracer is the tracer of the service.
func (r *Runtime) Tracer() trace.Tracer {
	return r.tracerProvider.Tracer(instrumentationName,
		trace.WithInstrumentationVersion(observability.OTelVersion))
}

// Meter is the meter of the service.
func (r *Runtime) Meter() metric.Meter {
	return r.meterProvider.Meter(instrumentationName,
		metric.WithInstrumentationVersion(observability.OTelVersion))
}

func (r *Runtime) MeterProvider() metric.MeterProvider { return r.meterProvider }

// Instruments is the platform catalogue, built once at boot.
func (r *Runtime) Instruments() *metrics.Instruments { return r.instruments }

func (r *Runtime) LoggerFor(scope string) *slog.Logger {
	if r.loggerProvider == nil {
		return r.logger
	}
	return logging.NewLogger(r.logs, scope)
}

func (r *Runtime) LoggerProvider() log.LoggerProvider { return r.logs }

func (r *Runtime) ForceFlush(ctx context.Context) error {
	err := errors.Join(r.tracerProvider.ForceFlush(ctx), r.meterProvider.ForceFlush(ctx))
	if r.loggerProvider != nil {
		err = errors.Join(err, r.loggerProvider.ForceFlush(ctx))
	}
	return err
}

// Shutdown closes the trace, metric and log pipelines in turn, each within its
// share of what is left of the deadline of ctx, and releases the process for a
// later Start. It is idempotent and reports the same result on every call.
func (r *Runtime) Shutdown(ctx context.Context) error {
	return r.shutdown(ctx, func(error) {})
}

// WHY: report runs before the log pipeline closes, since sdklog hands out a noop
// Logger once its Shutdown starts (sdk/log@v1.47.0/provider.go:126,136-137).
func (r *Runtime) shutdown(ctx context.Context, report func(error)) error {
	r.shutdownOnce.Do(func() {
		r.shutdownErr = errors.Join(
			withinShare(ctx, 3, r.tracerProvider.Shutdown),
			withinShare(ctx, 2, r.meterProvider.Shutdown),
		)
		if r.shutdownErr != nil {
			report(r.shutdownErr)
		}
		if r.loggerProvider != nil {
			r.shutdownErr = errors.Join(r.shutdownErr, r.loggerProvider.Shutdown(ctx))
		}
		running.Store(false)
	})
	return r.shutdownErr
}

// WHY: sdklog returns ctx.Err() without closing its processors once the context
// is spent (sdk/log@v1.47.0/provider.go:214,263-264), so a stalled export of one
// pipeline must not spend the window of the ones closed after it.
func withinShare(ctx context.Context, pipelinesLeft int, stop func(context.Context) error) error {
	deadline, bounded := ctx.Deadline()
	if !bounded {
		return stop(ctx)
	}
	share, cancel := context.WithDeadline(ctx, time.Now().Add(time.Until(deadline)/time.Duration(pipelinesLeft)))
	defer cancel()
	return stop(share)
}
