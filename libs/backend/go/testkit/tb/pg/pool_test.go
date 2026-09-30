//go:build integration

package pg

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
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

func TestEachTestGetsADatabaseOfItsOwnThatIsDroppedAfterIt(t *testing.T) {
	var first, second string
	t.Run("first", func(t *testing.T) {
		first = Config(t, "pgkit_scope").ConnConfig.Database
		if again := Config(t, "pgkit_scope").ConnConfig.Database; again != first {
			t.Fatalf("Config() = %s then %s, want one database per test", first, again)
		}
	})
	t.Run("second", func(t *testing.T) {
		second = Config(t, "pgkit_scope").ConnConfig.Database
	})

	if first == second || !strings.HasPrefix(first, "pgkit_scope_test_") {
		t.Fatalf("databases = %s, %s; want two distinct pgkit_scope_test_<id>", first, second)
	}
	for _, database := range []string{first, second} {
		if databaseExists(t, database) {
			t.Errorf("%s survived its test", database)
		}
	}
}

func TestConcurrentCallsOfOneTestShareItsDatabase(t *testing.T) {
	const callers = 8
	names := make([]string, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Go(func() { names[i] = DSN(t, "pgkit_race") })
	}
	wg.Wait()

	for _, name := range names[1:] {
		if name != names[0] {
			t.Fatalf("DSN() = %v, want one database for every call of the test", names)
		}
	}
}

func databaseExists(t *testing.T, database string) bool {
	t.Helper()
	admin, err := pgx.Connect(context.Background(), tb.Env(t, PostgresDSN))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = admin.Close(context.Background()) }()
	var found bool
	if err := admin.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", database).Scan(&found); err != nil {
		t.Fatalf("look up %s: %v", database, err)
	}
	return found
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
