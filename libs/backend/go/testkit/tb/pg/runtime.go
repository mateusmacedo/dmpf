package pg

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const RuntimeRole = "dmpf_test_runtime"

const MigratorRole = "dmpf_test_migrator"

// runtimePassword is not a secret: the role exists only on the loopback test
// cluster, which requireTestCluster enforces before the role is touched.
const runtimePassword = RuntimeRole

const migratorPassword = MigratorRole

const roleLockID = 0x646d7072

func OpenRuntimePool(t testing.TB, opts Options) *pgxpool.Pool {
	t.Helper()
	GrantRuntime(t, OpenPool(t, opts), opts)
	pool, err := pgxpool.NewWithConfig(context.Background(), RuntimeConfig(t, opts.Project))
	if err != nil {
		t.Fatalf("pg.OpenRuntimePool: pgxpool.NewWithConfig: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func GrantRuntime(t testing.TB, admin *pgxpool.Pool, opts Options) {
	t.Helper()
	ctx := context.Background()
	if err := grantRuntimeAccess(ctx, serverConfig(t), admin); err != nil {
		t.Fatalf("pg.GrantRuntime: %v", err)
	}
	grants := append(postgres.KernelGrants(opts.Capabilities...), opts.Grants...)
	if err := postgres.ApplyGrants(ctx, admin, RuntimeRole, grants...); err != nil {
		t.Fatalf("pg.GrantRuntime: %v", err)
	}
}

func RuntimeDSN(t testing.TB, project string) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgxpool.NewWithConfig(ctx, Config(t, project))
	if err != nil {
		t.Fatalf("pg.RuntimeDSN: pgxpool.NewWithConfig: %v", err)
	}
	defer admin.Close()
	if err := grantRuntimeAccess(ctx, serverConfig(t), admin); err != nil {
		t.Fatalf("pg.RuntimeDSN: %v", err)
	}
	return dsnAs(t, project, RuntimeRole, runtimePassword)
}

// WHY: only the database changes owner; a table that OpenPool migrated before
// stays owned by the server's administrative role.
func MigratorDSN(t testing.TB, project string) string {
	t.Helper()
	ctx := context.Background()
	if err := reconcileRole(ctx, serverConfig(t), MigratorRole, migratorPassword); err != nil {
		t.Fatalf("pg.MigratorDSN: %v", err)
	}
	if err := handOver(ctx, Config(t, project).ConnConfig, MigratorRole); err != nil {
		t.Fatalf("pg.MigratorDSN: %v", err)
	}
	return dsnAs(t, project, MigratorRole, migratorPassword)
}

func AssertDenied(t testing.TB, pool *pgxpool.Pool, statement string, args ...any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), statement, args...)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Errorf("pg.AssertDenied: %q = %v, want SQLSTATE 42501", statement, err)
	}
}

func RuntimeConfig(t testing.TB, project string) *pgxpool.Config {
	t.Helper()
	cfg := Config(t, project)
	cfg.ConnConfig.User = RuntimeRole
	cfg.ConnConfig.Password = runtimePassword
	return cfg
}

func dsnAs(t testing.TB, project, user, password string) string {
	t.Helper()
	u, err := url.Parse(DSN(t, project))
	if err != nil {
		t.Fatalf("pg: parse the DSN of the %s database: %v", project, err)
	}
	u.User = url.UserPassword(user, password)
	return u.String()
}

func grantRuntimeAccess(ctx context.Context, server *pgx.ConnConfig, admin *pgxpool.Pool) error {
	var cluster, database string
	if err := admin.QueryRow(ctx, "SELECT current_setting('cluster_name'), current_database()").Scan(&cluster, &database); err != nil {
		return err
	}
	if err := requireTestCluster(cluster); err != nil {
		return err
	}
	if err := reconcileRole(ctx, server, RuntimeRole, runtimePassword); err != nil {
		return err
	}
	role := pgx.Identifier{RuntimeRole}.Sanitize()
	db := pgx.Identifier{database}.Sanitize()
	return pgx.BeginFunc(ctx, admin, func(tx pgx.Tx) error {
		for _, statement := range []string{
			"GRANT CONNECT ON DATABASE " + db + " TO " + role,
			"REVOKE CREATE ON SCHEMA public FROM PUBLIC",
			"GRANT USAGE ON SCHEMA public TO " + role,
		} {
			if _, err := tx.Exec(ctx, statement); err != nil {
				return err
			}
		}
		return nil
	})
}

// WHY: concurrent ALTER ROLE on the shared pg_authid row fails with XX000, and advisory locks are per database.
func reconcileRole(ctx context.Context, server *pgx.ConnConfig, role, password string) error {
	ctx, cancel := context.WithTimeout(ctx, resetTimeout)
	defer cancel()
	conn, err := connectAdmin(ctx, server)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	quoted := pgx.Identifier{role}.Sanitize()
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(roleLockID)); err != nil {
			return err
		}
		for _, statement := range []string{
			"DO $$ BEGIN CREATE ROLE " + quoted + "; EXCEPTION WHEN duplicate_object OR unique_violation THEN NULL; END $$",
			"ALTER ROLE " + quoted + " LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS NOREPLICATION PASSWORD '" + password + "'",
		} {
			if _, err := tx.Exec(ctx, statement); err != nil {
				return err
			}
		}
		return nil
	})
}

func handOver(ctx context.Context, database *pgx.ConnConfig, owner string) error {
	ctx, cancel := context.WithTimeout(ctx, resetTimeout)
	defer cancel()
	conn, err := connectAdmin(ctx, database)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	_, err = conn.Exec(ctx, "ALTER DATABASE "+pgx.Identifier{database.Database}.Sanitize()+" OWNER TO "+pgx.Identifier{owner}.Sanitize())
	return err
}
