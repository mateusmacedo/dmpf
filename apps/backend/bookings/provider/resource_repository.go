package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
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

func NewResourceRepository(tx *postgres.Tx) ports.Repository[domain.ResourceCode, domain.ResourceSnapshot] {
	return resourceRepository{conn: tx.Conn()}
}

type resourceRepository struct{ conn pgx.Tx }

func (r resourceRepository) Load(ctx context.Context, code domain.ResourceCode) (domain.ResourceSnapshot, ports.Version, error) {
	var (
		version      int64
		registeredAt int64
	)
	err := r.conn.QueryRow(ctx, selectResource, string(code)).Scan(&version, &registeredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ResourceSnapshot{}, 0, ports.ErrNotFound
	}
	if err != nil {
		return domain.ResourceSnapshot{}, 0, fmt.Errorf("provider: load resource %s: %w", code, err)
	}
	return domain.ResourceSnapshot{
		Code:         code,
		RegisteredAt: domain.Instant(registeredAt),
	}, ports.Version(version), nil
}

func (r resourceRepository) Save(ctx context.Context, code domain.ResourceCode, s domain.ResourceSnapshot, expected ports.Version) error {
	if expected == 0 {
		tag, err := r.conn.Exec(ctx, insertResource, string(code), int64(s.RegisteredAt))
		if err != nil {
			return fmt.Errorf("provider: insert resource %s: %w", code, err)
		}
		if tag.RowsAffected() == 0 {
			return ports.ErrVersionConflict
		}
		return nil
	}
	tag, err := r.conn.Exec(ctx, updateResource, string(code), int64(expected), int64(s.RegisteredAt))
	if err != nil {
		return fmt.Errorf("provider: update resource %s: %w", code, err)
	}
	if tag.RowsAffected() == 0 {
		return ports.ErrVersionConflict
	}
	return nil
}
