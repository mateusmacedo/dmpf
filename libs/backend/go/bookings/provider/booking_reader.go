package provider

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func NewBookingReader(pool *pgxpool.Pool) ports.Reader[domain.BookingID, domain.BookingSnapshot] {
	return bookingReader{pool: pool}
}

type bookingReader struct{ pool *pgxpool.Pool }

func (r bookingReader) Load(ctx context.Context, id domain.BookingID) (domain.BookingSnapshot, ports.Version, error) {
	return loadBooking(ctx, r.pool, id)
}

const selectBookingsByResource = `SELECT booking_id, version, resource_id, quantity, status, reserved_at FROM bookings_booking WHERE resource_id = $1`

type BookingsByResourceReader struct{ pool *pgxpool.Pool }

func NewBookingsByResourceReader(pool *pgxpool.Pool) *BookingsByResourceReader {
	return &BookingsByResourceReader{pool: pool}
}

func (r *BookingsByResourceReader) LoadByResource(ctx context.Context, resourceID domain.ResourceID) ([]domain.BookingSnapshot, error) {
	rows, err := r.pool.Query(ctx, selectBookingsByResource, string(resourceID))
	if err != nil {
		return nil, fmt.Errorf("provider: find by resource %s: %w", resourceID, err)
	}
	defer rows.Close()

	var result []domain.BookingSnapshot
	for rows.Next() {
		var (
			bookingID  string
			version    int64
			resID      string
			quantity   int
			status     int
			reservedAt int64
		)
		if err := rows.Scan(&bookingID, &version, &resID, &quantity, &status, &reservedAt); err != nil {
			return nil, fmt.Errorf("provider: scan by resource %s: %w", resourceID, err)
		}
		result = append(result, domain.BookingSnapshot{
			ID:         domain.BookingID(bookingID),
			ResourceID: domain.ResourceID(resID),
			Quantity:   quantity,
			Status:     domain.BookingStatus(status),
			ReservedAt: domain.Instant(reservedAt),
		})
		_ = ports.Version(version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("provider: rows by resource %s: %w", resourceID, err)
	}
	return result, nil
}

// compile-time check
var _ interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
} = (*pgxpool.Pool)(nil)
