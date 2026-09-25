// Package pg is the Postgres side of tb, kept apart so a test that only reads
// fixtures through tb does not pull the provider and pgx into its closure —
// which is what the V29/V30 guard in fitness would name.
package pg

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// PostgresDSN is the variable every Postgres-backed suite reads. It names the
// server and an administrative database; each project's suite runs in a
// database of its own on that server.
const PostgresDSN = "PG_DSN"

// resetTimeout bounds every reset, so a lock left behind by a failed clause
// fails the cleanup instead of holding the binary until go test's -timeout.
const resetTimeout = 30 * time.Second

// Options is what a suite declares about the database it needs.
type Options struct {
	// Project names the database, <Project>_test, so projects whose suites
	// run in parallel never truncate each other's tables.
	Project      string
	Capabilities []postgres.Capability
	Schemas      []string
	Tables       []string
}

func Database(project string) string { return project + "_test" }

var cleaned sync.Map

// OpenPool migrates the project's test database and resets the declared tables
// around the test. Without the DSN it skips, or fails in CI. The DSN must be a
// loopback host: the reset is destructive.
func OpenPool(t testing.TB, opts Options) *pgxpool.Pool {
	t.Helper()
	if opts.Project == "" {
		t.Fatal("pg.OpenPool: Options.Project is required; it names the test database")
	}
	cfg := Config(t, opts.Project)
	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pg.OpenPool: pgxpool.NewWithConfig: %v", err)
	}
	t.Cleanup(pool.Close)

	// A table an earlier run created, under a name the schema no longer
	// declares, would otherwise survive and hide a missing CREATE.
	once, _ := cleaned.LoadOrStore(cfg.ConnConfig.Database, &cleanOnce{})
	if err := once.(*cleanOnce).do(ctx, pool); err != nil {
		t.Fatalf("pg.OpenPool: recreate schema public: %v", err)
	}
	if err := postgres.Migrate(ctx, pool, opts.Capabilities, opts.Schemas...); err != nil {
		t.Fatalf("pg.OpenPool: Migrate: %v", err)
	}
	tables := append(postgres.Tables(opts.Capabilities...), opts.Tables...)
	ResetTables(t, pool, tables...)
	t.Cleanup(func() {
		// Errorf, not Fatalf: FailNow inside a cleanup skips the cleanups still
		// pending, and pool.Close is one of them.
		if err := reset(pool, tables); err != nil {
			t.Errorf("pg.OpenPool: reset after the test: %v", err)
		}
	})
	return pool
}

// Config resolves the pool configuration of the project's test database,
// creating the database on the server the DSN names when it does not exist.
// A suite that hands the DSN to a process it starts reads it from here.
func Config(t testing.TB, project string) *pgxpool.Config {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(DSN(t, project))
	if err != nil {
		t.Fatalf("pg.Config: parse the DSN of %s: %v", Database(project), err)
	}
	return cfg
}

// DSN is the connection string of the project's test database, for a process
// the suite starts: it reads the database the way the binary does, from the
// environment. The server's DSN must be a URL.
func DSN(t testing.TB, project string) string {
	t.Helper()
	dsn := tb.Env(t, PostgresDSN)
	admin, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pg.DSN: parse %s: %v", PostgresDSN, err)
	}
	if host := admin.ConnConfig.Host; !loopback(host) {
		t.Fatalf("pg.DSN: %s points at %q; the harness truncates tables and only accepts a loopback host", PostgresDSN, host)
	}
	database := Database(project)
	if err := ensureDatabase(admin.ConnConfig, database); err != nil {
		t.Fatalf("pg.DSN: create %s: %v", database, err)
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		t.Fatalf("pg.DSN: %s is not a URL; the database of each project is set on its path", PostgresDSN)
	}
	u.Path = "/" + database
	return u.String()
}

// ResetTables empties the tables named.
func ResetTables(t testing.TB, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	if err := reset(pool, tables); err != nil {
		t.Fatalf("pg.ResetTables: %v", err)
	}
}

type cleanOnce struct {
	once sync.Once
	err  error
}

func (c *cleanOnce) do(ctx context.Context, pool *pgxpool.Pool) error {
	c.once.Do(func() {
		_, c.err = pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	})
	return c.err
}

// ensureDatabase treats a concurrent creation by another process as the same
// outcome, not a failure.
func ensureDatabase(admin *pgx.ConnConfig, database string) error {
	ctx, cancel := context.WithTimeout(context.Background(), resetTimeout)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, admin)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()

	var found bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", database).Scan(&found); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize())
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P04" {
		return nil
	}
	return err
}

func reset(pool *pgxpool.Pool, tables []string) error {
	if len(tables) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), resetTimeout)
	defer cancel()
	_, err := pool.Exec(ctx, "TRUNCATE "+strings.Join(tables, ", "))
	return err
}

// loopback accepts localhost, the loopback addresses and a Unix socket
// directory — everything pgx resolves without leaving the machine.
func loopback(host string) bool {
	if host == "localhost" || strings.HasPrefix(host, "/") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
