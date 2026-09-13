package tb_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/tb"
)

// spy records what a kit helper reports without failing the real test.
type spy struct {
	testing.TB
	errors []string
	fatal  string
}

func (s *spy) Helper() {}
func (s *spy) Errorf(format string, args ...any) {
	s.errors = append(s.errors, sprintf(format, args...))
}
func (s *spy) Fatalf(format string, args ...any) { s.fatal = sprintf(format, args...) }

func sprintf(format string, args ...any) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, format, args...)
	return b.String()
}

func TestReadFixtureResolvesFromTheRepoRoot(t *testing.T) {
	raw := tb.ReadFixture(t, "contracts/fixtures/orders/event/v1/item-added.golden")
	if !strings.Contains(string(raw), `"format_version": "1"`) {
		t.Fatalf("unexpected fixture content: %.80s", raw)
	}
}

func TestEnvReturnsTheValueWhenSet(t *testing.T) {
	t.Setenv("DMPF_TB_TEST", "x")
	if got := tb.Env(t, "DMPF_TB_TEST"); got != "x" {
		t.Fatalf("Env = %q", got)
	}
}

func TestEnvFailsInCIWhenUnset(t *testing.T) {
	t.Setenv("CI", "1")
	t.Setenv("DMPF_TB_MISSING", "")
	s := &spy{TB: t}
	tb.Env(s, "DMPF_TB_MISSING")
	if !strings.Contains(s.fatal, "DMPF_TB_MISSING") || !strings.Contains(s.fatal, "CI") {
		t.Fatalf("Env in CI did not fail naming the variable: %q", s.fatal)
	}
}

type verdict []string

func (v verdict) Failures() []string { return v }

func TestRequireReportsEveryFailure(t *testing.T) {
	s := &spy{TB: t}
	tb.Require(s, verdict{"first", "second"})
	if len(s.errors) != 2 {
		t.Fatalf("errors = %v, want two", s.errors)
	}
	s = &spy{TB: t}
	tb.Require(s, verdict(nil))
	if len(s.errors) != 0 {
		t.Fatalf("a passing verdict reported %v", s.errors)
	}
}

func TestRequireReportListsEveryReprovedOracle(t *testing.T) {
	var rep golden.Report
	rep.Add(
		golden.Outcome{Fixture: "f", Case: "c", Direction: golden.DirectionConsumer, Oracle: golden.OracleSemantic, Code: golden.CodeR001, OK: true},
		golden.Outcome{Fixture: "f", Case: "c", Direction: golden.DirectionProducer, Oracle: golden.OracleBytes, Code: golden.CodeR003, Field: "payload_bytes_hex", Expected: "00", Got: "01"},
		golden.Outcome{Fixture: "f", Case: "c", Direction: golden.DirectionProducer, Oracle: golden.OracleHash, Code: golden.CodeR002, Field: "payload_hash"},
	)
	s := &spy{TB: t}
	tb.RequireReport(s, rep)
	if len(s.errors) != 2 {
		t.Fatalf("errors = %v, want the two failed outcomes", s.errors)
	}
	if !strings.Contains(s.errors[0], "DMPF-R002") || !strings.Contains(s.errors[1], "DMPF-R003") {
		t.Fatalf("outcomes not in stable oracle order: %v", s.errors)
	}
}
