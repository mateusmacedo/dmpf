package provider

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

//go:embed schema.sql
var schema string

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if err := postgres.Migrate(ctx, pool); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, schema)
	return err
}
