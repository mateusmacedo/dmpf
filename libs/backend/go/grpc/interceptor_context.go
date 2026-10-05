package grpc

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
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

	// IdempotencyKey carries a command's key (IDM-01): a method declared with
	// WithCommands refuses its absence, and the use case receives it on the
	// request carrier, never in the ExecutionContext (IDM-10).
	IdempotencyKey = "idempotency-key"

	// ReplayedHeader is the response header a command answers with when the
	// inbox replayed its first outcome (IDM-08).
	ReplayedHeader = "idempotent-replayed"

	// LocaleKey carries the locale the edge resolved, which the hop preserves
	// (CTX-11); DefaultLocale answers when none, or no valid tag, arrived.
	LocaleKey     = "x-locale"
	DefaultLocale = "en"
)

// ServerInterceptors is the chain of SPEC-ACYKBF9V under the SERVER span of
// otelgrpc: outcome, call log, admission, deadline, context. Other services'
// methods pass untouched: the health probe has no limit nor deadline (ADR-044).
func ServerInterceptors(service string, ctrl *admission.Controller, instruments *metrics.Instruments, logs log.LoggerProvider, opts ...ServerOption) []grpc.UnaryServerInterceptor {
	options := serverOptions{commands: map[string]bool{}}
	logger := loggerOf(logs)
	for _, opt := range opts {
		opt(service, &options)
	}
	own := ownMethods(service)
	return []grpc.UnaryServerInterceptor{
		own(serverOutcome),
		own(callLog(logger)),
		own(Admission(ctrl, admissionTenant, instruments)),
		own(requireDeadline),
		own(requestContext(logger, options.commands)),
	}
}

type ServerOption func(service string, options *serverOptions)

type serverOptions struct{ commands map[string]bool }

// WithCommands declares the service's command methods by name: each requires an
// idempotency key (IDM-01, IDM-02) and may answer with ReplayedHeader (IDM-08).
func WithCommands(methods ...string) ServerOption {
	return func(service string, options *serverOptions) {
		for _, method := range methods {
			options.commands["/"+service+"/"+method] = true
		}
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

// serverOutcome keeps the server status to the public projection (ERR-20):
// otelgrpc copies its message into the span status (otelgrpc@v0.72.0/interceptor.go:86-97).
func serverOutcome(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	resp, err := handler(ctx, req)
	trace.SpanFromContext(ctx).SetAttributes(tracing.Attributes{}.OutcomeCategory(categoryOf(err)).KeyValues()...)
	return resp, publicStatus(err)
}

func publicStatus(err error) error {
	if err == nil {
		return nil
	}
	if carried, ok := errors.AsType[interface {
		error
		GRPCStatus() *status.Status
	}](err); ok {
		return carried.GRPCStatus().Err()
	}
	switch code := status.FromContextError(err).Code(); code {
	case codes.DeadlineExceeded:
		return status.Error(code, "deadline exceeded")
	case codes.Canceled:
		return status.Error(code, "canceled")
	default:
		return status.Error(codes.Unknown, "unknown failure")
	}
}

func callLog(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		slot, shared := ctx.Value(assembledKey{}).(*assembled)
		if !shared {
			slot = &assembled{}
			ctx = context.WithValue(ctx, assembledKey{}, slot)
		}
		resp, err := handler(ctx, req)
		noteCall(ctx, logger, info.FullMethod, slot, err)
		return resp, err
	}
}

// WHY: the record carries the code and category, never the status message,
// which would leave the process without redaction (LOG-13).
func logCall(ctx context.Context, logger *slog.Logger, fullMethod string, slot *assembled, err error) {
	code := status.Code(publicStatus(err))
	level := logging.Severity(logging.Server, outcomeOf(code))
	if !logger.Enabled(ctx, level) {
		return
	}
	attrs := []slog.Attr{
		slog.String(string(semconv.RPCSystemNameKey), semconv.RPCSystemNameGRPC.Value.AsString()),
		slog.String(string(semconv.RPCMethodKey), rpcMethod(fullMethod)),
		slog.String(string(semconv.RPCResponseStatusCodeKey), canonicalCode(code)),
		slog.String(tracing.KeyOutcomeCategory, categoryOf(err)),
	}
	attrs = append(attrs, slot.idempotency...)
	if err != nil {
		attrs = append(attrs, errorAttr(err))
	}
	logger.LogAttrs(slot.onto(ctx), level, "grpc call", attrs...)
}

func outcomeOf(code codes.Code) ports.OutcomeCategory {
	switch code {
	case codes.OK:
		return ports.OutcomeAccepted
	case codes.PermissionDenied, codes.Unauthenticated:
		return ports.OutcomeDenied
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange, codes.NotFound, codes.AlreadyExists, codes.Aborted:
		return ports.OutcomeRejected
	default:
		return ports.OutcomeFailed
	}
}

func errorAttr(err error) slog.Attr {
	var categorized redact.Categorized
	if errors.As(err, &categorized) {
		return redact.Error(err)
	}
	return redact.Error(transportFailure{err})
}

type transportFailure struct{ error }

func (f transportFailure) ErrorCategory() string { return categoryOf(f.error) }

func (transportFailure) ErrorCode() string { return "" }

func rpcMethod(fullMethod string) string { return strings.TrimPrefix(fullMethod, "/") }

func canonicalCode(code codes.Code) string {
	if name, known := canonicalNames[code]; known {
		return name
	}
	return "CODE(" + strconv.FormatUint(uint64(code), 10) + ")"
}

var canonicalNames = map[codes.Code]string{
	codes.OK: "OK", codes.Canceled: "CANCELLED", codes.Unknown: "UNKNOWN", codes.InvalidArgument: "INVALID_ARGUMENT",
	codes.DeadlineExceeded: "DEADLINE_EXCEEDED", codes.NotFound: "NOT_FOUND", codes.AlreadyExists: "ALREADY_EXISTS",
	codes.PermissionDenied: "PERMISSION_DENIED", codes.ResourceExhausted: "RESOURCE_EXHAUSTED",
	codes.FailedPrecondition: "FAILED_PRECONDITION", codes.Aborted: "ABORTED", codes.OutOfRange: "OUT_OF_RANGE",
	codes.Unimplemented: "UNIMPLEMENTED", codes.Internal: "INTERNAL", codes.Unavailable: "UNAVAILABLE",
	codes.DataLoss: "DATA_LOSS", codes.Unauthenticated: "UNAUTHENTICATED",
}

type assembled struct {
	execution   ports.ExecutionContext
	message     ports.MessageContext
	idempotency []slog.Attr
	ok          bool
}

type assembledKey struct{}

func (a *assembled) onto(ctx context.Context) context.Context {
	if !a.ok {
		return ctx
	}
	return tracing.WithExecutionBaggage(ports.WithMessageContext(ports.WithExecutionContext(ctx, a.execution), a.message), a.execution)
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
func requestContext(logger *slog.Logger, commands map[string]bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		incoming := MetadataCarrier(md)

		command := commands[info.FullMethod]
		key := incoming.Get(IdempotencyKey)
		slot, _ := ctx.Value(assembledKey{}).(*assembled)
		if slot == nil {
			slot = &assembled{}
		}
		slot.idempotency = idempotencyAttrs(key)
		if command {
			switch {
			case key == "":
				return nil, KeyStatus(ReasonMissingIdempotencyKey)
			case !ports.ValidIdempotencyKey(key):
				return nil, KeyStatus(ReasonInvalidIdempotencyKey)
			}
		}

		correlation := incoming.Get(CorrelationKey)
		if !ValidCorrelation(correlation) {
			correlation = NewID("grpc")
		}
		requestID := NewID("grpc")

		carrier := propagation.MapCarrier{}
		propagation.TraceContext{}.Inject(ctx, carrier)
		span := trace.SpanFromContext(ctx)

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

		message := ports.MessageContext{
			CorrelationID: correlation,
			CausationID:   requestID,
			Traceparent:   carrier.Get("traceparent"),
			Tracestate:    messageTracestate(carrier.Get("tracestate")),
		}
		span.SetAttributes(tracing.ExecutionAttributes(execution).KeyValues()...)
		slot.execution, slot.message, slot.ok = execution, message, true
		ctx = tracing.WithExecutionBaggage(ports.WithMessageContext(ports.WithExecutionContext(ctx, execution), message), execution)
		if !command {
			return handler(ctx, req)
		}

		ctx = ports.WithIdempotencySlot(ports.WithIdempotencyKey(ctx, key))
		resp, err := handler(ctx, req)
		if outcome, _ := ports.IdempotencyOutcomeFrom(ctx); err == nil && outcome == ports.IdempotencyReplayed {
			if headerErr := grpc.SetHeader(ctx, metadata.Pairs(ReplayedHeader, "true")); headerErr != nil {
				logger.WarnContext(ctx, "grpc replay header not sent", slog.String(string(semconv.RPCMethodKey), rpcMethod(info.FullMethod)))
			}
		}
		return resp, err
	}
}

// WHY: 512 is the floor W3C Trace Context §3.3.1.5 asks every vendor to propagate;
// past it the list is dropped whole, because a cut inside a member forwards a value
// its vendor never wrote, and the outbox stores the field in every row.
const maxMessageTracestate = 512

func messageTracestate(tracestate string) string {
	if len(tracestate) > maxMessageTracestate {
		return ""
	}
	return tracestate
}

func idempotencyAttrs(key string) []slog.Attr {
	switch {
	case key == "":
		return nil
	case ports.ValidIdempotencyKey(key):
		return []slog.Attr{slog.String(tracing.KeyIdempotencyKey, key)}
	default:
		return []slog.Attr{slog.Bool(tracing.KeyIdempotencyKeyInvalid, true)}
	}
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func tenantOf(incoming MetadataCarrier) *ports.TenantID {
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
	return MetadataCarrier(md).Get(TenantKey)
}

func localeOf(incoming MetadataCarrier) string {
	if locale := incoming.Get(LocaleKey); ValidLocale(locale) {
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
