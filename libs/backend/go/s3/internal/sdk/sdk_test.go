package sdk_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	awslog "github.com/aws/smithy-go/logging"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/sdk"
)

type exporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *exporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range records {
		e.records = append(e.records, r.Clone())
	}
	return nil
}

func (e *exporter) Shutdown(context.Context) error   { return nil }
func (e *exporter) ForceFlush(context.Context) error { return nil }

// TestLoggerWithholdsTheDiagnostic proves the SDK's own text, which may carry a
// signed request, a body or a session token, never reaches the log (DAT-22).
func TestLoggerWithholdsTheDiagnostic(t *testing.T) {
	recorded := &exporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(recorded)))
	logger := sdk.Logger(provider).(awslog.ContextLogger).WithContext(context.Background())
	logger.Logf(awslog.Warn, "PUT /tenant/acme/secret-key X-Amz-Security-Token=%s body=%s", "secret-token", "secret-body")
	logger.Logf(awslog.Debug, "%s", "secret-debug")
	recorded.mu.Lock()
	defer recorded.mu.Unlock()
	if len(recorded.records) != 2 {
		t.Fatalf("records = %d, want 2", len(recorded.records))
	}
	for _, r := range recorded.records {
		body := r.Body().String()
		if strings.Contains(body, "secret") || !strings.Contains(body, "diagnostic withheld") {
			t.Fatalf("log body = %q, want only the fixed diagnostic", body)
		}
	}
}
