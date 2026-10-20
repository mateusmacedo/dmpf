package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Unique maps one UNIQUE constraint of the hand-written schema to its rejection. Name
// is the server's ConstraintName: the folded, untruncated <table>_<columns>_key (ADR-053).
// Declare in DDL order: under several collisions the server reports the first index.
type Unique struct {
	Name      string
	Columns   []string
	Rejection *domain.Rejection
}

// UniqueSet is a table's declaration without its type parameters, so a
// composition root can verify every table at startup through one list.
type UniqueSet struct {
	Table       string
	Constraints []Unique
}

// ErrUniqueDivergent reports a declared constraint the catalog does not hold as
// declared: a Save over it would not translate the violation it promises.
var ErrUniqueDivergent = errors.New("postgres: a declared unique constraint diverges from the schema")

// maxIdentifier is NAMEDATALEN-1: past it the server truncates with a NOTICE and
// reports a ConstraintName that no longer matches the declaration.
const maxIdentifier = 63

// savepointCleanup bounds the undo that runs detached from the caller: a
// cancelled caller is exactly when the savepoint still has to be undone.
var savepointCleanup = 5 * time.Second

func (t Table[ID, S]) Uniques() UniqueSet {
	t.validate()
	return UniqueSet{Table: t.Name, Constraints: slices.Clone(t.Unique)}
}

func (t Table[ID, S]) declared(constraint string) (Unique, bool) {
	for _, u := range t.Unique {
		if u.Name == constraint {
			return u, true
		}
	}
	return Unique{}, false
}

// The name is fixed so each statement is prepared once per connection: savepoints
// of one name stack, and RELEASE or ROLLBACK TO reach the newest, this Save's own.
const (
	openSavepoint     = "SAVEPOINT dmpf_unique"
	releaseSavepoint  = "RELEASE SAVEPOINT dmpf_unique"
	rollbackSavepoint = "ROLLBACK TO SAVEPOINT dmpf_unique"
)

// saveUnderSavepoint batches the SAVEPOINT with the write only when no prepare comes
// first: pgx prepares ahead of the pipeline (conn.go:1170), before the SAVEPOINT.
func (r tableRepository[ID, S]) saveUnderSavepoint(ctx context.Context, tenant ports.TenantID, id ID, guard guardLock, statement string, args []any, creating bool) error {
	plan := r.tx.savepoints()
	inline := plan.inline(statement)
	batch := &pgx.Batch{}
	guarded := guard.statement != ""
	if inline {
		if guarded {
			batch.Queue(guard.statement, guard.args...)
		}
		batch.Queue(openSavepoint)
	} else {
		if guarded {
			if _, err := r.tx.conn.Exec(ctx, guard.statement, guard.args...); err != nil {
				return fmt.Errorf("postgres: save %s %s: guard: %w", r.table.Name, id, err)
			}
		}
		if _, err := r.tx.conn.Exec(ctx, openSavepoint); err != nil {
			return r.unopened(id, err)
		}
		if err := plan.pin(ctx, r.tx.conn, statement, guard.statement); err != nil {
			return r.undo(ctx, tenant, id, err, creating)
		}
	}
	batch.Queue(statement, args...)
	batch.Queue(releaseSavepoint)
	results := r.tx.conn.SendBatch(ctx, batch)
	var lockErr, openErr error
	if inline && guarded {
		_, lockErr = results.Exec()
	}
	if inline {
		_, openErr = results.Exec()
	}
	tag, err := results.Exec()
	_, releaseErr := results.Exec()
	closeErr := results.Close()

	switch {
	case lockErr != nil && lockFailed(lockErr, guard.statement):
		return fmt.Errorf("postgres: save %s %s: guard: %w", r.table.Name, id, unbatched(lockErr))
	case lockErr != nil:
		return r.unopened(id, unbatched(lockErr))
	case openErr != nil:
		return r.unopened(id, unbatched(openErr))
	case err != nil:
		return r.undo(ctx, tenant, id, unbatched(err), creating)
	case releaseErr != nil:
		return fmt.Errorf("postgres: save %s %s: release savepoint: %w", r.table.Name, id, releaseErr)
	case closeErr != nil:
		return fmt.Errorf("postgres: save %s %s: %w", r.table.Name, id, closeErr)
	}
	return r.settle(tag)
}

func (r tableRepository[ID, S]) unopened(id ID, err error) error {
	if _, refused := errors.AsType[*pgconn.PgError](err); refused {
		return fmt.Errorf("postgres: save %s %s: savepoint: %w", r.table.Name, id, err)
	}
	return fmt.Errorf("postgres: save %s %s: %w", r.table.Name, id, err)
}

func unbatched(err error) error {
	if preprocessing, ok := errors.AsType[pgx.ErrPreprocessingBatch](err); ok {
		return preprocessing.Unwrap()
	}
	return err
}

func (r tableRepository[ID, S]) undo(ctx context.Context, tenant ports.TenantID, id ID, cause error, creating bool) error {
	failed := fmt.Errorf("postgres: save %s %s: %w", r.table.Name, id, cause)
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), savepointCleanup)
	defer cancel()
	if _, err := r.tx.conn.Exec(cleanup, rollbackSavepoint); err != nil {
		return errors.Join(failed, fmt.Errorf("postgres: rollback to savepoint: %w", err))
	}
	return r.translate(ctx, tenant, id, cause, failed, creating)
}

const savepointPlanKey = "github.com/mateusmacedo/dmpf/libs/backend/go/postgres.savepointPlan"

// savepointPlan lives on the connection, as its prepared statements do. A mode that
// prepares or describes ahead of a batch would do it before the SAVEPOINT, where a
// refused write aborts the transaction (conn.go:1170): only a pinned write is batched.
type savepointPlan struct {
	unprepared bool
	pins       bool
	pinned     map[string]bool
}

func savepointPlanOf(conn *pgx.Conn) *savepointPlan {
	data := conn.PgConn().CustomData()
	if plan, ok := data[savepointPlanKey].(*savepointPlan); ok {
		return plan
	}
	plan := &savepointPlan{pinned: map[string]bool{}}
	switch conn.Config().DefaultQueryExecMode {
	case pgx.QueryExecModeExec, pgx.QueryExecModeSimpleProtocol:
		plan.unprepared = true
	case pgx.QueryExecModeCacheStatement:
		plan.pins = true
	}
	data[savepointPlanKey] = plan
	return plan
}

func (p *savepointPlan) inline(statement string) bool {
	return p.unprepared || p.pinned[statement]
}

// pin prepares by name, out of the statement cache's LRU: an evicted write would be
// prepared again ahead of the next batch (conn.go:998,1083).
func (p *savepointPlan) pin(ctx context.Context, conn pgx.Tx, statement, guard string) error {
	if !p.pins {
		return nil
	}
	pinned := []string{statement, openSavepoint, releaseSavepoint}
	if guard != "" {
		pinned = append(pinned, guard)
	}
	for _, sql := range pinned {
		if _, err := conn.Prepare(ctx, sql, sql); err != nil {
			return err
		}
	}
	p.pinned[statement] = true
	return nil
}

// translate never copies the PgError's Detail, which quotes the row (ERR-07):
// the violation names the constraint, and the driver error stays wrapped.
func (r tableRepository[ID, S]) translate(ctx context.Context, tenant ports.TenantID, id ID, cause, failed error, creating bool) error {
	var pgErr *pgconn.PgError
	if !errors.As(cause, &pgErr) || pgErr.Code != uniqueViolation || pgErr.TableName != strings.ToLower(r.table.Name) {
		return failed
	}
	violation := ports.UniqueViolation{Table: r.table.Name, Constraint: pgErr.ConstraintName}
	if unique, ok := r.table.declared(pgErr.ConstraintName); ok {
		if creating {
			taken, err := r.identityTaken(ctx, tenant, id)
			if err != nil {
				return errors.Join(failed, err)
			}
			if taken {
				return ports.ErrVersionConflict
			}
		}
		if r.tx.writes == 0 {
			violation.Rejection = unique.Rejection
		}
	}
	return fmt.Errorf("postgres: save %s %s: %w: %w", r.table.Name, id, violation, cause)
}

// identityTaken tells a concurrent create of the same identity, which the server
// may report on a declared constraint before the primary key, from a refusal.
func (r tableRepository[ID, S]) identityTaken(ctx context.Context, tenant ports.TenantID, id ID) (bool, error) {
	var one int
	switch err := r.tx.conn.QueryRow(ctx, r.statements.exists, string(tenant), string(id)).Scan(&one); {
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("postgres: probe identity of %s %s: %w", r.table.Name, id, err)
	}
	return true, nil
}

func checkUniques(table string, uniques []Unique) error {
	seen := make(map[string]bool, len(uniques))
	for _, u := range uniques {
		if err := checkUnique(table, u); err != nil {
			return err
		}
		if seen[u.Name] {
			return fmt.Errorf("postgres: Table %q declares unique %q twice", table, u.Name)
		}
		seen[u.Name] = true
	}
	return nil
}

func checkUnique(table string, u Unique) error {
	for _, column := range u.Columns {
		if !isLowerIdentifier(column) {
			return fmt.Errorf("postgres: Table %q unique %q column %q is not a lower-case SQL identifier", table, u.Name, column)
		}
	}
	want := strings.ToLower(table) + "_" + strings.Join(u.Columns, "_") + "_key"
	switch {
	case !isLowerIdentifier(u.Name):
		return fmt.Errorf("postgres: Table %q unique %q is not a lower-case SQL identifier", table, u.Name)
	case len(u.Columns) < 2 || u.Columns[0] != "tenant_id":
		return fmt.Errorf("postgres: Table %q unique %q has to lead with tenant_id and one more column", table, u.Name)
	case u.Name != want:
		return fmt.Errorf("postgres: Table %q unique %q has to be named %q", table, u.Name, want)
	case len(u.Name) > maxIdentifier:
		return fmt.Errorf("postgres: Table %q unique %q exceeds %d bytes", table, u.Name, maxIdentifier)
	case u.Rejection == nil:
		return fmt.Errorf("postgres: Table %q unique %q declares no rejection", table, u.Name)
	case !u.Rejection.Code().Valid():
		return fmt.Errorf("postgres: Table %q unique %q rejection code %q is not a domain code", table, u.Name, u.Rejection.Code())
	}
	return nil
}

func isLowerIdentifier(s string) bool {
	return isIdentifier(s) && s == strings.ToLower(s)
}

// The catalog is read by the table resolved in public, as wait.go resolves it:
// a CREATE UNIQUE INDEX has no pg_constraint row, a DEFERRABLE one fires at COMMIT,
// after the savepoint, and a partition reports its own table in the 23505.
const selectFirstDivergentUnique = `
WITH declared AS (
  SELECT d.tablename, d.conname, d.columns, d.position, c.oid AS conoid, c.contype, c.condeferrable, c.conindid,
    coalesce(i.indnullsnotdistinct, false) AS nullsnotdistinct, k.columns AS got,
    r.relkind = 'p' OR EXISTS (SELECT 1 FROM pg_inherits h WHERE h.inhparent = r.oid OR h.inhrelid = r.oid) AS inherited,
    lag(c.conindid) OVER sequence AS prior_index, lag(d.conname) OVER sequence AS prior_name
  FROM unnest($1::text[], $2::text[], $3::text[], $4::int[]) WITH ORDINALITY AS d(tablename, conname, columns, declaration, position)
  LEFT JOIN pg_class r ON r.oid = to_regclass('public.' || quote_ident(lower(d.tablename)))
  LEFT JOIN pg_constraint c ON c.conrelid = r.oid AND c.conname = d.conname
  LEFT JOIN pg_index i ON i.indexrelid = c.conindid
  LEFT JOIN LATERAL (
    SELECT string_agg(a.attname, ',' ORDER BY u.ord) AS columns
    FROM unnest(c.conkey) WITH ORDINALITY AS u(attnum, ord)
    JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = u.attnum
  ) k ON true
  WINDOW sequence AS (PARTITION BY d.declaration ORDER BY d.position)
)
SELECT tablename, conname, columns, coalesce(got, ''), coalesce(prior_name, ''),
  CASE
    WHEN inherited THEN 'partitioned or inherited table'
    WHEN conoid IS NULL THEN 'absent'
    WHEN contype <> 'u' THEN 'not a unique constraint'
    WHEN condeferrable THEN 'deferrable'
    WHEN nullsnotdistinct THEN 'nulls not distinct'
    WHEN got IS DISTINCT FROM columns THEN 'columns'
    ELSE 'order'
  END
FROM declared
WHERE inherited OR conoid IS NULL OR contype <> 'u' OR condeferrable OR nullsnotdistinct
   OR got IS DISTINCT FROM columns OR prior_index > conindid
ORDER BY position LIMIT 1`

var _ = declare(selectFirstDivergentUnique, "SELECT", "")

// VerifyUniques confirms at startup that every declared constraint exists in the
// catalog as declared, in the order of its index, and reports the first one that
// does not as ErrUniqueDivergent. Nothing declared makes no round trip.
func VerifyUniques(ctx context.Context, pool *pgxpool.Pool, sets ...UniqueSet) error {
	divergent, err := firstDivergentUnique(ctx, pool, sets)
	if err != nil || divergent == (divergence{}) {
		return err
	}
	return divergent.err()
}

type divergence struct {
	table, constraint, want, got, prior, reason string
}

func (d divergence) err() error {
	switch d.reason {
	case "columns":
		return fmt.Errorf("%w: %s on %s has columns (%s), want (%s)", ErrUniqueDivergent, d.constraint, d.table, d.got, d.want)
	case "order":
		return fmt.Errorf("%w: %s on %s is declared after %s but its index comes first: declare in DDL order", ErrUniqueDivergent, d.constraint, d.table, d.prior)
	}
	return fmt.Errorf("%w: %s on %s: %s", ErrUniqueDivergent, d.constraint, d.table, d.reason)
}

func firstDivergentUnique(ctx context.Context, pool *pgxpool.Pool, sets []UniqueSet) (divergence, error) {
	var tables, names, columns []string
	var declarations []int32
	for n, set := range sets {
		if err := checkUniques(set.Table, set.Constraints); err != nil {
			return divergence{}, err
		}
		for _, u := range set.Constraints {
			tables = append(tables, set.Table)
			names = append(names, u.Name)
			columns = append(columns, strings.Join(u.Columns, ","))
			declarations = append(declarations, int32(n))
		}
	}
	if len(names) == 0 {
		return divergence{}, nil
	}

	var d divergence
	switch err := pool.QueryRow(ctx, selectFirstDivergentUnique, tables, names, columns, declarations).Scan(&d.table, &d.constraint, &d.want, &d.got, &d.prior, &d.reason); {
	case errors.Is(err, pgx.ErrNoRows):
		return divergence{}, nil
	case err != nil:
		return divergence{}, fmt.Errorf("postgres: verify uniques: %w", err)
	}
	return d, nil
}
