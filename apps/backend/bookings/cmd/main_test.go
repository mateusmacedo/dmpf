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

	if code != exitUsage || !strings.Contains(errOut.String(), "--role api|relay") || out.Len() != 0 {
		t.Fatalf("run() = %d, stderr %q, stdout %q; want usage listing api|relay", code, errOut.String(), out.String())
	}
}

func TestRunRefusesARoleThisContextDoesNotHave(t *testing.T) {
	var out, errOut bytes.Buffer

	code := run(options{role: "consumer", lookup: lookup("PG_DSN", "postgres://x")}, &out, &errOut)

	if code != exitUsage || !strings.Contains(errOut.String(), "api|relay") {
		t.Fatalf("run() = %d, stderr %q; want the refusal to name the roles it has", code, errOut.String())
	}
}

func TestRunRefusesTheApiRoleWithoutTheDatabase(t *testing.T) {
	var out, errOut bytes.Buffer

	code := run(options{role: "api", lookup: lookup()}, &out, &errOut)

	if code != exitUsage || !strings.Contains(errOut.String(), "PG_DSN") {
		t.Fatalf("run() = %d, stderr %q; want the refusal to name the missing variable", code, errOut.String())
	}
}

func TestRunRefusesTheRelayRoleWithoutItsBroker(t *testing.T) {
	var out, errOut bytes.Buffer

	code := run(options{role: "relay", lookup: lookup("PG_DSN", "postgres://x")}, &out, &errOut)

	if code != exitUsage {
		t.Fatalf("run() = %d, want exitUsage: the relay needs the broker", code)
	}
	if !strings.Contains(errOut.String(), "KAFKA_BROKERS") {
		t.Fatalf("stderr = %q, want it to name KAFKA_BROKERS, the first missing variable of the relay", errOut.String())
	}
}
