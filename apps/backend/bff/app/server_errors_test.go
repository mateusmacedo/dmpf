package app_test

import (
	"context"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

func TestNetHTTPLogsItsServerErrorsUnderItsOwnScopeWithoutTheirValues(t *testing.T) {
	const panicked = "sentinel-panic-value"
	logs := &memoryLogs{}
	telemetry := app.TelemetryOf(app.Defaults())
	config := otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{ServiceName: telemetry.Service, ServiceVersion: "test",
			ServiceInstanceID: "bff-1", Role: telemetry.Role},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	provider := otelboot.NewLoggerProvider(config, logs)
	config.LoggerProvider = provider
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(panicked) }))
	server.Config.ErrorLog = app.ServerErrorLog(runtime)
	server.StartTLS()
	t.Cleanup(server.Close)

	if res, err := server.Client().Get(server.URL); err == nil {
		_ = res.Body.Close()
		t.Fatalf("GET = %d, want the connection closed by the panic", res.StatusCode)
	}
	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("Dial() = %v", err)
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = conn.Write([]byte("sentinel-not-tls\r\n\r\n"))
	_, _ = io.Copy(io.Discard, conn)
	_ = conn.Close()

	want := map[string]string{
		"http: panic serving":        "http: panic serving " + redact.Placeholder,
		"http: TLS handshake error ": "http: TLS handshake error from " + redact.Placeholder,
	}
	served := serverErrors(t, provider, logs, want)
	for _, record := range served {
		body := record.Body().AsString()
		if scope := record.InstrumentationScope().Name; scope != "net/http" {
			t.Errorf("%q scope = %q, want net/http, the import path of the emitter (RF-A1)", body, scope)
		}
		if record.Severity() != log.SeverityWarn {
			t.Errorf("%q severity = %v, want WARN", body, record.Severity())
		}
		if !slices.Contains(slices.Collect(maps.Values(want)), body) {
			t.Errorf("body = %q, want only the statement of net/http, one of %v (DAT-02, DAT-23)", body, want)
		}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			for _, leaked := range []string{"127.0.0.1", panicked, "goroutine", "tls:"} {
				if strings.Contains(kv.Value.String(), leaked) {
					t.Errorf("%q %s = %q, want no %q of the error (DAT-02, DAT-23)", body, kv.Key, kv.Value.String(), leaked)
				}
			}
			return true
		})
	}
}

func serverErrors(t *testing.T, provider *sdklog.LoggerProvider, logs *memoryLogs, want map[string]string) []sdklog.Record {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := provider.ForceFlush(context.Background()); err != nil {
			t.Fatalf("ForceFlush() = %v", err)
		}
		logs.mu.Lock()
		var served []sdklog.Record
		seen := map[string]bool{}
		for _, record := range logs.records {
			body := record.Body().AsString()
			for prefix := range want {
				if strings.HasPrefix(body, prefix) {
					served = append(served, record)
					seen[prefix] = true
				}
			}
		}
		logs.mu.Unlock()
		if len(seen) == len(want) {
			return served
		}
		if time.Now().After(deadline) {
			t.Fatalf("server errors = %v, want one of each %v", served, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
