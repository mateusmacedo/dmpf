// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-08 (RES-23, TRC-04, TRC-11, TRC-12, LOG-13) que o símbolo realiza, dentro do limite de 3 linhas.

// Package observe is the three observability positions of RES-22 — tracing,
// metrics, logging — that RES-23 requires of every composition and the
// resilience package leaves to the caller; only the failure category varies.
package observe

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// CategoryOK is the outcome category of a call that returned no error.
const CategoryOK = "ok"

// Config is what the decorators record through. MaxAttempts is the retry
// ceiling of the sheet; Category maps a failure to its bounded category and is
// the only transport-specific part.
type Config struct {
	Clock          clock.Clock
	Tracer         trace.Tracer
	LoggerProvider log.LoggerProvider
	Category       func(error) string
	MaxAttempts    int
}

func (c Config) tracer() trace.Tracer {
	if c.Tracer == nil {
		return noop.NewTracerProvider().Tracer("transport")
	}
	return c.Tracer
}

func (c Config) clock() clock.Clock {
	if c.Clock == nil {
		return clock.System()
	}
	return c.Clock
}

func (c Config) logger() *slog.Logger {
	return logging.NewLogger(c.LoggerProvider, reflect.TypeFor[Config]().PkgPath())
}

func (c Config) category(err error) string {
	if err == nil {
		return CategoryOK
	}
	if c.Category == nil {
		return redact.CategoryUnclassified
	}
	return c.Category(err)
}

// Slots fills the tracing, metrics and logging positions; the caller adds the
// rest of the composition to the returned value.
func Slots(cfg Config) resilience.Slots {
	return resilience.Slots{
		Tracing: Tracing(cfg),
		Metrics: passThrough,
		Logging: Logging(cfg),
	}
}

// passThrough holds the metrics position, which resilience.Compose refuses empty
// (resilience/compose.go:129-166): the client RED is otelgrpc's and otelhttp's (RF-D2).
func passThrough(next resilience.Call) resilience.Call { return next }

const (
	resilienceSpanPrefix = "dmpf.resilience "
	keyRetryMaxAttempts  = "dmpf.retry.max_attempts"
)

// Tracing opens the INTERNAL span over the attempts (TRC-11, RF-B5); under the
// send the relay owns, it records on that span instead, so the publisher's
// partition and offset land on the send. A failure is a category, never a message.
func Tracing(cfg Config) resilience.Decorator {
	tracer := cfg.tracer()
	starts := newCache[[]attribute.KeyValue]()
	outcomes := newCache[[]attribute.KeyValue]()
	return func(next resilience.Call) resilience.Call {
		return func(ctx context.Context, op resilience.Operation, do func(context.Context) error) error {
			attrs := starts.get(cacheKey{op.Dependency, ""}, func() []attribute.KeyValue {
				kv := tracing.Attributes{}.Dependency(op.Dependency).KeyValues()
				if cfg.MaxAttempts > 0 {
					kv = append(kv, attribute.Int(keyRetryMaxAttempts, cfg.MaxAttempts))
				}
				return kv
			})
			if deadline, declared := ctx.Deadline(); declared {
				attrs = append(attrs[:len(attrs):len(attrs)],
					attribute.Int64(tracing.KeyDeadlineRemainingMS, deadline.Sub(cfg.clock().Now()).Milliseconds()))
			}

			span, owned := tracing.OwnsSpan(ctx)
			if owned {
				span.SetAttributes(attrs...)
			} else {
				ctx, span = tracer.Start(ctx, resilienceSpanPrefix+op.Dependency,
					trace.WithSpanKind(trace.SpanKindInternal),
					trace.WithAttributes(attrs...))
				defer span.End()
			}

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

// Logging logs every attempt the retry below makes, with its category and no
// message: the message would leave the process without redaction (LOG-13). The
// level is Severity(Client, …) and a refusal below is the attempt it refused.
func Logging(cfg Config) resilience.Decorator {
	logger := cfg.logger()
	return func(next resilience.Call) resilience.Call {
		return func(ctx context.Context, op resilience.Operation, do func(context.Context) error) error {
			attempts := 0
			var last error
			err := next(ctx, op, func(attemptCtx context.Context) error {
				attempts++
				last = do(attemptCtx)
				logAttempt(attemptCtx, logger, cfg, op, attempts, last)
				return last
			})
			if err != nil && !errors.Is(err, last) {
				logAttempt(ctx, logger, cfg, op, attempts+1, err)
			}
			return err
		}
	}
}

func logAttempt(ctx context.Context, logger *slog.Logger, cfg Config, op resilience.Operation, attempt int, err error) {
	if err == nil {
		logger.LogAttrs(ctx, logging.Severity(logging.Client, ports.OutcomeAccepted), "transport: call",
			slog.String(tracing.KeyDependency, op.Dependency),
			slog.Int(logging.KeyAttempt, attempt),
			slog.String(logging.KeyOutcomeCategory, CategoryOK))
		return
	}
	category := cfg.category(err)
	logger.LogAttrs(ctx, logging.Severity(logging.Client, outcomeOf(category)), "transport: call failed",
		slog.String(tracing.KeyDependency, op.Dependency),
		slog.Int(logging.KeyAttempt, attempt),
		slog.String(logging.KeyOutcomeCategory, category),
		redact.Error(failureOf(err, category)))
}

var fnd07Outcomes = map[string]ports.OutcomeCategory{
	"Validation": ports.OutcomeRejected, "DomainRejection": ports.OutcomeRejected, "NotFound": ports.OutcomeRejected,
	"Conflict": ports.OutcomeRejected, "Forbidden": ports.OutcomeDenied, "Unauthenticated": ports.OutcomeDenied,
}

func outcomeOf(category string) ports.OutcomeCategory {
	if outcome, mapped := fnd07Outcomes[category]; mapped {
		return outcome
	}
	return ports.OutcomeFailed
}

type attemptFailure struct{ category, code string }

func failureOf(err error, category string) attemptFailure {
	failure := attemptFailure{category: category}
	var categorized redact.Categorized
	if errors.As(err, &categorized) {
		failure.code = categorized.ErrorCode()
	}
	return failure
}

func (f attemptFailure) Error() string         { return f.category }
func (f attemptFailure) ErrorCategory() string { return f.category }
func (f attemptFailure) ErrorCode() string     { return f.code }
