package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/auth"
	"github.com/mateusmacedo/dmpf/apps/backend/bff/rpc"
)

const identifierBytes = 16

// correlationFormat bounds what a client may name the chain: the value travels
// to every outbox row and envelope of the chain, so anything else is replaced.
var correlationFormat = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// idempotencyFormat bounds the key for the same reason, and one more: the value
// reaches gRPC metadata, which refuses anything outside %x20-%x7E and answers
// Internal. Without this the edge turns a client header into a permanent 500.
var idempotencyFormat = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type executionContextKey struct{}

// ExecutionContextFrom returns what the edge mounted for this request. Only the
// handler reads it: from there down the context travels as an explicit argument
// (CTX-03), so no block downstream depends on the ambient value (CTX-05).
func ExecutionContextFrom(ctx context.Context) (ports.ExecutionContext, bool) {
	execution, ok := ctx.Value(executionContextKey{}).(ports.ExecutionContext)
	return execution, ok
}

// withExecutionContext authenticates and mounts the nine-field context. It runs
// inside withRouteDeadline, never outside: deadline is mandatory in CTX-01, and
// mounting before the timeout existed would leave the field unresolvable.
func withExecutionContext(tracer trace.Tracer, authenticator ports.Authenticator, route provider.Route, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := extractTrustedTrace(r)

		correlation := r.Header.Get(CorrelationHeader)
		if !correlationFormat.MatchString(correlation) {
			correlation = newIdentifier()
		}
		requestID := newIdentifier()

		ctx, span := tracer.Start(ctx, "HTTP "+r.Pattern,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(tracing.Attributes{}.CorrelationID(correlation).RequestID(requestID).KeyValues()...))
		defer span.End()

		w.Header().Set(CorrelationHeader, correlation)

		deadline, governed := r.Context().Deadline()
		if !governed {
			tracing.RecordError(span, "deadline")
			writeRejection(r, w, http.StatusInternalServerError, "internal-failure", "the request could not be completed")
			return
		}

		identity, status, code := resolveIdentity(ctx, authenticator, route, r)
		if status != 0 {
			tracing.RecordError(span, code)
			writeRejection(r, w, status, code, rejectionMessage(status))
			return
		}

		execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
			RequestID:     requestID,
			CorrelationID: correlation,
			TraceContext:  span.SpanContext().TraceID().String(),
			Subject:       identity.subject,
			Tenant:        identity.tenant,
			Permissions:   identity.permissions,
			Deadline:      ports.Instant(deadline.UnixNano()),
			Locale:        localeOf(r),
		})
		if err != nil {
			tracing.RecordError(span, "context")
			writeRejection(r, w, http.StatusInternalServerError, "internal-failure", "the request could not be completed")
			return
		}

		call := rpc.Call{
			CorrelationID:  correlation,
			RequestID:      requestID,
			IdempotencyKey: r.Header.Get(IdempotencyHeader),
		}
		if tenant, ok := execution.Tenant(); ok {
			span.SetAttributes(tracing.Attributes{}.TenantID(string(tenant)).KeyValues()...)
			call.TenantID = string(tenant)
		}

		ctx = context.WithValue(ctx, executionContextKey{}, execution)
		ctx = rpc.WithCall(ctx, call)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolved is what verification yielded, already in the shape the context spec
// takes. A zero status means the request may proceed.
type resolved struct {
	subject     *ports.SubjectID
	tenant      *ports.TenantID
	permissions []ports.Permission
}

// resolveIdentity separates the two refusals IDN-06 keeps apart: 401 when
// authentication did not resolve a subject, 403 when it did and the operation
// demands something the subject does not carry (IDN-15).
func resolveIdentity(ctx context.Context, authenticator ports.Authenticator, route provider.Route, r *http.Request) (resolved, int, string) {
	credential := auth.CredentialFrom(r)
	if !credential.Presented() {
		if route.RequiresSubject() || route.RequiresTenant() {
			return resolved{}, http.StatusUnauthorized, "unauthenticated"
		}
		return resolved{}, 0, ""
	}

	identity, err := authenticator.Authenticate(ctx, credential)
	if err != nil {
		return resolved{}, http.StatusUnauthorized, "unauthenticated"
	}
	if route.RequiresTenant() && identity.Tenant == nil {
		return resolved{}, http.StatusForbidden, "tenant-unresolved"
	}

	subject := identity.Subject
	return resolved{subject: &subject, tenant: identity.Tenant, permissions: identity.Permissions}, 0, ""
}

func rejectionMessage(status int) string {
	if status == http.StatusForbidden {
		return "the resolved identity does not carry what this operation requires"
	}
	return "the request carries no verifiable credential"
}

// localeOf answers CTX-01's mandatory field from what the caller asked for,
// falling back to the edge default rather than leaving it unresolved.
func localeOf(r *http.Request) string {
	if requested := strings.TrimSpace(r.Header.Get("Accept-Language")); requested != "" {
		if first, _, found := strings.Cut(requested, ","); found {
			return strings.TrimSpace(first)
		}
		return requested
	}
	return DefaultLocale
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

func requireIdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(IdempotencyHeader)
		if r.Method == http.MethodPost && key == "" {
			writeRejection(r, w, http.StatusBadRequest, "missing-idempotency-key", "POST requires the "+IdempotencyHeader+" header")
			return
		}
		if key != "" && !idempotencyFormat.MatchString(key) {
			writeRejection(r, w, http.StatusBadRequest, "invalid-idempotency-key", IdempotencyHeader+" accepts up to 128 characters of [A-Za-z0-9._-]")
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
		header.Set("Access-Control-Expose-Headers", CorrelationHeader)
		if r.Method == http.MethodOptions {
			header.Set("Access-Control-Allow-Methods", strings.Join([]string{http.MethodGet, http.MethodPost, http.MethodOptions}, ", "))
			header.Set("Access-Control-Allow-Headers", strings.Join([]string{"Content-Type", IdempotencyHeader, CorrelationHeader}, ", "))
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

func newIdentifier() string {
	buffer := make([]byte, identifierBytes)
	if _, err := rand.Read(buffer); err != nil {
		panic("api: the operating system's entropy source failed: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}

func extractTrustedTrace(r *http.Request) context.Context {
	return propagation.TraceContext{}.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
}

// WHY: without this a panic reaches net/http, which closes the connection with
// no HTTP answer at all and writes the stack to the default logger, bypassing
// the redacting handler this process installs.
func withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				tracing.RecordError(trace.SpanFromContext(r.Context()), "panic")
				writeRejection(r, w, http.StatusInternalServerError, "internal-failure", "the request could not be completed")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
