//go:build integration

package appkit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/appkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

func TestTheDatabaseHoldsExactlyTheTablesOfThisContext(t *testing.T) {
	got := pg.Tables(t, appkit.OpenPool(t))

	want := []string{"inbox", "orders", "outbox", "quarantine"}
	if !slices.Equal(got, want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
}

func TestTheRuntimeRoleWritesOrdersButNeverRemovesThem(t *testing.T) {
	pool := pg.OpenRuntimePool(t, appkit.PoolOptions)

	if _, err := pool.Exec(context.Background(), "INSERT INTO orders (tenant_id, order_id, version, snapshot) VALUES ('t-1', 'o-1', 1, '{}')"); err != nil {
		t.Fatalf("INSERT INTO orders by the runtime role = %v, want nil", err)
	}
	if _, err := pool.Exec(context.Background(), "UPDATE orders SET version = version"); err != nil {
		t.Fatalf("UPDATE orders by the runtime role = %v, want nil", err)
	}
	for _, statement := range []string{"DELETE FROM orders", "TRUNCATE orders", "DROP TABLE orders", "ALTER TABLE orders ADD COLUMN intruder bigint"} {
		pg.AssertDenied(t, pool, statement)
	}
}

func TestTheRuntimeRoleNeitherRewritesTheKernelTablesNorCreatesATable(t *testing.T) {
	pool := pg.OpenRuntimePool(t, appkit.PoolOptions)

	for _, statement := range []string{
		"UPDATE quarantine SET reason = reason",
		"DELETE FROM quarantine",
		"TRUNCATE outbox",
		"TRUNCATE inbox",
		"CREATE TABLE intruder (id bigint)",
	} {
		pg.AssertDenied(t, pool, statement)
	}
}
