package app

import (
	"context"
	"log/slog"
	"reflect"
	"sync"

	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const messageConsumed = "message consumed"

var consumerLoggers sync.Map

func (c Consumer) logger() *slog.Logger {
	if !reflect.ValueOf(c.LoggerProvider).Comparable() {
		return newConsumerLogger(c.LoggerProvider)
	}
	if cached, ok := consumerLoggers.Load(c.LoggerProvider); ok {
		return cached.(*slog.Logger)
	}
	cached, _ := consumerLoggers.LoadOrStore(c.LoggerProvider, newConsumerLogger(c.LoggerProvider))
	return cached.(*slog.Logger)
}

func newConsumerLogger(provider log.LoggerProvider) *slog.Logger {
	return logging.NewLogger(provider, reflect.TypeFor[Consumer]().PkgPath())
}

func (c Consumer) logConsumed(ctx context.Context, env envelope.Envelope, delivery int, outcome Outcome, gesture *gestures, err error) {
	attributes := []slog.Attr{
		slog.String(string(semconv.MessagingOperationNameKey), processOperation),
		slog.String(string(semconv.MessagingMessageIDKey), env.ID),
		slog.String(string(semconv.CloudEventsEventTypeKey), env.Type),
		slog.Int(tracing.KeyInboxAttempt, delivery),
		slog.String(tracing.KeyOutcomeCategory, outcomeCategory(err)),
	}
	if c.System != "" {
		attributes = append(attributes, slog.String(string(semconv.MessagingSystemKey), c.System))
	}
	if c.Channel.Address != "" {
		attributes = append(attributes, slog.String(string(semconv.MessagingDestinationNameKey), c.Channel.Address))
	}
	if c.Channel.Group != "" {
		attributes = append(attributes, slog.String(string(semconv.MessagingConsumerGroupNameKey), c.Channel.Group))
	}
	if outcome.Classified {
		attributes = append(attributes, slog.String(tracing.KeyInboxDisposition, outcome.Disposition.String()))
	}
	if gesture.applied != "" {
		attributes = append(attributes, slog.String(tracing.KeyInboxGesture, gesture.applied))
	}
	if goType := panicTypeOf(err); goType != "" {
		attributes = append(attributes, slog.String(keyPanicType, goType))
	}
	c.logger().LogAttrs(ctx, consumedSeverity(outcome, gesture, err), messageConsumed, attributes...)
}

func consumedSeverity(outcome Outcome, gesture *gestures, err error) slog.Level {
	return logging.Severity(logging.Consumer, consumedOutcome(outcome, gesture, err))
}

func consumedOutcome(outcome Outcome, gesture *gestures, err error) ports.OutcomeCategory {
	switch {
	case !failed(outcome, gesture, err):
		return ports.OutcomeAccepted
	case !gesture.concluded():
		return ports.OutcomeFailed
	default:
		return fnd07Outcome(outcomeCategory(err))
	}
}

func (g *gestures) concluded() bool { return g.applied != "" && g.err == nil }

func fnd07Outcome(category string) ports.OutcomeCategory {
	switch application.Category(category) {
	case application.Validation, application.DomainRejection, application.NotFound, application.Conflict:
		return ports.OutcomeRejected
	case application.Forbidden, application.Unauthenticated:
		return ports.OutcomeDenied
	default:
		return ports.OutcomeFailed
	}
}
