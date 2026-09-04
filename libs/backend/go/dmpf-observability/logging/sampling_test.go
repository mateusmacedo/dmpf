package logging_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/logging"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
)

func lines(t *testing.T, config logging.Config, emit func(logger *slog.Logger)) int {
	t.Helper()

	var out bytes.Buffer
	emit(slog.New(logging.NewHandler(&out, config)))

	written := strings.TrimSpace(out.String())
	if written == "" {
		return 0
	}
	return len(strings.Split(written, "\n"))
}

func TestAnErrorIsNeverSampledAway(t *testing.T) {
	config := baseConfig()
	config.Class = tracing.ClassRead
	config.Rand = func() float64 { return 0.99 }

	got := lines(t, config, func(logger *slog.Logger) {
		for range 5 {
			logger.Error("payment failed")
		}
	})

	if got != 5 {
		t.Fatalf("wrote %d records, want 5 — an error is never sampled (LOG-10, TRC-14)", got)
	}
}

func TestARecordBelowErrorIsSampledByTheClassRate(t *testing.T) {
	config := baseConfig()
	config.Class = tracing.ClassRead
	config.Rand = func() float64 { return 0.5 }

	got := lines(t, config, func(logger *slog.Logger) {
		for range 5 {
			logger.Info("order read")
		}
	})

	if got != 0 {
		t.Fatalf("wrote %d records, want 0 — a draw of 0.5 is above the read rate of 0.01", got)
	}
}

func TestADrawBelowTheRateKeepsTheRecord(t *testing.T) {
	config := baseConfig()
	config.Class = tracing.ClassWrite
	config.Rand = func() float64 { return 0.05 }

	got := lines(t, config, func(logger *slog.Logger) { logger.Info("order written") })

	if got != 1 {
		t.Fatalf("wrote %d records, want 1 — a draw of 0.05 is below the write rate of 0.10", got)
	}
}

func TestARecordOfASampledTraceIsAlwaysKept(t *testing.T) {
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})
	ctx, span := provider.Tracer("dmpf-observability").Start(context.Background(), "under.test")
	defer span.End()

	config := baseConfig()
	config.Class = tracing.ClassRead
	config.Rand = func() float64 { return 0.99 }

	got := lines(t, config, func(logger *slog.Logger) { logger.InfoContext(ctx, "order read") })

	if got != 1 {
		t.Fatalf("wrote %d records, want 1 — the log is sampled with the trace (LOG-12)", got)
	}
}

func TestARecordOfAnUnsampledTraceFallsBackToTheRate(t *testing.T) {
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})
	ctx, span := provider.Tracer("dmpf-observability").Start(context.Background(), "under.test")
	defer span.End()

	config := baseConfig()
	config.Class = tracing.ClassRead
	config.Rand = func() float64 { return 0.99 }

	got := lines(t, config, func(logger *slog.Logger) { logger.InfoContext(ctx, "order read") })

	if got != 0 {
		t.Fatalf("wrote %d records, want 0 — an unsampled trace does not exempt the record from the rate", got)
	}
}

func TestAnUndeclaredClassTakesTheMostRestrictiveRate(t *testing.T) {
	config := baseConfig()
	config.Class = ""
	config.Rand = func() float64 { return 0.05 }

	got := lines(t, config, func(logger *slog.Logger) { logger.Info("order read") })

	if got != 0 {
		t.Fatalf("wrote %d records, want 0 — an undeclared class takes the 0.01 rate", got)
	}
}

func TestAClassAtFullRateKeepsEveryRecord(t *testing.T) {
	config := baseConfig()
	config.Class = tracing.ClassMaintenance
	config.Rand = func() float64 { return 0.99 }

	got := lines(t, config, func(logger *slog.Logger) {
		for range 3 {
			logger.Info("index rebuilt")
		}
	})

	if got != 3 {
		t.Fatalf("wrote %d records, want 3 — maintenance is sampled at 1.0", got)
	}
}

func TestADeclaredZeroRateDropsEverythingBelowError(t *testing.T) {
	config := baseConfig()
	config.Class = tracing.ClassRead
	config.Sampling = tracing.Rates{tracing.ClassRead: 0}
	config.Rand = func() float64 { return 0 }

	got := lines(t, config, func(logger *slog.Logger) {
		logger.Info("order read")
		logger.Error("payment failed")
	})

	if got != 1 {
		t.Fatalf("wrote %d records, want 1 — only the error survives a zero rate", got)
	}
}

func TestWithoutASourceOfRandomnessTheRecordIsKept(t *testing.T) {
	config := baseConfig()
	config.Class = tracing.ClassRead
	config.Rand = nil

	got := lines(t, config, func(logger *slog.Logger) { logger.Info("order read") })

	if got != 1 {
		t.Fatalf("wrote %d records, want 1 — losing a line is worse than writing one too many", got)
	}
}

func TestTheLevelStillFiltersBelowTheThreshold(t *testing.T) {
	config := baseConfig()
	config.Level = slog.LevelWarn

	got := lines(t, config, func(logger *slog.Logger) {
		logger.Info("order read")
		logger.Warn("slow dependency")
	})

	if got != 1 {
		t.Fatalf("wrote %d records, want 1 — the level threshold applies before sampling", got)
	}
}
