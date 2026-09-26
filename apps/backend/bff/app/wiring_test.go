package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
)

func TestTheReadinessLineCarriesThePortTheKernelChose(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true", "HTTP_ADDR", "127.0.0.1:0", devMock, "true")...))
	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	var logs logSink
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime, err := boot.StartTelemetry(ctx, &logs, app.TelemetryOf(cfg))
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	done := make(chan error, 1)
	go func() { done <- app.RunWith(ctx, cfg, runtime) }()

	addr := listeningAddr(t, &logs)
	if _, port, err := net.SplitHostPort(addr); err != nil || port == "0" || port == "" {
		t.Fatalf("addr = %q, want the address the listener resolved, not the configured one", addr)
	}
	if _, err := net.DialTimeout("tcp", addr, time.Second); err != nil {
		t.Fatalf("dial %s = %v, want the edge accepting on the address it logged", addr, err)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("RunWith() = %v, want nil on shutdown", err)
	}
}

// WHY: the edge writes its readiness line from the goroutine running the server
// while the test polls it from another; bytes.Buffer alone is a data race.
type logSink struct {
	mu  sync.Mutex
	out bytes.Buffer
}

func (s *logSink) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out.Write(b)
}

func (s *logSink) logs() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out.String()
}

func listeningAddr(t *testing.T, logs *logSink) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, line := range strings.Split(logs.logs(), "\n") {
			var record map[string]any
			if json.Unmarshal([]byte(line), &record) == nil && record["msg"] == "http listening" {
				addr, _ := record["addr"].(string)
				return addr
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no readiness line in %s", logs.logs())
	return ""
}
