package serve

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

var (
	ErrMigrator          = errors.New("serve: the migrating role failed")
	ErrMigratorIsRuntime = errors.New("serve: the migrating role is the runtime role")
	ErrMigratorElsewhere = errors.New("serve: MigrateDSN (PG_MIGRATE_DSN) and DSN (PG_DSN) reach different databases or servers")
	ErrRuntimeUngranted  = errors.New("serve: the runtime role still lacks what the migrating role granted")
)

// migratorFailure carries only the redacted text of the cause: pgconn writes
// the user of the connection into ConnectError (pgconn/errors.go:69), and
// MigrateDSN must never reach a log, a span or an error.
type migratorFailure struct {
	step   string
	detail string
}

func (f migratorFailure) Error() string {
	return fmt.Sprintf("%v: %s: %s", ErrMigrator, f.step, f.detail)
}

func (f migratorFailure) Unwrap() error { return ErrMigrator }

// WHY: no meter provider; postgres.NewPool never unregisters its callback, and this pool is short-lived.
func migrateAs(ctx context.Context, dsn string, runtime postgres.Session, caps []postgres.Capability, schema string, grants []postgres.Grant) error {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return migratorFailure{step: "parse", detail: "the connection string is not valid"}
	}
	user := config.ConnConfig.User
	migrator, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return redact("connect", err, dsn, user)
	}
	defer migrator.Close()
	session, err := postgres.SessionOf(ctx, migrator)
	switch {
	case err != nil:
		return redact("connect", err, dsn, user)
	case session.Role == runtime.Role:
		return ErrMigratorIsRuntime
	case !session.SameDatabase(runtime):
		return ErrMigratorElsewhere
	}
	if err := postgres.AssertGrantable(ctx, migrator, runtime.Role); err != nil {
		return redact("grants", err, dsn, user)
	}
	if err := postgres.Migrate(ctx, migrator, caps, schema); err != nil {
		return redact("migrate", err, dsn, user)
	}
	if err := postgres.ApplyGrants(ctx, migrator, runtime.Role, grants...); err != nil {
		return redact("grants", err, dsn, user)
	}
	return nil
}

func awaitGrants(ctx context.Context, pool *pgxpool.Pool, logs log.LoggerProvider, timeout time.Duration, grants []postgres.Grant) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := postgres.WaitForGrants(waitCtx, pool, time.Second, logs, grants...)
	if err != nil && ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrRuntimeUngranted, err)
	}
	return err
}

// WHY: the server never echoes the password, which only travels inside the DSN.
func redact(step string, err error, dsn, user string) error {
	detail := strings.ReplaceAll(err.Error(), dsn, "[redacted]")
	if user != "" {
		detail = strings.NewReplacer(
			"user="+user+" ", "user=[redacted] ",
			"user="+user+"`", "user=[redacted]`",
			`user "`+user+`"`, `user "[redacted]"`,
			`role "`+user+`"`, `role "[redacted]"`,
		).Replace(detail)
	}
	return migratorFailure{step: step, detail: detail}
}
