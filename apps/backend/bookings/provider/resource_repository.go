package provider

import (
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// resourceTable declares how the Resource snapshot maps to typed columns, under
// the same kernel statements as bookingTable (IDN-14).
var resourceTable = postgres.Table[domain.ResourceCode, domain.ResourceSnapshot]{
	Name:     "resources",
	IDColumn: "resource_id",
	Columns:  []string{"registered_at"},

	Encode: func(s domain.ResourceSnapshot) ([]any, error) {
		return []any{int64(s.RegisteredAt)}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.ResourceSnapshot, error) {
		var registeredAt int64
		if err := scan(&registeredAt); err != nil {
			return domain.ResourceSnapshot{}, err
		}
		return domain.ResourceSnapshot{RegisteredAt: domain.Instant(registeredAt)}, nil
	},

	WithID: func(s domain.ResourceSnapshot, code domain.ResourceCode) domain.ResourceSnapshot {
		s.Code = code
		return s
	},
}

func NewResourceRepository(tx *postgres.Tx) ports.Repository[domain.ResourceCode, domain.ResourceSnapshot] {
	return resourceTable.Repository(tx)
}
