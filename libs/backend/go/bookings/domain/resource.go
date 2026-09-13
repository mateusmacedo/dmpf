package bookingsdomain

import dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"

type Resource struct {
	code         ResourceCode
	registeredAt Instant
}

func NewResource(code ResourceCode) *Resource {
	return &Resource{code: code}
}

func FromResourceSnapshot(s ResourceSnapshot) *Resource {
	return &Resource{code: s.Code, registeredAt: s.RegisteredAt}
}

type ResourceSnapshot struct {
	Code         ResourceCode
	RegisteredAt Instant
}

func (r *Resource) Snapshot() ResourceSnapshot {
	return ResourceSnapshot{Code: r.code, RegisteredAt: r.registeredAt}
}

func (s ResourceSnapshot) Equal(other ResourceSnapshot) bool {
	return s.Code == other.Code && s.RegisteredAt == other.RegisteredAt
}

func (r *Resource) clone() Resource {
	return Resource{code: r.code, registeredAt: r.registeredAt}
}

func (r *Resource) Register(cmd RegisterResource) (dmpfdomain.Accepted[RegisteredResponse], *dmpfdomain.Rejection) {
	next := r.clone()
	if cmd.Code == "" {
		return dmpfdomain.Accepted[RegisteredResponse]{}, dmpfdomain.Reject(CodeCodeEmpty, "code must not be empty")
	}
	// WHY: spec says "when present: accept without changing or emitting".
	if next.registeredAt != 0 {
		return dmpfdomain.Accept(RegisteredResponse{Code: r.code}), nil
	}
	next.registeredAt = cmd.At
	*r = next
	return dmpfdomain.Accept(
		RegisteredResponse{Code: r.code},
		ResourceRegistered{Code: r.code, At: cmd.At},
	), nil
}
