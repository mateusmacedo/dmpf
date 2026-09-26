//go:build integration

package appkit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/appkit"
)

func TestTheDatabaseHoldsExactlyTheTablesOfThisContext(t *testing.T) {
	pool := appkit.OpenPool(t)

	rows, err := pool.Query(context.Background(),
		"SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name")
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tables: %v", err)
	}

	want := []string{"inbox", "outbox", "quarantine", "reservations"}
	if !slices.Equal(got, want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
}
