package http_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	sdklog "go.opentelemetry.io/otel/sdk/log"
)

const observeScope = "github.com/mateusmacedo/dmpf/libs/backend/go/transport/observe"

type recordingExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (e *recordingExporter) Shutdown(context.Context) error   { return nil }
func (e *recordingExporter) ForceFlush(context.Context) error { return nil }

func (e *recordingExporter) snapshot() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdklog.Record(nil), e.records...)
}

func TestEveryFailedAttemptIsLoggedThroughTheProviderOfTheConfig(t *testing.T) {
	exporter := &recordingExporter{}
	up := newUpstream(t, http.StatusServiceUnavailable)
	cfg := clientConfig(route(http.MethodGet))
	cfg.LoggerProvider = sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	client := newClient(t, cfg)

	resp, err := client.Do(budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodGet, up.URL, ""))
	if err != nil {
		t.Fatalf("Do() = %v, want the last 503 response", err)
	}
	_ = resp.Body.Close()

	failed := 0
	for _, record := range exporter.snapshot() {
		if record.Body().AsString() != "transport: call failed" {
			continue
		}
		failed++
		if scope := record.InstrumentationScope().Name; scope != observeScope {
			t.Errorf("scope = %q, want the import path of the emitting package %q (RF-A1)", scope, observeScope)
		}
	}
	if failed != up.count() {
		t.Fatalf("logged %d failed attempts, want one per request the upstream saw (%d)", failed, up.count())
	}
}
