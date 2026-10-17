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
			var errOut bytes.Buffer

			code := run(options{lookup: tc.env}, &errOut)

			if code != exitUsage {
				t.Fatalf("run() = %d, want %d (usage)", code, exitUsage)
			}
			if !strings.Contains(errOut.String(), tc.named) {
				t.Fatalf("stderr = %q, want %s named", errOut.String(), tc.named)
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
		return lookup("GRPC_INSECURE", "true", "AUTH_DEV_MOCK", "true", "ADMIN_ADDR", addr,
			"ORDERS_GRPC_TARGET", "o:9090", "RESERVATIONS_GRPC_TARGET", "r:9090", "BOOKINGS_GRPC_TARGET", "b:9090",
			"REDIS_URL", "redis://revocation:secret@127.0.0.1:1/0")
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
			var errOut bytes.Buffer

			if code := run(options{lookup: tc.env, args: []string{"healthcheck"}}, &errOut); code != tc.want {
				t.Fatalf("run(healthcheck) = %d, want %d (stderr %q)", code, tc.want, errOut.String())
			}
		})
	}
}

func TestTheSessionEventsRoleReadsOnlyItsOwnConfiguration(t *testing.T) {
	var errOut bytes.Buffer

	code := run(options{lookup: lookup(), args: []string{"session-events"}}, &errOut)

	if code != exitUsage {
		t.Fatalf("run(session-events) = %d, want %d (usage)", code, exitUsage)
	}
	if !strings.Contains(errOut.String(), "OIDC_ISSUER") || strings.Contains(errOut.String(), "GRPC_TARGET") {
		t.Fatalf("stderr = %q, want the role's own OIDC_ISSUER named and no variable of the API", errOut.String())
	}
}

func TestTheSessionEventsHealthcheckNeedsOnlyTheAdministrationAddress(t *testing.T) {
	admin := func(code int) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }))
		t.Cleanup(srv.Close)
		return srv.Listener.Addr().String()
	}
	cases := []struct {
		name string
		env  func(string) string
		want int
	}{
		{"role ready", lookup("ADMIN_ADDR", admin(http.StatusNoContent)), exitOK},
		{"role not ready", lookup("ADMIN_ADDR", admin(http.StatusServiceUnavailable)), exitFailure},
		{"invalid address", lookup("ADMIN_ADDR", "8092"), exitFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var errOut bytes.Buffer

			if code := run(options{lookup: tc.env, args: []string{"session-events", "healthcheck"}}, &errOut); code != tc.want {
				t.Fatalf("run(session-events healthcheck) = %d, want %d (stderr %q)", code, tc.want, errOut.String())
			}
		})
	}
}
