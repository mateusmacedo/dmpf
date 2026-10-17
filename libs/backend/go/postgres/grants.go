package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WHY: TRUNCATE, REFERENCES and TRIGGER stay out by design (ADR-063, item 4).
type Privilege string

const (
	Select Privilege = "SELECT"
	Insert Privilege = "INSERT"
	Update Privilege = "UPDATE"
	Delete Privilege = "DELETE"
)

type Grant struct {
	Table      string
	Privileges []Privilege
}

func Mutable(table string) Grant {
	return Grant{Table: table, Privileges: []Privilege{Select, Insert, Update}}
}

func AppendOnly(table string) Grant {
	return Grant{Table: table, Privileges: []Privilege{Select, Insert}}
}

func KernelGrants(caps ...Capability) []Grant {
	var grants []Grant
	for _, c := range dedupe(caps) {
		for _, g := range capabilities[c].grants {
			grants = append(grants, Grant{Table: g.Table, Privileges: slices.Clone(g.Privileges)})
		}
	}
	return grants
}

var readWrite = []Privilege{Select, Insert, Update, Delete}

// WHY: the postcondition reads the effective privilege, which PUBLIC and
// membership add to the ACL just written (ADR-063, item 5).
func ApplyGrants(ctx context.Context, pool *pgxpool.Pool, role string, grants ...Grant) error {
	declared, err := validateGrants(role, grants)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, pinSearchPath); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(migrateLockID)); err != nil {
		return err
	}
	if err := refuseUnsafeRole(ctx, tx, role); err != nil {
		return err
	}
	if err := refuseUndeclaredTables(ctx, tx, declared); err != nil {
		return err
	}
	if err := refuseSequencesOutsideIdentity(ctx, tx); err != nil {
		return err
	}
	for _, statement := range grantStatements(role, grants) {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("postgres: grants: %w", err)
		}
	}
	if err := refuseDivergentPrivilege(ctx, tx, role, grants); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// WHY: run before Migrate, a role that still owns objects fails here with the
// REASSIGN OWNED hint, not as SQLSTATE 42501 in the DDL.
func AssertGrantable(ctx context.Context, pool *pgxpool.Pool, role string) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, pinSearchPath); err != nil {
			return err
		}
		return refuseUnsafeRole(ctx, tx, role)
	})
}

func validateGrants(role string, grants []Grant) ([]string, error) {
	if role == "" {
		return nil, fmt.Errorf("postgres: grants: no runtime role")
	}
	declared := make([]string, 0, len(grants))
	for _, g := range grants {
		if !isIdentifier(g.Table) {
			return nil, fmt.Errorf("postgres: grants: table %q is not an identifier", g.Table)
		}
		table := strings.ToLower(g.Table)
		if slices.Contains(declared, table) {
			return nil, fmt.Errorf("postgres: grants: table %s is declared twice", table)
		}
		if len(g.Privileges) == 0 {
			return nil, fmt.Errorf("postgres: grants: table %s declares no privilege", table)
		}
		for _, p := range g.Privileges {
			if !slices.Contains(readWrite, p) {
				return nil, fmt.Errorf("postgres: grants: privilege %q on %s is not grantable", p, table)
			}
		}
		declared = append(declared, table)
	}
	return declared, nil
}

const selectRuntimeRole = `
SELECT r.rolname = current_user, r.rolsuper, r.rolbypassrls, pg_has_role(r.rolname, current_user, 'MEMBER'), current_user
FROM pg_roles r WHERE r.rolname = $1`

const selectOwnedObjects = `
SELECT owned FROM (
  SELECT 1, c.oid::regclass::text FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema' AND pg_has_role($1, c.relowner, 'MEMBER')
  UNION ALL
  SELECT 2, 'function ' || p.oid::regprocedure FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
  WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema' AND pg_has_role($1, p.proowner, 'MEMBER')
  UNION ALL
  SELECT 3, 'type ' || t.oid::regtype FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
  WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema' AND pg_has_role($1, t.typowner, 'MEMBER')
    AND NOT EXISTS (SELECT FROM pg_class c WHERE c.oid = t.typrelid AND c.relkind <> 'c')
    AND NOT EXISTS (SELECT FROM pg_type e WHERE e.typarray = t.oid)
  UNION ALL
  SELECT 4, 'large object ' || l.oid FROM pg_largeobject_metadata l WHERE pg_has_role($1, l.lomowner, 'MEMBER')
) AS objects(kind, owned)
ORDER BY kind, owned`

func refuseUnsafeRole(ctx context.Context, tx pgx.Tx, role string) error {
	var applying, super, bypass, member bool
	var applier string
	err := tx.QueryRow(ctx, selectRuntimeRole, role).Scan(&applying, &super, &bypass, &member, &applier)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("postgres: grants: role %s does not exist", role)
	case err != nil:
		return err
	case applying:
		return fmt.Errorf("postgres: grants: role %s is the role applying the grants", role)
	case super:
		return fmt.Errorf("postgres: grants: role %s is a superuser", role)
	case bypass:
		return fmt.Errorf("postgres: grants: role %s bypasses row security", role)
	case member:
		return fmt.Errorf("postgres: grants: role %s is a member of the role applying the grants", role)
	}
	rows, err := tx.Query(ctx, selectOwnedObjects, role)
	if err != nil {
		return err
	}
	owned, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(owned) > 0 {
		return fmt.Errorf("postgres: grants: role %s owns %s; a superuser hands them over with REASSIGN OWNED BY %s TO %s",
			role, strings.Join(owned, ", "), pgx.Identifier{role}.Sanitize(), pgx.Identifier{applier}.Sanitize())
	}
	return nil
}

const selectUndeclaredTables = `
SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename <> ALL($1) ORDER BY 1`

func refuseUndeclaredTables(ctx context.Context, tx pgx.Tx, declared []string) error {
	rows, err := tx.Query(ctx, selectUndeclaredTables, declared)
	if err != nil {
		return err
	}
	undeclared, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(undeclared) > 0 {
		return fmt.Errorf("postgres: grants: no grant declared for %s", strings.Join(undeclared, ", "))
	}
	return nil
}

const selectSequencesOutsideIdentity = `
SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public' AND c.relkind = 'S' AND NOT EXISTS (
  SELECT FROM pg_depend d WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.deptype = 'i')
ORDER BY 1`

func refuseSequencesOutsideIdentity(ctx context.Context, tx pgx.Tx) error {
	rows, err := tx.Query(ctx, selectSequencesOutsideIdentity)
	if err != nil {
		return err
	}
	sequences, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(sequences) > 0 {
		return fmt.Errorf("postgres: grants: sequence %s does not back an IDENTITY column and the runtime role gets no sequence privilege; declare the column GENERATED ALWAYS AS IDENTITY instead of serial or nextval",
			strings.Join(sequences, ", "))
	}
	return nil
}

const selectMissingPrivilege = `
SELECT format('%s on %s', d.privilege, d.tablename)
FROM unnest($2::text[], $3::text[]) WITH ORDINALITY AS d(tablename, privilege, position)
LEFT JOIN pg_class c ON c.relname = d.tablename AND c.relnamespace = 'public'::regnamespace
WHERE c.oid IS NULL OR NOT has_table_privilege($1, c.oid, d.privilege)
ORDER BY d.position`

const selectExcessPrivilege = `
SELECT leftover FROM (
  SELECT 1, format('%s on %s', p.privilege, c.relname)
  FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
  CROSS JOIN unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER']
    || CASE WHEN current_setting('server_version_num')::int >= 170000 THEN ARRAY['MAINTAIN'] ELSE ARRAY[]::text[] END) AS p(privilege)
  WHERE n.nspname = 'public' AND c.relkind IN ('r','p','v','m','f')
    AND CASE WHEN p.privilege IN ('SELECT','INSERT','UPDATE','REFERENCES') THEN has_any_column_privilege($1, c.oid, p.privilege)
             ELSE has_table_privilege($1, c.oid, p.privilege) END
    AND NOT EXISTS (SELECT FROM unnest($2::text[], $3::text[]) AS d(tablename, privilege)
                    WHERE d.tablename = c.relname AND d.privilege = p.privilege)
  UNION ALL
  SELECT 2, format('%s on sequence %s', p.privilege, c.relname)
  FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
  CROSS JOIN unnest(ARRAY['USAGE','SELECT','UPDATE']) AS p(privilege)
  WHERE n.nspname = 'public' AND c.relkind = 'S' AND has_sequence_privilege($1, c.oid, p.privilege)
  UNION ALL
  SELECT 3, format('CREATE on schema %s', n.nspname) FROM pg_namespace n
  WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema' AND has_schema_privilege($1, n.oid, 'CREATE')
  UNION ALL
  SELECT 4, format('%s on database %s', p.privilege, current_database())
  FROM unnest(ARRAY['CREATE','TEMPORARY']) AS p(privilege)
  WHERE has_database_privilege($1, current_database(), p.privilege)
  UNION ALL
  SELECT 5, format('membership in %s, owner of database %s', pg_get_userbyid(d.datdba), d.datname)
  FROM pg_database d WHERE d.datname = current_database() AND pg_has_role($1, d.datdba, 'MEMBER')
  UNION ALL
  SELECT 6, format('membership in %s, owner of schema %s', pg_get_userbyid(n.nspowner), n.nspname)
  FROM pg_namespace n
  WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema' AND pg_has_role($1, n.nspowner, 'MEMBER')
  UNION ALL
  SELECT 7, a.attribute FROM pg_roles r
  CROSS JOIN LATERAL (VALUES ('REPLICATION', r.rolreplication), ('CREATEROLE', r.rolcreaterole), ('CREATEDB', r.rolcreatedb)) AS a(attribute, held)
  WHERE r.rolname = $1 AND a.held
  UNION ALL
  SELECT 8, format('membership in %s', r.rolname) || COALESCE(', a role with ' || a.attributes, '') FROM pg_roles r
  CROSS JOIN LATERAL (
    SELECT string_agg(h.attribute, ' and ' ORDER BY h.position)
    FROM (VALUES (1, 'SUPERUSER', r.rolsuper), (2, 'BYPASSRLS', r.rolbypassrls), (3, 'REPLICATION', r.rolreplication),
                 (4, 'CREATEROLE', r.rolcreaterole), (5, 'CREATEDB', r.rolcreatedb)) AS h(position, attribute, held)
    WHERE h.held) AS a(attributes)
  WHERE r.rolname <> $1 AND pg_has_role($1, r.oid, 'MEMBER')
    AND r.oid NOT IN (SELECT d.datdba FROM pg_database d WHERE d.datname = current_database()
                      UNION SELECT n.nspowner FROM pg_namespace n WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema')
  UNION ALL
  SELECT 9, format('EXECUTE on SECURITY DEFINER function %s', p.oid::regprocedure)
  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
  WHERE p.prosecdef AND n.nspname !~ '^pg_' AND n.nspname <> 'information_schema' AND has_function_privilege($1, p.oid, 'EXECUTE')
) AS excess(kind, leftover)
ORDER BY kind, leftover`

// WHY: an unqualified CREATE lands in the first schema of the path, and pg_catalog
// is searched before it anyway when left out (PostgreSQL 16, ddl-schemas-path).
const pinSearchPath = "SET LOCAL search_path = public"

func refuseDivergentPrivilege(ctx context.Context, tx pgx.Tx, role string, grants []Grant) error {
	var tables, privileges []string
	for _, g := range grants {
		for _, p := range g.Privileges {
			tables = append(tables, strings.ToLower(g.Table))
			privileges = append(privileges, string(p))
		}
	}
	leftovers := func(query string) ([]string, error) {
		rows, err := tx.Query(ctx, query, role, tables, privileges)
		if err != nil {
			return nil, err
		}
		return pgx.CollectRows(rows, pgx.RowTo[string])
	}
	missing, err := leftovers(selectMissingPrivilege)
	if err != nil {
		return err
	}
	excess, err := leftovers(selectExcessPrivilege)
	if err != nil {
		return err
	}
	var refusals []string
	if len(missing) > 0 {
		refusals = append(refusals, fmt.Sprintf("lacks %s after the grants; the role applying them must own the table", strings.Join(missing, ", ")))
	}
	if len(excess) > 0 {
		refusals = append(refusals, fmt.Sprintf("still holds %s beyond the declared grants", strings.Join(excess, ", ")))
	}
	if len(refusals) > 0 {
		return fmt.Errorf("postgres: grants: role %s %s", role, strings.Join(refusals, "; and "))
	}
	return nil
}

func grantStatements(role string, grants []Grant) []string {
	quoted := pgx.Identifier{role}.Sanitize()
	statements := []string{
		"REVOKE ALL ON ALL TABLES IN SCHEMA public FROM " + quoted,
		"REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM " + quoted,
	}
	for _, g := range grants {
		privileges := make([]string, len(g.Privileges))
		for i, p := range g.Privileges {
			privileges[i] = string(p)
		}
		table := pgx.Identifier{"public", strings.ToLower(g.Table)}.Sanitize()
		statements = append(statements, fmt.Sprintf("GRANT %s ON TABLE %s TO %s", strings.Join(privileges, ", "), table, quoted))
	}
	return statements
}
