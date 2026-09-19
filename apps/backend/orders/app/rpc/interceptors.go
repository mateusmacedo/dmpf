package rpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

const (
	// Tenant is the single tenant the context admits: the identity of FND-07
	// has no realization in the kernel, so every call resolves to it (MET-07).
	Tenant = "public"

	// CorrelationKey is the metadata the BFF propagates and this context adopts.
	// CausationKey is what the BFF sends and this context deliberately does NOT
	// adopt: CTX-08 fixes causation as the step immediately before, which for an
	// outbox record is this RPC, not the edge request two steps back.
	CorrelationKey = "x-correlation-id"
	CausationKey   = "x-causation-id"

	// IdempotencyKey is propagated by the BFF for the log alone: the context
	// keeps no replay store, so the key decides nothing here.
	IdempotencyKey = "idempotency-key"

	idBytes = 16
)

var correlationFormat = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// Interceptors is the server chain in the order SPEC-ACYKBF9V fixes: span,
// admission, deadline, message context.
func Interceptors(tracer trace.Tracer, ctrl *admission.Controller, instruments *metrics.Instruments, logger *slog.Logger) []grpc.UnaryServerInterceptor {
	return []grpc.UnaryServerInterceptor{
		ownMethods(serverSpan(tracer)),
		ownMethods(provider.Admission(ctrl, func(context.Context) string { return Tenant }, instruments)),
		ownMethods(requireDeadline),
		ownMethods(messageContext(logger)),
	}
}

// ownMethods restricts an interceptor to the methods of this service. The
// health probe declares no admission limit and no deadline of its own, and
// Kubernetes calls it before the service is ready (ADR-044).
func ownMethods(next grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	prefix := "/" + ServiceName + "/"
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !strings.HasPrefix(info.FullMethod, prefix) {
			return handler(ctx, req)
		}
		return next(ctx, req, info, handler)
	}
}

// Limits declares one admission limit per method of the service (RES-16).
func Limits(limit admission.Limit) map[string]admission.Limit {
	methods := descriptor.Methods()
	limits := make(map[string]admission.Limit, methods.Len())
	for i := range methods.Len() {
		limits[FullMethod(string(methods.Get(i).Name()))] = limit
	}
	return limits
}

func serverSpan(tracer trace.Tracer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		ctx = propagation.TraceContext{}.Extract(ctx, metadataCarrier(md))
		ctx, span := tracer.Start(ctx, "dmpf.grpc.server "+info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(tracing.Attributes{}.Operation(info.FullMethod).KeyValues()...))
		defer span.End()

		resp, err := handler(ctx, req)
		if err != nil {
			tracing.RecordError(span, status.Code(err).String())
		}
		return resp, err
	}
}

func requireDeadline(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if _, err := deadline.Require(ctx); err != nil {
		return nil, status.Error(codes.InvalidArgument, "the call declares no deadline (GRP-04)")
	}
	return handler(ctx, req)
}

// messageContext authors what the outbox records: the correlation the BFF
// propagated, this request as the cause (CTX-08) and the server span as trace.
func messageContext(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		correlation := metadataCarrier(md).Get(CorrelationKey)
		if !correlationFormat.MatchString(correlation) {
			correlation = newID()
		}
		requestID := newID()

		carrier := propagation.MapCarrier{}
		propagation.TraceContext{}.Inject(ctx, carrier)
		trace.SpanFromContext(ctx).SetAttributes(tracing.Attributes{}.CorrelationID(correlation).RequestID(requestID).KeyValues()...)

		ctx = ports.WithMessageContext(ctx, ports.MessageContext{
			CorrelationID: correlation,
			CausationID:   requestID,
			Traceparent:   carrier.Get("traceparent"),
		})
		if key := metadataCarrier(md).Get(IdempotencyKey); key != "" && logger != nil {
			logger.InfoContext(ctx, "grpc request", "operation", info.FullMethod, "idempotency_key", key)
		}
		return handler(ctx, req)
	}
}

type metadataCarrier metadata.MD

func (c metadataCarrier) Get(key string) string {
	if values := metadata.MD(c).Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}

func (c metadataCarrier) Set(key, value string) { metadata.MD(c).Set(key, value) }

func (c metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}

func newID() string {
	buffer := make([]byte, idBytes)
	if _, err := rand.Read(buffer); err != nil {
		panic("rpc: the operating system's entropy source failed: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}
