// Package pg is the Postgres side of tb, kept apart so a test that only reads
// fixtures through tb does not pull the provider and pgx into its closure —
// which is what the V29/V30 guard in fitness would name.
package pg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// PostgresDSN is the variable every Postgres-backed suite reads. It names the
// server and an administrative database; each test runs in a database of its
// own on that server, created for it and dropped after it.
const PostgresDSN = "PG_DSN"

// TestCluster is the cluster_name of the test server (infra/test/compose.yml).
// A server under any other name is refused before a database is created or
// dropped, so a suite pointed at the runtime infra fails instead of writing to it.
const TestCluster = "test"

var ErrNotTestCluster = errors.New("pg: the server is not the test cluster")

// resetTimeout bounds every reset, so a lock left behind by a failed clause
// fails the cleanup instead of holding the binary until go test's -timeout.
const resetTimeout = 30 * time.Second

// Options is what a suite declares about the database it needs.
type Options struct {
	// Project prefixes the database, <Project>_test_<id>, so a leftover of a
	// crashed run is attributable to the suite that left it.
	Project      string
	Capabilities []postgres.Capability
	Schemas      []string
	Tables       []string
}

type scope struct {
	t       testing.TB
	project string
}

type testDatabase struct {
	once sync.Once
	dsn  string
}

var databases sync.Map

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

// Config resolves the pool configuration of the test's database, creating it
// on the first call of the test. A suite that hands the DSN to a process it
// starts reads it from here.
func Config(t testing.TB, project string) *pgxpool.Config {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(DSN(t, project))
	if err != nil {
		t.Fatalf("pg.Config: parse the DSN of the %s database: %v", project, err)
	}
	return cfg
}

// DSN is the connection string of the test's database, for a process the
// suite starts. Every call of one test answers the same database, which is
// dropped when the test ends. The server's DSN must be a URL.
func DSN(t testing.TB, project string) string {
	t.Helper()
	entry, _ := databases.LoadOrStore(scope{t, project}, &testDatabase{})
	db := entry.(*testDatabase)
	db.once.Do(func() { db.dsn = create(t, project) })
	if db.dsn == "" {
		t.Fatalf("pg.DSN: the %s database of this test was not created", project)
	}
	return db.dsn
}

func create(t testing.TB, project string) string {
	t.Helper()
	dsn := tb.Env(t, PostgresDSN)
	admin, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pg.DSN: parse %s: %v", PostgresDSN, err)
	}
	if host := admin.ConnConfig.Host; !loopback(host) {
		t.Fatalf("pg.DSN: %s points at %q; the harness creates and drops databases and only accepts a loopback host", PostgresDSN, host)
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		t.Fatalf("pg.DSN: %s is not a URL; the database of each test is set on its path", PostgresDSN)
	}
	database := project + "_test_" + suffix()
	if err := createDatabase(admin.ConnConfig, database); err != nil {
		t.Fatalf("pg.DSN: create %s: %v", database, err)
	}
	t.Cleanup(func() {
		if err := dropDatabase(admin.ConnConfig, database); err != nil {
			t.Errorf("pg.DSN: drop %s: %v", database, err)
		}
		databases.Delete(scope{t, project})
	})
	u.Path = "/" + database
	return u.String()
}

func suffix() string {
	buffer := make([]byte, 6)
	_, _ = rand.Read(buffer)
	return hex.EncodeToString(buffer)
}

// ResetTables empties the tables named.
func ResetTables(t testing.TB, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	if err := reset(pool, tables); err != nil {
		t.Fatalf("pg.ResetTables: %v", err)
	}
}

func requireTestCluster(name string) error {
	if name != TestCluster {
		return fmt.Errorf("%w: cluster_name is %q, want %q (bash tools/test-infra.sh up)", ErrNotTestCluster, name, TestCluster)
	}
	return nil
}

func connectAdmin(ctx context.Context, config *pgx.ConnConfig) (*pgx.Conn, error) {
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	var name string
	if err := conn.QueryRow(ctx, "SELECT current_setting('cluster_name')").Scan(&name); err != nil {
		_ = conn.Close(ctx)
		return nil, err
	}
	if err := requireTestCluster(name); err != nil {
		_ = conn.Close(ctx)
		return nil, err
	}
	return conn, nil
}

func createDatabase(config *pgx.ConnConfig, database string) error {
	ctx, cancel := context.WithTimeout(context.Background(), resetTimeout)
	defer cancel()
	conn, err := connectAdmin(ctx, config)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize())
	return err
}

// dropDatabase forces the drop: a connection the test leaked, or a process it
// started that is still exiting, would otherwise keep the database alive.
func dropDatabase(config *pgx.ConnConfig, database string) error {
	ctx, cancel := context.WithTimeout(context.Background(), resetTimeout)
	defer cancel()
	conn, err := connectAdmin(ctx, config)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{database}.Sanitize()+" WITH (FORCE)")
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
