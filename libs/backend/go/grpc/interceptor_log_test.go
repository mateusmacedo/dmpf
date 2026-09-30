package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

type executionFields struct{ slog.Handler }

func (h executionFields) Handle(ctx context.Context, record slog.Record) error {
	if execution, ok := ports.ExecutionContextFrom(ctx); ok {
		record.AddAttrs(slog.String("correlation_id", execution.CorrelationID()))
		if tenant, scoped := execution.Tenant(); scoped {
			record.AddAttrs(slog.String("tenant_id", string(tenant)))
		}
	}
	return h.Handler.Handle(ctx, record)
}

func loggedCall(t *testing.T, handler grpc.UnaryHandler, pairs ...string) map[string]any {
	t.Helper()
	ctrl, err := admission.New(admission.Config{
		Limits:  kernel.MethodLimits(chainService, []string{"Probe"}, generous),
		MaxKeys: 16,
		Clock:   clock.System(),
	})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	var out bytes.Buffer
	logger := slog.New(executionFields{slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug})})
	chain := kernel.ServerInterceptors(chainService, noop.NewTracerProvider().Tracer("log-test"), ctrl, nil, logger)
	info := &grpc.UnaryServerInfo{FullMethod: chainMethod}
	next := handler
	for i := len(chain) - 1; i >= 0; i-- {
		interceptor, inner := chain[i], next
		next = func(ctx context.Context, req any) (any, error) { return interceptor(ctx, req, info, inner) }
	}
	_, _ = next(incoming(t, pairs...), nil)

	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var record map[string]any
		if json.Unmarshal([]byte(line), &record) == nil && record["msg"] == "grpc call" {
			return record
		}
	}
	t.Fatalf("log = %q, want a grpc call record", out.String())
	return nil
}

func TestEveryCallIsLoggedWithItsCode(t *testing.T) {
	record := loggedCall(t, func(context.Context, any) (any, error) { return nil, nil })

	if record["operation"] != chainMethod || record["code"] != "OK" || record["level"] != "DEBUG" {
		t.Fatalf("grpc call = %v, want the method, OK, at DEBUG", record)
	}
	if _, timed := record["duration_ms"]; !timed {
		t.Fatalf("grpc call = %v, want the duration", record)
	}
}

func TestAFailedCallIsLoggedWithTheCodeOnly(t *testing.T) {
	record := loggedCall(t, func(context.Context, any) (any, error) {
		return nil, status.Error(codes.Unavailable, "secret detail")
	})

	if record["code"] != "Unavailable" || record["level"] != "WARN" {
		t.Fatalf("grpc call = %v, want Unavailable at WARN", record)
	}
	if strings.Contains(record["msg"].(string)+record["code"].(string), "secret") {
		t.Fatalf("grpc call = %v, want no error message (LOG-13)", record)
	}
}

func TestTheCallLogCarriesTheExecutionContext(t *testing.T) {
	record := loggedCall(t, func(context.Context, any) (any, error) { return nil, nil },
		kernel.CorrelationKey, "corr-1", kernel.TenantKey, "tenant-a")

	if record["correlation_id"] != "corr-1" || record["tenant_id"] != "tenant-a" {
		t.Fatalf("grpc call = %v, want the correlation and the tenant of the execution", record)
	}
}
