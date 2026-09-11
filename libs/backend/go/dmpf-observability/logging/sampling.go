package logging

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/tracing"
)

// sampler decides whether a record is written. It follows LOG-12 — the log is
// sampled with the trace, not on its own — and LOG-10: a failure is never
// dropped, whatever the rate says.
type sampler struct {
	class tracing.Class
	rates tracing.Rates
	rand  func() float64
}

func (s sampler) allows(ctx context.Context, level slog.Level) bool {
	if level >= slog.LevelError {
		return true
	}

	// A record that belongs to a sampled trace is kept, so a trace under
	// inspection is not missing the lines that explain it (LOG-12).
	if span := trace.SpanContextFromContext(ctx); span.IsValid() && span.IsSampled() {
		return true
	}

	rate := s.rates.RateFor(s.class)
	switch {
	case rate >= 1:
		return true
	case rate <= 0:
		return false
	case s.rand == nil:
		// No source of randomness is read as "keep": losing a line is worse
		// than writing one too many.
		return true
	default:
		return s.rand() < rate
	}
}
