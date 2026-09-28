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

func TestUnsetSignalsKeepThePlatformBaseline(t *testing.T) {
	signals, err := boot.SignalsFromEnv(env(nil))
	if err != nil || signals.Sampling != nil || signals.Level != slog.LevelInfo {
		t.Fatalf("SignalsFromEnv() = %+v, %v; want baseline sampling, info, nil", signals, err)
	}
}

func TestADeclaredRateSamplesEveryClassButErrorsAtIt(t *testing.T) {
	signals, err := boot.SignalsFromEnv(env(map[string]string{boot.EnvTraceSampleRate: "1", boot.EnvLogLevel: "debug"}))
	if err != nil {
		t.Fatalf("SignalsFromEnv() = %v, want nil", err)
	}
	if signals.Level != slog.LevelDebug || signals.Sampling.RateFor(tracing.ClassRead) != 1 {
		t.Fatalf("SignalsFromEnv() = %+v, want debug and read at 1", signals)
	}
}

func TestInvalidSignalsAreRefusedTogether(t *testing.T) {
	_, err := boot.SignalsFromEnv(env(map[string]string{boot.EnvTraceSampleRate: "2", boot.EnvLogLevel: "loud"}))
	if !errors.Is(err, envconfig.ErrInvalidVariable) {
		t.Fatalf("SignalsFromEnv() = %v, want ErrInvalidVariable", err)
	}
	for _, variable := range []string{boot.EnvTraceSampleRate, boot.EnvLogLevel} {
		if !strings.Contains(err.Error(), variable) {
			t.Fatalf("SignalsFromEnv() = %v, want it to name %s", err, variable)
		}
	}
}

func TestLogExportIsOptIn(t *testing.T) {
	unset, err := boot.SignalsFromEnv(env(nil))
	if err != nil || unset.ExportLogs {
		t.Fatalf("SignalsFromEnv() = %+v, %v; want logs kept to stdout", unset, err)
	}
	declared, err := boot.SignalsFromEnv(env(map[string]string{boot.EnvOTLPLogs: "true"}))
	if err != nil || !declared.ExportLogs {
		t.Fatalf("SignalsFromEnv(OTLP_LOGS=true) = %+v, %v; want logs exported", declared, err)
	}
}
