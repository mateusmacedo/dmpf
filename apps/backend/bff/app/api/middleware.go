package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

// withExecutionContext authenticates and mounts the nine-field context. It runs
// inside withRouteDeadline, never outside: deadline is mandatory in CTX-01, and
// mounting before the timeout existed would leave the field unresolvable.
func withExecutionContext(logger *slog.Logger, authenticator ports.Authenticator, route kernelhttp.Route, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		markSelfLogged(rw)
		ctx := r.Context()
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(allowedHeaders(r)...)

		correlation := r.Header.Get(CorrelationHeader)
		if !kernelgrpc.ValidCorrelation(correlation) {
			correlation = kernelgrpc.NewID("api")
		}
		requestID := kernelgrpc.NewID("api")

		w := newRecorder(rw)
		clientKey := r.Header.Get(idempotencyHeaderOf(route))
		derivedKey := ""
		defer func() { finishRequest(ctx, span, logger, route, r, w, clientKey, derivedKey) }()
		defer recordPanicStatus(w)

		w.Header().Set(CorrelationHeader, correlation)

		deadline, governed := r.Context().Deadline()
		if !governed {
			tracing.RecordError(span, rpc.CategoryUnexpected)
			writeInternal(w, r)
			return
		}

		identity, status, code := kernelhttp.ResolveIdentity(ctx, authenticator, route, authn.CredentialFrom(r))
		if status == 0 {
			status, code = kernelhttp.RefuseAssertedIdentity(r, identity)
		}
		if status != 0 {
			writeRejection(r, w, status, code, rejectionMessage(status))
			return
		}

		execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
			RequestID:     requestID,
			CorrelationID: correlation,
			TraceContext:  span.SpanContext().TraceID().String(),
			Subject:       identity.Subject,
			Tenant:        identity.Tenant,
			Permissions:   identity.Permissions,
			Deadline:      ports.Instant(deadline.UnixNano()),
			Locale:        localeOf(r),
		})
		if err != nil {
			tracing.RecordError(span, rpc.CategoryUnexpected)
			writeInternal(w, r)
			return
		}

		derivedKey = deriveIdempotencyKey(identity.Subject, clientKey)
		call := rpc.Call{
			CorrelationID:  correlation,
			RequestID:      requestID,
			IdempotencyKey: derivedKey,
			Locale:         execution.Locale(),
		}
		if tenant, ok := execution.Tenant(); ok {
			call.TenantID = string(tenant)
		}
		span.SetAttributes(tracing.ExecutionAttributes(execution).KeyValues()...)

		ctx = tracing.WithExecutionBaggage(kernelhttp.WithExecutionContext(ctx, execution), execution)
		ctx = rpc.WithReplaySlot(rpc.WithCall(ctx, call))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func allowedHeaders(r *http.Request) []attribute.KeyValue {
	var kept []attribute.KeyValue
	for _, name := range []string{"content-type", "accept"} {
		if values := r.Header.Values(name); len(values) > 0 {
			kept = append(kept, semconv.HTTPRequestHeader(name, values...))
		}
	}
	return kept
}

type statusRecorder struct {
	http.ResponseWriter
	status     int
	selfLogged bool
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func markSelfLogged(w http.ResponseWriter) {
	if recorder, ok := w.(*statusRecorder); ok {
		recorder.selfLogged = true
	}
}

func recordPanicStatus(w *statusRecorder) {
	if recovered := recover(); recovered != nil {
		w.status = http.StatusInternalServerError
		panic(recovered)
	}
}

// deriveIdempotencyKey scopes the client's key to its subject, which CTX-12
// keeps from crossing to the context: two callers never share an entry (IDM-03).
func deriveIdempotencyKey(subject *ports.SubjectID, key string) string {
	if key == "" {
		return ""
	}
	var owner string
	if subject != nil {
		owner = string(*subject)
	}
	digest := sha256.Sum256([]byte(owner + "\x00" + key))
	return hex.EncodeToString(digest[:])
}

func finishRequest(ctx context.Context, span trace.Span, logger *slog.Logger, route kernelhttp.Route, r *http.Request, w *statusRecorder, clientKey, derivedKey string) {
	span.SetAttributes(tracing.Attributes{}.OutcomeCategory(string(outcomeOf(w.status))).KeyValues()...)

	level, enabled := accessLevel(ctx, logger, w.status)
	if !enabled {
		return
	}
	logRequest(ctx, r, logger, level, route.Path, w.status, idempotencyAttrs(clientKey, derivedKey)...)
}

func idempotencyAttrs(clientKey, derivedKey string) []slog.Attr {
	var attrs []slog.Attr
	switch {
	case clientKey == "":
	case ports.ValidIdempotencyKey(clientKey):
		attrs = append(attrs, slog.String(tracing.KeyIdempotencyKey, clientKey))
	default:
		attrs = append(attrs, slog.Bool(tracing.KeyIdempotencyKeyInvalid, true))
	}
	if derivedKey != "" {
		attrs = append(attrs, slog.String(tracing.KeyIdempotencyKeyDerived, derivedKey))
	}
	return attrs
}

func accessLevel(ctx context.Context, logger *slog.Logger, status int) (slog.Level, bool) {
	level := logging.Severity(logging.Server, outcomeOf(status))
	return level, logger.Enabled(ctx, level)
}

func logRequest(ctx context.Context, r *http.Request, logger *slog.Logger, level slog.Level, route string, status int, extra ...slog.Attr) {
	logger.LogAttrs(ctx, level, "http request", append(accessAttrs(r.Method, route, status, outcomeOf(status)), extra...)...)
}

func newRecorder(rw http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: rw, status: http.StatusOK}
}

func writeInternal(w http.ResponseWriter, r *http.Request) {
	writeRejection(r, w, http.StatusInternalServerError, "internal-failure", "the request could not be completed")
}

func accessAttrs(method, route string, status int, outcome ports.OutcomeCategory) []slog.Attr {
	attrs := []slog.Attr{slog.String(string(semconv.HTTPRequestMethodKey), semconvMethod(method))}
	if route != "" {
		attrs = append(attrs, slog.String(string(semconv.HTTPRouteKey), route))
	}
	return append(attrs,
		slog.Int(string(semconv.HTTPResponseStatusCodeKey), status),
		slog.String(tracing.KeyOutcomeCategory, string(outcome)),
	)
}

func semconvMethod(method string) string {
	switch upper := strings.ToUpper(method); upper {
	case http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead, http.MethodOptions,
		http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace:
		return upper
	}
	return semconv.HTTPRequestMethodOther.Value.AsString()
}

func withAccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w := newRecorder(rw)
		next.ServeHTTP(w, r)
		if w.selfLogged {
			return
		}
		level, enabled := accessLevel(r.Context(), logger, w.status)
		if !enabled {
			return
		}
		_, route, _ := strings.Cut(r.Pattern, " ")
		logRequest(r.Context(), r, logger, level, route, w.status)
	})
}

func outcomeOf(status int) ports.OutcomeCategory {
	switch {
	case status >= http.StatusInternalServerError:
		return ports.OutcomeFailed
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return ports.OutcomeDenied
	case status >= http.StatusBadRequest:
		return ports.OutcomeRejected
	default:
		return ports.OutcomeAccepted
	}
}

func rejectionMessage(status int) string {
	if status == http.StatusForbidden {
		return "the resolved identity does not carry what this operation requires"
	}
	return "the request carries no verifiable credential"
}

// localeOf preserves the first language the caller declared and falls back to
// the edge default when none is declared or it is not a tag (CTX-11).
func localeOf(r *http.Request) string {
	first, _, _ := strings.Cut(r.Header.Get("Accept-Language"), ",")
	tag, _, _ := strings.Cut(first, ";")
	if tag = strings.TrimSpace(tag); kernelgrpc.ValidLocale(tag) {
		return tag
	}
	return kernelgrpc.DefaultLocale
}

// withRouteDeadline governs the time of the whole request at the edge (GRP-05)
// and grants the retry budget only the edge may mint (RES-24, RES-31).
func withRouteDeadline(budget deadline.Budget, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), budget.Limit)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(retry.WithBudget(ctx)))
	})
}

func idempotencyHeaderOf(route kernelhttp.Route) string {
	if route.IdempotencyKey != "" {
		return route.IdempotencyKey
	}
	return IdempotencyHeader
}

func requireIdempotencyKey(route kernelhttp.Route, next http.Handler) http.Handler {
	header := idempotencyHeaderOf(route)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(header)
		if route.IdempotencyKey != "" && key == "" {
			writeRejection(r, w, http.StatusBadRequest, "missing-idempotency-key", route.Method+" requires the "+header+" header")
			return
		}
		if key != "" && !ports.ValidIdempotencyKey(key) {
			writeRejection(r, w, http.StatusBadRequest, "invalid-idempotency-key", header+" accepts up to 128 characters of [A-Za-z0-9._-]")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withCORS(origins []string, next http.Handler) http.Handler {
	if len(origins) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		// WHY: a shared cache that stores the answer given to an allowed origin
		// would serve it to a denied one; Vary has to be declared even when the
		// origin does not match, because that is the answer being cached.
		header.Add("Vary", "Origin")

		origin := r.Header.Get("Origin")
		if origin == "" || !slices.Contains(origins, origin) {
			next.ServeHTTP(w, r)
			return
		}
		header.Set("Access-Control-Allow-Origin", origin)
		header.Set("Access-Control-Expose-Headers", strings.Join([]string{CorrelationHeader, ReplayedHeader}, ", "))
		if r.Method == http.MethodOptions {
			header.Set("Access-Control-Allow-Methods", strings.Join([]string{http.MethodGet, http.MethodPost, http.MethodOptions}, ", "))
			header.Set("Access-Control-Allow-Headers", strings.Join([]string{"Authorization", "Content-Type", IdempotencyHeader, CorrelationHeader}, ", "))
			header.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func serveContract(document []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(document)
	})
}

// WHY: without this a panic reaches net/http, which closes the connection with
// no HTTP answer at all and writes the stack to the default logger, bypassing
// the redacting handler this process installs.
func withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				tracing.RecordError(trace.SpanFromContext(r.Context()), rpc.CategoryUnexpected)
				writeInternal(w, r)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
