package http_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

// upstream is a server that answers the scripted statuses in order, repeating
// the last one, and records every request it saw.
type upstream struct {
	*httptest.Server
	statuses []int
	delay    time.Duration

	mu       sync.Mutex
	requests []*http.Request
	bodies   []string
}

func newUpstream(t *testing.T, statuses ...int) *upstream {
	t.Helper()
	u := &upstream{statuses: statuses}
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		u.mu.Lock()
		n := len(u.requests)
		u.requests = append(u.requests, r.Clone(context.Background()))
		u.bodies = append(u.bodies, string(body))
		u.mu.Unlock()

		if u.delay > 0 {
			select {
			case <-time.After(u.delay):
			case <-r.Context().Done():
				return
			}
		}
		status := u.statuses[min(n, len(u.statuses)-1)]
		w.WriteHeader(status)
		_, _ = io.WriteString(w, http.StatusText(status))
	}))
	t.Cleanup(u.Close)
	return u
}

func (u *upstream) count() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.requests)
}

func (u *upstream) header(i int, key string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.requests[i].Header.Get(key)
}

func clientConfig(routes ...provider.Route) provider.Config {
	sheet := resilience.Defaults("orders")
	sheet.Backoff = resilience.Declare(resilience.BackoffPolicy{Base: time.Millisecond, Factor: 2, Cap: 10 * time.Millisecond})
	sheet.MaxAttempts = resilience.Declare(3)
	byName := map[string]provider.Route{}
	for _, r := range routes {
		byName[r.Name] = r
	}
	return provider.Config{
		Routes:  byName,
		Sheet:   sheet,
		Service: "checkout",
		Clock:   clock.System(),
		Rand:    func() float64 { return 0 },
	}
}

func budgeted(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return retry.WithBudget(ctx, retry.WithTotal(d))
}

func newClient(t *testing.T, cfg provider.Config) *provider.Client {
	t.Helper()
	client, err := provider.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() = %v, want nil", err)
	}
	return client
}

func request(t *testing.T, method, url, body string) *http.Request {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestNewClientRefusesAnInvalidConfig(t *testing.T) {
	t.Run("route without contract", func(t *testing.T) {
		r := route(http.MethodGet)
		r.ContractRef = ""
		if _, err := provider.NewClient(clientConfig(r)); !errors.Is(err, provider.ErrContractRequired) {
			t.Fatalf("NewClient() = %v, want ErrContractRequired (RST-04)", err)
		}
	})
	t.Run("no routes", func(t *testing.T) {
		if _, err := provider.NewClient(clientConfig()); !errors.Is(err, provider.ErrIncompleteConfig) {
			t.Fatalf("NewClient() = %v, want ErrIncompleteConfig", err)
		}
	})
	t.Run("no clock", func(t *testing.T) {
		cfg := clientConfig(route(http.MethodGet))
		cfg.Clock = nil
		if _, err := provider.NewClient(cfg); !errors.Is(err, provider.ErrIncompleteConfig) {
			t.Fatalf("NewClient() = %v, want ErrIncompleteConfig", err)
		}
	})
}

func TestDoRefusesBeforeAnyRequest(t *testing.T) {
	up := newUpstream(t, http.StatusOK)
	get := route(http.MethodGet)
	post := route(http.MethodPost)
	post.Name = "postWithoutKey"
	client := newClient(t, clientConfig(get, post))

	t.Run("undeclared route", func(t *testing.T) {
		_, err := client.Do(budgeted(t, time.Second), "missing", request(t, http.MethodGet, up.URL, ""))
		if !errors.Is(err, provider.ErrRouteNotDeclared) {
			t.Fatalf("Do() = %v, want ErrRouteNotDeclared", err)
		}
	})
	t.Run("method mismatch", func(t *testing.T) {
		_, err := client.Do(budgeted(t, time.Second), get.Name, request(t, http.MethodDelete, up.URL, ""))
		if !errors.Is(err, provider.ErrMethodMismatch) {
			t.Fatalf("Do() = %v, want ErrMethodMismatch", err)
		}
	})
	t.Run("context without deadline", func(t *testing.T) {
		_, err := client.Do(context.Background(), get.Name, request(t, http.MethodGet, up.URL, ""))
		if !errors.Is(err, deadline.ErrNoDeadline) {
			t.Fatalf("Do() = %v, want ErrNoDeadline (RST-03)", err)
		}
	})
	t.Run("non-replayable body on a retryable route", func(t *testing.T) {
		req := request(t, http.MethodGet, up.URL, "payload")
		req.GetBody = nil
		_, err := client.Do(budgeted(t, time.Second), get.Name, req)
		if !errors.Is(err, provider.ErrBodyNotReplayable) {
			t.Fatalf("Do() = %v, want ErrBodyNotReplayable", err)
		}
	})
	if up.count() != 0 {
		t.Fatalf("the upstream saw %d requests, want 0: every refusal comes before the wire", up.count())
	}
}

func TestDoReturnsTheResponseOnSuccess(t *testing.T) {
	up := newUpstream(t, http.StatusOK)
	client := newClient(t, clientConfig(route(http.MethodGet)))

	resp, err := client.Do(budgeted(t, time.Second), "placeOrder", request(t, http.MethodGet, up.URL, ""))
	if err != nil {
		t.Fatalf("Do() = %v, want nil", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "OK" {
		t.Fatalf("response = %d %q, want 200 OK", resp.StatusCode, body)
	}
}

func TestPostWithoutIdempotencyKeyIsNeverRetried(t *testing.T) {
	up := newUpstream(t, http.StatusServiceUnavailable)
	client := newClient(t, clientConfig(route(http.MethodPost)))

	resp, err := client.Do(budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodPost, up.URL, `{"a":1}`))
	if err != nil {
		t.Fatalf("Do() = %v, want the 503 response, not an error", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 returned to the caller", resp.StatusCode)
	}
	if up.count() != 1 {
		t.Fatalf("upstream saw %d requests, want 1 (RST-02)", up.count())
	}
}

func TestPostWithIdempotencyKeyIsRetriedWithAStableKeyAndBody(t *testing.T) {
	up := newUpstream(t, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK)
	r := route(http.MethodPost)
	r.IdempotencyKey = "Idempotency-Key"
	cfg := clientConfig(r)
	cfg.NewKey = func() string { return "key-123" }
	client := newClient(t, cfg)

	resp, err := client.Do(budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodPost, up.URL, `{"a":1}`))
	if err != nil {
		t.Fatalf("Do() = %v, want nil", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want the third response, 200", resp.StatusCode)
	}
	if up.count() != 3 {
		t.Fatalf("upstream saw %d requests, want 3", up.count())
	}
	for i := range 3 {
		if up.header(i, "Idempotency-Key") != "key-123" {
			t.Errorf("attempt %d Idempotency-Key = %q, want key-123 on every attempt", i, up.header(i, "Idempotency-Key"))
		}
		if up.bodies[i] != `{"a":1}` {
			t.Errorf("attempt %d body = %q, want the whole body again", i, up.bodies[i])
		}
	}
}

func TestACallerProvidedIdempotencyKeyIsKept(t *testing.T) {
	up := newUpstream(t, http.StatusOK)
	r := route(http.MethodPost)
	r.IdempotencyKey = "Idempotency-Key"
	client := newClient(t, clientConfig(r))

	req := request(t, http.MethodPost, up.URL, `{}`)
	req.Header.Set("Idempotency-Key", "caller-key")
	resp, err := client.Do(budgeted(t, time.Second), "placeOrder", req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if up.header(0, "Idempotency-Key") != "caller-key" {
		t.Fatalf("Idempotency-Key = %q, want the caller's", up.header(0, "Idempotency-Key"))
	}
}

func TestPersistentTransientStatusIsReturnedAfterTheAttempts(t *testing.T) {
	up := newUpstream(t, http.StatusServiceUnavailable)
	client := newClient(t, clientConfig(route(http.MethodGet)))

	resp, err := client.Do(budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodGet, up.URL, ""))
	if err != nil {
		t.Fatalf("Do() = %v, want the last 503 response", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 with its original status", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != http.StatusText(http.StatusServiceUnavailable) {
		t.Fatalf("body = %q, want the buffered body of the last response", body)
	}
	if up.count() != 3 {
		t.Fatalf("upstream saw %d requests, want MaxAttempts = 3", up.count())
	}
}

func TestWithoutABudgetThereIsNoRepetition(t *testing.T) {
	up := newUpstream(t, http.StatusServiceUnavailable)
	client := newClient(t, clientConfig(route(http.MethodGet)))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := client.Do(ctx, "placeOrder", request(t, http.MethodGet, up.URL, ""))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if up.count() != 1 {
		t.Fatalf("upstream saw %d requests, want 1 without retry.WithBudget (RES-31)", up.count())
	}
}

func TestTimeoutDerivesFromTheDeadline(t *testing.T) {
	up := newUpstream(t, http.StatusOK)
	up.delay = 3 * time.Second
	r := route(http.MethodGet)
	r.Budget.Limit = 5 * time.Second
	r.Budget.Slack = 50 * time.Millisecond
	client := newClient(t, clientConfig(r))

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := client.Do(ctx, "placeOrder", request(t, http.MethodGet, up.URL, ""))
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("Do() = nil, want a deadline failure (RST-03)")
	}
	if !errors.Is(err, resilience.ErrDeadlineExceeded) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Do() = %v, want a deadline category", err)
	}
	// The connection is cut at the caller's remaining time minus the slack:
	// ≈ 750ms, well before the 3s upstream and the 5s route limit.
	if elapsed < 600*time.Millisecond || elapsed > 1100*time.Millisecond {
		t.Fatalf("Do() returned after %v, want ≈ 750ms", elapsed)
	}
}

func TestNetworkFailureIsRetriedOnAnIdempotentRoute(t *testing.T) {
	var attempts atomic.Int32
	up := newUpstream(t, http.StatusOK)
	r := route(http.MethodGet)
	cfg := clientConfig(r)
	cfg.Transport = roundTripper(func(req *http.Request) (*http.Response, error) {
		if attempts.Add(1) == 1 {
			return nil, &timeoutError{}
		}
		return http.DefaultTransport.RoundTrip(req)
	})
	client := newClient(t, cfg)

	resp, err := client.Do(budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodGet, up.URL, ""))
	if err != nil {
		t.Fatalf("Do() = %v, want nil after one network retry", err)
	}
	_ = resp.Body.Close()
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type timeoutError struct{}

func (*timeoutError) Error() string   { return "dial tcp: i/o timeout" }
func (*timeoutError) Timeout() bool   { return true }
func (*timeoutError) Temporary() bool { return true }

func TestClassifier(t *testing.T) {
	cases := map[string]struct {
		err  error
		want retry.Retryability
	}{
		"transient status": {&provider.RetryableStatusError{Status: 503}, retry.Retryable},
		"network":          {&timeoutError{}, retry.Retryable},
		"cancelled":        {context.Canceled, retry.NotRetryable},
		"deadline":         {context.DeadlineExceeded, retry.NotRetryable},
		"plain error":      {errors.New("boom"), retry.NotRetryable},
		"nil":              {nil, retry.NotRetryable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := provider.Classifier(tc.err); got != tc.want {
				t.Fatalf("Classifier = %v, want %v (RST-02)", got, tc.want)
			}
		})
	}
}

func TestTheResponseBodyIsReadableAfterDoReturns(t *testing.T) {
	big := strings.Repeat("x", 4<<20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 4; i++ {
			_, _ = io.WriteString(w, big[:1<<20])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	t.Cleanup(server.Close)
	client := newClient(t, clientConfig(route(http.MethodGet)))

	resp, err := client.Do(budgeted(t, 5*time.Second), "placeOrder", request(t, http.MethodGet, server.URL, ""))
	if err != nil {
		t.Fatalf("Do() = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the body after Do returned = %v: the attempt context must not be cancelled while the body streams", err)
	}
	if len(body) != len(big) {
		t.Fatalf("read %d bytes, want %d", len(body), len(big))
	}
}
