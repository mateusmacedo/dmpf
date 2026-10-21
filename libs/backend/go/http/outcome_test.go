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
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func outcomeTable() provider.OutcomeTable {
	return provider.OutcomeTable{
		Version: "partner-v1",
		Status: map[int]ports.ExternalClass{
			http.StatusCreated:             ports.ClassAccepted,
			http.StatusBadRequest:          ports.ClassFinalValidation,
			http.StatusForbidden:           ports.ClassFinalAuthorization,
			http.StatusUnprocessableEntity: ports.ClassFinalBusiness,
			http.StatusServiceUnavailable:  ports.ClassRetryableUpstream,
		},
	}
}

func TestOutcomeTableValidate(t *testing.T) {
	if err := outcomeTable().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	cases := map[string]func(*provider.OutcomeTable){
		"empty version": func(o *provider.OutcomeTable) { o.Version = "" },
		"status 0":      func(o *provider.OutcomeTable) { o.Status[0] = ports.ClassAccepted },
		"status 99":     func(o *provider.OutcomeTable) { o.Status[99] = ports.ClassAccepted },
		"status 600":    func(o *provider.OutcomeTable) { o.Status[600] = ports.ClassAccepted },
		"class 0":       func(o *provider.OutcomeTable) { o.Status[http.StatusOK] = 0 },
		"class 10":      func(o *provider.OutcomeTable) { o.Status[http.StatusOK] = ports.ClassSchemaMismatch + 1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			table := outcomeTable()
			mutate(&table)
			if err := table.Validate(); !errors.Is(err, provider.ErrInvalidOutcomeTable) {
				t.Fatalf("Validate() = %v, want ErrInvalidOutcomeTable", err)
			}
		})
	}
}

func TestClassifyMapsEachDeclaredStatusAndTheUnknownOnes(t *testing.T) {
	table := outcomeTable()
	for status, want := range table.Status {
		if got := table.Classify(&http.Response{StatusCode: status}, nil); got != want {
			t.Errorf("Classify(%d) = %v, want %v", status, got, want)
		}
	}
	for _, status := range []int{http.StatusOK, http.StatusTeapot, http.StatusInternalServerError} {
		if got := table.Classify(&http.Response{StatusCode: status}, nil); got != ports.ClassUnknownAmbiguous {
			t.Errorf("Classify(%d) = %v, want unknown_ambiguous", status, got)
		}
	}
}

func TestClassifyDecidesARetryableStatusErrorByItsStatus(t *testing.T) {
	table := outcomeTable()
	declared := fmt.Errorf("attempts exhausted: %w", &provider.RetryableStatusError{Status: http.StatusServiceUnavailable})
	if got := table.Classify(nil, declared); got != ports.ClassRetryableUpstream {
		t.Errorf("Classify(503 error) = %v, want retryable_upstream", got)
	}
	undeclared := &provider.RetryableStatusError{Status: http.StatusBadGateway}
	if got := table.Classify(nil, undeclared); got != ports.ClassUnknownAmbiguous {
		t.Errorf("Classify(502 error) = %v, want unknown_ambiguous", got)
	}
}

func TestClassifyTakesOnlyADialErrorAsRetryableTransport(t *testing.T) {
	table := outcomeTable()
	cases := map[string]struct {
		err  error
		want ports.ExternalClass
	}{
		"dial refused":         {&net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, ports.ClassRetryableTransport},
		"dial cancelled":       {fmt.Errorf("wrapped: %w", &net.OpError{Op: "dial", Net: "tcp", Err: context.Canceled}), ports.ClassRetryableTransport},
		"dial deadline":        {&net.OpError{Op: "dial", Net: "tcp", Err: context.DeadlineExceeded}, ports.ClassRetryableTransport},
		"read reset":           {&net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset")}, ports.ClassUnknownSent},
		"cancelled":            {context.Canceled, ports.ClassUnknownSent},
		"deadline":             {context.DeadlineExceeded, ports.ClassUnknownSent},
		"unexpected EOF":       {io.ErrUnexpectedEOF, ports.ClassUnknownSent},
		"no response no error": {nil, ports.ClassUnknownAmbiguous},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := table.Classify(nil, tc.err); got != tc.want {
				t.Fatalf("Classify(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func classifyDo(t *testing.T, ctx context.Context, transport http.RoundTripper, url string) ports.ExternalClass {
	t.Helper()
	cfg := clientConfig(route(http.MethodPost))
	cfg.Transport = transport
	resp, err := newClient(t, cfg).Do(ctx, "placeOrder", request(t, http.MethodPost, url, `{"a":1}`))
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("Do() = nil, want an error")
	}
	return outcomeTable().Classify(resp, err)
}

func TestClassifyAfterDoWithARefusedDialIsRetryableTransport(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := "http://" + ln.Addr().String()
	_ = ln.Close()

	got := classifyDo(t, budgeted(t, 2*time.Second), ownTransport(t, &http.Transport{}), closed)
	if got != ports.ClassRetryableTransport {
		t.Fatalf("Classify() = %v, want retryable_transport", got)
	}
}

func TestClassifyAfterDoCancelledDuringTheDialIsUnknownSent(t *testing.T) {
	ctx, cancel := context.WithCancel(budgeted(t, 2*time.Second))
	defer cancel()
	transport := ownTransport(t, &http.Transport{
		DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
			cancel()
			<-dialCtx.Done()
			return nil, &net.OpError{Op: "dial", Net: network, Err: dialCtx.Err()}
		},
	})

	// net/http's getConn returns context.Cause instead of the dial error once
	// the request context is done (transport.go, golang.org/issue/16049).
	got := classifyDo(t, ctx, transport, "http://127.0.0.1:1")
	if got != ports.ClassUnknownSent {
		t.Fatalf("Classify() = %v, want unknown_sent", got)
	}
}

func TestClassifyAfterDoWithAServerThatHangsUpIsUnknownSent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		hangUp(t, w)
	}))
	t.Cleanup(srv.Close)

	got := classifyDo(t, budgeted(t, 2*time.Second), ownTransport(t, &http.Transport{}), srv.URL)
	if got != ports.ClassUnknownSent {
		t.Fatalf("Classify() = %v, want unknown_sent", got)
	}
}

func stallingServer(t *testing.T) (*httptest.Server, <-chan struct{}) {
	t.Helper()
	read := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		read <- struct{}{}
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	return srv, read
}

func TestClassifyAfterDoCancelledOnceConnectedIsUnknownSent(t *testing.T) {
	srv, read := stallingServer(t)
	ctx, cancel := context.WithCancel(budgeted(t, 2*time.Second))
	defer cancel()
	go func() {
		<-read
		cancel()
	}()

	got := classifyDo(t, ctx, ownTransport(t, &http.Transport{}), srv.URL)
	if got != ports.ClassUnknownSent {
		t.Fatalf("Classify() = %v, want unknown_sent", got)
	}
}

func TestClassifyAfterDoPastTheDeadlineOnceConnectedIsUnknownSent(t *testing.T) {
	srv, _ := stallingServer(t)

	got := classifyDo(t, budgeted(t, 300*time.Millisecond), ownTransport(t, &http.Transport{}), srv.URL)
	if got != ports.ClassUnknownSent {
		t.Fatalf("Classify() = %v, want unknown_sent", got)
	}
}

func TestWithOutboundKeyRefusesAnEmptyKeyOrARouteWithoutHeader(t *testing.T) {
	keyed := route(http.MethodPost)
	keyed.IdempotencyKey = "X-Partner-Key"
	cases := map[string]struct {
		route provider.Route
		key   ports.OutboundKey
	}{
		"empty key":            {keyed, ""},
		"route without header": {route(http.MethodPost), "out-1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.route.WithOutboundKey(request(t, http.MethodPost, "http://example.invalid", `{}`), tc.key)
			if !errors.Is(err, provider.ErrOutboundKeyRequired) {
				t.Fatalf("WithOutboundKey() = %v, want ErrOutboundKeyRequired", err)
			}
		})
	}
}

func TestDoSendsAnOutboundKeyedRequestOnceWithItsKey(t *testing.T) {
	var arrivals atomic.Int32
	var key atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		key.Store(r.Header.Get("X-Partner-Key"))
		arrivals.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	r := route(http.MethodPost)
	r.IdempotencyKey = "X-Partner-Key"
	cfg := clientConfig(r)
	cfg.Transport = ownTransport(t, &http.Transport{})
	cfg.NewKey = func() string { return "random" }

	req, err := r.WithOutboundKey(request(t, http.MethodPost, srv.URL, `{"a":1}`), "out-123")
	if err != nil {
		t.Fatalf("WithOutboundKey() = %v, want nil", err)
	}
	resp, err := newClient(t, cfg).Do(budgeted(t, 2*time.Second), "placeOrder", req)
	if err != nil {
		t.Fatalf("Do() = %v, want the 503 response", err)
	}
	_ = resp.Body.Close()

	if got := outcomeTable().Classify(resp, nil); got != ports.ClassRetryableUpstream {
		t.Errorf("Classify() = %v, want retryable_upstream", got)
	}
	if got := arrivals.Load(); got != 1 {
		t.Fatalf("server saw %d attempts, want 1: the kernel retries by the class, not Do", got)
	}
	if got := key.Load(); got != "out-123" {
		t.Errorf("attempt carried key %v, want the outbound key", got)
	}
}
