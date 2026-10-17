//go:build integration

package pg

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const widgetsSchema = `CREATE TABLE IF NOT EXISTS widgets (tenant_id text NOT NULL, widget_id text NOT NULL, version bigint NOT NULL,
CONSTRAINT widgets_pkey PRIMARY KEY (tenant_id, widget_id))`

func widgetOptions(project string) Options {
	return Options{
		Project:      project,
		Capabilities: []postgres.Capability{postgres.Outbox},
		Schemas:      []string{widgetsSchema},
		Tables:       []string{"widgets"},
		Grants:       []postgres.Grant{postgres.Mutable("widgets")},
	}
}

func TestOpenRuntimePoolConnectsAsTheRuntimeRole(t *testing.T) {
	pool := OpenRuntimePool(t, widgetOptions("pgkit_runtime"))

	var user string
	var super bool
	err := pool.QueryRow(context.Background(), "SELECT current_user, rolsuper FROM pg_roles WHERE rolname = current_user").Scan(&user, &super)
	if err != nil {
		t.Fatalf("current_user = %v", err)
	}
	if user != RuntimeRole || super {
		t.Fatalf("connected as %s (superuser %v), want %s without superuser", user, super, RuntimeRole)
	}
}

func TestOpenRuntimePoolGrantsTheDeclaredTablesOnly(t *testing.T) {
	pool := OpenRuntimePool(t, widgetOptions("pgkit_runtime"))
	ctx := context.Background()

	if _, err := pool.Exec(ctx, "INSERT INTO widgets (tenant_id, widget_id, version) VALUES ('t1', 'w1', 1)"); err != nil {
		t.Fatalf("INSERT into a Mutable table = %v, want nil", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE widgets SET version = 2"); err != nil {
		t.Fatalf("UPDATE of a Mutable table = %v, want nil", err)
	}
	AssertDenied(t, pool, "DELETE FROM widgets")
	AssertDenied(t, pool, "SELECT 1 FROM pg_catalog.pg_authid")
}

func TestTheRuntimeRoleCannotChangeTheSchema(t *testing.T) {
	pool := OpenRuntimePool(t, widgetOptions("pgkit_runtime"))

	for _, statement := range []string{
		"CREATE TABLE intruders (id bigint)",
		"CREATE TEMP TABLE intruders (id bigint)",
		"ALTER TABLE widgets ADD COLUMN intruder bigint",
		"DROP TABLE widgets",
		"TRUNCATE outbox",
	} {
		AssertDenied(t, pool, statement)
	}
}

func TestOpenRuntimePoolSurvivesConcurrentTests(t *testing.T) {
	for i := range 6 {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()
			pool := OpenRuntimePool(t, widgetOptions("pgkit_runtime"))

			if err := pool.Ping(context.Background()); err != nil {
				t.Fatalf("Ping() = %v", err)
			}
		})
	}
}

func TestRuntimeDSNReachesTheTestDatabaseAsTheRuntimeRole(t *testing.T) {
	const project = "pgkit_runtime_dsn"
	admin, err := url.Parse(DSN(t, project))
	if err != nil {
		t.Fatalf("parse DSN = %v", err)
	}

	runtime, err := url.Parse(RuntimeDSN(t, project))
	if err != nil {
		t.Fatalf("parse RuntimeDSN = %v", err)
	}
	if runtime.User.Username() != RuntimeRole || runtime.Path != admin.Path || runtime.Host != admin.Host {
		t.Fatalf("RuntimeDSN() = %s, want %s on %s%s", runtime.Redacted(), RuntimeRole, admin.Host, admin.Path)
	}
	conn, err := pgx.Connect(context.Background(), runtime.String())
	if err != nil {
		t.Fatalf("connect with RuntimeDSN() = %v", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	var user string
	if err := conn.QueryRow(context.Background(), "SELECT current_user").Scan(&user); err != nil || user != RuntimeRole {
		t.Fatalf("current_user = %q, %v; want %s", user, err, RuntimeRole)
	}
}

func TestMigratorDSNOwnsTheDatabaseWithoutBeingASuperuser(t *testing.T) {
	const project = "pgkit_migrator"
	ctx := context.Background()
	migrator, err := pgxpool.New(ctx, MigratorDSN(t, project))
	if err != nil {
		t.Fatalf("pgxpool.New(MigratorDSN) = %v", err)
	}
	t.Cleanup(migrator.Close)

	var user, owner string
	var super bool
	err = migrator.QueryRow(ctx, `SELECT current_user, r.rolsuper, pg_get_userbyid(d.datdba)
FROM pg_roles r, pg_database d WHERE r.rolname = current_user AND d.datname = current_database()`).Scan(&user, &super, &owner)
	if err != nil || user != MigratorRole || super || owner != MigratorRole {
		t.Fatalf("connected as %s (superuser %v) to a database of %s, %v; want %s owning it without superuser", user, super, owner, err, MigratorRole)
	}
	opts := widgetOptions(project)
	if err := postgres.Migrate(ctx, migrator, opts.Capabilities, opts.Schemas...); err != nil {
		t.Fatalf("Migrate() as %s = %v", MigratorRole, err)
	}
	RuntimeDSN(t, project)
	if err := postgres.ApplyGrants(ctx, migrator, RuntimeRole, append(postgres.KernelGrants(opts.Capabilities...), opts.Grants...)...); err != nil {
		t.Fatalf("ApplyGrants() as %s = %v", MigratorRole, err)
	}
}

func TestReconcileRoleReappliesThePasswordEveryTime(t *testing.T) {
	const role = "dmpf_test_reconciled"
	ctx := context.Background()
	server := serverConfig(t)
	if err := reconcileRole(ctx, server, role, "reconciled"); err != nil {
		t.Fatalf("reconcileRole() = %v", err)
	}
	admin := OpenPool(t, Options{Project: "pgkit_reconcile"})
	if _, err := admin.Exec(ctx, "ALTER ROLE "+role+" PASSWORD 'drifted'"); err != nil {
		t.Fatalf("ALTER ROLE = %v", err)
	}

	if err := reconcileRole(ctx, server, role, "reconciled"); err != nil {
		t.Fatalf("reconcileRole() = %v", err)
	}

	cfg := Config(t, "pgkit_reconcile").ConnConfig
	cfg.User, cfg.Password = role, "reconciled"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect with the reconciled password = %v, want nil", err)
	}
	_ = conn.Close(ctx)
}

type errorRecorder struct {
	testing.TB
	errors []string
}

func (e *errorRecorder) Helper() {}

func (e *errorRecorder) Errorf(format string, args ...any) {
	e.errors = append(e.errors, fmt.Sprintf(format, args...))
}

func TestAssertDeniedFailsWhenTheStatementPasses(t *testing.T) {
	pool := OpenRuntimePool(t, widgetOptions("pgkit_runtime"))
	r := &errorRecorder{TB: t}

	AssertDenied(r, pool, "SELECT count(*) FROM widgets")

	if len(r.errors) != 1 || !strings.Contains(r.errors[0], "42501") {
		t.Fatalf("AssertDenied() on an allowed statement recorded %v, want one error naming 42501", r.errors)
	}
}
