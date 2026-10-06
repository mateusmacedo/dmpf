// Package boot assembles the telemetry of a process: the logger, the SDK error
// handler and the pipelines, in the order a composition root needs them.
//
// It is a package of its own because it reaches the exporters and the
// bootstrap at once, and otelboot stays testable without a network.
package boot

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"os"
	"reflect"
	"strings"
	"sync"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

const EnvSDKDisabled = "OTEL_SDK_DISABLED"

const keySamplerDeclared = "dmpf.sampler.declared"

// Telemetry is what a process declares about itself before the pipelines
// start.
type Telemetry struct {
	Service  string
	Version  string
	Instance string
	Role     string
	Class    tracing.Class
	Settings []slog.Attr

	Signals Signals
}

func StartTelemetry(ctx context.Context, t Telemetry) (*otelboot.Runtime, error) {
	muteLibraries()
	held := holdErrors()
	config := otelboot.Config{
		Disabled:   strings.EqualFold(os.Getenv(EnvSDKDisabled), "true"),
		Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{
			ServiceName:       t.Service,
			ServiceVersion:    t.Version,
			ServiceInstanceID: t.Instance,
			Role:              t.Role,
		},
		Sampling: t.Signals.Sampling,
		Class:    t.Class,
		Rand:     rand.Float64,
	}

	var logger *slog.Logger
	if !config.Disabled {
		logs, err := logExporter(ctx)
		if err != nil {
			return nil, err
		}
		if logs != nil {
			config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)
			config.LogLevel = t.Signals.Level
			leveled := otelboot.Leveled(config.LoggerProvider, t.Signals.Level)
			logger = logging.NewLogger(leveled, reflect.TypeFor[Telemetry]().PkgPath())
			hearLibraries(leveled, config.LoggerProvider.ForceFlush)
		}
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
		config.Logger = logger
	}

	// The SDK's default handler is a bare log.Print (otel@v1.47.0/internal/errorhandler/errorhandler.go:41);
	// routing it through the logger keeps one record shape in Loki.
	held.handOver(otel.ErrorHandlerFunc(func(err error) {
		if isSamplerVariableError(err) {
			return
		}
		logger.WarnContext(ctx, "telemetry export failed", redact.Error(err))
	}))

	if sampler, declared := declaredSampler(t.Signals.IgnoredSampler); declared {
		logger.WarnContext(ctx, "OTEL_TRACES_SAMPLER is ignored: the platform sampler decides the head",
			slog.String(keySamplerDeclared, sampler))
	}
	if config.Disabled {
		logger.WarnContext(ctx, "telemetry disabled: OTEL_SDK_DISABLED is set")
		return otelboot.Start(ctx, config)
	}
	warnOfAnExportInThePlain(ctx, logger)

	shutdownLogs := func() {
		if config.LoggerProvider != nil {
			_ = config.LoggerProvider.Shutdown(ctx)
		}
	}
	exporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		shutdownLogs()
		return nil, err
	}
	reader, err := metricReader(ctx)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		shutdownLogs()
		return nil, err
	}
	config.TraceExporter, config.MetricReader = exporter, reader
	runtime, err := otelboot.Start(ctx, config)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		_ = reader.Shutdown(ctx)
		shutdownLogs()
		return nil, err
	}
	if err := StartRuntimeMetrics(runtime.MeterProvider()); err != nil {
		_ = runtime.Shutdown(ctx)
		return nil, err
	}
	return runtime, nil
}

func metricReader(ctx context.Context) (sdkmetric.Reader, error) {
	registerRuntimeProducer()
	return autoexport.NewMetricReader(ctx)
}

// logExporter answers nil for none: the records are then discarded.
func logExporter(ctx context.Context) (sdklog.Exporter, error) {
	exporter, err := autoexport.NewLogExporter(ctx)
	if err != nil || autoexport.IsNoneLogExporter(exporter) {
		return nil, err
	}
	return exporter, nil
}

var plaintextExportWarning = new(sync.Once)

func warnOfAnExportInThePlain(ctx context.Context, logger *slog.Logger) {
	if !exportsInThePlain() {
		return
	}
	plaintextExportWarning.Do(func() {
		logger.WarnContext(ctx, "telemetry: OTLP export without TLS by explicit development-only opt-out (http:// endpoint)")
	})
}

var otlpSignals = [...]struct{ exporter, endpoint string }{
	{"OTEL_TRACES_EXPORTER", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"},
	{"OTEL_METRICS_EXPORTER", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"},
	{"OTEL_LOGS_EXPORTER", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"},
}

// exportsInThePlain reads the variables as the exporters do: an undeclared
// exporter is otlp (autoexport@v0.72.0/signal.go:31-36), and the endpoint of a
// signal wins over the general one (otlptracegrpc@v1.47.0/internal/otlpconfig/envconfig.go:50,65).
func exportsInThePlain() bool {
	for _, signal := range otlpSignals {
		if exporter := os.Getenv(signal.exporter); exporter != "" && exporter != "otlp" {
			continue
		}
		endpoint := os.Getenv(signal.endpoint)
		if endpoint == "" {
			endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		}
		if strings.HasPrefix(strings.ToLower(endpoint), "http://") {
			return true
		}
	}
	return false
}
