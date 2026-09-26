package provider

import (
	"encoding/json"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

type resourceState struct {
	RegisteredAt int64 `json:"registeredAt"`
}

// resourceTable declares how the Resource snapshot maps to columns, under the
// same kernel statements as bookingTable (IDN-14). No query filters a resource
// by anything but its id, so the whole state is the snapshot.
var resourceTable = postgres.Table[domain.ResourceCode, domain.ResourceSnapshot]{
	Name:     "resources",
	IDColumn: "resource_id",
	Columns:  []string{"snapshot"},

	Encode: func(s domain.ResourceSnapshot) ([]any, error) {
		raw, err := json.Marshal(resourceState{RegisteredAt: int64(s.RegisteredAt)})
		if err != nil {
			return nil, err
		}
		return []any{raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.ResourceSnapshot, error) {
		var raw []byte
		if err := scan(&raw); err != nil {
			return domain.ResourceSnapshot{}, err
		}
		var state resourceState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.ResourceSnapshot{}, err
		}
		return domain.ResourceSnapshot{RegisteredAt: domain.Instant(state.RegisteredAt)}, nil
	},

	WithID: func(s domain.ResourceSnapshot, code domain.ResourceCode) domain.ResourceSnapshot {
		s.Code = code
		return s
	},
}

func NewResourceRepository(tx *postgres.Tx) ports.Repository[domain.ResourceCode, domain.ResourceSnapshot] {
	return resourceTable.Repository(tx)
}
