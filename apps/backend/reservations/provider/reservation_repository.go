package provider

import (
	"encoding/json"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// reservationState is the snapshot column. The JSON names are fixed here so a
// rename in the domain does not rewrite stored rows; the order is the key.
type reservationState struct {
	Items  int `json:"items"`
	Status int `json:"status"`
}

// reservationTable declares how the Reservation snapshot maps to columns,
// under the kernel statements and their tenant predicate (IDN-14). No query
// filters a reservation by anything but its order, so the whole state is the
// snapshot.
var reservationTable = postgres.Table[domain.OrderID, domain.Snapshot]{
	Name:     "reservations",
	IDColumn: "order_id",
	Columns:  []string{"snapshot"},

	Encode: func(s domain.Snapshot) ([]any, error) {
		raw, err := json.Marshal(reservationState{Items: s.Items, Status: int(s.Status)})
		if err != nil {
			return nil, err
		}
		return []any{raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.Snapshot, error) {
		var raw []byte
		if err := scan(&raw); err != nil {
			return domain.Snapshot{}, err
		}
		var state reservationState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.Snapshot{}, err
		}
		return domain.Snapshot{Items: state.Items, Status: domain.Status(state.Status)}, nil
	},

	WithID: func(s domain.Snapshot, order domain.OrderID) domain.Snapshot {
		s.Order = order
		return s
	},
}

// NewReservationRepository binds the repository to an open transaction, because every read
// and write of the aggregate has to run on the same transaction as the outbox
// row (UOW-01).
func NewReservationRepository(tx *postgres.Tx) ports.Repository[domain.OrderID, domain.Snapshot] {
	return reservationTable.Repository(tx)
}
