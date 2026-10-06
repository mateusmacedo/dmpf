package boot_test

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestUnsetSignalsSampleEveryClassAtOne(t *testing.T) {
	signals, err := boot.SignalsFromEnv(env(nil))
	if err != nil || signals.Level != slog.LevelInfo {
		t.Fatalf("SignalsFromEnv() = %+v, %v; want info, nil", signals, err)
	}
	for _, class := range []tracing.Class{tracing.ClassError, tracing.ClassWrite, tracing.ClassRead,
		tracing.ClassMaintenance, tracing.ClassUnclassified} {
		if got := signals.Sampling.RateFor(class); got != 1 {
			t.Errorf("RateFor(%q) = %v, want 1.0, the default of traceidratio", class, got)
		}
	}
}

func TestTheSamplerArgumentIsTheUniformRateOfTheHead(t *testing.T) {
	signals, err := boot.SignalsFromEnv(env(map[string]string{boot.EnvTracesSamplerArg: "0.5", "TRACE_SAMPLE_RATE": "0.2"}))
	if err != nil {
		t.Fatalf("SignalsFromEnv() = %v", err)
	}
	for class, want := range map[tracing.Class]float64{
		tracing.ClassWrite: 0.5, tracing.ClassRead: 0.5, tracing.ClassUnclassified: 0.5,
		tracing.ClassError: 1, tracing.ClassMaintenance: 1,
	} {
		if got := signals.Sampling.RateFor(class); got != want {
			t.Errorf("RateFor(%q) = %v, want %v from %s, with the legacy TRACE_SAMPLE_RATE ignored (RF-E1)", class, got, want, boot.EnvTracesSamplerArg)
		}
	}
}

func TestAnInvalidSamplerArgumentIsRefused(t *testing.T) {
	_, err := boot.SignalsFromEnv(env(map[string]string{boot.EnvTracesSamplerArg: "1.5"}))
	if !errors.Is(err, envconfig.ErrInvalidVariable) || !strings.Contains(err.Error(), boot.EnvTracesSamplerArg) {
		t.Fatalf("SignalsFromEnv() = %v, want ErrInvalidVariable naming %s", err, boot.EnvTracesSamplerArg)
	}
}

func TestADeclaredSamplerIsReportedToBeIgnored(t *testing.T) {
	signals, err := boot.SignalsFromEnv(env(map[string]string{boot.EnvTracesSampler: "always_on"}))
	if err != nil || signals.IgnoredSampler != "always_on" {
		t.Fatalf("SignalsFromEnv() = %+v, %v; want the declared sampler kept to be warned about", signals, err)
	}
}

func TestADeclaredRateSamplesEveryClassButErrorsAtIt(t *testing.T) {
	signals, err := boot.SignalsFromEnv(env(map[string]string{boot.EnvTracesSamplerArg: "1", boot.EnvLogLevel: "debug"}))
	if err != nil {
		t.Fatalf("SignalsFromEnv() = %v, want nil", err)
	}
	if signals.Level != slog.LevelDebug || signals.Sampling.RateFor(tracing.ClassRead) != 1 {
		t.Fatalf("SignalsFromEnv() = %+v, want debug and read at 1", signals)
	}
}

func TestInvalidSignalsAreRefusedTogether(t *testing.T) {
	_, err := boot.SignalsFromEnv(env(map[string]string{boot.EnvTracesSamplerArg: "2", boot.EnvLogLevel: "loud"}))
	if !errors.Is(err, envconfig.ErrInvalidVariable) {
		t.Fatalf("SignalsFromEnv() = %v, want ErrInvalidVariable", err)
	}
	for _, variable := range []string{boot.EnvTracesSamplerArg, boot.EnvLogLevel} {
		if !strings.Contains(err.Error(), variable) {
			t.Fatalf("SignalsFromEnv() = %v, want it to name %s", err, variable)
		}
	}
}
