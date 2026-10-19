package postgres

import (
	"errors"
	"strings"
	"testing"
)

type guarded struct{ Owner, Status string }

func guardedTable(encode func(guarded) ([]any, error)) Table[string, guarded] {
	return Table[string, guarded]{
		Name:     "series",
		IDColumn: "entry_id",
		Columns:  []string{"owner_id", "status"},
		Encode:   encode,
		Decode:   func(func(dest ...any) error) (guarded, error) { return guarded{}, nil },
		Guard:    "owner_id",
	}
}

func TestAGuardValueThatIsNotAStringNeverReachesTheServer(t *testing.T) {
	ctx := tenantScoped(t)
	byOwner := guardedTable(func(g guarded) ([]any, error) { return []any{g.Owner, g.Status}, nil }).Relation("owner_id")
	numbered := guardedTable(func(g guarded) ([]any, error) { return []any{len(g.Owner), g.Status}, nil })

	for name, value := range map[string]any{"a number": 42, "nil": nil} {
		if _, err := byOwner.QueryWithin(ctx, nil, value); !errors.Is(err, ErrGuardValue) {
			t.Fatalf("QueryWithin(%s) = %v, want ErrGuardValue before the nil transaction is touched", name, err)
		}
	}
	err := numbered.Repository(nil).Save(ctx, "E-1", guarded{Owner: "O-1", Status: "enabled"}, 0)
	if !errors.Is(err, ErrGuardValue) || strings.Contains(err.Error(), "O-1") {
		t.Fatalf("Save() of a numeric guard = %v, want ErrGuardValue naming no value (ERR-07)", err)
	}
}
