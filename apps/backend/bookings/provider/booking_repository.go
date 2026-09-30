package provider

import (
	"encoding/json"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// bookingState is the snapshot column: the fields no query filters by, under
// JSON names fixed here so a rename in the domain does not rewrite stored rows.
type bookingState struct {
	Quantity   int   `json:"quantity"`
	Status     int   `json:"status"`
	ReservedAt int64 `json:"reservedAt"`
}

// bookingTable declares how the Booking snapshot maps to columns. The
// statements, and with them the tenant predicate, are the kernel's: a read or
// write written here could not omit the scope even by mistake (IDN-14).
var bookingTable = postgres.Table[domain.BookingID, domain.BookingSnapshot]{
	Name:     "bookings",
	IDColumn: "booking_id",
	Columns:  []string{"resource_id", "snapshot"},

	Encode: func(s domain.BookingSnapshot) ([]any, error) {
		raw, err := json.Marshal(bookingState{Quantity: s.Quantity, Status: int(s.Status), ReservedAt: int64(s.ReservedAt)})
		if err != nil {
			return nil, err
		}
		return []any{string(s.ResourceID), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.BookingSnapshot, error) {
		var (
			resourceID string
			raw        []byte
		)
		if err := scan(&resourceID, &raw); err != nil {
			return domain.BookingSnapshot{}, err
		}
		var state bookingState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.BookingSnapshot{}, err
		}
		return domain.BookingSnapshot{
			ResourceID: domain.ResourceID(resourceID),
			Quantity:   state.Quantity,
			Status:     domain.BookingStatus(state.Status),
			ReservedAt: domain.Instant(state.ReservedAt),
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
