package boot

import (
	"errors"
	"log/slog"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

const (
	EnvTraceSampleRate = "TRACE_SAMPLE_RATE"
	EnvLogLevel        = "LOG_LEVEL"
	EnvOTLPLogs        = "OTLP_LOGS"
)

// Signals is how much a process records. A nil Sampling keeps the platform
// baseline of TRC-13. ExportLogs is opt-in because a container's stdout is
// already shipped by Alloy, and exporting it too would store every record twice.
type Signals struct {
	Sampling   tracing.Rates
	Level      slog.Level
	ExportLogs bool
}

func SignalsFromEnv(lookup func(string) string) (Signals, error) {
	var signals Signals
	rate, declared, rateErr := envconfig.ParseFraction(EnvTraceSampleRate, lookup(EnvTraceSampleRate))
	if declared {
		signals.Sampling = tracing.UniformRates(rate)
	}
	level, levelErr := envconfig.ParseLevel(EnvLogLevel, lookup(EnvLogLevel), slog.LevelInfo)
	signals.Level = level
	exportLogs, logsErr := envconfig.ParseBool(EnvOTLPLogs, lookup(EnvOTLPLogs))
	signals.ExportLogs = exportLogs
	if err := errors.Join(rateErr, levelErr, logsErr); err != nil {
		return Signals{}, err
	}
	return signals, nil
}
