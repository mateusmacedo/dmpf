package otelboot

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
)

func Leveled(provider log.LoggerProvider, level slog.Leveler) log.LoggerProvider {
	if level == nil {
		level = slog.LevelInfo
	}
	return leveledProvider{LoggerProvider: provider, level: level}
}

// WHY: otelslog.Handler.Enabled only asks the provider (otelslog@v0.21.0/handler.go:266-270),
// so LOG_LEVEL is applied by the provider, and never to the audit trail (LOG-13).
type leveledProvider struct {
	log.LoggerProvider
	level slog.Leveler
}

func (p leveledProvider) Logger(name string, options ...log.LoggerOption) log.Logger {
	return leveledLogger{Logger: p.LoggerProvider.Logger(name, options...), level: p.level}
}

type leveledLogger struct {
	log.Logger
	level slog.Leveler
}

func (l leveledLogger) Enabled(ctx context.Context, param log.EnabledParameters) bool {
	return l.admits(param.EventName, param.Severity) && l.Logger.Enabled(ctx, param)
}

func (l leveledLogger) Emit(ctx context.Context, record log.Record) {
	if l.admits(record.EventName(), record.Severity()) {
		l.Logger.Emit(ctx, record)
	}
}

func (l leveledLogger) admits(eventName string, severity log.Severity) bool {
	return eventName == audit.EventName || levelOf(severity) >= l.level.Level()
}
