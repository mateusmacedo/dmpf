package otelboot

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/embedded"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

type handlerProvider struct {
	embedded.LoggerProvider
	handler slog.Handler
}

func (p handlerProvider) Logger(string, ...log.LoggerOption) log.Logger {
	return handlerLogger{handler: p.handler}
}

type handlerLogger struct {
	embedded.Logger
	handler slog.Handler
}

func (l handlerLogger) Enabled(ctx context.Context, param log.EnabledParameters) bool {
	return l.handler.Enabled(ctx, slogLevel(param.Severity))
}

func (l handlerLogger) Emit(ctx context.Context, record log.Record) {
	at := record.Timestamp()
	if at.IsZero() {
		at = time.Now()
	}
	converted := slog.NewRecord(at, slogLevel(record.Severity()), record.Body().String(), 0)
	if name := record.EventName(); name != "" {
		converted.AddAttrs(slog.String(string(semconv.OTelEventNameKey), name))
	}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		converted.AddAttrs(slogAttr(kv))
		return true
	})
	_ = l.handler.Handle(ctx, converted)
}

func slogLevel(severity log.Severity) slog.Level {
	return slog.Level(int(severity) - int(log.SeverityInfo))
}

func slogAttr(kv attribute.KeyValue) slog.Attr {
	if kv.Value.Type() != attribute.MAP {
		return slog.Any(string(kv.Key), kv.Value.AsInterface())
	}
	members := kv.Value.AsMap()
	group := make([]slog.Attr, 0, len(members))
	for _, member := range members {
		group = append(group, slogAttr(member))
	}
	return slog.Attr{Key: string(kv.Key), Value: slog.GroupValue(group...)}
}
