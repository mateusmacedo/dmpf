package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func guarding(guard string) postgres.Table[string, string] {
	return postgres.Table[string, string]{
		Name:     "series",
		IDColumn: "entry_id",
		Columns:  []string{"owner_id", "status"},
		Encode:   func(s string) ([]any, error) { return []any{s, s}, nil },
		Decode:   func(func(dest ...any) error) (string, error) { return "", nil },
		Guard:    guard,
	}
}

func TestAGuardOutsideTheStateColumnsIsRefused(t *testing.T) {
	for name, guard := range map[string]string{
		"not a column":      "partner_id",
		"the id column":     "entry_id",
		"the tenant":        "tenant_id",
		"a fragment of SQL": "owner_id; DROP TABLE series",
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("Guard %q compiled; a lock keyed on a value no Save writes would serialize nothing", guard)
				}
			}()
			guarding(guard).Reader(postgres.ReadPool{})
		})
	}
}

func TestATableWithoutAGuardStillCompiles(t *testing.T) {
	guarding("").Reader(postgres.ReadPool{})
}

func TestQueryWithinOverAColumnThatIsNotTheGuardPanics(t *testing.T) {
	for name, table := range map[string]postgres.Table[string, string]{
		"another column": guarding("owner_id"),
		"no guard":       guarding(""),
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				message, _ := recover().(string)
				if !strings.Contains(message, `"series"`) || !strings.Contains(message, `"status"`) {
					t.Fatalf("panic = %q, want one naming the table and the column; a read in the transaction without the guard serializes nothing", message)
				}
			}()
			_, _ = table.Relation("status").QueryWithin(context.Background(), nil, "enabled")
		})
	}
}
