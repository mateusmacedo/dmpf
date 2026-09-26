//go:build integration

package pg

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func tableExists(t *testing.T, pool *pgxpool.Pool, table string) bool {
	t.Helper()
	var found bool
	err := pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)`, table).Scan(&found)
	if err != nil {
		t.Fatalf("look up %s: %v", table, err)
	}
	return found
}

func TestAProducerDatabaseCarriesOnlyTheOutbox(t *testing.T) {
	pool := OpenPool(t, Options{Project: "pgkit_producer", Capabilities: []postgres.Capability{postgres.Outbox}})

	if !tableExists(t, pool, "outbox") {
		t.Error("outbox is missing from a database that declared the Outbox capability")
	}
	for _, table := range []string{"inbox", "quarantine"} {
		if tableExists(t, pool, table) {
			t.Errorf("%s exists in a database that did not declare the Inbox capability", table)
		}
	}
}

func TestAConsumerDatabaseCarriesTheInboxAndTheQuarantine(t *testing.T) {
	pool := OpenPool(t, Options{Project: "pgkit_consumer", Capabilities: []postgres.Capability{postgres.Outbox, postgres.Inbox}})

	for _, table := range []string{"outbox", "inbox", "quarantine"} {
		if !tableExists(t, pool, table) {
			t.Errorf("%s is missing from a database that declared Outbox and Inbox", table)
		}
	}
}

func TestATableOfAnEarlierRunDoesNotSurvive(t *testing.T) {
	const project = "pgkit_clean"
	leftover, err := pgxpool.NewWithConfig(context.Background(), Config(t, project))
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig: %v", err)
	}
	t.Cleanup(leftover.Close)
	if _, err := leftover.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS leftovers (id text)"); err != nil {
		t.Fatalf("create leftovers: %v", err)
	}
	cleaned.Delete(Database(project))

	pool := OpenPool(t, Options{Project: project, Capabilities: []postgres.Capability{postgres.Outbox}})

	if tableExists(t, pool, "leftovers") {
		t.Error("a table no schema declares survived the first open of the process")
	}
}

func TestTheProjectTablesAreMigratedAndReset(t *testing.T) {
	opts := Options{
		Project:      "pgkit_context",
		Capabilities: []postgres.Capability{postgres.Outbox},
		Schemas:      []string{"CREATE TABLE IF NOT EXISTS widgets (id text PRIMARY KEY)"},
		Tables:       []string{"widgets"},
	}
	pool := OpenPool(t, opts)
	if _, err := pool.Exec(context.Background(), "INSERT INTO widgets VALUES ('w-1')"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	again := OpenPool(t, opts)

	var count int
	if err := again.QueryRow(context.Background(), "SELECT count(*) FROM widgets").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("widgets holds %d rows after a reopen, want 0: the declared table was not reset", count)
	}
}
