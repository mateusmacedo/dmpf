package redact_test

import (
	"regexp"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

var wordsStatement = regexp.MustCompile(`^[A-Za-z][A-Za-z ,.-]*`)

func TestWithoutValuesKeepsTheStatementAndRedactsWhatFollowsIt(t *testing.T) {
	cases := map[string]struct {
		message string
		want    string
	}{
		"statement only":            {"server shutting down\n", "server shutting down"},
		"value after the statement": {"accept failed: dial tcp 10.0.0.7:443", "accept failed " + redact.Placeholder},
		"separators before a value": {"closing conn, 10.0.0.7", "closing conn " + redact.Placeholder},
		"no statement at all":       {"10.0.0.7 refused", redact.Placeholder},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := redact.WithoutValues(tc.message, wordsStatement); got != tc.want {
				t.Fatalf("WithoutValues(%q) = %q, want %q", tc.message, got, tc.want)
			}
		})
	}
}
