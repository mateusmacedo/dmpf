// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-08 (RES-21, RES-22, RES-23, RES-31) que o símbolo realiza, dentro do limite de 3 linhas.

// Package compose builds the composition of RES-22 for one transport
// dependency — the positions of RES-23, breaker, bulkhead, timeout and retry —
// so each provider keeps only its classifier and its failure category.
package compose

import (
	"math/rand/v2"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/observe"
)

// Config is what every provider hands over: the sheet of RES-21, the clock,
// the sinks of telemetry, the source of randomness for the backoff, the
// failure category, the retry classifier and what the breaker counts as failure.
type Config struct {
	Sheet          resilience.Sheet
	Clock          clock.Clock
	Tracer         trace.Tracer
	Instruments    *metrics.Instruments
	LoggerProvider log.LoggerProvider
	Rand           func() float64
	Category       func(error) string
	Classifier     retry.Classifier
	BreakerFailure func(error) bool
}

// Build is the whole composition, one Call the provider reuses for every
// operation of the dependency.
func Build(cfg Config) (resilience.Call, error) {
	slots := Shared(cfg)
	retryDecorator, err := Retry(cfg, cfg.Classifier)
	if err != nil {
		return nil, err
	}
	slots.Retry = retryDecorator
	return resilience.Compose(cfg.Sheet, slots)
}

// Shared fills every position but retry: the observability of RES-23, with the
// degradation of RES-37 under its span, and the sheet's breaker, bulkhead and
// timeout, built once because their state is the dependency's, whatever the method.
func Shared(cfg Config) resilience.Slots {
	slots := observe.Slots(observe.Config{
		Clock:          cfg.Clock,
		Tracer:         cfg.Tracer,
		LoggerProvider: cfg.LoggerProvider,
		Category:       cfg.Category,
		MaxAttempts:    maxAttempts(cfg.Sheet),
	})
	slots.Tracing = degrading(slots.Tracing, cfg)
	dependency := cfg.Sheet.Dependency
	if policy, declared := cfg.Sheet.Breaker.Get(); declared {
		slots.Breaker = resilience.NewBreaker(dependency, policy, cfg.Clock, cfg.Instruments).CountsAsFailure(cfg.BreakerFailure).LogsTo(cfg.LoggerProvider).Decorate()
	}
	if policy, declared := cfg.Sheet.Bulkhead.Get(); declared {
		slots.Bulkhead = resilience.NewBulkhead(dependency, policy, cfg.Clock, cfg.Instruments).Decorate()
	}
	if _, declared := cfg.Sheet.Deadline.Get(); declared {
		var reserve time.Duration
		if backoff, ok := cfg.Sheet.Backoff.Get(); ok {
			reserve = backoff.Base
		}
		slots.Timeout = resilience.Timeout(cfg.Clock, cfg.Instruments, reserve)
	}
	return slots
}

// Retry is the retry position for one classifier: the decorator of KRN-09
// when the sheet enables retry, the identity when it declares retry off — the
// position may not be left empty (RES-21) — and nil when the sheet skips it.
func Retry(cfg Config, classifier retry.Classifier) (resilience.Decorator, error) {
	enabled, declared := cfg.Sheet.Retry.Get()
	if !declared {
		return nil, nil
	}
	if !enabled {
		return identity, nil
	}
	maxAttempts, _ := cfg.Sheet.MaxAttempts.Get()
	backoff, _ := cfg.Sheet.Backoff.Get()
	random := cfg.Rand
	if random == nil {
		random = rand.Float64
	}
	return resilience.NewRetry(resilience.RetryConfig{
		Dependency:  cfg.Sheet.Dependency,
		Classifier:  classifier,
		Category:    cfg.Category,
		MaxAttempts: maxAttempts,
		Backoff:     backoff,
		Rand:        random,
	}, cfg.Clock, clock.NewSleeper(cfg.Clock), cfg.Instruments)
}

func identity(next resilience.Call) resilience.Call { return next }

func degrading(tracing resilience.Decorator, cfg Config) resilience.Decorator {
	mode, _ := cfg.Sheet.Degradation.Get()
	degradation, err := resilience.NewDegradation(mode, cfg.Sheet.Dependency, cfg.Instruments)
	if err != nil {
		return tracing
	}
	return func(next resilience.Call) resilience.Call { return tracing(degradation(next)) }
}

func maxAttempts(sheet resilience.Sheet) int {
	enabled, declared := sheet.Retry.Get()
	if !declared {
		return 0
	}
	if !enabled {
		return 1
	}
	ceiling, _ := sheet.MaxAttempts.Get()
	return ceiling
}

// Operation is one remote gesture on the dependency, bounded by the sheet's
// deadline; the estimate is a quarter of it, what the retry budget reserves.
func Operation(cfg Config, method string, idempotent bool) resilience.Operation {
	deadline, _ := cfg.Sheet.Deadline.Get()
	return resilience.Operation{
		Dependency:        cfg.Sheet.Dependency,
		Method:            method,
		Kind:              resilience.Remote,
		Idempotent:        idempotent,
		Deadline:          deadline,
		EstimatedDuration: max(deadline/4, time.Millisecond),
	}
}
