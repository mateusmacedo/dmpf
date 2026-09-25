package provider

import (
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// bookingTable declares how the Booking snapshot maps to typed columns. The
// statements, and with them the tenant predicate, are the kernel's: a read or
// write written here could not omit the scope even by mistake (IDN-14).
var bookingTable = postgres.Table[domain.BookingID, domain.BookingSnapshot]{
	Name:     "bookings",
	IDColumn: "booking_id",
	Columns:  []string{"resource_id", "quantity", "status", "reserved_at"},

	Encode: func(s domain.BookingSnapshot) ([]any, error) {
		return []any{string(s.ResourceID), s.Quantity, int(s.Status), int64(s.ReservedAt)}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.BookingSnapshot, error) {
		var (
			resourceID string
			quantity   int
			status     int
			reservedAt int64
		)
		if err := scan(&resourceID, &quantity, &status, &reservedAt); err != nil {
			return domain.BookingSnapshot{}, err
		}
		return domain.BookingSnapshot{
			ResourceID: domain.ResourceID(resourceID),
			Quantity:   quantity,
			Status:     domain.BookingStatus(status),
			ReservedAt: domain.Instant(reservedAt),
		}, nil
	},

	WithID: func(s domain.BookingSnapshot, id domain.BookingID) domain.BookingSnapshot {
		s.ID = id
		return s
	},
}

func NewBookingRepository(tx *postgres.Tx) ports.Repository[domain.BookingID, domain.BookingSnapshot] {
	return bookingTable.Repository(tx)
}
