package dmpfgrpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	dmpfgrpc "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-grpc"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/deadline"
)

// recordingInvoker captures the context the interceptor hands to the invoker
// without touching the network.
func recordingInvoker(captured *context.Context) grpc.UnaryInvoker {
	return func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		*captured = ctx
		return nil
	}
}

func TestDeadlineUnaryInterceptorDerivesTheOutgoingDeadline(t *testing.T) {
	c := clock.NewFake(start)
	cfg := validConfig()
	cfg.Clock = c
	interceptor := dmpfgrpc.DeadlineUnaryInterceptor(cfg)

	ctx, cancel := c.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var seen context.Context
	if err := interceptor(ctx, checkMethod, nil, nil, nil, recordingInvoker(&seen)); err != nil {
		t.Fatalf("interceptor = %v, want nil", err)
	}
	got, ok := seen.Deadline()
	if !ok {
		t.Fatal("the invoker received a context with no deadline")
	}
	// Limit 1s, slack 50ms: the hop gets 950ms, below the caller's 10s.
	if want := start.Add(950 * time.Millisecond); !got.Equal(want) {
		t.Fatalf("deadline = %v, want %v", got, want)
	}
}

func TestDeadlineUnaryInterceptorRefusesBeforeInvoking(t *testing.T) {
	c := clock.NewFake(start)
	cfg := validConfig()
	cfg.Clock = c
	interceptor := dmpfgrpc.DeadlineUnaryInterceptor(cfg)

	cases := map[string]struct {
		ctx    func() context.Context
		method string
		want   error
	}{
		"no deadline": {func() context.Context { return context.Background() }, checkMethod, deadline.ErrNoDeadline},
		"exhausted": {func() context.Context {
			ctx, cancel := c.WithTimeout(context.Background(), 50*time.Millisecond)
			t.Cleanup(cancel)
			return ctx
		}, checkMethod, deadline.ErrDeadlineExhausted},
		"undeclared method": {func() context.Context {
			ctx, cancel := c.WithTimeout(context.Background(), time.Second)
			t.Cleanup(cancel)
			return ctx
		}, "/orders.v1.Orders/Place", dmpfgrpc.ErrMethodNotDeclared},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			invoked := false
			invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
				invoked = true
				return nil
			}
			err := interceptor(tc.ctx(), tc.method, nil, nil, nil, invoker)
			if !errors.Is(err, tc.want) {
				t.Fatalf("interceptor = %v, want %v", err, tc.want)
			}
			if invoked {
				t.Fatal("the invoker ran: a refused call must not reach the wire (CTX-21)")
			}
		})
	}
}

func TestDeadlineStreamInterceptorRefusesBeforeStreaming(t *testing.T) {
	c := clock.NewFake(start)
	cfg := validConfig()
	cfg.Clock = c
	interceptor := dmpfgrpc.DeadlineStreamInterceptor(cfg)

	streamed := false
	streamer := func(context.Context, *grpc.StreamDesc, *grpc.ClientConn, string, ...grpc.CallOption) (grpc.ClientStream, error) {
		streamed = true
		return nil, nil
	}
	_, err := interceptor(context.Background(), &grpc.StreamDesc{}, nil, checkMethod, streamer)
	if !errors.Is(err, deadline.ErrNoDeadline) {
		t.Fatalf("interceptor = %v, want ErrNoDeadline", err)
	}
	if streamed {
		t.Fatal("the streamer ran: a refused stream must not reach the wire")
	}
}

func TestDeadlineStreamInterceptorBoundsTheStreamContext(t *testing.T) {
	c := clock.NewFake(start)
	cfg := validConfig()
	cfg.Clock = c
	interceptor := dmpfgrpc.DeadlineStreamInterceptor(cfg)

	ctx, cancel := c.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var seen context.Context
	streamer := func(ctx context.Context, _ *grpc.StreamDesc, _ *grpc.ClientConn, _ string, _ ...grpc.CallOption) (grpc.ClientStream, error) {
		seen = ctx
		return &fakeStream{}, nil
	}
	if _, err := interceptor(ctx, &grpc.StreamDesc{}, nil, checkMethod, streamer); err != nil {
		t.Fatalf("interceptor = %v, want nil", err)
	}
	if got, ok := seen.Deadline(); !ok || !got.Equal(start.Add(950*time.Millisecond)) {
		t.Fatalf("stream deadline = %v (%v), want %v", got, ok, start.Add(950*time.Millisecond))
	}
}

// fakeStream is a client stream whose calls fail on demand, so the wrapper's
// cancellation on a terminal error is observable through the derived context.
type fakeStream struct {
	grpc.ClientStream
	headerErr, sendErr, recvErr, closeErr error
}

func (f *fakeStream) Header() (metadata.MD, error) { return nil, f.headerErr }
func (f *fakeStream) SendMsg(any) error            { return f.sendErr }
func (f *fakeStream) RecvMsg(any) error            { return f.recvErr }
func (f *fakeStream) CloseSend() error             { return f.closeErr }
func (f *fakeStream) Context() context.Context     { return context.Background() }
func (f *fakeStream) Trailer() metadata.MD         { return nil }

func TestDeadlineStreamInterceptorCancelsOnTheFirstTerminalError(t *testing.T) {
	c := clock.NewFake(start)
	cfg := validConfig()
	cfg.Clock = c
	interceptor := dmpfgrpc.DeadlineStreamInterceptor(cfg)
	boom := errors.New("boom")

	cases := map[string]struct {
		fake *fakeStream
		act  func(grpc.ClientStream) error
	}{
		"Header":    {&fakeStream{headerErr: boom}, func(s grpc.ClientStream) error { _, err := s.Header(); return err }},
		"SendMsg":   {&fakeStream{sendErr: boom}, func(s grpc.ClientStream) error { return s.SendMsg(nil) }},
		"RecvMsg":   {&fakeStream{recvErr: boom}, func(s grpc.ClientStream) error { return s.RecvMsg(nil) }},
		"CloseSend": {&fakeStream{closeErr: boom}, func(s grpc.ClientStream) error { return s.CloseSend() }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := c.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var derived context.Context
			streamer := func(ctx context.Context, _ *grpc.StreamDesc, _ *grpc.ClientConn, _ string, _ ...grpc.CallOption) (grpc.ClientStream, error) {
				derived = ctx
				return tc.fake, nil
			}
			stream, err := interceptor(ctx, &grpc.StreamDesc{}, nil, checkMethod, streamer)
			if err != nil {
				t.Fatal(err)
			}
			if derived.Err() != nil {
				t.Fatal("the derived context is cancelled before any terminal error")
			}
			if err := tc.act(stream); !errors.Is(err, boom) {
				t.Fatalf("%s = %v, want boom", name, err)
			}
			if !errors.Is(derived.Err(), context.Canceled) {
				t.Fatalf("derived context after %s error = %v, want cancelled", name, derived.Err())
			}
		})
	}

	t.Run("a successful CloseSend is only the half-close", func(t *testing.T) {
		ctx, cancel := c.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var derived context.Context
		streamer := func(ctx context.Context, _ *grpc.StreamDesc, _ *grpc.ClientConn, _ string, _ ...grpc.CallOption) (grpc.ClientStream, error) {
			derived = ctx
			return &fakeStream{}, nil
		}
		stream, _ := interceptor(ctx, &grpc.StreamDesc{}, nil, checkMethod, streamer)
		if err := stream.CloseSend(); err != nil || derived.Err() != nil {
			t.Fatalf("CloseSend = %v, derived = %v; want nil and a live context", err, derived.Err())
		}
	})
}
