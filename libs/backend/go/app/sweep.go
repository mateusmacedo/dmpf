package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	sweepOperation = "sweep"
	keySweepSwept  = "dmpf.sweep.swept"
	keySweepBefore = "dmpf.sweep.before"
)

var ErrInvalidSweepConfig = errors.New("app: invalid sweep config")

var ErrSweepPanicked = errors.New("app: the sweep loop panicked")

// SweepConfig declares the loop that closes the begun attempts no redelivery
// took back (RES-43). StaleAfter is the RES-21 deadline plus the margin, the
// same cut Dispatch receives as StaleAfter in ns.
type SweepConfig struct {
	Interval   time.Duration
	Batch      int
	StaleAfter time.Duration
}

type SweepPage = application.SweepPage

type SweepFunc func(ctx context.Context, cutoff ports.Instant, after ports.AttemptCursor, batch int) (SweepPage, error)

func validateSweep(c SweepConfig, clock ports.Clock, logs log.LoggerProvider, fn SweepFunc) error {
	switch {
	case c.Interval <= 0:
		return fmt.Errorf("%w: interval %v", ErrInvalidSweepConfig, c.Interval)
	case c.Batch <= 0:
		return fmt.Errorf("%w: batch %d", ErrInvalidSweepConfig, c.Batch)
	case c.StaleAfter <= 0:
		return fmt.Errorf("%w: stale after %v", ErrInvalidSweepConfig, c.StaleAfter)
	case clock == nil, logs == nil, fn == nil:
		return fmt.Errorf("%w: the sweep needs a clock, a logger and a sweep function", ErrInvalidSweepConfig)
	}
	return nil
}

// RunSweep never submits: a resend is always a redelivery's (RES-43).
func RunSweep(ctx context.Context, cfg SweepConfig, clock ports.Clock, logs log.LoggerProvider, fn SweepFunc) (err error) {
	if err := validateSweep(cfg, clock, logs, fn); err != nil {
		return err
	}
	defer func() {
		if recover() != nil {
			err = ErrSweepPanicked
		}
	}()
	logger := logging.NewLogger(logs, reflect.TypeFor[SweepConfig]().PkgPath())
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		sweepCycle(ctx, cfg, clock.Now()-ports.Instant(cfg.StaleAfter), logger, fn)
		timer.Reset(cfg.Interval)
	}
}

func sweepCycle(ctx context.Context, cfg SweepConfig, cutoff ports.Instant, logger *slog.Logger, fn SweepFunc) {
	var after ports.AttemptCursor
	for {
		page, err := fn(ctx, cutoff, after, cfg.Batch)
		if err != nil && ctx.Err() == nil {
			logger.WarnContext(ctx, "sweep cycle failed", redact.Error(err))
		}
		if page.Swept > 0 {
			logger.InfoContext(ctx, "swept", keySweepSwept, page.Swept, keySweepBefore, int64(cutoff))
		}
		if page.Seen < cfg.Batch || page.Next == after || ctx.Err() != nil {
			return
		}
		after = page.Next
	}
}

// ExternalSweep takes Abandon as a function because the context builds it over
// its own UnitOfWork, profile and step; Timeout and Locale fill the execution
// context no caller states for work the kernel starts (CTX-17, CTX-28).
type ExternalSweep struct {
	Store   ports.AttemptLedger
	Abandon func(ctx context.Context, attempt ports.Attempt) (ports.Finished, error)
	Timeout time.Duration
	Locale  string
	Tracer  trace.Tracer
}

func SweepExternalAttempts(s ExternalSweep) (SweepFunc, error) {
	if s.Store == nil || s.Abandon == nil || s.Timeout <= 0 || s.Locale == "" {
		return nil, fmt.Errorf("%w: the external sweep needs a store, an abandon, a timeout and a locale", ErrInvalidSweepConfig)
	}
	return func(ctx context.Context, cutoff ports.Instant, after ports.AttemptCursor, batch int) (SweepPage, error) {
		return application.SweepStale(ctx, s.Store, cutoff, after, batch, s.close)
	}, nil
}

func (s ExternalSweep) close(ctx context.Context, attempt ports.Attempt) error {
	ctx, span := s.tracer().Start(ctx, sweepOperation, trace.WithNewRoot(), trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()

	ctx, err := s.mount(ctx, attempt, span)
	if err != nil {
		tracing.RecordError(span, outcomeCategory(err))
		return fmt.Errorf("app: sweep attempt %d: %w", attempt.N, err)
	}
	finished, err := s.Abandon(ctx, attempt)
	if err != nil {
		tracing.RecordError(span, outcomeCategory(err))
		return fmt.Errorf("app: sweep attempt %d: %w", attempt.N, err)
	}
	if finished.Recorded != 0 {
		span.SetAttributes(attribute.String(tracing.KeyExternalClass, finished.Recorded.String()))
	}
	return nil
}

// The ledger keeps no correlation of the chain, so one is minted here (CTX-07,
// CTX-17) and the attempt's message stands as the cause.
func (s ExternalSweep) mount(ctx context.Context, attempt ports.Attempt, span trace.Span) (context.Context, error) {
	deadline, _ := ctx.Deadline()
	causation := string(attempt.MessageID)
	var tenant *ports.TenantID
	if attempt.Tenant != "" {
		tenant = &attempt.Tenant
	}
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID:     newAttemptID(),
		CorrelationID: newAttemptID(),
		CausationID:   &causation,
		TraceContext:  span.SpanContext().TraceID().String(),
		Tenant:        tenant,
		Deadline:      ports.Instant(deadline.UnixNano()),
		Locale:        s.Locale,
	})
	if err != nil {
		return nil, err
	}
	span.SetAttributes(tracing.ExecutionAttributes(execution).KeyValues()...)

	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	ctx = ports.WithMessageContext(ctx, ports.MessageContext{
		CorrelationID: execution.CorrelationID(),
		CausationID:   causation,
		Traceparent:   carrier.Get("traceparent"),
		Tracestate:    carrier.Get("tracestate"),
	})
	return ports.WithExecutionContext(tracing.WithExecutionBaggage(ctx, execution), execution), nil
}

func (s ExternalSweep) tracer() trace.Tracer {
	if s.Tracer == nil {
		return noop.NewTracerProvider().Tracer("")
	}
	return s.Tracer
}
