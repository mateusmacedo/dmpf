//go:build integration

package appkit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/appkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

func TestTheDatabaseHoldsExactlyTheTablesOfThisContext(t *testing.T) {
	got := pg.Tables(t, appkit.OpenPool(t))

	want := []string{"bookings", "inbox", "outbox", "quarantine", "resources"}
	if !slices.Equal(got, want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
}

func TestTheRuntimeRoleWritesBookingsAndResourcesButNeverRemovesThem(t *testing.T) {
	pool := pg.OpenRuntimePool(t, appkit.PoolOptions)

	for table, insert := range map[string]string{
		"bookings":  "INSERT INTO bookings (tenant_id, booking_id, resource_id, snapshot) VALUES ('t-1', 'b-1', 'r-1', '{}')",
		"resources": "INSERT INTO resources (tenant_id, resource_id, snapshot) VALUES ('t-1', 'r-1', '{}')",
	} {
		if _, err := pool.Exec(context.Background(), insert); err != nil {
			t.Fatalf("INSERT INTO %s by the runtime role = %v, want nil", table, err)
		}
		if _, err := pool.Exec(context.Background(), "UPDATE "+table+" SET version = version"); err != nil {
			t.Fatalf("UPDATE %s by the runtime role = %v, want nil", table, err)
		}
		for _, statement := range []string{"DELETE FROM " + table, "TRUNCATE " + table, "DROP TABLE " + table, "ALTER TABLE " + table + " ADD COLUMN intruder bigint"} {
			pg.AssertDenied(t, pool, statement)
		}
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
