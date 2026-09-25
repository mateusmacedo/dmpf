package main

import (
	"bytes"
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
