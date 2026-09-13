package dmpfgrpc_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

// hop is a Health server whose Check records the deadline it received and,
// when it has a next hop, forwards the check through it: the chain A → B → C of
// the two-hops test is three of these, each on its own bufconn.
type hop struct {
	healthpb.UnimplementedHealthServer
	name      string
	deadlines chan deadlineSeen
	next      healthpb.HealthClient
	block     <-chan struct{}
}

type deadlineSeen struct {
	hop      string
	deadline int64
	ok       bool
	err      error
}

func (h *hop) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	d, ok := ctx.Deadline()
	seen := deadlineSeen{hop: h.name, deadline: d.UnixNano(), ok: ok}
	if h.block != nil {
		select {
		case <-h.block:
		case <-ctx.Done():
			seen.err = ctx.Err()
			h.deadlines <- seen
			return nil, ctx.Err()
		}
	}
	if h.next != nil {
		_, seen.err = h.next.Check(ctx, req)
	}
	h.deadlines <- seen
	if seen.err != nil {
		return nil, seen.err
	}
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

// serve starts a gRPC server on a bufconn and returns a dialer for it.
func serve(t *testing.T, server healthpb.HealthServer, opts ...grpc.ServerOption) func(context.Context, string) (net.Conn, error) {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(opts...)
	healthpb.RegisterHealthServer(srv, server)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = listener.Close()
	})
	return func(context.Context, string) (net.Conn, error) { return listener.Dial() }
}

// connect opens a client to a bufconn server with the given dial options. The
// target must be passthrough: NewClient resolves by dns otherwise.
func connect(t *testing.T, dialer func(context.Context, string) (net.Conn, error), opts ...grpc.DialOption) *grpc.ClientConn {
	t.Helper()
	all := append([]grpc.DialOption{
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}, opts...)
	conn, err := grpc.NewClient("passthrough:///bufnet", all...)
	if err != nil {
		t.Fatalf("NewClient() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// listen serves an already built server on a bufconn and returns its dialer.
func listen(t *testing.T, srv *grpc.Server) func(context.Context, string) (net.Conn, error) {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = listener.Close()
	})
	return func(context.Context, string) (net.Conn, error) { return listener.Dial() }
}

// healthClient serves the server on a bufconn and returns a health client to it.
func healthClient(t *testing.T, srv *grpc.Server) healthpb.HealthClient {
	t.Helper()
	return healthpb.NewHealthClient(connect(t, listen(t, srv)))
}
