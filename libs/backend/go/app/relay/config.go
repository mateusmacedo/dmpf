package relay

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ErrInvalidConfig is what New reports for a value the relay cannot run with.
// It always names the field, so a misconfigured process fails at startup with
// the reason instead of at the first scan.
var ErrInvalidConfig = errors.New("relay: invalid configuration")

// Config is every operational value the relay needs, declared by the caller.
// FND-08 catalogues them under ANC-06; this block only refuses the ones that
// would make the loop incoherent.
type Config struct {
	// Source is the producer URI the envelope carries (ENV-08).
	Source string

	// Interval is how long an empty scan waits before claiming again, and
	// BatchSize how many records one claim acquires.
	Interval  time.Duration
	BatchSize int

	// Lease is how long a claim stays valid (OBX-09), and Concurrency caps
	// publishers in flight.
	Lease       time.Duration
	Concurrency int

	// MaxAttempts is the ceiling of OBX-06; zero or less leaves a record
	// retrying forever, which is a choice, not an oversight.
	MaxAttempts int

	// BackoffBase and BackoffCeiling bound the transient retry window; zero
	// base means a transient failure comes back on the next scan.
	BackoffBase    time.Duration
	BackoffCeiling time.Duration

	// ShutdownGrace bounds the writes that run detached from the caller's
	// cancellation; zero falls back to the block's own deadline.
	ShutdownGrace time.Duration

	Tracer         trace.Tracer
	MeterProvider  metric.MeterProvider
	System         string
	Address        func(destination string) string
	LoggerProvider log.LoggerProvider
}

// Validate refuses the values that cannot produce a working loop. A lease that
// never holds, a batch of nothing, and a scan that never waits are not tuning
// choices; they are configurations with no coherent behaviour.
func (c Config) Validate() error {
	checks := []struct {
		field string
		bad   bool
	}{
		{"source", c.Source == ""},
		{"interval", c.Interval <= 0},
		{"batch size", c.BatchSize <= 0},
		{"lease", c.Lease <= 0},
		{"concurrency", c.Concurrency <= 0},
		{"backoff base", c.BackoffBase < 0},
		{"backoff ceiling", c.BackoffCeiling < c.BackoffBase},
		{"shutdown grace", c.ShutdownGrace < 0},
	}
	for _, check := range checks {
		if check.bad {
			return fmt.Errorf("%w: %s", ErrInvalidConfig, check.field)
		}
	}
	return nil
}

func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("source", c.Source),
		slog.String("interval", c.Interval.String()),
		slog.Int("batch_size", c.BatchSize),
		slog.String("lease", c.Lease.String()),
		slog.Int("concurrency", c.Concurrency),
		slog.Int("max_attempts", c.MaxAttempts),
		slog.String("backoff_base", c.BackoffBase.String()),
		slog.String("backoff_ceiling", c.BackoffCeiling.String()),
		slog.String("shutdown_grace", c.ShutdownGrace.String()),
	)
}

// New assembles the relay and validates at startup rather than at the first
// scan: a process configured wrong should refuse to start, not drain half a
// batch and then discover it.
func New(store Store, publisher Publisher, ids ClaimIDs, clock ports.Clock, config Config) (Relay, error) {
	if store == nil || publisher == nil || ids == nil || clock == nil {
		return Relay{}, ErrIncompleteRelay
	}
	if err := config.Validate(); err != nil {
		return Relay{}, err
	}
	tracer := config.Tracer
	if tracer == nil {
		tracer = noop.NewTracerProvider().Tracer("")
	}

	return Relay{
		Store:          store,
		Publisher:      publisher,
		ClaimIDs:       ids,
		Clock:          clock,
		Source:         config.Source,
		Interval:       config.Interval,
		BatchSize:      config.BatchSize,
		Lease:          config.Lease,
		Concurrency:    config.Concurrency,
		MaxAttempts:    config.MaxAttempts,
		ShutdownGrace:  config.ShutdownGrace,
		Backoff:        ExponentialBackoff(config.BackoffBase, config.BackoffCeiling, rand.Float64),
		Tracer:         tracer,
		MeterProvider:  config.MeterProvider,
		System:         config.System,
		Address:        config.Address,
		LoggerProvider: config.LoggerProvider,
	}, nil
}
