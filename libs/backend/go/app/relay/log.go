package relay

import (
	"context"
	"log/slog"
	"reflect"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	publishFailedMessage = "outbox publish failed"
	claimFailedMessage   = "outbox claim failed"
	releaseFailedMessage = "outbox release failed"
)

func (r Relay) logger() *slog.Logger {
	if r.logs != nil {
		return r.logs
	}
	return logging.NewLogger(r.LoggerProvider, reflect.TypeFor[Relay]().PkgPath())
}

func (r Relay) logPublishFailed(ctx context.Context, record postgres.Claimed, err error, category string) {
	if ctx.Err() != nil {
		return
	}
	r.logger().LogAttrs(ctx, logging.Severity(logging.Client, ports.OutcomeFailed), publishFailedMessage,
		slog.String(string(semconv.MessagingMessageIDKey), record.MessageID),
		slog.Int(tracing.KeyOutboxAttempt, record.AttemptCount),
		failureOf(err, category))
}

func failureOf(err error, category string) slog.Attr {
	if category != sendCategory(err) {
		return slog.String(redact.KeyErrorType, category)
	}
	return redact.Error(err)
}

func (r Relay) logClaimFailed(ctx context.Context, err error) {
	r.logger().LogAttrs(ctx, slog.LevelError, claimFailedMessage, redact.Error(err))
}

func (r Relay) logReleaseFailed(ctx context.Context, record postgres.Claimed, err error) {
	r.logger().LogAttrs(ctx, slog.LevelWarn, releaseFailedMessage,
		slog.String(string(semconv.MessagingMessageIDKey), record.MessageID),
		redact.Error(err))
}
