package grpc_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

const trustedWorkload = "bff"

func platformChain(t *testing.T, opts ...kernel.ServerOption) grpc.UnaryServerInterceptor {
	t.Helper()
	return platformChainOf(t, kernel.TrustedPeers([]string{trustedWorkload}), opts...)
}

func untrustingChain(t *testing.T, opts ...kernel.ServerOption) grpc.UnaryServerInterceptor {
	t.Helper()
	return platformChainOf(t, nil, opts...)
}

func platformChainOf(t *testing.T, first grpc.UnaryServerInterceptor, opts ...kernel.ServerOption) grpc.UnaryServerInterceptor {
	t.Helper()
	ctrl, err := admission.New(admission.Config{
		Limits:  kernel.MethodLimits(chainService, []string{"Probe", "Find"}, generous),
		MaxKeys: 16,
		Clock:   clock.System(),
	})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	chain := kernel.ServerInterceptors(chainService, ctrl, nil, nil, opts...)
	if first != nil {
		chain = append([]grpc.UnaryServerInterceptor{first}, chain...)
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		next := handler
		for i := len(chain) - 1; i >= 0; i-- {
			interceptor, inner := chain[i], next
			next = func(ctx context.Context, req any) (any, error) { return interceptor(ctx, req, info, inner) }
		}
		return next(ctx, req)
	}
}

func fromPeer(ctx context.Context, state tls.ConnectionState) context.Context {
	return peer.NewContext(ctx, &peer.Peer{AuthInfo: credentials.TLSInfo{State: state}})
}

func verifiedCaller(t *testing.T, pairs ...string) context.Context {
	t.Helper()
	return verifiedAs(incoming(t, pairs...), trustedWorkload)
}

func verifiedAs(ctx context.Context, workload string) context.Context {
	return fromPeer(ctx, tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{{DNSNames: []string{workload}}}}})
}

func reachOfCall(t *testing.T, chain grpc.UnaryServerInterceptor, method string, pairs ...string) (ports.PlatformReach, bool) {
	t.Helper()
	var (
		reach    ports.PlatformReach
		declared bool
	)
	_, err := chain(verifiedCaller(t, pairs...), nil, &grpc.UnaryServerInfo{FullMethod: "/" + chainService + "/" + method},
		func(ctx context.Context, _ any) (any, error) {
			reach, declared = ports.PlatformReachFrom(ctx)
			return nil, nil
		})
	if err != nil {
		t.Fatalf("call %s = %v", method, err)
	}
	return reach, declared
}

func TestTheChainDepositsTheReachOnlyOnADeclaredPlatformMethod(t *testing.T) {
	chain := platformChain(t, kernel.WithPlatformMethods("Probe"))

	if reach, declared := reachOfCall(t, chain, "Probe"); !declared || reach != ports.ReachAllTenants {
		t.Fatalf("declared method reach = %v, %v, want ReachAllTenants", reach, declared)
	}
	if reach, declared := reachOfCall(t, chain, "Find"); declared {
		t.Fatalf("undeclared method reach = %v, want none", reach)
	}
}

func TestTheChainNeverDepositsTheReachFromTheCallerMetadata(t *testing.T) {
	chain := platformChain(t)

	for _, method := range []string{"Probe", "Find"} {
		if reach, declared := reachOfCall(t, chain, method, "x-platform-reach", "all-tenants", "platform-reach", "1"); declared {
			t.Fatalf("%s reach = %v from metadata, want none", method, reach)
		}
	}
}

func TestTheChainRefusesAPlatformMethodToACallerItDidNotVerify(t *testing.T) {
	chain := untrustingChain(t, kernel.WithPlatformMethods("Probe"))

	for name, ctx := range map[string]context.Context{
		"plaintext":           incoming(t),
		"tls without a chain": fromPeer(incoming(t), tls.ConnectionState{}),
	} {
		t.Run(name, func(t *testing.T) {
			called := false
			_, err := chain(ctx, nil, &grpc.UnaryServerInfo{FullMethod: chainMethod},
				func(context.Context, any) (any, error) { called = true; return nil, nil })

			if status.Code(err) != codes.PermissionDenied || called {
				t.Fatalf("call = %v, handler called %v, want PermissionDenied before the handler", err, called)
			}
		})
	}
}

func TestTheChainRefusesAPlatformMethodToAVerifiedWorkloadOutsideTheAllowlist(t *testing.T) {
	for name, chain := range map[string]grpc.UnaryServerInterceptor{
		"with TrustedPeers":    platformChain(t, kernel.WithPlatformMethods("Probe")),
		"without TrustedPeers": untrustingChain(t, kernel.WithPlatformMethods("Probe")),
	} {
		t.Run(name, func(t *testing.T) {
			called := false
			_, err := chain(verifiedAs(incoming(t), "intruder"), nil, &grpc.UnaryServerInfo{FullMethod: chainMethod},
				func(context.Context, any) (any, error) { called = true; return nil, nil })

			if status.Code(err) != codes.PermissionDenied || called {
				t.Fatalf("call = %v, handler called %v, want PermissionDenied before the handler", err, called)
			}
		})
	}
}

func TestTheChainRefusesAPlatformMethodWhenNoAllowlistAdmittedTheCaller(t *testing.T) {
	chain := untrustingChain(t, kernel.WithPlatformMethods("Probe"))

	called := false
	_, err := chain(verifiedCaller(t), nil, &grpc.UnaryServerInfo{FullMethod: chainMethod},
		func(context.Context, any) (any, error) { called = true; return nil, nil })

	if status.Code(err) != codes.PermissionDenied || called {
		t.Fatalf("call = %v, handler called %v, want PermissionDenied: a verified chain alone grants no reach", err, called)
	}
}

func TestTheChainServesAnOrdinaryMethodToAVerifiedCallerWithoutAnAllowlist(t *testing.T) {
	chain := untrustingChain(t, kernel.WithPlatformMethods("Probe"))

	reach, declared := reachOfCall(t, chain, "Find")

	if declared {
		t.Fatalf("ordinary method reach = %v, want none", reach)
	}
}

func TestTrustedStreamPeersMarksOnlyAWorkloadOfTheAllowlist(t *testing.T) {
	interceptor := kernel.TrustedStreamPeers([]string{trustedWorkload})

	for workload, want := range map[string]bool{trustedWorkload: true, "intruder": false} {
		t.Run(workload, func(t *testing.T) {
			marked, called := false, false
			err := interceptor(nil, streamOf(verifiedAs(context.Background(), workload)), &grpc.StreamServerInfo{},
				func(_ any, stream grpc.ServerStream) error {
					called, marked = true, kernel.PeerTrusted(stream.Context())
					return nil
				})

			if called != want || marked != want || (err == nil) != want {
				t.Fatalf("handler called %v, marked %v, err %v; want %v", called, marked, err, want)
			}
		})
	}
}

type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s contextStream) Context() context.Context { return s.ctx }

func streamOf(ctx context.Context) grpc.ServerStream { return contextStream{ctx: ctx} }

func TestTheChainServesAnOrdinaryMethodToAnUnverifiedCaller(t *testing.T) {
	chain := untrustingChain(t, kernel.WithPlatformMethods("Probe"))

	_, err := chain(incoming(t), nil, &grpc.UnaryServerInfo{FullMethod: "/" + chainService + "/Find"},
		func(context.Context, any) (any, error) { return nil, nil })

	if err != nil {
		t.Fatalf("call Find = %v, want nil: only a platform method demands a verified caller", err)
	}
}
