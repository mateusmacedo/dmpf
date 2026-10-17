package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WHY: pg_postmaster_start_time() tells server instances apart, every role can
// call it, and a pooler in between does not change it.
type Session struct {
	Role     string
	Database string
	oid      uint32
	started  time.Time
}

func (s Session) SameDatabase(other Session) bool {
	return s.Database == other.Database && s.oid == other.oid && s.started.Equal(other.started)
}

const selectSession = `
SELECT current_user, d.datname, d.oid, pg_postmaster_start_time()
FROM pg_database d WHERE d.datname = current_database()`

var _ = declare(selectSession, "SELECT", "")

func SessionOf(ctx context.Context, pool *pgxpool.Pool) (Session, error) {
	var s Session
	if err := pool.QueryRow(ctx, selectSession).Scan(&s.Role, &s.Database, &s.oid, &s.started); err != nil {
		return Session{}, err
	}
	return s, nil
}
