package app_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type purgeClock ports.Instant

func (c purgeClock) Now() ports.Instant { return ports.Instant(c) }

type purgeCall struct {
	cutoff ports.Instant
	batch  int
}

type purgeRecorder struct {
	mu      sync.Mutex
	calls   []purgeCall
	results []int64
	err     error
	called  chan struct{}
}

func newPurgeRecorder(results ...int64) *purgeRecorder {
	return &purgeRecorder{results: results, called: make(chan struct{}, 16)}
}

func (r *purgeRecorder) purge(_ context.Context, cutoff ports.Instant, batch int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, purgeCall{cutoff: cutoff, batch: batch})
	removed := int64(0)
	if len(r.results) > 0 {
		removed, r.results = r.results[0], r.results[1:]
	}
	r.called <- struct{}{}
	return removed, r.err
}

func (r *purgeRecorder) snapshot() []purgeCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]purgeCall(nil), r.calls...)
}

var purgeLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func runPurge(t *testing.T, cfg app.PurgeConfig, fn app.PurgeFunc) (cancel func() error) {
	t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.RunPurge(ctx, cfg, purgeClock(10_000), purgeLog, fn) }()
	return func() error {
		stop()
		select {
		case err := <-done:
			return err
		case <-time.After(2 * time.Second):
			t.Fatal("Run did not stop after its context was cancelled")
			return nil
		}
	}
}

func TestRunPurgesTheRetentionBeforeNowInBatches(t *testing.T) {
	rec := newPurgeRecorder()
	stop := runPurge(t, app.PurgeConfig{Name: "outbox", Interval: 10 * time.Millisecond, Batch: 100, Retention: 4_000}, rec.purge)

	<-rec.called
	<-rec.called
	if err := stop(); err != nil {
		t.Fatalf("Run() = %v, want nil: stopping is not a failure", err)
	}
	for _, c := range rec.snapshot() {
		if c.cutoff != 6_000 || c.batch != 100 {
			t.Fatalf("purgeCall = %+v, want cutoff now − retention (6000) and batch 100", c)
		}
	}
}

func TestRunWithoutRetentionCutsAtNow(t *testing.T) {
	rec := newPurgeRecorder()
	stop := runPurge(t, app.PurgeConfig{Name: "commands", Interval: 10 * time.Millisecond, Batch: 100}, rec.purge)

	<-rec.called
	_ = stop()
	if c := rec.snapshot()[0]; c.cutoff != 10_000 {
		t.Fatalf("cutoff = %d, want now (10000): an entry carrying its own expiry is not aged again", c.cutoff)
	}
}

func TestAFullBatchIsFollowedAtOnceByTheNext(t *testing.T) {
	rec := newPurgeRecorder(100, 100, 3)
	stop := runPurge(t, app.PurgeConfig{Name: "outbox", Interval: time.Hour, Batch: 100}, rec.purge)

	for range 3 {
		select {
		case <-rec.called:
		case <-time.After(2 * time.Second):
			t.Fatal("a full batch waited for the next interval; a backlog would take hours to drain")
		}
	}
	_ = stop()
	if got := len(rec.snapshot()); got != 3 {
		t.Fatalf("calls = %d, want 3: two full batches, then one that came back short", got)
	}
}

func TestAFailedPurgeWaitsForTheNextInterval(t *testing.T) {
	rec := newPurgeRecorder()
	rec.err = errors.New("purge_test: database away")
	stop := runPurge(t, app.PurgeConfig{Name: "outbox", Interval: 10 * time.Millisecond, Batch: 100}, rec.purge)

	<-rec.called
	<-rec.called
	if err := stop(); err != nil {
		t.Fatalf("Run() = %v, want nil: a failed cycle is retried on the next interval, not fatal", err)
	}
}

func TestRunRefusesAConfigWithoutIntervalOrBatch(t *testing.T) {
	for name, cfg := range map[string]app.PurgeConfig{
		"no interval": {Name: "outbox", Batch: 100},
		"no batch":    {Name: "outbox", Interval: time.Second},
		"no name":     {Interval: time.Second, Batch: 100},
		"negative":    {Name: "outbox", Interval: time.Second, Batch: 100, Retention: -1},
	} {
		t.Run(name, func(t *testing.T) {
			err := app.RunPurge(context.Background(), cfg, purgeClock(1), purgeLog, newPurgeRecorder().purge)
			if !errors.Is(err, app.ErrInvalidPurgeConfig) {
				t.Fatalf("Run() = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestRunRefusesALoopWithoutItsCollaborators(t *testing.T) {
	cfg := app.PurgeConfig{Name: "outbox", Interval: time.Second, Batch: 100}
	for name, run := range map[string]func() error{
		"no clock": func() error { return app.RunPurge(context.Background(), cfg, nil, purgeLog, newPurgeRecorder().purge) },
		"no logger": func() error {
			return app.RunPurge(context.Background(), cfg, purgeClock(1), nil, newPurgeRecorder().purge)
		},
		"no purge": func() error { return app.RunPurge(context.Background(), cfg, purgeClock(1), purgeLog, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); !errors.Is(err, app.ErrInvalidPurgeConfig) {
				t.Fatalf("RunPurge() = %v, want ErrInvalidPurgeConfig: the loop would panic on its first cycle", err)
			}
		})
	}
}

func TestStartPurgeRunsUntilStopReturns(t *testing.T) {
	rec := newPurgeRecorder()
	stop, err := app.StartPurge(context.Background(), app.PurgeConfig{Name: "outbox", Interval: 10 * time.Millisecond, Batch: 100}, purgeClock(1), purgeLog, rec.purge)
	if err != nil {
		t.Fatalf("StartPurge() = %v, want nil", err)
	}
	<-rec.called

	stop()
	calls := len(rec.snapshot())
	time.Sleep(30 * time.Millisecond)
	if after := len(rec.snapshot()); after != calls {
		t.Fatalf("calls went from %d to %d after stop returned: the loop outlived the pool it purges", calls, after)
	}
}

func TestStartPurgeRefusesAConfigBeforeStarting(t *testing.T) {
	stop, err := app.StartPurge(context.Background(), app.PurgeConfig{Name: "outbox", Batch: 100}, purgeClock(1), purgeLog, newPurgeRecorder().purge)

	if !errors.Is(err, app.ErrInvalidPurgeConfig) || stop != nil {
		t.Fatalf("StartPurge() = (stop set %v, %v), want ErrInvalidPurgeConfig and no loop", stop != nil, err)
	}
}
