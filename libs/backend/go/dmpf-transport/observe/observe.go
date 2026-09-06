// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-08 (RES-23, TRC-04, TRC-12, MET-08..10, LOG-13) que o símbolo realiza, dentro do limite de 3 linhas.

// Package observe is the three observability positions of RES-22 — tracing,
// metrics, logging — that RES-23 requires of every composition and the
// resilience package leaves to the caller; only the failure category varies.
package observe

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
)

// CategoryOK is the outcome category of a call that returned no error.
const CategoryOK = "ok"

// Config is what the three decorators record through. Service labels the
// series of MET-08 to MET-10; SpanPrefix names the span; Category maps a
// failure to its bounded category and is the only transport-specific part.
type Config struct {
	Service     string
	SpanPrefix  string
	Clock       clock.Clock
	Tracer      trace.Tracer
	Instruments *metrics.Instruments
	Logger      *slog.Logger
	Category    func(error) string
}

func (c Config) tracer() trace.Tracer {
	if c.Tracer == nil {
		return noop.NewTracerProvider().Tracer("dmpf-transport")
	}
	return c.Tracer
}

func (c Config) logger() *slog.Logger {
	if c.Logger == nil {
		return slog.Default()
	}
	return c.Logger
}

func (c Config) category(err error) string {
	if err == nil {
		return CategoryOK
	}
	if c.Category == nil {
		return "unclassified"
	}
	return c.Category(err)
}

// Slots fills the tracing, metrics and logging positions; the caller adds the
// rest of the composition to the returned value.
func Slots(cfg Config) resilience.Slots {
	return resilience.Slots{
		Tracing: Tracing(cfg),
		Metrics: Metrics(cfg),
		Logging: Logging(cfg),
	}
}

// Tracing opens the client span of the call with the closed attribute set of
// TRC-04 and closes it with the outcome category; a failure is a status and a
// category, never a message (TRC-12).
func Tracing(cfg Config) resilience.Decorator {
	tracer := cfg.tracer()
	starts := newCache[[]attribute.KeyValue]()
	outcomes := newCache[[]attribute.KeyValue]()
	return func(next resilience.Call) resilience.Call {
		return func(ctx context.Context, op resilience.Operation, do func(context.Context) error) error {
			attrs := starts.get(cacheKey{op.Dependency, op.Method}, func() []attribute.KeyValue {
				return tracing.Attributes{}.Service(cfg.Service).Dependency(op.Dependency).Operation(op.Method).KeyValues()
			})
			ctx, span := tracer.Start(ctx, cfg.SpanPrefix+op.Method,
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(attrs...))
			defer span.End()

			err := next(ctx, op, do)
			category := cfg.category(err)
			span.SetAttributes(outcomes.get(cacheKey{category, ""}, func() []attribute.KeyValue {
				return tracing.Attributes{}.OutcomeCategory(category).KeyValues()
			})...)
			if err != nil {
				tracing.RecordError(span, category)
			}
			return err
		}
	}
}

// Metrics records the call under the three service series of MET-08 to MET-10
// with the operation as the method name. Without instruments or clock it still
// wraps the call, so the position is never empty (RES-23).
func Metrics(cfg Config) resilience.Decorator {
	measured := newCache[metric.MeasurementOption]()
	failures := newCache[metric.MeasurementOption]()
	return func(next resilience.Call) resilience.Call {
		return func(ctx context.Context, op resilience.Operation, do func(context.Context) error) error {
			if cfg.Instruments == nil || cfg.Clock == nil {
				return next(ctx, op, do)
			}
			started := cfg.Clock.Now()
			err := next(ctx, op, do)
			category := cfg.category(err)

			outcome := measured.get(cacheKey{op.Method, category}, func() metric.MeasurementOption {
				return metric.WithAttributeSet(attribute.NewSet(metrics.Labels{}.Service(cfg.Service).Operation(op.Method).OutcomeCategory(category).Attributes()...))
			})
			cfg.Instruments.RequestDuration.Record(ctx, cfg.Clock.Now().Sub(started).Seconds(), outcome)
			cfg.Instruments.Requests.Add(ctx, 1, outcome)
			if err != nil {
				cfg.Instruments.Errors.Add(ctx, 1, failures.get(cacheKey{op.Method, category}, func() metric.MeasurementOption {
					return metric.WithAttributeSet(attribute.NewSet(metrics.Labels{}.Service(cfg.Service).Operation(op.Method).ErrorCategory(category).Attributes()...))
				}))
			}
			return err
		}
	}
}

// maxCached bounds every attribute cache: the key spaces are bounded by MET-07
// (method, category), so the cap is a guard against a category function that
// is not, and past it the attributes are built per call as before.
const maxCached = 4096

type cacheKey struct{ a, b string }

// cache memoizes the attribute sets of the hot path: building and sorting them
// per call was the dominant allocation of every provider's publish.
type cache[T any] struct {
	entries sync.Map
	size    atomic.Int64
}

func newCache[T any]() *cache[T] { return &cache[T]{} }

func (c *cache[T]) get(key cacheKey, build func() T) T {
	if v, ok := c.entries.Load(key); ok {
		return v.(T)
	}
	v := build()
	if c.size.Load() < maxCached {
		if _, loaded := c.entries.LoadOrStore(key, v); !loaded {
			c.size.Add(1)
		}
	}
	return v
}

// Logging logs a failed call with its category and no message: the message
// would leave the process without redaction (LOG-13).
func Logging(cfg Config) resilience.Decorator {
	logger := cfg.logger()
	return func(next resilience.Call) resilience.Call {
		return func(ctx context.Context, op resilience.Operation, do func(context.Context) error) error {
			err := next(ctx, op, do)
			if err != nil {
				logger.WarnContext(ctx, "dmpf-transport: call failed",
					slog.String("dependency", op.Dependency),
					slog.String("operation", op.Method),
					slog.String("error_category", cfg.category(err)))
			}
			return err
		}
	}
}
