package boot

import (
	"errors"
	"log/slog"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

const (
	EnvLogLevel         = "LOG_LEVEL"
	EnvTracesSampler    = "OTEL_TRACES_SAMPLER"
	EnvTracesSamplerArg = "OTEL_TRACES_SAMPLER_ARG"
)

// Signals is how much a process records. Sampling is the uniform head rate of
// OTEL_TRACES_SAMPLER_ARG, else 1.0; IgnoredSampler is a declared
// OTEL_TRACES_SAMPLER, which the platform sampler overrides.
type Signals struct {
	Sampling       tracing.Rates
	Level          slog.Level
	IgnoredSampler string
}

func SignalsFromEnv(lookup func(string) string) (Signals, error) {
	signals := Signals{Sampling: tracing.UniformRates(1), IgnoredSampler: lookup(EnvTracesSampler)}
	arg, argDeclared, argErr := envconfig.ParseFraction(EnvTracesSamplerArg, lookup(EnvTracesSamplerArg))
	if argDeclared {
		signals.Sampling = tracing.UniformRates(arg)
	}
	level, levelErr := envconfig.ParseLevel(EnvLogLevel, lookup(EnvLogLevel), slog.LevelInfo)
	signals.Level = level
	if err := errors.Join(argErr, levelErr); err != nil {
		return Signals{}, err
	}
	return signals, nil
}
