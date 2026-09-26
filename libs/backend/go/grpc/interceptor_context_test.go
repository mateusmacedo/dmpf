package grpc_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

const chainService = "company.probe.service.v1.ProbeService"

var chainMethod = "/" + chainService + "/Probe"

func chainOf(t *testing.T, limit admission.Limit) grpc.UnaryServerInterceptor {
	t.Helper()
	ctrl, err := admission.New(admission.Config{
		Limits:  kernel.MethodLimits(chainService, []string{"Probe"}, limit),
		MaxKeys: 16,
		Clock:   clock.System(),
	})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	chain := kernel.ServerInterceptors(chainService, noop.NewTracerProvider().Tracer("chain-test"), ctrl, nil, nil)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		next := handler
		for i := len(chain) - 1; i >= 0; i-- {
			interceptor, inner := chain[i], next
			next = func(ctx context.Context, req any) (any, error) { return interceptor(ctx, req, info, inner) }
		}
		return next(ctx, req)
	}
}

func incoming(t *testing.T, pairs ...string) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return metadata.NewIncomingContext(ctx, metadata.Pairs(pairs...))
}

var generous = admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 100}

func TestTheChainRefusesACallWithoutDeadline(t *testing.T) {
	called := false
	_, err := chainOf(t, generous)(metadata.NewIncomingContext(context.Background(), metadata.MD{}), nil,
		&grpc.UnaryServerInfo{FullMethod: chainMethod},
		func(context.Context, any) (any, error) { called = true; return nil, nil })

	if status.Code(err) != codes.InvalidArgument || called {
		t.Fatalf("call without deadline = %v, handler called = %t; want InvalidArgument before the handler (GRP-04)", err, called)
	}
}

func TestTheChainRebuildsTheExecutionContextFromTheMetadata(t *testing.T) {
	var got ports.ExecutionContext
	_, err := chainOf(t, generous)(incoming(t,
		kernel.CorrelationKey, "corr-1", kernel.CausationKey, "bff-req-1", kernel.TenantKey, "acme", kernel.LocaleKey, "pt-BR"),
		nil, &grpc.UnaryServerInfo{FullMethod: chainMethod},
		func(ctx context.Context, _ any) (any, error) {
			got, _ = ports.ExecutionContextFrom(ctx)
			return nil, nil
		})
	if err != nil {
		t.Fatalf("call = %v", err)
	}

	if tenant, scoped := got.Tenant(); !scoped || tenant != "acme" {
		t.Errorf("Tenant() = %q, %t; want acme (CTX-13)", tenant, scoped)
	}
	if got.CorrelationID() != "corr-1" {
		t.Errorf("CorrelationID() = %q, want corr-1 (CTX-07)", got.CorrelationID())
	}
	if causation, caused := got.CausationID(); !caused || causation != "bff-req-1" {
		t.Errorf("CausationID() = %q, %t; want bff-req-1 (CTX-08)", causation, caused)
	}
	if got.Locale() != "pt-BR" {
		t.Errorf("Locale() = %q, want pt-BR (CTX-11)", got.Locale())
	}
	if got.Deadline() <= 0 {
		t.Errorf("Deadline() = %d, want the governed instant (GRP-04)", got.Deadline())
	}
}

func TestTheChainReplacesAMalformedCorrelationAndDefaultsTheLocale(t *testing.T) {
	var got ports.ExecutionContext
	if _, err := chainOf(t, generous)(incoming(t, kernel.CorrelationKey, "not valid!", kernel.LocaleKey, "pt BR"),
		nil, &grpc.UnaryServerInfo{FullMethod: chainMethod},
		func(ctx context.Context, _ any) (any, error) {
			got, _ = ports.ExecutionContextFrom(ctx)
			return nil, nil
		}); err != nil {
		t.Fatalf("call = %v", err)
	}

	if c := got.CorrelationID(); c == "not valid!" || !regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`).MatchString(c) {
		t.Errorf("CorrelationID() = %q, want a minted correlation", c)
	}
	if got.Locale() != kernel.DefaultLocale {
		t.Errorf("Locale() = %q, want %q", got.Locale(), kernel.DefaultLocale)
	}
	if _, scoped := got.Tenant(); scoped {
		t.Error("Tenant() resolved a tenant nobody sent (IDN-20)")
	}
}

func TestTheChainLeavesMethodsOfOtherServicesAlone(t *testing.T) {
	called := false
	_, err := chainOf(t, generous)(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"},
		func(context.Context, any) (any, error) { called = true; return nil, nil })

	if err != nil || !called {
		t.Fatalf("health probe = %v, handler called = %t; want it answered without deadline or admission", err, called)
	}
}

func TestTheChainAdmitsPerTenant(t *testing.T) {
	chain := chainOf(t, admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 1})
	call := func(tenant string) error {
		_, err := chain(incoming(t, kernel.TenantKey, tenant), nil, &grpc.UnaryServerInfo{FullMethod: chainMethod},
			func(context.Context, any) (any, error) { return nil, nil })
		return err
	}

	if err := call("acme"); err != nil {
		t.Fatalf("first acme call = %v, want admitted", err)
	}
	if err := call("acme"); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("second acme call = %v, want ResourceExhausted (RES-17)", err)
	}
	if err := call("globex"); err != nil {
		t.Fatalf("globex call = %v, want admitted: the buckets are per tenant (RES-16)", err)
	}
}

func TestMethodLimitsDeclaresEveryMethodOfTheService(t *testing.T) {
	limit := admission.Limit{PerSecond: 5, Burst: 5, Concurrency: 1}
	limits := kernel.MethodLimits(chainService, []string{"Probe", "Find"}, limit)

	for _, method := range []string{chainMethod, "/" + chainService + "/Find"} {
		if limits[method] != limit {
			t.Errorf("limit of %s = %+v, want %+v", method, limits[method], limit)
		}
	}
	if len(limits) != 2 {
		t.Errorf("limits = %v, want exactly the two methods", limits)
	}
}
