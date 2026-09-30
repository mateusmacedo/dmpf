package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func lookup(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

func TestRunRefusesToStartWithoutConfiguration(t *testing.T) {
	cases := []struct {
		name  string
		env   func(string) string
		named string
	}{
		{"orders target", lookup("GRPC_INSECURE", "true", "RESERVATIONS_GRPC_TARGET", "r:9090"), "ORDERS_GRPC_TARGET"},
		{"reservations target", lookup("GRPC_INSECURE", "true", "ORDERS_GRPC_TARGET", "o:9090"), "RESERVATIONS_GRPC_TARGET"},
		{"bookings target", lookup("GRPC_INSECURE", "true", "ORDERS_GRPC_TARGET", "o:9090", "RESERVATIONS_GRPC_TARGET", "r:9090"), "BOOKINGS_GRPC_TARGET"},
		{"transport policy", lookup("ORDERS_GRPC_TARGET", "o:9090", "RESERVATIONS_GRPC_TARGET", "r:9090", "BOOKINGS_GRPC_TARGET", "b:9090"), "GRPC_CA_FILE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer

			code := run(options{lookup: tc.env}, &out, &errOut)

			if code != exitUsage {
				t.Fatalf("run() = %d, want %d (usage)", code, exitUsage)
			}
			if !strings.Contains(errOut.String(), tc.named) {
				t.Fatalf("stderr = %q, want %s named", errOut.String(), tc.named)
			}
			if out.Len() != 0 {
				t.Fatalf("stdout = %q, want nothing on a refused start", out.String())
			}
		})
	}
}

func TestHealthcheckAnswersOnlyHealthyOrUnhealthy(t *testing.T) {
	edge := func(code int) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }))
		t.Cleanup(srv.Close)
		return srv.Listener.Addr().String()
	}
	configured := func(addr string) func(string) string {
		return lookup("GRPC_INSECURE", "true", "AUTH_DEV_MOCK", "true", "HTTP_ADDR", addr,
			"ORDERS_GRPC_TARGET", "o:9090", "RESERVATIONS_GRPC_TARGET", "r:9090", "BOOKINGS_GRPC_TARGET", "b:9090")
	}
	cases := []struct {
		name string
		env  func(string) string
		want int
	}{
		{"edge ready", configured(edge(http.StatusNoContent)), exitOK},
		{"edge not ready", configured(edge(http.StatusServiceUnavailable)), exitFailure},
		{"no configuration", lookup(), exitFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer

			if code := run(options{lookup: tc.env, args: []string{"healthcheck"}}, &out, &errOut); code != tc.want {
				t.Fatalf("run(healthcheck) = %d, want %d (stderr %q)", code, tc.want, errOut.String())
			}
		})
	}
}
