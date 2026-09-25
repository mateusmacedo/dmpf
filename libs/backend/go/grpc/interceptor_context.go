package grpc

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

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

const (
	// CorrelationKey and CausationKey are what the BFF propagates. The causation
	// is the step preceding this execution, never the one preceding what this
	// execution publishes: for an outbox record CTX-08 makes the cause this RPC.
	CorrelationKey = "x-correlation-id"
	CausationKey   = "x-causation-id"

	// TenantKey carries the tenant the edge resolved (CTX-13). Its absence means
	// a chain without a subject, and no value is invented to replace it.
	TenantKey = "x-tenant-id"

	// IdempotencyKey is propagated by the BFF for the log alone: a context keeps
	// no replay store, so the key decides nothing here.
	IdempotencyKey = "idempotency-key"

	// LocaleKey carries the locale the edge resolved, which the hop preserves
	// (CTX-11); DefaultLocale answers when none, or no valid tag, arrived.
	LocaleKey     = "x-locale"
	DefaultLocale = "en"

	idBytes = 16
)

var (
	correlationFormat = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	localeFormat      = regexp.MustCompile(`^[A-Za-z]{1,8}(-[A-Za-z0-9]{1,8})*$`)
)

// ServerInterceptors is the chain of SPEC-ACYKBF9V: span, admission, deadline,
// context. Other services' methods pass untouched: the health probe has no limit
// nor deadline and is called before the service is ready (ADR-044).
func ServerInterceptors(service string, tracer trace.Tracer, ctrl *admission.Controller, instruments *metrics.Instruments, logger *slog.Logger) []grpc.UnaryServerInterceptor {
	own := ownMethods(service)
	return []grpc.UnaryServerInterceptor{
		own(serverSpan(tracer)),
		own(Admission(ctrl, admissionTenant, instruments)),
		own(requireDeadline),
		own(requestContext(logger)),
	}
}

// MethodLimits declares one admission limit per method of the service (RES-16).
func MethodLimits(service string, methods []string, limit admission.Limit) map[string]admission.Limit {
	limits := make(map[string]admission.Limit, len(methods))
	for _, method := range methods {
		limits["/"+service+"/"+method] = limit
	}
	return limits
}

func ownMethods(service string) func(grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	prefix := "/" + service + "/"
	return func(next grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			if !strings.HasPrefix(info.FullMethod, prefix) {
				return handler(ctx, req)
			}
			return next(ctx, req, info, handler)
		}
	}
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

// requestContext authors the two contexts of this execution from what crossed
// the hop: the nine-field context the application service takes as an argument,
// and the message context the outbox records.
func requestContext(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		incoming := metadataCarrier(md)

		correlation := incoming.Get(CorrelationKey)
		if !correlationFormat.MatchString(correlation) {
			correlation = newID()
		}
		requestID := newID()

		carrier := propagation.MapCarrier{}
		propagation.TraceContext{}.Inject(ctx, carrier)
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(tracing.Attributes{}.CorrelationID(correlation).RequestID(requestID).KeyValues()...)

		execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
			RequestID:     requestID,
			CorrelationID: correlation,
			CausationID:   optional(incoming.Get(CausationKey)),
			TraceContext:  span.SpanContext().TraceID().String(),
			Tenant:        tenantOf(incoming),
			Deadline:      deadlineOf(ctx),
			Locale:        localeOf(incoming),
		})
		if err != nil {
			return nil, status.Error(codes.Internal, "the execution context could not be assembled")
		}

		ctx = ports.WithExecutionContext(ctx, execution)
		ctx = ports.WithMessageContext(ctx, ports.MessageContext{
			CorrelationID: correlation,
			CausationID:   requestID,
			Traceparent:   carrier.Get("traceparent"),
		})
		if key := incoming.Get(IdempotencyKey); key != "" && logger != nil {
			logger.InfoContext(ctx, "grpc request", "operation", info.FullMethod, "idempotency_key", key)
		}
		return handler(ctx, req)
	}
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func tenantOf(incoming metadataCarrier) *ports.TenantID {
	value := incoming.Get(TenantKey)
	if value == "" {
		return nil
	}
	tenant := ports.TenantID(value)
	return &tenant
}

// admissionTenant keys the bucket by the tenant the edge propagated (RES-16),
// read where the context will read it; a call without one shares the bucket of
// every undeclared tenant.
func admissionTenant(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	return metadataCarrier(md).Get(TenantKey)
}

func localeOf(incoming metadataCarrier) string {
	if locale := incoming.Get(LocaleKey); localeFormat.MatchString(locale) {
		return locale
	}
	return DefaultLocale
}

func deadlineOf(ctx context.Context) ports.Instant {
	governed, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	return ports.Instant(governed.UnixNano())
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
		panic("grpc: the operating system's entropy source failed: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}
