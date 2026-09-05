package reservationspg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/reservations"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
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
func NewRepository(tx *dmpfpostgres.Tx) dmpfports.Repository[reservations.OrderID, reservations.Snapshot] {
	return repository{conn: tx.Conn()}
}

type repository struct{ conn pgx.Tx }

func (r repository) Load(ctx context.Context, id reservations.OrderID) (reservations.Snapshot, dmpfports.Version, error) {
	var (
		version int64
		raw     []byte
	)
	switch err := r.conn.QueryRow(ctx, selectReservation, string(id)).Scan(&version, &raw); {
	case errors.Is(err, pgx.ErrNoRows):
		return reservations.Snapshot{}, 0, dmpfports.ErrNotFound
	case err != nil:
		return reservations.Snapshot{}, 0, fmt.Errorf("reservationspg: load %s: %w", id, err)
	}

	var snapshot reservations.Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return reservations.Snapshot{}, 0, fmt.Errorf("reservationspg: decode snapshot of %s: %w", id, err)
	}
	return snapshot, dmpfports.Version(version), nil
}

func (r repository) Save(ctx context.Context, id reservations.OrderID, state reservations.Snapshot, expected dmpfports.Version) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("reservationspg: encode snapshot of %s: %w", id, err)
	}

	statement, args := updateReservation, []any{string(id), int64(expected), raw}
	if expected == 0 {
		statement, args = insertReservation, []any{string(id), raw}
	}

	tag, err := r.conn.Exec(ctx, statement, args...)
	if err != nil {
		return fmt.Errorf("reservationspg: save %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return dmpfports.ErrVersionConflict
	}
	return nil
}
