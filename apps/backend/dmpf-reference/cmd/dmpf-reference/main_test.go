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
	var out, errOut bytes.Buffer

	code := run(options{role: "", lookup: lookup()}, &out, &errOut)

	if code != exitUsage {
		t.Fatalf("run() = %d, want %d (usage)", code, exitUsage)
	}
	if !strings.Contains(errOut.String(), "--role api|relay|consumer") {
		t.Fatalf("stderr = %q, want the usage listing the three roles", errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing on a usage error", out.String())
	}
}

func TestRunRefusesAnUnknownRole(t *testing.T) {
	var out, errOut bytes.Buffer

	code := run(options{role: "worker", lookup: lookup("DMPF_PG_DSN", "postgres://x")}, &out, &errOut)

	if code != exitUsage {
		t.Fatalf("run() = %d, want %d (usage)", code, exitUsage)
	}
	if !strings.Contains(errOut.String(), "worker") || !strings.Contains(errOut.String(), "api|relay|consumer") {
		t.Fatalf("stderr = %q, want the unknown role named and the three accepted ones", errOut.String())
	}
}

func TestRunNamesTheMissingVariable(t *testing.T) {
	cases := []struct {
		role     string
		env      func(string) string
		variable string
	}{
		{"api", lookup(), "DMPF_PG_DSN"},
		{"relay", lookup("DMPF_PG_DSN", "postgres://x"), "DMPF_KAFKA_BROKERS"},
		{"consumer", lookup("DMPF_PG_DSN", "postgres://x", "DMPF_KAFKA_BROKERS", "b:9092", "DMPF_KAFKA_TOPIC", "t", "DMPF_KAFKA_DLQ", "d",
			"DMPF_KAFKA_RESERVATIONS_TOPIC", "rt", "DMPF_KAFKA_RESERVATIONS_DLQ", "rd"), "DMPF_KAFKA_GROUP"},
	}
	for _, tc := range cases {
		t.Run(tc.role+" without "+tc.variable, func(t *testing.T) {
			var out, errOut bytes.Buffer

			code := run(options{role: tc.role, lookup: tc.env}, &out, &errOut)

			if code != exitUsage {
				t.Fatalf("run() = %d, want %d (usage)", code, exitUsage)
			}
			if !strings.Contains(errOut.String(), tc.variable) {
				t.Fatalf("stderr = %q, want %s named", errOut.String(), tc.variable)
			}
		})
	}
}
