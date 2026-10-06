package app_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/api"
)

func edgeAnswering(t *testing.T, code int, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != api.ReadinessPath {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort() = %v", err)
	}
	return port
}

func probe(t *testing.T, listen string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return app.Probe(ctx, app.Config{AdminAddr: listen})
}

func TestProbeReachesTheEdgeOnTheLoopbackWhenItListensOnEveryAddress(t *testing.T) {
	port := edgeAnswering(t, http.StatusNoContent, "")

	for _, listen := range []string{":" + port, "0.0.0.0:" + port, "127.0.0.1:" + port} {
		if err := probe(t, listen); err != nil {
			t.Fatalf("Probe(%s) = %v, want nil", listen, err)
		}
	}
}

func TestProbeCarriesWhyTheEdgeIsNotReady(t *testing.T) {
	port := edgeAnswering(t, http.StatusServiceUnavailable, "rpc: contexts not ready: orders\n")

	err := probe(t, ":"+port)

	if !errors.Is(err, app.ErrNotReady) || !strings.Contains(err.Error(), "orders") {
		t.Fatalf("Probe() = %v, want ErrNotReady naming orders", err)
	}
}

func TestProbeFailsWhenNothingListens(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() = %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	if err := probe(t, addr); !errors.Is(err, app.ErrNotReady) {
		t.Fatalf("Probe() = %v, want ErrNotReady", err)
	}
}

func TestProbeAsksTheAdministrationPortAndNotThePublicOne(t *testing.T) {
	public := edgeAnswering(t, http.StatusNoContent, "")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() = %v", err)
	}
	admin := listener.Addr().String()
	_ = listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.Probe(ctx, app.Config{HTTPAddr: "127.0.0.1:" + public, AdminAddr: admin}); !errors.Is(err, app.ErrNotReady) {
		t.Fatalf("Probe() = %v, want ErrNotReady from the silent administration port despite the public one answering", err)
	}
}
