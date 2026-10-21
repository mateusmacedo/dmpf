package postgres

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestExternalAttemptChecksCapAtTheKernelLimit(t *testing.T) {
	for _, column := range []string{`cardinality\(observation_ids\)`, `jsonb_array_length\(conflicts\)`} {
		match := regexp.MustCompile(column + `\s*<=\s*(\d+)`).FindStringSubmatch(externalAttemptSchema)
		if match == nil {
			t.Fatalf("external_attempt.sql has no CHECK on %s", column)
		}
		if n, _ := strconv.Atoi(match[1]); n != ports.MaxAttemptEntries {
			t.Fatalf("CHECK on %s caps at %d, want ports.MaxAttemptEntries = %d", column, n, ports.MaxAttemptEntries)
		}
	}
}
