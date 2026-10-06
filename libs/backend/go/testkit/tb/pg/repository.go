package pg

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// Within runs fn over the repository the provider binds to one transaction of
// pool: a repository never opens a transaction of its own.
func Within[ID comparable, S any](ctx context.Context, pool *pgxpool.Pool, repository func(*postgres.Tx) ports.Repository[ID, S], fn func(ctx context.Context, repo ports.Repository[ID, S]) error) error {
	return postgres.NewUnitOfWork(pool, repository).Within(ctx, fn)
}

func Seed[ID comparable, S any](t testing.TB, ctx context.Context, pool *pgxpool.Pool, repository func(*postgres.Tx) ports.Repository[ID, S], id ID, state S) {
	t.Helper()
	err := Within(ctx, pool, repository, func(ctx context.Context, repo ports.Repository[ID, S]) error {
		return repo.Save(ctx, id, state, 0)
	})
	if err != nil {
		t.Fatalf("pg.Seed(%v): %v", id, err)
	}
}

func Load[ID comparable, S any](t testing.TB, ctx context.Context, pool *pgxpool.Pool, repository func(*postgres.Tx) ports.Repository[ID, S], id ID) (S, ports.Version) {
	t.Helper()
	var (
		state   S
		version ports.Version
	)
	err := Within(ctx, pool, repository, func(ctx context.Context, repo ports.Repository[ID, S]) error {
		var err error
		state, version, err = repo.Load(ctx, id)
		return err
	})
	if err != nil {
		t.Fatalf("pg.Load(%v): %v", id, err)
	}
	return state, version
}
