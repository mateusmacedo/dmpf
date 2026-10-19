package postgres

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Equal values printed as different text would take different locks (ADR-067).
var ErrGuardValue = errors.New("postgres: a guard value must be a string")

const deadlockDetected = "40P01"

// guardKey hashes the text Go sends for the value, not the column's typed value,
// so a uuid or citext guard would split equal values; the tenant's length keeps
// a tenant holding chr(31) from meeting another's key (ADR-067).
const guardKey = "hashtextextended(concat_ws(chr(31), '%s', '%s', length($1::text), $1::text, $2::text), 0)"

type guardStatements struct {
	shared    string
	exclusive string
}

func (t Table[ID, S]) compileGuard() guardStatements {
	if t.Guard == "" {
		return guardStatements{}
	}
	key := fmt.Sprintf(guardKey, t.Name, t.Guard)
	return guardStatements{
		shared:    declare("SELECT pg_advisory_xact_lock_shared("+key+")", "SELECT", t.Name),
		exclusive: declare("SELECT pg_advisory_xact_lock("+key+")", "SELECT", t.Name),
	}
}

func (t Table[ID, S]) guardIndex() int {
	for i, column := range t.Columns {
		if column == t.Guard {
			return i
		}
	}
	return -1
}

func guardText(table, column string, value any) (string, error) {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.String {
		return "", fmt.Errorf("%w: %s.%s got %T", ErrGuardValue, table, column, value)
	}
	return v.String(), nil
}

// pgx prepares and encodes the whole batch before sending it and reports any of
// that on the first result, the lock's (pgx v5 conn.go:1198,1232).
func lockFailed(err error, lock string) bool {
	if preprocessing, ok := errors.AsType[pgx.ErrPreprocessingBatch](err); ok {
		return preprocessing.SQL() == lock
	}
	return true
}

// conflictOnDeadlock turns a deadlock into a version conflict once the
// transaction holds a guard: the victim may be any statement of the cycle, not
// the lock, and either way the decision raced another one over the same series.
func (t *Tx) conflictOnDeadlock(err error) error {
	if err == nil || !t.guarded {
		return err
	}
	if coded, ok := errors.AsType[*pgconn.PgError](err); ok && coded.Code == deadlockDetected {
		return fmt.Errorf("%w: %w", ports.ErrVersionConflict, err)
	}
	return err
}
