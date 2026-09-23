package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ErrTenantUnresolved reports a read or write reached persistence with no tenant
// on the carrier. It is a refusal, not an empty result: an unscoped query is the
// one thing IDN-14 exists to make impossible.
var ErrTenantUnresolved = errors.New("postgres: the execution context resolved no tenant")

// Table names one aggregate table and how its state crosses the SQL boundary.
// Every statement it builds is scoped to the tenant on the carrier, and Columns
// holds the state columns alone, in the order Encode and Decode use.
//
// WHY: IDN-14 refuses an isolation whose correctness depends on each author
// remembering the predicate, so the statements are written here, once, and the
// context block only declares how S maps to columns.
type Table[ID ~string, S any] struct {
	Name     string
	IDColumn string
	Columns  []string

	Encode func(S) ([]any, error)
	Decode func(scan func(dest ...any) error) (S, error)

	// WithID puts the identifier back on a state that carries its own identity.
	// The table owns the ID column, so Decode never scans it; nil means S does
	// not hold one (a JSON snapshot already carries it).
	WithID func(S, ID) S
}

// ReadPool is the pool as a context provider holds it: only statements this
// package compiles run on it, so a provider cannot write a query that omits the
// tenant predicate (IDN-14). The driver's pool stays with the composition root.
type ReadPool struct{ pool *pgxpool.Pool }

func NewReadPool(pool *pgxpool.Pool) ReadPool {
	if pool == nil {
		panic("postgres: NewReadPool received a nil pool")
	}
	return ReadPool{pool: pool}
}

// Reader serves the read side without the write side (UOW-11), over a pooled
// connection in autocommit: a query must not open a transaction.
func (t Table[ID, S]) Reader(pool ReadPool) ports.Reader[ID, S] {
	return tableReader[ID, S]{table: t, statements: t.compile(), pool: pool.pool}
}

// Repository binds the table to an open transaction, so aggregate state and the
// outbox record commit together (UOW-01).
func (t Table[ID, S]) Repository(tx *Tx) ports.Repository[ID, S] {
	return tableRepository[ID, S]{table: t, statements: t.compile(), conn: tx.conn}
}

// Relation serves the query the generic ports cannot express: many rows of one
// table, filtered by a column the context chooses. The kernel still writes the
// statement, so the tenant predicate is not the caller's to remember (IDN-14).
//
// WHY: a context that had to write this SELECT by hand would need pgx, which is
// the access the scope exists to close. What it does not cover — joins,
// aggregates, anything beyond one column of one table — is a named exception,
// never a reason to reopen pgx.
func (t Table[ID, S]) Relation(filterColumn string) Relation[ID, S] {
	if !isIdentifier(filterColumn) {
		panic(fmt.Sprintf("postgres: Table %q relation column %q is not a SQL identifier", t.Name, filterColumn))
	}
	t.validate()

	return Relation[ID, S]{
		table: t,
		statement: fmt.Sprintf(
			"SELECT %s, version, %s FROM %s WHERE tenant_id = $1 AND %s = $2",
			t.IDColumn, strings.Join(t.Columns, ", "), t.Name, filterColumn),
	}
}

type Relation[ID ~string, S any] struct {
	table     Table[ID, S]
	statement string
}

// Query returns every row of this tenant whose filter column equals value, in
// no declared order. A row of another tenant is not filtered out afterwards: it
// never leaves the server.
func (r Relation[ID, S]) Query(ctx context.Context, pool ReadPool, value any) ([]S, error) {
	tenant, err := tenantOf(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := pool.pool.Query(ctx, r.statement, string(tenant), value)
	if err != nil {
		return nil, fmt.Errorf("postgres: query %s: %w", r.table.Name, err)
	}
	defer rows.Close()

	var out []S
	for rows.Next() {
		var (
			id      ID
			version int64
		)
		state, err := r.table.Decode(func(dest ...any) error {
			return rows.Scan(append([]any{&id, &version}, dest...)...)
		})
		if err != nil {
			return nil, fmt.Errorf("postgres: scan %s: %w", r.table.Name, err)
		}
		out = append(out, r.table.identify(state, id))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: rows of %s: %w", r.table.Name, err)
	}
	return out, nil
}

type statements struct {
	selectOne string
	owner     string
	insert    string
	update    string
}

// compile runs once per bind rather than once per operation.
func (t Table[ID, S]) compile() statements {
	t.validate()

	columns := strings.Join(t.Columns, ", ")
	placeholders := make([]string, len(t.Columns))
	assignments := make([]string, len(t.Columns))
	for i, column := range t.Columns {
		placeholders[i] = fmt.Sprintf("$%d", i+4)
		assignments[i] = fmt.Sprintf("%s = $%d", column, i+4)
	}

	return statements{
		selectOne: fmt.Sprintf(
			"SELECT version, %s FROM %s WHERE tenant_id = $1 AND %s = $2",
			columns, t.Name, t.IDColumn),
		// Only the owning tenant comes back, never a state column: the probe
		// feeds the security record (IDN-12), not the caller (IDN-13).
		owner: fmt.Sprintf(
			"SELECT tenant_id FROM %s WHERE %s = $1 AND tenant_id <> $2 ORDER BY tenant_id LIMIT 1",
			t.Name, t.IDColumn),
		// ON CONFLICT DO NOTHING rather than an upsert: a create over an
		// existing aggregate is the same lost update as a stale expected
		// version, and both have to come back as ErrVersionConflict.
		insert: fmt.Sprintf(
			"INSERT INTO %s (tenant_id, %s, version, %s) VALUES ($1, $2, $3, %s) ON CONFLICT (tenant_id, %s) DO NOTHING",
			t.Name, t.IDColumn, columns, strings.Join(placeholders, ", "), t.IDColumn),
		// The version in the WHERE clause is the lock: two concurrent writers
		// read the same version, and only the first UPDATE matches a row.
		update: fmt.Sprintf(
			"UPDATE %s SET version = $3 + 1, %s WHERE tenant_id = $1 AND %s = $2 AND version = $3",
			t.Name, strings.Join(assignments, ", "), t.IDColumn),
	}
}

func (t Table[ID, S]) validate() {
	switch {
	case !isIdentifier(t.Name):
		panic(fmt.Sprintf("postgres: Table.Name %q is not a SQL identifier", t.Name))
	case !isIdentifier(t.IDColumn):
		panic(fmt.Sprintf("postgres: Table.IDColumn %q is not a SQL identifier", t.IDColumn))
	case len(t.Columns) == 0:
		panic(fmt.Sprintf("postgres: Table %q declares no state column", t.Name))
	case t.Encode == nil || t.Decode == nil:
		panic(fmt.Sprintf("postgres: Table %q declares no Encode/Decode pair", t.Name))
	}
	for _, column := range t.Columns {
		if !isIdentifier(column) {
			panic(fmt.Sprintf("postgres: Table %q column %q is not a SQL identifier", t.Name, column))
		}
		if column == "tenant_id" || column == "version" || column == t.IDColumn {
			panic(fmt.Sprintf("postgres: Table %q lists %q, which the table owns", t.Name, column))
		}
	}
}

// SAFETY: Name, IDColumn and Columns reach the statement unquoted, so a Table
// built from a variable must not be able to carry a fragment of SQL into it.
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		alpha := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if alpha || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// tenantOf is the single place the scope is resolved. Absence refuses (IDN-15)
// instead of widening the query, and no value is invented for it (IDN-20).
func tenantOf(ctx context.Context) (ports.TenantID, error) {
	execution, err := ports.RequireExecutionContext(ctx)
	if err != nil {
		return "", err
	}
	tenant, ok := execution.Tenant()
	if !ok {
		return "", ErrTenantUnresolved
	}
	return tenant, nil
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (t Table[ID, S]) load(ctx context.Context, q querier, stmts statements, id ID) (S, ports.Version, error) {
	var zero S

	tenant, err := tenantOf(ctx)
	if err != nil {
		return zero, 0, err
	}

	var version int64
	row := q.QueryRow(ctx, stmts.selectOne, string(tenant), string(id))
	state, err := t.Decode(func(dest ...any) error {
		return row.Scan(append([]any{&version}, dest...)...)
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return zero, 0, t.miss(ctx, q, stmts.owner, tenant, id)
	case err != nil:
		return zero, 0, fmt.Errorf("postgres: load %s %s: %w", t.Name, id, err)
	}
	return t.identify(state, id), ports.Version(version), nil
}

// miss runs on every miss, owner or not, so the latency does not tell the two
// apart either. A failed probe is a failure: a lost security record must not
// pass for a plain not-found.
func (t Table[ID, S]) miss(ctx context.Context, q querier, stmt string, tenant ports.TenantID, id ID) error {
	var owner string
	switch err := q.QueryRow(ctx, stmt, string(id), string(tenant)).Scan(&owner); {
	case errors.Is(err, pgx.ErrNoRows):
		return ports.ErrNotFound
	case err != nil:
		return fmt.Errorf("postgres: probe owner of %s %s: %w", t.Name, id, err)
	}
	return ports.CrossTenantAccess{
		Object:        t.Name + "/" + string(id),
		ContextTenant: tenant,
		DataTenant:    ports.TenantID(owner),
	}
}

func (t Table[ID, S]) identify(state S, id ID) S {
	if t.WithID == nil {
		return state
	}
	return t.WithID(state, id)
}

type tableReader[ID ~string, S any] struct {
	table      Table[ID, S]
	statements statements
	pool       *pgxpool.Pool
}

func (r tableReader[ID, S]) Load(ctx context.Context, id ID) (S, ports.Version, error) {
	return r.table.load(ctx, r.pool, r.statements, id)
}

type tableRepository[ID ~string, S any] struct {
	table      Table[ID, S]
	statements statements
	conn       pgx.Tx
}

func (r tableRepository[ID, S]) Load(ctx context.Context, id ID) (S, ports.Version, error) {
	return r.table.load(ctx, r.conn, r.statements, id)
}

func (r tableRepository[ID, S]) Save(ctx context.Context, id ID, state S, expected ports.Version) error {
	tenant, err := tenantOf(ctx)
	if err != nil {
		return err
	}

	values, err := r.table.Encode(state)
	if err != nil {
		return fmt.Errorf("postgres: encode %s %s: %w", r.table.Name, id, err)
	}
	if len(values) != len(r.table.Columns) {
		return fmt.Errorf("postgres: encode %s %s: %d values for %d columns",
			r.table.Name, id, len(values), len(r.table.Columns))
	}

	statement, version := r.statements.update, int64(expected)
	if expected == 0 {
		statement, version = r.statements.insert, 1
	}
	args := append([]any{string(tenant), string(id), version}, values...)

	tag, err := r.conn.Exec(ctx, statement, args...)
	if err != nil {
		return fmt.Errorf("postgres: save %s %s: %w", r.table.Name, id, err)
	}
	if tag.RowsAffected() == 0 {
		return ports.ErrVersionConflict
	}
	return nil
}
