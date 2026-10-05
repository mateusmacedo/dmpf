package rpc

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
)

// Call is what the edge authored for one request and hands to the contexts:
// the correlation of the chain, the edge's own request id as causation, and the
// tenant CTX-13 preserves across the hop.
//
// WHY: the authenticated subject and its permissions are deliberately absent.
// CTX-12 turns them into provenance at this boundary, and the callee resolves
// its own caller identity instead of trusting one the edge asserts (IDN-02).
type Call struct {
	CorrelationID  string
	RequestID      string
	IdempotencyKey string
	TenantID       string
	Locale         string
}

type callKey struct{}

func WithCall(ctx context.Context, call Call) context.Context {
	return context.WithValue(ctx, callKey{}, call)
}

func CallFrom(ctx context.Context) (Call, bool) {
	call, ok := ctx.Value(callKey{}).(Call)
	return call, ok
}

func contextInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	if call, ok := CallFrom(ctx); ok {
		setPresent(md, kernelgrpc.CorrelationKey, call.CorrelationID)
		setPresent(md, kernelgrpc.CausationKey, call.RequestID)
		setPresent(md, kernelgrpc.IdempotencyKey, call.IdempotencyKey)
		setPresent(md, kernelgrpc.TenantKey, call.TenantID)
		setPresent(md, kernelgrpc.LocaleKey, call.Locale)
	}
	propagation.TraceContext{}.Inject(ctx, kernelgrpc.MetadataCarrier(md))
	return invoker(metadata.NewOutgoingContext(ctx, md), method, req, reply, cc, opts...)
}

func setPresent(md metadata.MD, key, value string) {
	if value != "" {
		md.Set(key, value)
	}
}
