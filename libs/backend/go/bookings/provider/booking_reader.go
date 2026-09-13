package bookingspostgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

func NewBookingReader(pool *pgxpool.Pool) dmpfports.Reader[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot] {
	return bookingReader{pool: pool}
}

type bookingReader struct{ pool *pgxpool.Pool }

func (r bookingReader) Load(ctx context.Context, id bookingsdomain.BookingID) (bookingsdomain.BookingSnapshot, dmpfports.Version, error) {
	return loadBooking(ctx, r.pool, id)
}

const selectBookingsByResource = `SELECT booking_id, version, resource_id, quantity, status, reserved_at FROM bookings_booking WHERE resource_id = $1`

type BookingsByResourceReader struct{ pool *pgxpool.Pool }

func NewBookingsByResourceReader(pool *pgxpool.Pool) *BookingsByResourceReader {
	return &BookingsByResourceReader{pool: pool}
}

func (r *BookingsByResourceReader) LoadByResource(ctx context.Context, resourceID bookingsdomain.ResourceID) ([]bookingsdomain.BookingSnapshot, error) {
	rows, err := r.pool.Query(ctx, selectBookingsByResource, string(resourceID))
	if err != nil {
		return nil, fmt.Errorf("bookingspostgres: find by resource %s: %w", resourceID, err)
	}
	defer rows.Close()

	var result []bookingsdomain.BookingSnapshot
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
			return nil, fmt.Errorf("bookingspostgres: scan by resource %s: %w", resourceID, err)
		}
		result = append(result, bookingsdomain.BookingSnapshot{
			ID:         bookingsdomain.BookingID(bookingID),
			ResourceID: bookingsdomain.ResourceID(resID),
			Quantity:   quantity,
			Status:     bookingsdomain.BookingStatus(status),
			ReservedAt: bookingsdomain.Instant(reservedAt),
		})
		_ = dmpfports.Version(version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bookingspostgres: rows by resource %s: %w", resourceID, err)
	}
	return result, nil
}

// compile-time check
var _ interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
} = (*pgxpool.Pool)(nil)
