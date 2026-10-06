package boot

import (
	"context"
	"log/slog"
	"reflect"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

// Boot starts the telemetry, hands the running runtime to the work and closes
// the pipelines on every exit, including the failing one.
//
// The work is a closure rather than a signature of its own because each
// composition root delegates differently, and what is common is only the
// order: no work without telemetry, and no exit without the drain.
func Boot(ctx context.Context, t Telemetry, work func(context.Context, *otelboot.Runtime) error) error {
	rt, err := StartTelemetry(ctx, t)
	if err != nil {
		return err
	}
	defer rt.ShutdownGracefully(ctx)
	rt.LoggerFor(reflect.TypeFor[Telemetry]().PkgPath()).LogAttrs(ctx, slog.LevelInfo, "process configured", configured(t.Settings)...)
	return work(ctx, rt)
}

const settingPrefix = "dmpf.config."

func configured(settings []slog.Attr) []slog.Attr {
	flat := make([]slog.Attr, 0, len(settings))
	for _, setting := range settings {
		flat = flatten(flat, settingPrefix, setting)
	}
	return flat
}

func flatten(flat []slog.Attr, prefix string, setting slog.Attr) []slog.Attr {
	value := setting.Value.Resolve()
	if value.Kind() != slog.KindGroup {
		if setting.Key == "" {
			return flat
		}
		return append(flat, slog.Attr{Key: prefix + setting.Key, Value: value})
	}
	if setting.Key != "" {
		prefix += setting.Key + "."
	}
	for _, member := range value.Group() {
		flat = flatten(flat, prefix, member)
	}
	return flat
}
