package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

const (
	selectReservation = `SELECT version, snapshot FROM dmpf_example_reservations WHERE order_id = $1`

	insertReservation = `
		INSERT INTO dmpf_example_reservations (order_id, version, snapshot) VALUES ($1, 1, $2)
		ON CONFLICT (order_id) DO NOTHING`

	updateReservation = `
		UPDATE dmpf_example_reservations SET version = $2 + 1, snapshot = $3
		WHERE order_id = $1 AND version = $2`
)

// NewRepository binds the repository to an open transaction (UOW-01).
func NewRepository(tx *postgres.Tx) ports.Repository[domain.OrderID, domain.Snapshot] {
	return repository{conn: tx.Conn()}
}

type repository struct{ conn pgx.Tx }

func (r repository) Load(ctx context.Context, id domain.OrderID) (domain.Snapshot, ports.Version, error) {
	return load(ctx, r.conn, id)
}

// querier is the one method Load needs, satisfied by a transaction and by the
// pool alike: the SELECT is the same, only the boundary around it differs.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func load(ctx context.Context, q querier, id domain.OrderID) (domain.Snapshot, ports.Version, error) {
	var (
		version int64
		raw     []byte
	)
	switch err := q.QueryRow(ctx, selectReservation, string(id)).Scan(&version, &raw); {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Snapshot{}, 0, ports.ErrNotFound
	case err != nil:
		return domain.Snapshot{}, 0, fmt.Errorf("provider: load %s: %w", id, err)
	}

	var snapshot domain.Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return domain.Snapshot{}, 0, fmt.Errorf("provider: decode snapshot of %s: %w", id, err)
	}
	return snapshot, ports.Version(version), nil
}

func (r repository) Save(ctx context.Context, id domain.OrderID, state domain.Snapshot, expected ports.Version) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("provider: encode snapshot of %s: %w", id, err)
	}

	statement, args := updateReservation, []any{string(id), int64(expected), raw}
	if expected == 0 {
		statement, args = insertReservation, []any{string(id), raw}
	}

	tag, err := r.conn.Exec(ctx, statement, args...)
	if err != nil {
		return fmt.Errorf("provider: save %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ports.ErrVersionConflict
	}
	return nil
}
