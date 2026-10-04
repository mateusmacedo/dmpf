package app_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	lognoop "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
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

var purgeLog log.LoggerProvider = lognoop.NewLoggerProvider()

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
	ctx, abort := context.WithCancelCause(context.Background())
	defer abort(nil)
	stop, err := app.StartPurge(ctx, abort, app.PurgeConfig{Name: "outbox", Interval: 10 * time.Millisecond, Batch: 100}, purgeClock(1), purgeLog, rec.purge)
	if err != nil {
		t.Fatalf("StartPurge() = %v, want nil", err)
	}
	<-rec.called

	if err := stop(); err != nil || context.Cause(ctx) != nil {
		t.Fatalf("stop() = %v, cause = %v, want nil and nil: stopping the purge is not a failure of the role", err, context.Cause(ctx))
	}
	calls := len(rec.snapshot())
	time.Sleep(30 * time.Millisecond)
	if after := len(rec.snapshot()); after != calls {
		t.Fatalf("calls went from %d to %d after stop returned: the loop outlived the pool it purges", calls, after)
	}
}

func TestStartPurgeRefusesAConfigBeforeStarting(t *testing.T) {
	stop, err := app.StartPurge(context.Background(), func(error) {}, app.PurgeConfig{Name: "outbox", Batch: 100}, purgeClock(1), purgeLog, newPurgeRecorder().purge)

	if !errors.Is(err, app.ErrInvalidPurgeConfig) || stop != nil {
		t.Fatalf("StartPurge() = (stop set %v, %v), want ErrInvalidPurgeConfig and no loop", stop != nil, err)
	}
}

func runLoggedPurge(t *testing.T, role string, cfg app.PurgeConfig, fn app.PurgeFunc, called <-chan struct{}) map[string]any {
	t.Helper()
	logs := &purgeLogs{}
	provider := otelboot.NewLoggerProvider(otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource:   otelboot.Resource{ServiceName: "reservations", ServiceVersion: "1.0.0", ServiceInstanceID: "reservations-1", Role: role},
	}, logs)
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- app.RunPurge(ctx, cfg, purgeClock(10_000), provider, fn)
	}()
	<-called
	deadline := time.Now().Add(2 * time.Second)
	for len(logs.snapshot()) == 0 && time.Now().Before(deadline) {
		if err := provider.ForceFlush(context.Background()); err != nil {
			t.Fatalf("ForceFlush() = %v", err)
		}
		time.Sleep(time.Millisecond)
	}
	stop()
	<-done
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	records := logs.snapshot()
	if len(records) == 0 {
		t.Fatal("the purge logged no record")
	}
	return records[0]
}

func TestAFailedPurgeLogsTheRedactedErrorUnderCanonicalKeys(t *testing.T) {
	rec := newPurgeRecorder()
	rec.err = errors.New("purge_test: database away at 10.0.0.7")
	record := runLoggedPurge(t, "relay", app.PurgeConfig{Name: "outbox", Table: "outbox", Interval: time.Hour, Batch: 100}, rec.purge, rec.called)

	if record["msg"] != "purge cycle failed" || record["db.collection.name"] != "outbox" {
		t.Fatalf("record = %v, want purge cycle failed on db.collection.name=outbox", record)
	}
	if _, free := record["table"]; free {
		t.Fatalf("record = %v still carries the free key table", record)
	}
	for key, value := range record {
		if text, ok := value.(string); ok && strings.Contains(text, "database away") {
			t.Fatalf("%s = %q carries the error message", key, text)
		}
	}
	reduced := []slog.Attr{redact.Error(rec.err)}
	if reduced[0].Value.Kind() == slog.KindGroup {
		reduced = reduced[0].Value.Group()
	}
	for _, attr := range reduced {
		if record[attr.Key] != attr.Value.String() {
			t.Fatalf("%s = %v, want %q from redact.Error; record %v", attr.Key, record[attr.Key], attr.Value.String(), record)
		}
	}
}

func TestAPurgedBatchIsLoggedUnderCanonicalKeys(t *testing.T) {
	for _, role := range []string{"api", "consumer", "relay"} {
		t.Run(role, func(t *testing.T) {
			rec := newPurgeRecorder(3)
			record := runLoggedPurge(t, role, app.PurgeConfig{Name: "inbox", Table: "inbox", Interval: time.Hour, Batch: 100, Retention: 4_000}, rec.purge, rec.called)

			want := map[string]any{"msg": "purged", "scope": consumerScope, "db.collection.name": "inbox", "dmpf.purge.removed": int64(3), "dmpf.purge.before": int64(6_000)}
			for key, value := range want {
				if record[key] != value {
					t.Fatalf("%s = %v, want %v; record %v", key, record[key], value, record)
				}
			}
			for _, free := range []string{"table", "removed", "before"} {
				if _, ok := record[free]; ok {
					t.Fatalf("record = %v still carries the free key %s", record, free)
				}
			}
		})
	}
}

func TestAPurgeNamesItsTableApartFromItsLoop(t *testing.T) {
	for _, tc := range []struct {
		name    string
		removed []int64
		err     error
		msg     string
	}{
		{name: "failed cycle", err: errors.New("purge_test: database away"), msg: "purge cycle failed"},
		{name: "purged batch", removed: []int64{3}, msg: "purged"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := newPurgeRecorder(tc.removed...)
			rec.err = tc.err
			cfg := app.PurgeConfig{Name: "command-inbox", Table: "inbox", Interval: time.Hour, Batch: 100}

			record := runLoggedPurge(t, "api", cfg, rec.purge, rec.called)

			if record["msg"] != tc.msg || record["db.collection.name"] != "inbox" || record["dmpf.purge.name"] != "command-inbox" {
				t.Fatalf("record = %v, want %s on db.collection.name=inbox, the table of the DELETE span, and dmpf.purge.name=command-inbox", record, tc.msg)
			}
		})
	}
}

func TestAPurgeWithoutATableNeverPassesItsLoopNameAsOne(t *testing.T) {
	rec := newPurgeRecorder(3)

	record := runLoggedPurge(t, "api", app.PurgeConfig{Name: "command-inbox", Interval: time.Hour, Batch: 100}, rec.purge, rec.called)

	if _, named := record["db.collection.name"]; named || record["dmpf.purge.name"] != "command-inbox" {
		t.Fatalf("record = %v, want dmpf.purge.name=command-inbox and no db.collection.name", record)
	}
}

type purgeLogs struct {
	mu      sync.Mutex
	records []map[string]any
}

func (l *purgeLogs) Export(_ context.Context, records []sdklog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, record := range records {
		fields := map[string]any{"msg": record.Body().AsString(), "scope": record.InstrumentationScope().Name}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			fields[string(kv.Key)] = kv.Value.AsInterface()
			return true
		})
		l.records = append(l.records, fields)
	}
	return nil
}

func (l *purgeLogs) snapshot() []map[string]any {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]map[string]any(nil), l.records...)
}

func (*purgeLogs) Shutdown(context.Context) error   { return nil }
func (*purgeLogs) ForceFlush(context.Context) error { return nil }
