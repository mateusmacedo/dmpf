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
	selectBooking = `SELECT version, resource_id, quantity, status, reserved_at FROM bookings_booking WHERE booking_id = $1`

	insertBooking = `
		INSERT INTO bookings_booking (booking_id, version, resource_id, quantity, status, reserved_at)
		VALUES ($1, 1, $2, $3, $4, $5)
		ON CONFLICT (booking_id) DO NOTHING`

	updateBooking = `
		UPDATE bookings_booking SET version = $2 + 1, resource_id = $3, quantity = $4, status = $5, reserved_at = $6
		WHERE booking_id = $1 AND version = $2`
)

func NewBookingRepository(tx *dmpfpostgres.Tx) dmpfports.Repository[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot] {
	return bookingRepository{conn: tx.Conn()}
}

type bookingRepository struct{ conn pgx.Tx }

func (r bookingRepository) Load(ctx context.Context, id bookingsdomain.BookingID) (bookingsdomain.BookingSnapshot, dmpfports.Version, error) {
	return loadBooking(ctx, r.conn, id)
}

func (r bookingRepository) Save(ctx context.Context, id bookingsdomain.BookingID, s bookingsdomain.BookingSnapshot, expected dmpfports.Version) error {
	if expected == 0 {
		return r.insert(ctx, id, s)
	}
	return r.update(ctx, id, s, expected)
}

func (r bookingRepository) insert(ctx context.Context, id bookingsdomain.BookingID, s bookingsdomain.BookingSnapshot) error {
	tag, err := r.conn.Exec(ctx, insertBooking, string(id), string(s.ResourceID), s.Quantity, int(s.Status), int64(s.ReservedAt))
	if err != nil {
		return fmt.Errorf("bookingspostgres: insert booking %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return dmpfports.ErrVersionConflict
	}
	return nil
}

func (r bookingRepository) update(ctx context.Context, id bookingsdomain.BookingID, s bookingsdomain.BookingSnapshot, expected dmpfports.Version) error {
	tag, err := r.conn.Exec(ctx, updateBooking, string(id), int64(expected), string(s.ResourceID), s.Quantity, int(s.Status), int64(s.ReservedAt))
	if err != nil {
		return fmt.Errorf("bookingspostgres: update booking %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return dmpfports.ErrVersionConflict
	}
	return nil
}

func loadBooking(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, id bookingsdomain.BookingID) (bookingsdomain.BookingSnapshot, dmpfports.Version, error) {
	var (
		version    int64
		resourceID string
		quantity   int
		status     int
		reservedAt int64
	)
	err := q.QueryRow(ctx, selectBooking, string(id)).Scan(&version, &resourceID, &quantity, &status, &reservedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return bookingsdomain.BookingSnapshot{}, 0, dmpfports.ErrNotFound
	}
	if err != nil {
		return bookingsdomain.BookingSnapshot{}, 0, fmt.Errorf("bookingspostgres: load booking %s: %w", id, err)
	}
	return bookingsdomain.BookingSnapshot{
		ID:         id,
		ResourceID: bookingsdomain.ResourceID(resourceID),
		Quantity:   quantity,
		Status:     bookingsdomain.BookingStatus(status),
		ReservedAt: bookingsdomain.Instant(reservedAt),
	}, dmpfports.Version(version), nil
}
