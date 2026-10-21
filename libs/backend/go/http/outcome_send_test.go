package http_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

func resetAfterReading(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		t.Errorf("Hijack() = %v", err)
		return
	}
	_ = conn.(*net.TCPConn).SetLinger(0)
	_ = conn.Close()
}

func TestAnOutboundKeyedPostWrittenAndResetIsSentOnceAndUnknownSent(t *testing.T) {
	var arrivals atomic.Int32
	srv := httptest.NewUnstartedServer(nil)
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		arrivals.Add(1)
		_ = srv.Listener.Close()
		resetAfterReading(t, w)
	})
	srv.Start()
	t.Cleanup(srv.Close)
	r := route(http.MethodPost)
	r.IdempotencyKey = "X-Partner-Key"
	cfg := clientConfig(r)
	cfg.Transport = ownTransport(t, &http.Transport{})

	req, err := r.WithOutboundKey(request(t, http.MethodPost, srv.URL, `{"a":1}`), "out-1")
	if err != nil {
		t.Fatalf("WithOutboundKey() = %v, want nil", err)
	}
	resp, err := newClient(t, cfg).Do(budgeted(t, 2*time.Second), "placeOrder", req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("Do() = nil, want the reset connection's error")
	}

	if got := arrivals.Load(); got != 1 {
		t.Fatalf("server saw %d attempts, want 1", got)
	}
	if got := outcomeTable().Classify(resp, err); got != ports.ClassUnknownSent {
		t.Fatalf("Classify(%v) = %v, want unknown_sent: the partner read the body", err, got)
	}
}

func TestWithOutboundKeyRefusesAHeaderTheTransportResendsOn(t *testing.T) {
	for _, header := range []string{"Idempotency-Key", "idempotency-key", "X-Idempotency-Key", "x-idempotency-key"} {
		t.Run(header, func(t *testing.T) {
			r := route(http.MethodPost)
			r.IdempotencyKey = header
			_, err := r.WithOutboundKey(request(t, http.MethodPost, "http://example.invalid", `{}`), "out-1")
			if !errors.Is(err, provider.ErrOutboundKeyHeaderReplayable) {
				t.Fatalf("WithOutboundKey() = %v, want ErrOutboundKeyHeaderReplayable", err)
			}
		})
	}
	r := route(http.MethodPost)
	r.IdempotencyKey = "X-Partner-Key"
	if _, err := r.WithOutboundKey(request(t, http.MethodPost, "http://example.invalid", `{}`), "out-1"); err != nil {
		t.Fatalf("WithOutboundKey(X-Partner-Key) = %v, want nil", err)
	}
}

type onlyReader struct{ io.Reader }

func TestClassifyAfterDoRefusedBeforeSendingIsWhatTheRefusalIs(t *testing.T) {
	var arrivals atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		arrivals.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)
	post := route(http.MethodPost)
	put := route(http.MethodPut)
	put.Name = "replaceOrder"
	cfg := clientConfig(post, put)
	cfg.Transport = ownTransport(t, &http.Transport{})
	client := newClient(t, cfg)
	unreplayable, err := http.NewRequest(http.MethodPut, srv.URL, onlyReader{Reader: io.LimitReader(nil, 0)})
	if err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	t.Cleanup(cancel)

	cases := map[string]struct {
		ctx       context.Context
		routeName string
		req       *http.Request
		sentinel  error
		want      ports.ExternalClass
	}{
		"route not declared":  {budgeted(t, 2*time.Second), "nope", request(t, http.MethodPost, srv.URL, `{}`), provider.ErrRouteNotDeclared, ports.ClassFinalValidation},
		"method mismatch":     {budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodGet, srv.URL, ""), provider.ErrMethodMismatch, ports.ClassFinalValidation},
		"body not replayable": {budgeted(t, 2*time.Second), "replaceOrder", unreplayable, provider.ErrBodyNotReplayable, ports.ClassFinalValidation},
		"no deadline":         {context.Background(), "placeOrder", request(t, http.MethodPost, srv.URL, `{}`), deadline.ErrNoDeadline, ports.ClassFinalValidation},
		"budget exhausted":    {short, "placeOrder", request(t, http.MethodPost, srv.URL, `{}`), deadline.ErrDeadlineExhausted, ports.ClassRetryableTransport},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp, err := client.Do(tc.ctx, tc.routeName, tc.req)
			if !errors.Is(err, tc.sentinel) {
				if resp != nil {
					_ = resp.Body.Close()
				}
				t.Fatalf("Do() = %v, want %v", err, tc.sentinel)
			}
			if got := outcomeTable().Classify(resp, err); got != tc.want {
				t.Fatalf("Classify(%v) = %v, want %v", err, got, tc.want)
			}
		})
	}
	if got := arrivals.Load(); got != 0 {
		t.Fatalf("server saw %d requests, want none", got)
	}
}

func TestClassifyTakesADecoratorRefusalAsUnsentOnlyBeforeAnyAttempt(t *testing.T) {
	refusals := map[string]error{
		"breaker open":        resilience.ErrBreakerOpen,
		"bulkhead saturated":  resilience.ErrBulkheadSaturated,
		"no time left":        resilience.ErrDeadlineExceeded,
		"deadline exhausted":  deadline.ErrDeadlineExhausted,
		"unrecognised before": errors.New("surprise"),
	}
	for name, refusal := range refusals {
		t.Run(name, func(t *testing.T) {
			wrapped := fmt.Errorf("orders: %w", refusal)
			want := ports.ClassRetryableTransport
			if name == "unrecognised before" {
				want = ports.ClassUnknownSent
			}
			if got := outcomeTable().Classify(nil, provider.NotSent(wrapped)); got != want {
				t.Errorf("Classify(not sent: %v) = %v, want %v", wrapped, got, want)
			}
			if got := outcomeTable().Classify(nil, wrapped); got != ports.ClassUnknownSent {
				t.Errorf("Classify(%v) after an attempt = %v, want unknown_sent", wrapped, got)
			}
		})
	}
}
