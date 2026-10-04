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

func TestRunRefusesToStartWithoutARole(t *testing.T) {
	var errOut bytes.Buffer

	code := run(options{role: "", lookup: lookup()}, &errOut)

	if code != exitUsage || !strings.Contains(errOut.String(), "--role api|relay") {
		t.Fatalf("run() = %d, stderr %q; want usage listing api|relay", code, errOut.String())
	}
}

func TestRunRefusesTheConsumerRole(t *testing.T) {
	var errOut bytes.Buffer

	code := run(options{role: "consumer", lookup: lookup("PG_DSN", "postgres://x")}, &errOut)

	if code != exitUsage || !strings.Contains(errOut.String(), "consumer") {
		t.Fatalf("run() = %d, stderr %q; want usage naming the consumer role", code, errOut.String())
	}
}

func TestRunNamesTheMissingVariable(t *testing.T) {
	cases := []struct {
		role     string
		env      func(string) string
		variable string
	}{
		{"api", lookup("GRPC_INSECURE", "true"), "PG_DSN"},
		{"api", lookup("PG_DSN", "postgres://x"), "GRPC_INSECURE"},
		{"relay", lookup("PG_DSN", "postgres://x"), "KAFKA_BROKERS"},
	}
	for _, tc := range cases {
		t.Run(tc.role+" without "+tc.variable, func(t *testing.T) {
			var errOut bytes.Buffer

			code := run(options{role: tc.role, lookup: tc.env}, &errOut)

			if code != exitUsage || !strings.Contains(errOut.String(), tc.variable) {
				t.Fatalf("run() = %d, stderr %q; want usage naming %s", code, errOut.String(), tc.variable)
			}
		})
	}
}

func TestRunRefusesAnUnknownRole(t *testing.T) {
	var errOut bytes.Buffer

	code := run(options{role: "banana", lookup: lookup("PG_DSN", "postgres://x")}, &errOut)

	if code != exitUsage || !strings.Contains(errOut.String(), "banana") {
		t.Fatalf("run() = %d, stderr %q; want usage naming the role it does not know", code, errOut.String())
	}
}
