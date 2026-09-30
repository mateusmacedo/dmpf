package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ErrInvalidPurgeConfig is RunPurge's refusal of a loop it could not run safely.
var ErrInvalidPurgeConfig = errors.New("app: invalid purge config")

// PurgeConfig declares the loop that keeps one kernel table inside its retention
// (INB-14, OBX-17, IDM-09). Retention is zero for rows that carry their own
// expiry, whose cutoff is now; any other cutoff is now minus Retention.
type PurgeConfig struct {
	Name      string
	Interval  time.Duration
	Batch     int
	Retention time.Duration
}

func validatePurge(c PurgeConfig, clock ports.Clock, logger *slog.Logger, fn PurgeFunc) error {
	switch {
	case c.Name == "":
		return fmt.Errorf("%w: name empty", ErrInvalidPurgeConfig)
	case c.Interval <= 0:
		return fmt.Errorf("%w: %s interval %v", ErrInvalidPurgeConfig, c.Name, c.Interval)
	case c.Batch <= 0:
		return fmt.Errorf("%w: %s batch %d", ErrInvalidPurgeConfig, c.Name, c.Batch)
	case c.Retention < 0:
		return fmt.Errorf("%w: %s retention %v", ErrInvalidPurgeConfig, c.Name, c.Retention)
	case clock == nil, logger == nil, fn == nil:
		return fmt.Errorf("%w: %s needs a clock, a logger and a purge function", ErrInvalidPurgeConfig, c.Name)
	}
	return nil
}

type PurgeFunc func(ctx context.Context, cutoff ports.Instant, batch int) (int64, error)

// StartPurge runs RunPurge in its own goroutine. stop cancels the loop and
// returns only once it ended, so a caller can close the pool right after.
func StartPurge(ctx context.Context, cfg PurgeConfig, clock ports.Clock, logger *slog.Logger, fn PurgeFunc) (stop func(), err error) {
	if err := validatePurge(cfg, clock, logger, fn); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = RunPurge(ctx, cfg, clock, logger, fn)
	}()
	return func() {
		cancel()
		<-done
	}, nil
}

// RunPurge purges one batch every Interval until ctx is done, and returns nil
// then: stopping is not a failure. A full batch is followed at once by the next;
// a failed cycle is logged and retried on the next interval.
func RunPurge(ctx context.Context, cfg PurgeConfig, clock ports.Clock, logger *slog.Logger, fn PurgeFunc) error {
	if err := validatePurge(cfg, clock, logger, fn); err != nil {
		return err
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		for {
			cutoff := clock.Now() - ports.Instant(cfg.Retention)
			removed, err := fn(ctx, cutoff, cfg.Batch)
			if err != nil {
				if ctx.Err() == nil {
					logger.WarnContext(ctx, "purge cycle failed", "table", cfg.Name, "error", err.Error())
				}
				break
			}
			if removed > 0 {
				logger.InfoContext(ctx, "purged", "table", cfg.Name, "removed", removed, "before", int64(cutoff))
			}
			if removed < int64(cfg.Batch) || ctx.Err() != nil {
				break
			}
		}
		timer.Reset(cfg.Interval)
	}
}
