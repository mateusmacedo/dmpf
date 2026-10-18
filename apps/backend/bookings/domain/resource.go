package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type Resource struct {
	code         ResourceCode
	name         ResourceName
	registeredAt kernel.Instant
}

func NewResource(code ResourceCode) *Resource {
	return &Resource{code: code}
}

func FromResourceSnapshot(s ResourceSnapshot) *Resource {
	return &Resource{code: s.Code, name: s.Name, registeredAt: s.RegisteredAt}
}

type ResourceSnapshot struct {
	Code         ResourceCode
	Name         ResourceName
	RegisteredAt kernel.Instant
}

func (r *Resource) Snapshot() ResourceSnapshot {
	return ResourceSnapshot{Code: r.code, Name: r.name, RegisteredAt: r.registeredAt}
}

func (s ResourceSnapshot) Equal(other ResourceSnapshot) bool {
	return s.Code == other.Code && s.Name == other.Name && s.RegisteredAt == other.RegisteredAt
}

func (r *Resource) clone() Resource {
	return Resource{code: r.code, name: r.name, registeredAt: r.registeredAt}
}

func (r *Resource) Register(cmd RegisterResource) (kernel.Accepted[RegisteredResponse], *kernel.Rejection) {
	return kernel.DecideOver(r, (*Resource).clone, func(next *Resource) (kernel.Accepted[RegisteredResponse], *kernel.Rejection) {
		if cmd.Code == "" {
			return kernel.Refuse[RegisteredResponse](CodeResourceCodeEmpty, "code must not be empty")
		}
		if cmd.Name == "" {
			return kernel.Refuse[RegisteredResponse](CodeResourceNameEmpty, "name must not be empty")
		}
		// WHY: spec says "when present: accept without changing or emitting".
		if next.registeredAt != 0 {
			return kernel.Accept(RegisteredResponse{Code: next.code}), nil
		}
		next.name = cmd.Name
		next.registeredAt = cmd.At
		return kernel.Accept(
			RegisteredResponse{Code: next.code},
			ResourceRegistered{Code: next.code, Name: next.name, At: cmd.At},
		), nil
	})
}
