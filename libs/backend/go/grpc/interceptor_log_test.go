package grpc_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

func loggedRecords(t *testing.T, opts []kernel.ServerOption, handler grpc.UnaryHandler, pairs ...string) []map[string]any {
	t.Helper()
	ctrl, err := admission.New(admission.Config{
		Limits:  kernel.MethodLimits(chainService, []string{"Probe"}, generous),
		MaxKeys: 16,
		Clock:   clock.System(),
	})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	provider, logs := newMemoryLogs(slog.LevelDebug)
	chain := kernel.ServerInterceptors(chainService, ctrl, nil, provider, opts...)
	info := &grpc.UnaryServerInfo{FullMethod: chainMethod}
	next := handler
	for i := len(chain) - 1; i >= 0; i-- {
		interceptor, inner := chain[i], next
		next = func(ctx context.Context, req any) (any, error) { return interceptor(ctx, req, info, inner) }
	}
	_, _ = next(incoming(t, pairs...), nil)
	return logs.snapshot()
}

func loggedCall(t *testing.T, handler grpc.UnaryHandler, pairs ...string) map[string]any {
	t.Helper()
	records := loggedRecords(t, nil, handler, pairs...)
	for _, record := range records {
		if record["msg"] == "grpc call" {
			return record
		}
	}
	t.Fatalf("log = %v, want a grpc call record", records)
	return nil
}

func TestEveryCallIsLoggedWithItsCode(t *testing.T) {
	record := loggedCall(t, func(context.Context, any) (any, error) { return nil, nil })

	want := map[string]any{
		"rpc.system.name":          "grpc",
		"rpc.method":               probeSpanName,
		"rpc.response.status_code": "OK",
		tracing.KeyOutcomeCategory: "ok",
		"level":                    "INFO",
		"scope":                    grpcScope,
	}
	for key, value := range want {
		if record[key] != value {
			t.Errorf("grpc call %s = %v, want %v (RF-A3, RF-A5)", key, record[key], value)
		}
	}
	for _, gone := range []string{"duration_ms", "operation", "code", "error.type"} {
		if _, present := record[gone]; present {
			t.Errorf("grpc call = %v, want no %s: the duration is the span's and the keys are semconv (RF-A2, RF-A3)", record, gone)
		}
	}
}

func TestAFailedCallIsLoggedWithTheCodeOnly(t *testing.T) {
	record := loggedCall(t, func(context.Context, any) (any, error) {
		return nil, status.Error(codes.Unavailable, "secret detail")
	})

	if record["rpc.response.status_code"] != "UNAVAILABLE" || record["level"] != "ERROR" {
		t.Fatalf("grpc call = %v, want UNAVAILABLE at ERROR: a failure is error on the server (RF-A5)", record)
	}
	if record["error.type"] != "TransientDependency" || record[tracing.KeyOutcomeCategory] != "TransientDependency" {
		t.Fatalf("grpc call = %v, want the FND-07 category of the code as error.type and outcome (RF-A3, RF-B1)", record)
	}
	if line, _ := json.Marshal(record); strings.Contains(string(line), "secret") {
		t.Fatalf("grpc call = %v, want no error message (LOG-13)", record)
	}
}

func TestTheCallLevelFollowsTheServerSeverity(t *testing.T) {
	for name, c := range map[string]struct {
		err   error
		level string
	}{
		"rejected": {status.Error(codes.NotFound, "absent"), "INFO"},
		"denied":   {status.Error(codes.PermissionDenied, "forbidden"), "WARN"},
		"failed":   {status.Error(codes.Internal, "boom"), "ERROR"},
	} {
		t.Run(name, func(t *testing.T) {
			record := loggedCall(t, func(context.Context, any) (any, error) { return nil, c.err })

			if record["level"] != c.level {
				t.Fatalf("grpc call = %v, want %s: Severity(Server, %s) (RF-A5)", record, c.level, name)
			}
		})
	}
}

type categorizedFailure struct{}

func (categorizedFailure) Error() string         { return "secret detail" }
func (categorizedFailure) ErrorCategory() string { return "Unexpected" }
func (categorizedFailure) ErrorCode() string     { return "orders.lost" }

func TestACategorizedFailureIsLoggedByRedaction(t *testing.T) {
	record := loggedCall(t, func(context.Context, any) (any, error) { return nil, categorizedFailure{} })

	if record["error.type"] != "Unexpected" || record[tracing.KeyErrorCode] != "orders.lost" {
		t.Fatalf("grpc call = %v, want the FND-07 category and code from redact.Error (RF-A3)", record)
	}
	if line, _ := json.Marshal(record); strings.Contains(string(line), "secret") {
		t.Fatalf("grpc call = %v, want no error message (DAT-23)", record)
	}
}

func TestTheIdempotencyKeyIsLoggedOnTheCallRecord(t *testing.T) {
	for name, c := range map[string]struct {
		key     string
		present string
		absent  string
		value   any
	}{
		"valid":   {"k-1", tracing.KeyIdempotencyKey, tracing.KeyIdempotencyKeyInvalid, "k-1"},
		"invalid": {"k 1 forged=true", tracing.KeyIdempotencyKeyInvalid, tracing.KeyIdempotencyKey, true},
	} {
		t.Run(name, func(t *testing.T) {
			records := loggedRecords(t, nil, func(context.Context, any) (any, error) { return nil, nil },
				kernel.IdempotencyKey, c.key)

			if len(records) != 1 || records[0]["msg"] != "grpc call" {
				t.Fatalf("log = %v, want one grpc call record: grpc request is merged into it (RF-A5)", records)
			}
			if records[0][c.present] != c.value {
				t.Fatalf("grpc call = %v, want %s = %v (RF-A5)", records[0], c.present, c.value)
			}
			if _, logged := records[0][c.absent]; logged {
				t.Fatalf("grpc call = %v, want no %s", records[0], c.absent)
			}
		})
	}
}

func TestAnUnsentReplayHeaderIsLoggedByTheMethodOnly(t *testing.T) {
	records := loggedRecords(t, []kernel.ServerOption{kernel.WithCommands("Probe")},
		func(ctx context.Context, _ any) (any, error) {
			ports.MarkIdempotency(ctx, ports.IdempotencyReplayed)
			return nil, nil
		}, kernel.IdempotencyKey, "k-1")

	for _, record := range records {
		if record["msg"] != "grpc replay header not sent" {
			continue
		}
		if record["rpc.method"] != probeSpanName {
			t.Fatalf("replay warning = %v, want rpc.method %q", record, probeSpanName)
		}
		line, _ := json.Marshal(record)
		if _, legacy := record["operation"]; legacy || strings.Contains(string(line), kernel.ReplayedHeader) {
			t.Fatalf("replay warning = %s, want no operation key and no %s header", line, kernel.ReplayedHeader)
		}
		return
	}
	t.Fatalf("log = %v, want a grpc replay header not sent record outside a server stream", records)
}

func TestTheCallLogCarriesTheExecutionContext(t *testing.T) {
	record := loggedCall(t, func(context.Context, any) (any, error) { return nil, nil },
		kernel.CorrelationKey, "corr-1", kernel.TenantKey, "tenant-a")

	if record["correlation_id"] != "corr-1" || record["tenant_id"] != "tenant-a" {
		t.Fatalf("grpc call = %v, want the correlation and the tenant of the execution", record)
	}
}

func TestTheCallLogCarriesTheExecutionBaggage(t *testing.T) {
	record := loggedCall(t, func(context.Context, any) (any, error) { return nil, nil },
		kernel.CorrelationKey, "corr-1", kernel.TenantKey, "tenant-a")

	want := map[string]any{tracing.KeyCorrelationID: "corr-1", tracing.KeyTenantID: "tenant-a"}
	for key, value := range want {
		if record["baggage:"+key] != value {
			t.Errorf("grpc call baggage %s = %v, want %v: the log processor copies it to the record (RF-B8)", key, record["baggage:"+key], value)
		}
	}
	if record["baggage:"+tracing.KeyRequestID] == nil {
		t.Errorf("grpc call = %v, want the request id of this execution in the baggage (RF-B8)", record)
	}
}
