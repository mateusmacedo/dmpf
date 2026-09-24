package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Tx is one open transaction. It is a struct rather than an interface because
// the composition root binds concrete ports off it. The pgx.Tx never leaves
// this package: a statement outside it could omit the tenant (IDN-14).
type Tx struct{ conn pgx.Tx }

// NewTx wraps a pgx.Tx so tests and composition roots outside this package can
// construct the same Tx that NewUnitOfWork passes to its bind callback.
func NewTx(conn pgx.Tx) *Tx { return &Tx{conn: conn} }

// NewUnitOfWork binds an open transaction to the resource set R that a use case
// declares. bind is written by the composition root, never here: provider →
// application is a forbidden cell, so this package cannot know R's shape.
func NewUnitOfWork[R any](pool *pgxpool.Pool, bind func(tx *Tx) R) ports.UnitOfWork[R] {
	return unitOfWork[R]{pool: pool, bind: bind}
}

type unitOfWork[R any] struct {
	pool *pgxpool.Pool
	bind func(tx *Tx) R
}

func (u unitOfWork[R]) Within(ctx context.Context, fn func(context.Context, R) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tx, err := u.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	// WithoutCancel porque o rollback é justamente o que precisa acontecer com o
	// ctx do chamador já morto. Cobre o caminho de panic, que sobe sem recover e
	// com a pilha original (ERR-22); após o commit devolve ErrTxClosed e é no-op.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if err := fn(ctx, u.bind(&Tx{conn: tx})); err != nil {
		rbErr := tx.Rollback(context.WithoutCancel(ctx))
		if rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			// Join, e não embrulho: errors.Is do chamador precisa continuar
			// alcançando o erro do callback, que é o que o contrato promete.
			return errors.Join(err, rbErr)
		}
		return err
	}

	return tx.Commit(ctx)
}
