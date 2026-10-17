package serve

import (
	"errors"
	"strings"
	"testing"
)

func TestRedactHidesTheMigratorWithoutHidingTheMessage(t *testing.T) {
	const dsn = "postgres://postgres:postgres@db:5432/orders"
	cases := map[string]string{
		"failed to connect to `user=postgres database=orders`: refused":              "failed to connect to `user=[redacted] database=orders`: refused",
		`FATAL: password authentication failed for user "postgres" (SQLSTATE 28P01)`: `FATAL: password authentication failed for user "[redacted]" (SQLSTATE 28P01)`,
		`FATAL: role "postgres" does not exist (SQLSTATE 28000)`:                     `FATAL: role "[redacted]" does not exist (SQLSTATE 28000)`,
		"postgres: grants: no grant declared for ledger":                             "postgres: grants: no grant declared for ledger",
		"cannot parse `" + dsn + "`":                                                 "cannot parse `[redacted]`",
	}
	for cause, want := range cases {
		err := redact("grants", errors.New(cause), dsn, "postgres")

		if got := err.Error(); !strings.HasSuffix(got, want) {
			t.Errorf("redact(%q) = %q, want it ending in %q", cause, got, want)
		}
	}
}
