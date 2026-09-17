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
	selectBooking = `SELECT version, resource_id, quantity, status, reserved_at FROM bookings_booking WHERE booking_id = $1`

	insertBooking = `
		INSERT INTO bookings_booking (booking_id, version, resource_id, quantity, status, reserved_at)
		VALUES ($1, 1, $2, $3, $4, $5)
		ON CONFLICT (booking_id) DO NOTHING`

	updateBooking = `
		UPDATE bookings_booking SET version = $2 + 1, resource_id = $3, quantity = $4, status = $5, reserved_at = $6
		WHERE booking_id = $1 AND version = $2`
)

func NewBookingRepository(tx *postgres.Tx) ports.Repository[domain.BookingID, domain.BookingSnapshot] {
	return bookingRepository{conn: tx.Conn()}
}

type bookingRepository struct{ conn pgx.Tx }

func (r bookingRepository) Load(ctx context.Context, id domain.BookingID) (domain.BookingSnapshot, ports.Version, error) {
	return loadBooking(ctx, r.conn, id)
}

func (r bookingRepository) Save(ctx context.Context, id domain.BookingID, s domain.BookingSnapshot, expected ports.Version) error {
	if expected == 0 {
		return r.insert(ctx, id, s)
	}
	return r.update(ctx, id, s, expected)
}

func (r bookingRepository) insert(ctx context.Context, id domain.BookingID, s domain.BookingSnapshot) error {
	tag, err := r.conn.Exec(ctx, insertBooking, string(id), string(s.ResourceID), s.Quantity, int(s.Status), int64(s.ReservedAt))
	if err != nil {
		return fmt.Errorf("provider: insert booking %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ports.ErrVersionConflict
	}
	return nil
}

func (r bookingRepository) update(ctx context.Context, id domain.BookingID, s domain.BookingSnapshot, expected ports.Version) error {
	tag, err := r.conn.Exec(ctx, updateBooking, string(id), int64(expected), string(s.ResourceID), s.Quantity, int(s.Status), int64(s.ReservedAt))
	if err != nil {
		return fmt.Errorf("provider: update booking %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ports.ErrVersionConflict
	}
	return nil
}

func loadBooking(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, id domain.BookingID) (domain.BookingSnapshot, ports.Version, error) {
	var (
		version    int64
		resourceID string
		quantity   int
		status     int
		reservedAt int64
	)
	err := q.QueryRow(ctx, selectBooking, string(id)).Scan(&version, &resourceID, &quantity, &status, &reservedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.BookingSnapshot{}, 0, ports.ErrNotFound
	}
	if err != nil {
		return domain.BookingSnapshot{}, 0, fmt.Errorf("provider: load booking %s: %w", id, err)
	}
	return domain.BookingSnapshot{
		ID:         id,
		ResourceID: domain.ResourceID(resourceID),
		Quantity:   quantity,
		Status:     domain.BookingStatus(status),
		ReservedAt: domain.Instant(reservedAt),
	}, ports.Version(version), nil
}
