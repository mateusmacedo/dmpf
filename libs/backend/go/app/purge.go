package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	keyPurgeName    = "dmpf.purge.name"
	keyPurgeRemoved = "dmpf.purge.removed"
	keyPurgeBefore  = "dmpf.purge.before"
)

// ErrInvalidPurgeConfig is RunPurge's refusal of a loop it could not run safely.
var ErrInvalidPurgeConfig = errors.New("app: invalid purge config")

var ErrPurgePanicked = errors.New("app: the purge loop panicked")

// PurgeConfig declares the loop that keeps one kernel table inside its retention
// (INB-14, OBX-17, IDM-09). Retention is zero for rows that carry their own
// expiry, whose cutoff is now; any other cutoff is now minus Retention.
type PurgeConfig struct {
	Name      string
	Table     string
	Interval  time.Duration
	Batch     int
	Retention time.Duration
}

func (c PurgeConfig) subject() []any {
	subject := []any{keyPurgeName, c.Name}
	if c.Table != "" {
		subject = append(subject, string(semconv.DBCollectionNameKey), c.Table)
	}
	return slices.Clip(subject)
}

func validatePurge(c PurgeConfig, clock ports.Clock, logs log.LoggerProvider, fn PurgeFunc) error {
	switch {
	case c.Name == "":
		return fmt.Errorf("%w: name empty", ErrInvalidPurgeConfig)
	case c.Interval <= 0:
		return fmt.Errorf("%w: %s interval %v", ErrInvalidPurgeConfig, c.Name, c.Interval)
	case c.Batch <= 0:
		return fmt.Errorf("%w: %s batch %d", ErrInvalidPurgeConfig, c.Name, c.Batch)
	case c.Retention < 0:
		return fmt.Errorf("%w: %s retention %v", ErrInvalidPurgeConfig, c.Name, c.Retention)
	case clock == nil, logs == nil, fn == nil:
		return fmt.Errorf("%w: %s needs a clock, a logger and a purge function", ErrInvalidPurgeConfig, c.Name)
	}
	return nil
}

type PurgeFunc func(ctx context.Context, cutoff ports.Instant, batch int) (int64, error)

// StartPurge runs RunPurge in its own goroutine and passes the error that ends
// it to abort. stop cancels the loop and returns that error only once the loop
// has ended, so a caller can close the pool right after.
func StartPurge(ctx context.Context, abort context.CancelCauseFunc, cfg PurgeConfig, clock ports.Clock, logs log.LoggerProvider, fn PurgeFunc) (stop func() error, err error) {
	if err := validatePurge(cfg, clock, logs, fn); err != nil {
		return nil, err
	}
	if abort == nil {
		return nil, fmt.Errorf("%w: %s needs an abort function", ErrInvalidPurgeConfig, cfg.Name)
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	var ended error
	go func() {
		defer close(done)
		if ended = RunPurge(ctx, cfg, clock, logs, fn); ended != nil {
			abort(ended)
		}
	}()
	return func() error {
		cancel()
		<-done
		return ended
	}, nil
}

// RunPurge purges one batch every Interval until ctx is done, and returns nil
// then: stopping is not a failure. A full batch is followed at once by the next;
// a failed cycle is logged and retried on the next interval.
func RunPurge(ctx context.Context, cfg PurgeConfig, clock ports.Clock, logs log.LoggerProvider, fn PurgeFunc) (err error) {
	if err := validatePurge(cfg, clock, logs, fn); err != nil {
		return err
	}
	defer func() {
		if recover() != nil {
			err = ErrPurgePanicked
		}
	}()
	logger := logging.NewLogger(logs, reflect.TypeFor[PurgeConfig]().PkgPath())
	subject := cfg.subject()
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
					logger.WarnContext(ctx, "purge cycle failed", append(subject, redact.Error(err))...)
				}
				break
			}
			if removed > 0 {
				logger.InfoContext(ctx, "purged", append(subject, keyPurgeRemoved, removed, keyPurgeBefore, int64(cutoff))...)
			}
			if removed < int64(cfg.Batch) || ctx.Err() != nil {
				break
			}
		}
		timer.Reset(cfg.Interval)
	}
}

func PurgeOutbox(pool *pgxpool.Pool) PurgeFunc {
	return func(ctx context.Context, cutoff ports.Instant, batch int) (int64, error) {
		purged, err := postgres.PurgePublished(ctx, pool, cutoff, batch)
		return purged.Count, err
	}
}

// PurgeCommandInbox purges the command entries of consumer whose own expiry has
// come, so the loop runs it with no retention (IDM-09).
func PurgeCommandInbox(pool *pgxpool.Pool, consumer string) PurgeFunc {
	return func(ctx context.Context, cutoff ports.Instant, batch int) (int64, error) {
		purged, err := postgres.PurgeExpiredInbox(ctx, pool, consumer, cutoff, batch)
		return purged.Removed, err
	}
}

func PurgeMessageInbox(pool *pgxpool.Pool, consumer string) PurgeFunc {
	return func(ctx context.Context, cutoff ports.Instant, batch int) (int64, error) {
		purged, err := postgres.PurgeInbox(ctx, pool, consumer, cutoff, batch)
		return purged.Removed, err
	}
}
