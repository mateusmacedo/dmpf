package bookingspostgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

const (
	selectResource = `SELECT version, registered_at FROM bookings_resource WHERE code = $1`

	insertResource = `
		INSERT INTO bookings_resource (code, version, registered_at)
		VALUES ($1, 1, $2)
		ON CONFLICT (code) DO NOTHING`

	updateResource = `
		UPDATE bookings_resource SET version = $2 + 1, registered_at = $3
		WHERE code = $1 AND version = $2`
)

func NewResourceRepository(tx *dmpfpostgres.Tx) dmpfports.Repository[bookingsdomain.ResourceCode, bookingsdomain.ResourceSnapshot] {
	return resourceRepository{conn: tx.Conn()}
}

type resourceRepository struct{ conn pgx.Tx }

func (r resourceRepository) Load(ctx context.Context, code bookingsdomain.ResourceCode) (bookingsdomain.ResourceSnapshot, dmpfports.Version, error) {
	var (
		version      int64
		registeredAt int64
	)
	err := r.conn.QueryRow(ctx, selectResource, string(code)).Scan(&version, &registeredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return bookingsdomain.ResourceSnapshot{}, 0, dmpfports.ErrNotFound
	}
	if err != nil {
		return bookingsdomain.ResourceSnapshot{}, 0, fmt.Errorf("bookingspostgres: load resource %s: %w", code, err)
	}
	return bookingsdomain.ResourceSnapshot{
		Code:         code,
		RegisteredAt: bookingsdomain.Instant(registeredAt),
	}, dmpfports.Version(version), nil
}

func (r resourceRepository) Save(ctx context.Context, code bookingsdomain.ResourceCode, s bookingsdomain.ResourceSnapshot, expected dmpfports.Version) error {
	if expected == 0 {
		tag, err := r.conn.Exec(ctx, insertResource, string(code), int64(s.RegisteredAt))
		if err != nil {
			return fmt.Errorf("bookingspostgres: insert resource %s: %w", code, err)
		}
		if tag.RowsAffected() == 0 {
			return dmpfports.ErrVersionConflict
		}
		return nil
	}
	tag, err := r.conn.Exec(ctx, updateResource, string(code), int64(expected), int64(s.RegisteredAt))
	if err != nil {
		return fmt.Errorf("bookingspostgres: update resource %s: %w", code, err)
	}
	if tag.RowsAffected() == 0 {
		return dmpfports.ErrVersionConflict
	}
	return nil
}
