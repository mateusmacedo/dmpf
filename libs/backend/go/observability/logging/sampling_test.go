package logging_test

import (
	"context"
	"log/slog"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func kept(sampler logging.Sampler, ctx context.Context, level slog.Level, times int) int {
	count := 0
	for range times {
		if sampler.Allows(ctx, level) {
			count++
		}
	}
	return count
}

func insideAnUnsampledTrace() context.Context {
	return trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1}, SpanID: trace.SpanID{1},
	}))
}

func TestAnErrorIsNeverSampledAway(t *testing.T) {
	sampler := logging.NewSampler(tracing.ClassRead, nil, func() float64 { return 0.99 })

	got := kept(sampler, insideAnUnsampledTrace(), slog.LevelError, 5)

	if got != 5 {
		t.Fatalf("allowed %d records, want 5 — an error is never sampled (LOG-10, TRC-14)", got)
	}
}

func TestARecordBelowErrorIsSampledByTheClassRate(t *testing.T) {
	sampler := logging.NewSampler(tracing.ClassRead, nil, func() float64 { return 0.5 })

	got := kept(sampler, insideAnUnsampledTrace(), slog.LevelInfo, 5)

	if got != 0 {
		t.Fatalf("allowed %d records, want 0 — a draw of 0.5 is above the read rate of 0.01", got)
	}
}

func TestADrawBelowTheRateKeepsTheRecord(t *testing.T) {
	sampler := logging.NewSampler(tracing.ClassRead, nil, func() float64 { return 0.005 })

	got := kept(sampler, insideAnUnsampledTrace(), slog.LevelInfo, 1)

	if got != 1 {
		t.Fatalf("allowed %d records, want 1 — a draw of 0.005 is below the read rate of 0.01", got)
	}
}

func TestARecordOfASampledTraceIsAlwaysKept(t *testing.T) {
	if got := keptUnder(t, sdktrace.AlwaysSample()); got != 1 {
		t.Fatalf("allowed %d records, want 1 — the log is sampled with the trace (LOG-12)", got)
	}
}

func TestARecordOfAnUnsampledTraceFallsBackToTheRate(t *testing.T) {
	if got := keptUnder(t, sdktrace.NeverSample()); got != 0 {
		t.Fatalf("allowed %d records, want 0 — an unsampled trace does not exempt the record from the rate", got)
	}
}

func keptUnder(t *testing.T, traceSampler sdktrace.Sampler) int {
	t.Helper()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(traceSampler))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})
	ctx, span := provider.Tracer("observability").Start(context.Background(), "under.test")
	defer span.End()

	sampler := logging.NewSampler(tracing.ClassRead, nil, func() float64 { return 0.99 })
	return kept(sampler, ctx, slog.LevelInfo, 1)
}

func TestAnUndeclaredClassTakesTheMostRestrictiveRate(t *testing.T) {
	sampler := logging.NewSampler("", nil, func() float64 { return 0.05 })

	got := kept(sampler, insideAnUnsampledTrace(), slog.LevelInfo, 1)

	if got != 0 {
		t.Fatalf("allowed %d records, want 0 — an undeclared class takes the 0.01 rate", got)
	}
}

func TestAClassAtFullRateKeepsEveryRecord(t *testing.T) {
	sampler := logging.NewSampler(tracing.ClassMaintenance, nil, func() float64 { return 0.99 })

	got := kept(sampler, insideAnUnsampledTrace(), slog.LevelInfo, 3)

	if got != 3 {
		t.Fatalf("allowed %d records, want 3 — maintenance is sampled at 1.0", got)
	}
}

func TestADeclaredZeroRateDropsEverythingBelowError(t *testing.T) {
	sampler := logging.NewSampler(tracing.ClassRead, tracing.Rates{tracing.ClassRead: 0}, func() float64 { return 0 })

	got := kept(sampler, insideAnUnsampledTrace(), slog.LevelInfo, 1) + kept(sampler, insideAnUnsampledTrace(), slog.LevelError, 1)

	if got != 1 {
		t.Fatalf("allowed %d records, want 1 — only the error survives a zero rate", got)
	}
}

func TestWithoutASourceOfRandomnessTheRecordIsKept(t *testing.T) {
	sampler := logging.NewSampler(tracing.ClassRead, nil, nil)

	got := kept(sampler, insideAnUnsampledTrace(), slog.LevelInfo, 1)

	if got != 1 {
		t.Fatalf("allowed %d records, want 1 — losing a line is worse than writing one too many", got)
	}
}

func TestARecordOutsideATraceIsKeptWhateverTheRate(t *testing.T) {
	sampler := logging.NewSampler(tracing.ClassRead, tracing.Rates{tracing.ClassRead: 0}, func() float64 { return 0.99 })

	got := kept(sampler, context.Background(), slog.LevelInfo, 3)

	if got != 3 {
		t.Fatalf("allowed %d records, want 3 — a record outside a trace (start-up, shutdown) is not traffic, and LOG-12 samples only an unsampled trace", got)
	}
}
