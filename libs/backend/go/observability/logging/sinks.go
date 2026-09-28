package logging

import (
	"context"
	"errors"
	"log/slog"
)

// fanout hands the sinks only what the writer accepts, so the level and the
// sampling of the service hold on every destination.
type fanout []slog.Handler

func fanOut(writer slog.Handler, sinks []slog.Handler) slog.Handler {
	if len(sinks) == 0 {
		return writer
	}
	return append(fanout{writer}, sinks...)
}

func (f fanout) Enabled(ctx context.Context, level slog.Level) bool {
	return f[0].Enabled(ctx, level)
}

func (f fanout) Handle(ctx context.Context, record slog.Record) error {
	var errs []error
	for _, h := range f {
		if h.Enabled(ctx, record.Level) {
			errs = append(errs, h.Handle(ctx, record.Clone()))
		}
	}
	return errors.Join(errs...)
}

func (f fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make(fanout, len(f))
	for i, h := range f {
		next[i] = h.WithAttrs(attrs)
	}
	return next
}

func (f fanout) WithGroup(name string) slog.Handler {
	next := make(fanout, len(f))
	for i, h := range f {
		next[i] = h.WithGroup(name)
	}
	return next
}
