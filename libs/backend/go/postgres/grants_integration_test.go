//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	lognoop "go.opentelemetry.io/otel/log/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const grantsRuntime = "dmpf_grants_runtime"

const safeRuntime = "LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS NOREPLICATION"

func probeGrants() []postgres.Grant {
	return append(postgres.KernelGrants(allCapabilities...), postgres.Mutable("probes"), postgres.AppendOnly("tagged_probes"), postgres.Mutable("unique_probes"),
		postgres.Mutable("guarded_heads"), postgres.Mutable("guarded_series"))
}

func TestApplyGrantsConvergesTheRoleToTheDeclaredSet(t *testing.T) {
	admin := openAdminPool(t)
	ctx := context.Background()
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	mustExec(t, admin, "GRANT ALL ON probes TO "+grantsRuntime)
	mustExec(t, admin, "GRANT TRUNCATE ON outbox TO "+grantsRuntime)

	for range 2 {
		if err := postgres.ApplyGrants(ctx, admin, grantsRuntime, probeGrants()...); err != nil {
			t.Fatalf("ApplyGrants() = %v, want nil", err)
		}
	}

	readWrite := []string{"DELETE", "INSERT", "SELECT", "UPDATE"}
	want := map[string][]string{
		"outbox":         readWrite,
		"inbox":          readWrite,
		"quarantine":     {"INSERT", "SELECT"},
		"probes":         {"INSERT", "SELECT", "UPDATE"},
		"tagged_probes":  {"INSERT", "SELECT"},
		"unique_probes":  {"INSERT", "SELECT", "UPDATE"},
		"guarded_heads":  {"INSERT", "SELECT", "UPDATE"},
		"guarded_series": {"INSERT", "SELECT", "UPDATE"},
	}
	for table, privileges := range want {
		if got := privilegesOf(t, admin, grantsRuntime, table); !reflect.DeepEqual(got, privileges) {
			t.Errorf("privileges of %s on %s = %v, want %v", grantsRuntime, table, got, privileges)
		}
	}
}

func TestApplyGrantsRefusesAnUndeclaredTable(t *testing.T) {
	admin := openAdminPool(t)
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	mustExec(t, admin, "GRANT SELECT ON tagged_probes TO "+grantsRuntime)
	grants := append(postgres.KernelGrants(allCapabilities...), postgres.Mutable("probes"))

	err := postgres.ApplyGrants(context.Background(), admin, grantsRuntime, grants...)

	if err == nil || !strings.Contains(err.Error(), "tagged_probes") {
		t.Fatalf("ApplyGrants() without tagged_probes = %v, want an error naming it", err)
	}
	if got := privilegesOf(t, admin, grantsRuntime, "tagged_probes"); !reflect.DeepEqual(got, []string{"SELECT"}) {
		t.Fatalf("a refused ApplyGrants changed tagged_probes to %v", got)
	}
}

func TestApplyGrantsRefusesAnUnsafeRuntimeRole(t *testing.T) {
	cases := []struct {
		name    string
		role    string
		want    string
		prepare func(t *testing.T, admin *pgxpool.Pool)
	}{
		{"the connected role", "postgres", "is the role applying", func(*testing.T, *pgxpool.Pool) {}},
		{"an absent role", "dmpf_grants_absent", "does not exist", func(*testing.T, *pgxpool.Pool) {}},
		{"a superuser", "dmpf_grants_super", "superuser", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, "dmpf_grants_super", "LOGIN SUPERUSER")
		}},
		{"a role that bypasses row security", "dmpf_grants_bypass", "bypasses row security", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, "dmpf_grants_bypass", "LOGIN NOSUPERUSER BYPASSRLS")
		}},
		{"a member of the migrating role", "dmpf_grants_member", "is a member", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, "dmpf_grants_member", safeRuntime)
			mustExec(t, admin, "GRANT postgres TO dmpf_grants_member")
			t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "REVOKE postgres FROM dmpf_grants_member") })
		}},
		{"the owner of a table", grantsRuntime, "owns tagged_probes", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "ALTER TABLE tagged_probes OWNER TO "+grantsRuntime)
		}},
		{"the owner of a table, told how to hand it over", grantsRuntime, `REASSIGN OWNED BY "dmpf_grants_runtime" TO "postgres"`, func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "ALTER TABLE tagged_probes OWNER TO "+grantsRuntime)
		}},
		{"the owner of a table outside public", grantsRuntime, "owns staging.drafts", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "CREATE SCHEMA staging")
			mustExec(t, admin, "CREATE TABLE staging.drafts (draft_id bigint)")
			mustExec(t, admin, "ALTER TABLE staging.drafts OWNER TO "+grantsRuntime)
		}},
		{"the owner of a function", grantsRuntime, "owns function helper()", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "CREATE FUNCTION helper() RETURNS integer LANGUAGE sql AS 'SELECT 1'")
			mustExec(t, admin, "ALTER FUNCTION helper() OWNER TO "+grantsRuntime)
		}},
		{"the owner of a type", grantsRuntime, "owns type mood", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "CREATE TYPE mood AS ENUM ('calm')")
			mustExec(t, admin, "ALTER TYPE mood OWNER TO "+grantsRuntime)
		}},
		{"the owner of a large object", grantsRuntime, "owns large object", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			var object uint32
			if err := admin.QueryRow(context.Background(), "SELECT lo_create(0)").Scan(&object); err != nil {
				t.Fatalf("lo_create() = %v", err)
			}
			mustExec(t, admin, fmt.Sprintf("ALTER LARGE OBJECT %d OWNER TO %s", object, grantsRuntime))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			admin := openAdminPool(t)
			tc.prepare(t, admin)

			err := postgres.ApplyGrants(context.Background(), admin, tc.role, probeGrants()...)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ApplyGrants(%s) = %v, want a refusal saying %q", tc.role, err, tc.want)
			}
		})
	}
}

func TestApplyGrantsRefusesAPrivilegeLeftAfterTheGrants(t *testing.T) {
	const heir = "dmpf_grants_heir"
	const unsafeBase = "LOGIN NOSUPERUSER NOBYPASSRLS"
	cases := []struct {
		name    string
		role    string
		want    string
		prepare func(t *testing.T, admin *pgxpool.Pool)
	}{
		{"a table privilege granted to PUBLIC", grantsRuntime, "DELETE on probes", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "GRANT DELETE ON probes TO PUBLIC")
		}},
		{"a table privilege inherited from another role", heir, "TRUNCATE on outbox", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, heir, safeRuntime)
			ensureRole(t, admin, "dmpf_grants_carrier", "NOLOGIN")
			mustExec(t, admin, "GRANT TRUNCATE ON outbox TO dmpf_grants_carrier")
			grantMembership(t, admin, "dmpf_grants_carrier", heir)
		}},
		{"pg_write_all_data", heir, "DELETE on probes", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, heir, safeRuntime)
			grantMembership(t, admin, "pg_write_all_data", heir)
		}},
		{"CREATE on the public schema", grantsRuntime, "CREATE on schema public", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "GRANT CREATE ON SCHEMA public TO "+grantsRuntime)
		}},
		{"CREATE on the database", grantsRuntime, "CREATE on database", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "GRANT CREATE ON DATABASE "+databaseOf(t, admin)+" TO "+grantsRuntime)
		}},
		{"TEMPORARY on the database through PUBLIC", grantsRuntime, "TEMPORARY on database", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "GRANT TEMPORARY ON DATABASE "+databaseOf(t, admin)+" TO PUBLIC")
		}},
		{"membership in the owner of the database", heir, "owner of database", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, heir, safeRuntime)
			ensureRole(t, admin, "dmpf_grants_db_owner", "NOLOGIN")
			mustExec(t, admin, "ALTER DATABASE "+databaseOf(t, admin)+" OWNER TO dmpf_grants_db_owner")
			grantMembership(t, admin, "dmpf_grants_db_owner", heir)
		}},
		{"membership in the owner of a schema", heir, "owner of schema audit", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, heir, safeRuntime)
			ensureRole(t, admin, "dmpf_grants_schema_owner", "NOLOGIN")
			mustExec(t, admin, "CREATE SCHEMA audit AUTHORIZATION dmpf_grants_schema_owner")
			grantMembership(t, admin, "dmpf_grants_schema_owner", heir)
		}},
		{"REPLICATION", "dmpf_grants_replication", "REPLICATION", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, "dmpf_grants_replication", unsafeBase+" NOCREATEDB NOCREATEROLE REPLICATION")
		}},
		{"CREATEROLE", "dmpf_grants_createrole", "CREATEROLE", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, "dmpf_grants_createrole", unsafeBase+" NOCREATEDB NOREPLICATION CREATEROLE")
		}},
		{"CREATEDB", "dmpf_grants_createdb", "CREATEDB", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, "dmpf_grants_createdb", unsafeBase+" NOCREATEROLE NOREPLICATION CREATEDB")
		}},
		{"UPDATE on an IDENTITY sequence through pg_write_all_data", heir, "UPDATE on sequence outbox_id_seq", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, heir, safeRuntime)
			grantMembership(t, admin, "pg_write_all_data", heir)
		}},
		{"USAGE on an IDENTITY sequence granted to PUBLIC", grantsRuntime, "USAGE on sequence quarantine_id_seq", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "GRANT USAGE ON SEQUENCE quarantine_id_seq TO PUBLIC")
		}},
		{"a membership without INHERIT in a superuser", heir, "membership in dmpf_grants_superior, a role with SUPERUSER", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, heir, safeRuntime)
			ensureRole(t, admin, "dmpf_grants_superior", "NOLOGIN SUPERUSER")
			mustExec(t, admin, "GRANT dmpf_grants_superior TO "+heir+" WITH INHERIT FALSE")
			t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "REVOKE dmpf_grants_superior FROM "+heir) })
		}},
		{"an indirect membership in a CREATEDB role", heir, "membership in dmpf_grants_creator, a role with CREATEDB", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, heir, safeRuntime)
			ensureRole(t, admin, "dmpf_grants_middle", "NOLOGIN")
			ensureRole(t, admin, "dmpf_grants_creator", "NOLOGIN NOSUPERUSER CREATEDB")
			grantMembership(t, admin, "dmpf_grants_creator", "dmpf_grants_middle")
			grantMembership(t, admin, "dmpf_grants_middle", heir)
		}},
		{"membership in pg_execute_server_program", heir, "membership in pg_execute_server_program", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, heir, safeRuntime)
			grantMembership(t, admin, "pg_execute_server_program", heir)
		}},
		{"a membership without INHERIT in pg_write_all_data", heir, "membership in pg_write_all_data", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, heir, safeRuntime)
			mustExec(t, admin, "GRANT pg_write_all_data TO "+heir+" WITH INHERIT FALSE")
			t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "REVOKE pg_write_all_data FROM "+heir) })
		}},
		{"an UPDATE on a column granted to PUBLIC", grantsRuntime, "UPDATE on quarantine", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "GRANT UPDATE (envelope) ON quarantine TO PUBLIC")
		}},
		{"CREATE on a schema other than public", grantsRuntime, "CREATE on schema staging", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "CREATE SCHEMA staging")
			mustExec(t, admin, "GRANT CREATE ON SCHEMA staging TO "+grantsRuntime)
		}},
		{"EXECUTE on a SECURITY DEFINER function through PUBLIC", grantsRuntime, "EXECUTE on SECURITY DEFINER function escalate()", func(t *testing.T, admin *pgxpool.Pool) {
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "CREATE FUNCTION escalate() RETURNS integer LANGUAGE sql SECURITY DEFINER AS 'SELECT 1'")
		}},
		{"MAINTAIN through PUBLIC", grantsRuntime, "MAINTAIN on probes", func(t *testing.T, admin *pgxpool.Pool) {
			var version int
			if err := admin.QueryRow(context.Background(), "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil {
				t.Fatalf("server_version_num = %v", err)
			}
			if version < 170000 {
				t.Skipf("MAINTAIN exists from PostgreSQL 17; the server is %d", version)
			}
			ensureRole(t, admin, grantsRuntime, safeRuntime)
			mustExec(t, admin, "GRANT MAINTAIN ON probes TO PUBLIC")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			admin := openAdminPool(t)
			tc.prepare(t, admin)

			err := postgres.ApplyGrants(context.Background(), admin, tc.role, probeGrants()...)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ApplyGrants(%s) = %v, want a refusal naming %q", tc.role, err, tc.want)
			}
		})
	}
}

func TestApplyGrantsRollsBackTheGrantsWhenAPrivilegeIsLeft(t *testing.T) {
	admin := openAdminPool(t)
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	mustExec(t, admin, "GRANT SELECT ON tagged_probes TO "+grantsRuntime)
	mustExec(t, admin, "GRANT DELETE ON probes TO PUBLIC")

	if err := postgres.ApplyGrants(context.Background(), admin, grantsRuntime, probeGrants()...); err == nil {
		t.Fatal("ApplyGrants() with DELETE on probes through PUBLIC = nil, want a refusal")
	}

	if got := privilegesOf(t, admin, grantsRuntime, "tagged_probes"); !reflect.DeepEqual(got, []string{"SELECT"}) {
		t.Errorf("a refused ApplyGrants left tagged_probes at %v, want [SELECT]", got)
	}
	if got := privilegesOf(t, admin, grantsRuntime, "outbox"); len(got) > 0 {
		t.Errorf("a refused ApplyGrants left outbox at %v, want nothing", got)
	}
}

func TestApplyGrantsRefusesADeclaredPrivilegeAMigratorThatDoesNotOwnTheTableLeavesOut(t *testing.T) {
	admin := openAdminPool(t)
	migrator := migratorWithoutTaggedProbes(t, admin)

	err := postgres.ApplyGrants(context.Background(), migrator, grantsRuntime, probeGrants()...)

	if err == nil || !strings.Contains(err.Error(), "lacks SELECT on tagged_probes, INSERT on tagged_probes") || !strings.Contains(err.Error(), "must own the table") {
		t.Fatalf("ApplyGrants() by a migrator that does not own tagged_probes = %v, want a refusal naming each privilege it left out", err)
	}
}

func TestApplyGrantsNamesTheMissingAndTheLeftoverPrivilegeTogether(t *testing.T) {
	admin := openAdminPool(t)
	migrator := migratorWithoutTaggedProbes(t, admin)
	mustExec(t, admin, "GRANT DELETE ON probes TO PUBLIC")

	err := postgres.ApplyGrants(context.Background(), migrator, grantsRuntime, probeGrants()...)

	if err == nil || !strings.Contains(err.Error(), "lacks SELECT on tagged_probes") || !strings.Contains(err.Error(), "still holds DELETE on probes") {
		t.Fatalf("ApplyGrants() with a privilege missing and one left = %v, want a refusal naming both", err)
	}
}

func TestApplyGrantsRefusesASequenceOutsideIdentity(t *testing.T) {
	admin := openAdminPool(t)
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	mustExec(t, admin, "CREATE TABLE serial_probes (serial_probe_id bigserial PRIMARY KEY)")

	err := postgres.ApplyGrants(context.Background(), admin, grantsRuntime, append(probeGrants(), postgres.Mutable("serial_probes"))...)

	if err == nil || !strings.Contains(err.Error(), "serial_probes_serial_probe_id_seq") || !strings.Contains(err.Error(), "IDENTITY") {
		t.Fatalf("ApplyGrants() over a bigserial = %v, want a refusal naming the sequence and IDENTITY", err)
	}
}

func TestApplyGrantsAndTheWaitsQuoteAReservedTableName(t *testing.T) {
	admin := openAdminPool(t)
	ctx := context.Background()
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	mustExec(t, admin, `CREATE TABLE "order" (order_id text PRIMARY KEY)`)

	if err := postgres.ApplyGrants(ctx, admin, grantsRuntime, append(probeGrants(), postgres.Mutable("order"))...); err != nil {
		t.Fatalf("ApplyGrants() over the table order = %v, want nil", err)
	}
	if got := privilegesOf(t, admin, grantsRuntime, `public."order"`); !reflect.DeepEqual(got, []string{"INSERT", "SELECT", "UPDATE"}) {
		t.Fatalf("privileges of %s on order = %v", grantsRuntime, got)
	}
	runtime := openRuntime(t, grantsRuntime)
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := postgres.WaitForTables(waitCtx, runtime, 20*time.Millisecond, lognoop.NewLoggerProvider(), "order"); err != nil {
		t.Fatalf("WaitForTables(order) = %v, want nil", err)
	}
	if err := postgres.WaitForGrants(waitCtx, runtime, 20*time.Millisecond, lognoop.NewLoggerProvider(), postgres.Mutable("order")); err != nil {
		t.Fatalf("WaitForGrants(order) = %v, want nil", err)
	}
}

func TestWaitForGrantsNamesTheFirstPrivilegeTheRoleLacksInDeclarationOrder(t *testing.T) {
	admin := openAdminPool(t)
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	if err := postgres.ApplyGrants(context.Background(), admin, grantsRuntime, probeGrants()...); err != nil {
		t.Fatalf("ApplyGrants() = %v, want nil", err)
	}
	runtime := openRuntime(t, grantsRuntime)
	if err := runtime.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	grants := append(probeGrants(),
		postgres.Grant{Table: "tagged_probes", Privileges: []postgres.Privilege{postgres.Update}},
		postgres.Grant{Table: "probes", Privileges: []postgres.Privilege{postgres.Delete}})

	err := postgres.WaitForGrants(ctx, runtime, 20*time.Millisecond, lognoop.NewLoggerProvider(), grants...)

	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "UPDATE on tagged_probes") {
		t.Fatalf("WaitForGrants() of an ungranted UPDATE before an ungranted DELETE = %v, want the deadline naming UPDATE on tagged_probes", err)
	}
}

func TestAssertGrantableRefusesAnOwnerBeforeAnyChange(t *testing.T) {
	admin := openAdminPool(t)
	ctx := context.Background()
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	if err := postgres.AssertGrantable(ctx, admin, grantsRuntime); err != nil {
		t.Fatalf("AssertGrantable() of a role that owns nothing = %v, want nil", err)
	}
	mustExec(t, admin, "GRANT SELECT ON probes TO "+grantsRuntime)
	mustExec(t, admin, "ALTER TABLE tagged_probes OWNER TO "+grantsRuntime)

	err := postgres.AssertGrantable(ctx, admin, grantsRuntime)

	if err == nil || !strings.Contains(err.Error(), `owns tagged_probes`) || !strings.Contains(err.Error(), `REASSIGN OWNED BY "dmpf_grants_runtime" TO "postgres"`) {
		t.Fatalf("AssertGrantable() of the owner of tagged_probes = %v, want a refusal with the REASSIGN OWNED that fixes it", err)
	}
	if got := privilegesOf(t, admin, grantsRuntime, "probes"); !reflect.DeepEqual(got, []string{"SELECT"}) {
		t.Fatalf("AssertGrantable() changed the privileges on probes to %v", got)
	}
}

func TestMigrateCreatesInPublicEvenUnderASchemaNamedAfterTheRole(t *testing.T) {
	admin := openAdminPool(t)
	ctx := context.Background()
	var user string
	if err := admin.QueryRow(ctx, "SELECT current_user").Scan(&user); err != nil {
		t.Fatalf("current_user = %v", err)
	}
	shadow := pgx.Identifier{user}.Sanitize()
	mustExec(t, admin, "CREATE SCHEMA "+shadow)

	if err := postgres.Migrate(ctx, admin, nil, "CREATE TABLE IF NOT EXISTS shadowed_probes (id bigint)"); err != nil {
		t.Fatalf("Migrate() = %v, want nil", err)
	}

	var inPublic, inShadow bool
	if err := admin.QueryRow(ctx, "SELECT to_regclass('public.shadowed_probes') IS NOT NULL, to_regclass($1) IS NOT NULL", shadow+".shadowed_probes").Scan(&inPublic, &inShadow); err != nil {
		t.Fatalf("to_regclass() = %v", err)
	}
	if !inPublic || inShadow {
		t.Fatalf("shadowed_probes in public = %v, in the schema %s = %v; want it only in public", inPublic, shadow, inShadow)
	}
}

func TestApplyGrantsRefusesAMalformedDeclaration(t *testing.T) {
	cases := []struct {
		name   string
		role   string
		want   string
		grants []postgres.Grant
	}{
		{"no role", "", "no runtime role", probeGrants()},
		{"a table that is not an identifier", grantsRuntime, "not an identifier", append(probeGrants(), postgres.Mutable("probes; DROP TABLE outbox"))},
		{"a table declared twice", grantsRuntime, "declared twice", append(probeGrants(), postgres.AppendOnly("probes"))},
		{"a privilege outside the set", grantsRuntime, "not grantable", append(postgres.KernelGrants(allCapabilities...), postgres.Mutable("probes"), postgres.Grant{Table: "tagged_probes", Privileges: []postgres.Privilege{"TRUNCATE"}})},
		{"a grant without privileges", grantsRuntime, "declares no privilege", append(postgres.KernelGrants(allCapabilities...), postgres.Mutable("probes"), postgres.Grant{Table: "tagged_probes"})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			admin := openAdminPool(t)
			ensureRole(t, admin, grantsRuntime, safeRuntime)

			err := postgres.ApplyGrants(context.Background(), admin, tc.role, tc.grants...)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ApplyGrants() = %v, want a refusal saying %q", err, tc.want)
			}
		})
	}
}

func TestTheRuntimeRoleCannotChangeTheSchema(t *testing.T) {
	admin := openAdminPool(t)
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	if err := postgres.ApplyGrants(context.Background(), admin, grantsRuntime, probeGrants()...); err != nil {
		t.Fatalf("ApplyGrants() = %v, want nil", err)
	}
	runtime := openRuntime(t, grantsRuntime)

	for _, statement := range []string{
		"CREATE TABLE intruders (id bigint)",
		"DROP TABLE probes",
		"ALTER TABLE probes ADD COLUMN intruder bigint",
		"TRUNCATE outbox",
	} {
		expectDenied(t, runtime, statement)
	}
}

func TestAnAppendOnlyTableTakesNewRowsButRefusesChange(t *testing.T) {
	admin := openAdminPool(t)
	ctx := context.Background()
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	if err := postgres.ApplyGrants(ctx, admin, grantsRuntime, probeGrants()...); err != nil {
		t.Fatalf("ApplyGrants() = %v, want nil", err)
	}
	runtime := openRuntime(t, grantsRuntime)
	const insert = `INSERT INTO tagged_probes (tenant_id, probe_id, version, label, snapshot)
VALUES ('t1', 'p1', 1, 'audit', '{}') ON CONFLICT (tenant_id, probe_id) DO NOTHING`

	for range 2 {
		if _, err := runtime.Exec(ctx, insert); err != nil {
			t.Fatalf("INSERT ... ON CONFLICT DO NOTHING by the runtime role = %v, want nil", err)
		}
	}
	var rows int
	if err := runtime.QueryRow(ctx, "SELECT count(*) FROM tagged_probes").Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("SELECT by the runtime role = %d rows, %v; want 1, nil", rows, err)
	}
	expectDenied(t, runtime, "UPDATE tagged_probes SET version = 2")
	expectDenied(t, runtime, "DELETE FROM tagged_probes")
}

func TestReapplyingGrantsNeverLeavesTheRuntimeWithoutPrivilege(t *testing.T) {
	admin := openAdminPool(t)
	ctx := context.Background()
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	if err := postgres.ApplyGrants(ctx, admin, grantsRuntime, probeGrants()...); err != nil {
		t.Fatalf("ApplyGrants() = %v, want nil", err)
	}
	runtime := openRuntime(t, grantsRuntime)
	fixed, err := runtime.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() = %v", err)
	}
	defer fixed.Release()
	if _, err := fixed.Conn().Prepare(ctx, "count_probes", "SELECT count(*) FROM probes"); err != nil {
		t.Fatalf("Prepare() = %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	record := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		failures = append(failures, err)
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			var n int
			if err := runtime.QueryRow(ctx, "SELECT count(*) FROM outbox").Scan(&n); err != nil {
				record(err)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			var n int
			if err := fixed.Conn().QueryRow(ctx, "count_probes").Scan(&n); err != nil {
				record(err)
			}
		}
	}()

	for range 20 {
		if err := postgres.ApplyGrants(ctx, admin, grantsRuntime, probeGrants()...); err != nil {
			record(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(stop)
	wg.Wait()

	if len(failures) > 0 {
		t.Fatalf("reapplying grants under load failed %d times; first: %v", len(failures), failures[0])
	}
}

func TestApplyGrantsQuotesARoleWithAHyphen(t *testing.T) {
	admin := openAdminPool(t)
	const role = "dmpf-grants-hyphen"
	ensureRole(t, admin, role, safeRuntime)

	if err := postgres.ApplyGrants(context.Background(), admin, role, probeGrants()...); err != nil {
		t.Fatalf("ApplyGrants(%s) = %v, want nil", role, err)
	}
	if got := privilegesOf(t, admin, role, "probes"); !reflect.DeepEqual(got, []string{"INSERT", "SELECT", "UPDATE"}) {
		t.Fatalf("privileges of %s on probes = %v", role, got)
	}
}

func ensureRole(t *testing.T, admin *pgxpool.Pool, role, attributes string) {
	t.Helper()
	quoted := pgx.Identifier{role}.Sanitize()
	mustExec(t, admin, fmt.Sprintf(`DO $$ BEGIN CREATE ROLE %s; EXCEPTION WHEN duplicate_object OR unique_violation THEN NULL; END $$`, quoted))
	mustExec(t, admin, fmt.Sprintf("ALTER ROLE %s %s PASSWORD '%s'", quoted, attributes, role))
	mustExec(t, admin, fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", databaseOf(t, admin), quoted))
	mustExec(t, admin, "GRANT USAGE ON SCHEMA public TO "+quoted)
}

func migratorWithoutTaggedProbes(t *testing.T, admin *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	const migrator = "dmpf_grants_migrator"
	const stranger = "dmpf_grants_stranger"
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	ensureRole(t, admin, migrator, safeRuntime)
	ensureRole(t, admin, stranger, "NOLOGIN")
	for _, table := range []string{"outbox", "inbox", "quarantine", "probes", "unique_probes", "guarded_heads", "guarded_series"} {
		mustExec(t, admin, "ALTER TABLE "+table+" OWNER TO "+migrator)
	}
	mustExec(t, admin, "ALTER TABLE tagged_probes OWNER TO "+stranger)
	// WHY: with no privilege at all on the table, GRANT fails outright; with one but
	// no grant option, it only warns and concedes nothing (PostgreSQL 16, GRANT, Notes).
	mustExec(t, admin, "GRANT SELECT ON tagged_probes TO "+migrator)
	return openRuntime(t, migrator)
}

func databaseOf(t *testing.T, admin *pgxpool.Pool) string {
	t.Helper()
	var database string
	if err := admin.QueryRow(context.Background(), "SELECT current_database()").Scan(&database); err != nil {
		t.Fatalf("current_database() = %v", err)
	}
	return pgx.Identifier{database}.Sanitize()
}

func grantMembership(t *testing.T, admin *pgxpool.Pool, granted, member string) {
	t.Helper()
	mustExec(t, admin, fmt.Sprintf("GRANT %s TO %s", granted, member))
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), fmt.Sprintf("REVOKE %s FROM %s", granted, member)) })
}

func openRuntime(t *testing.T, role string) *pgxpool.Pool {
	t.Helper()
	cfg := pg.Config(t, "postgres")
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = role
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig(%s) = %v", role, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func privilegesOf(t *testing.T, admin *pgxpool.Pool, role, table string) []string {
	t.Helper()
	rows, err := admin.Query(context.Background(), `
SELECT p FROM unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER']) AS p
WHERE has_table_privilege($1, $2, p) ORDER BY p`, role, table)
	if err != nil {
		t.Fatalf("has_table_privilege(%s, %s) = %v", role, table, err)
	}
	privileges, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect privileges of %s on %s: %v", role, table, err)
	}
	return privileges
}

func expectDenied(t *testing.T, pool *pgxpool.Pool, statement string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), statement)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Errorf("%q by the runtime role = %v, want SQLSTATE 42501", statement, err)
	}
}

func mustExec(t *testing.T, pool *pgxpool.Pool, statement string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), statement); err != nil {
		t.Fatalf("%s = %v", statement, err)
	}
}
