package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

const correlationBytes = 16

// correlationFormat is what the edge accepts from a client as correlationid:
// it is persisted in outbox metadata and travels in every envelope of the
// chain, so it is bounded in length and alphabet; anything else is replaced.
var correlationFormat = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// requireIdempotencyKey refuses a POST without the key before the service
// runs (RST-02): a write the client cannot replay safely is not accepted.
func requireIdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.Header.Get(IdempotencyHeader) == "" {
			writeRejection(w, http.StatusBadRequest, "missing-idempotency-key", "POST requires the "+IdempotencyHeader+" header")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withMessageContext is where the chain is authored (FND-07 §8.6 item 3): the
// server span yields the traceparent, the client or the edge names the
// correlation, and the service finds both on the context.
func withMessageContext(tracer trace.Tracer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "HTTP "+r.Pattern, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		carrier := propagation.MapCarrier{}
		propagation.TraceContext{}.Inject(ctx, carrier)

		correlation := r.Header.Get(CorrelationHeader)
		if !correlationFormat.MatchString(correlation) {
			correlation = newCorrelationID()
		}
		w.Header().Set(CorrelationHeader, correlation)

		ctx = dmpfports.WithMessageContext(ctx, dmpfports.MessageContext{
			CorrelationID: correlation,
			Traceparent:   carrier.Get("traceparent"),
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withCORS answers the preflight and marks the response for a browser on one
// of the allowed origins; with no origin declared it is a pass-through and the
// edge stays same-origin only, which is the secure default.
func withCORS(origins []string, next http.Handler) http.Handler {
	if len(origins) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || !slices.Contains(origins, origin) {
			next.ServeHTTP(w, r)
			return
		}
		header := w.Header()
		header.Set("Access-Control-Allow-Origin", origin)
		header.Add("Vary", "Origin")
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

func serveOpenAPI(document []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(document)
	})
}

func newCorrelationID() string {
	buffer := make([]byte, correlationBytes)
	if _, err := rand.Read(buffer); err != nil {
		panic("api: the operating system's entropy source failed: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}
