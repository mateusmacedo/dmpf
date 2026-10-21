package http_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// WroteRequest runs in the transport's write goroutine (net/http/request.go,
// Request.write defer), so Do may return before it: tests wait on wrote.
type traceLog struct {
	mu     sync.Mutex
	events []string
	wrote  chan struct{}
}

func newTraceLog() *traceLog { return &traceLog{wrote: make(chan struct{}, 8)} }

func (l *traceLog) awaitWrites(t *testing.T, n int) {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for i := range n {
		select {
		case <-l.wrote:
		case <-timeout:
			t.Fatalf("WroteRequest fired %d times, want %d; trace events = %v", i, n, l.snapshot())
		}
	}
}

func (l *traceLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *traceLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.events)
}

func (l *traceLog) context(ctx context.Context) context.Context {
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			if info.Reused {
				l.add("conn-reused")
				return
			}
			l.add("conn-new")
		},
		ConnectDone: func(_, _ string, err error) {
			if err != nil {
				l.add("connect-error")
			}
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err != nil {
				l.add("wrote-error")
				l.wrote <- struct{}{}
				return
			}
			l.add("wrote")
			l.wrote <- struct{}{}
		},
		GotFirstResponseByte: func() { l.add("first-byte") },
	})
}

func ownTransport(t *testing.T, tr *http.Transport) *http.Transport {
	t.Helper()
	t.Cleanup(tr.CloseIdleConnections)
	return tr
}

func hangUp(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		t.Errorf("Hijack() = %v", err)
		return
	}
	_ = conn.Close()
}

func indexOf(events []string, event string) int { return slices.Index(events, event) }

func TestTheClientTraceOfTheCallerCrossesDo(t *testing.T) {
	answer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, "ok")
	})
	cases := map[string]struct {
		server    func() *httptest.Server
		transport func(srv *httptest.Server) *http.Transport
		proto     int
	}{
		"HTTP/1.1": {
			server:    func() *httptest.Server { return httptest.NewServer(answer) },
			transport: func(*httptest.Server) *http.Transport { return &http.Transport{} },
			proto:     1,
		},
		"HTTP/2": {
			server: func() *httptest.Server {
				srv := httptest.NewUnstartedServer(answer)
				srv.EnableHTTP2 = true
				srv.StartTLS()
				return srv
			},
			transport: func(srv *httptest.Server) *http.Transport {
				var protocols http.Protocols
				protocols.SetHTTP2(true)
				return &http.Transport{
					TLSClientConfig: srv.Client().Transport.(*http.Transport).TLSClientConfig.Clone(),
					Protocols:       &protocols,
				}
			},
			proto: 2,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := tc.server()
			t.Cleanup(srv.Close)
			cfg := clientConfig(route(http.MethodPost))
			cfg.Transport = ownTransport(t, tc.transport(srv))
			client := newClient(t, cfg)
			log := newTraceLog()

			resp, err := client.Do(log.context(budgeted(t, 2*time.Second)), "placeOrder", request(t, http.MethodPost, srv.URL, `{"a":1}`))
			if err != nil {
				t.Fatalf("Do() = %v, want nil", err)
			}
			_ = resp.Body.Close()
			if resp.ProtoMajor != tc.proto {
				t.Fatalf("response protocol = %s, want HTTP/%d", resp.Proto, tc.proto)
			}
			log.awaitWrites(t, 1)
			events := log.snapshot()
			if indexOf(events, "wrote") < 0 || indexOf(events, "first-byte") < 0 {
				t.Fatalf("trace events = %v, want WroteRequest without error and GotFirstResponseByte", events)
			}
		})
	}
}

func TestWroteRequestFiresForAServerThatHangsUpAfterReadingTheBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		hangUp(t, w)
	}))
	t.Cleanup(srv.Close)
	cfg := clientConfig(route(http.MethodPost))
	cfg.Transport = ownTransport(t, &http.Transport{})
	client := newClient(t, cfg)
	log := newTraceLog()

	resp, err := client.Do(log.context(budgeted(t, 2*time.Second)), "placeOrder", request(t, http.MethodPost, srv.URL, `{"a":1}`))
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("Do() = nil, want the error of the hung-up connection")
	}

	log.awaitWrites(t, 1)
	events := log.snapshot()
	if indexOf(events, "wrote") < 0 {
		t.Fatalf("trace events = %v, want WroteRequest without error: the server read the whole body", events)
	}
	if indexOf(events, "first-byte") >= 0 {
		t.Fatalf("trace events = %v, want no GotFirstResponseByte", events)
	}
}

func TestWroteRequestDoesNotFireWhenTheDialFails(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := "http://" + ln.Addr().String()
	_ = ln.Close()
	cfg := clientConfig(route(http.MethodPost))
	cfg.Transport = ownTransport(t, &http.Transport{})
	client := newClient(t, cfg)
	log := newTraceLog()

	resp, err := client.Do(log.context(budgeted(t, 2*time.Second)), "placeOrder", request(t, http.MethodPost, closed, `{"a":1}`))
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("Do() = nil, want the dial error")
	}

	events := log.snapshot()
	if indexOf(events, "connect-error") < 0 {
		t.Fatalf("trace events = %v, want the failed ConnectDone", events)
	}
	if indexOf(events, "wrote") >= 0 || indexOf(events, "wrote-error") >= 0 {
		t.Fatalf("trace events = %v, want no WroteRequest", events)
	}
}

type keepAliveKiller struct {
	*httptest.Server
	arrivals atomic.Int32

	mu     sync.Mutex
	keys   []string
	bodies []string
}

func newKeepAliveKiller(t *testing.T) *keepAliveKiller {
	t.Helper()
	k := &keepAliveKiller{}
	k.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		k.mu.Lock()
		k.keys = append(k.keys, r.Header.Get("Idempotency-Key"))
		k.bodies = append(k.bodies, string(body))
		k.mu.Unlock()
		if k.arrivals.Add(1) == 2 {
			hangUp(t, w)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(k.Close)
	return k
}

func TestTheTransportResendsAKeyedPostOnADeadKeepAliveUnseenByResendCount(t *testing.T) {
	srv := newKeepAliveKiller(t)
	spans, tracer := tracedConfig(t)
	r := route(http.MethodPost)
	r.IdempotencyKey = "Idempotency-Key"
	cfg := clientConfig(r)
	cfg.Tracer = tracer
	cfg.Transport = ownTransport(t, &http.Transport{})
	keys := []string{"warm-up", "key-123"}
	cfg.NewKey = func() string {
		key := keys[0]
		keys = keys[1:]
		return key
	}
	client := newClient(t, cfg)

	warm, err := client.Do(budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodPost, srv.URL, `{"warm":1}`))
	if err != nil {
		t.Fatalf("warm-up Do() = %v", err)
	}
	_, _ = io.Copy(io.Discard, warm.Body)
	_ = warm.Body.Close()
	before := len(clientSpans(spans()))

	log := newTraceLog()
	resp, err := client.Do(log.context(budgeted(t, 2*time.Second)), "placeOrder", request(t, http.MethodPost, srv.URL, `{"a":1}`))
	if err != nil {
		t.Fatalf("Do() = %v, want the transport's own resend to answer", err)
	}
	_ = resp.Body.Close()

	if got := srv.arrivals.Load(); got != 3 {
		t.Fatalf("server saw %d requests, want 3: warm-up, the hung-up one, the resend", got)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	for i := 1; i <= 2; i++ {
		if srv.keys[i] != "key-123" || srv.bodies[i] != `{"a":1}` {
			t.Errorf("request %d = key %q body %q, want the same key and body on the resend", i, srv.keys[i], srv.bodies[i])
		}
	}
	log.awaitWrites(t, 2)
	events := log.snapshot()
	if indexOf(events, "conn-reused") != 0 || indexOf(events, "conn-new") < 0 {
		t.Errorf("trace events = %v, want a reused connection, then a new one for the resend", events)
	}
	clients := clientSpans(spans())[before:]
	if len(clients) != 1 {
		t.Fatalf("%d CLIENT spans for the call, want 1: the resend happens below otelhttp", len(clients))
	}
	if count, present := attrsOf(clients[0])["http.request.resend_count"]; present {
		t.Errorf("http.request.resend_count = %v, want it absent: resendCounter never sees the transport's resend", count)
	}
}

func TestTheTransportDoesNotResendAnUnkeyedPostOnADeadKeepAlive(t *testing.T) {
	srv := newKeepAliveKiller(t)
	cfg := clientConfig(route(http.MethodPost))
	cfg.Transport = ownTransport(t, &http.Transport{})
	client := newClient(t, cfg)

	warm, err := client.Do(budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodPost, srv.URL, `{"warm":1}`))
	if err != nil {
		t.Fatalf("warm-up Do() = %v", err)
	}
	_, _ = io.Copy(io.Discard, warm.Body)
	_ = warm.Body.Close()

	resp, err := client.Do(budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodPost, srv.URL, `{"a":1}`))
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("Do() = nil, want the hung-up connection's error")
	}
	if got := srv.arrivals.Load(); got != 2 {
		t.Fatalf("server saw %d requests, want 2: without the key the transport does not resend", got)
	}
}
