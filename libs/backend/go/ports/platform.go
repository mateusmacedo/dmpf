package ports

import (
	"context"
	"errors"
)

// The zero value is ReachUndeclared, so an unset reach never widens access (IDN-19).
type PlatformReach uint8

const (
	ReachUndeclared PlatformReach = iota
	ReachAllTenants
)

const PlatformPageMax = 500

// All-tenant reach is never inferred (IDN-19, IDN-20); not to be confused with
// http.ErrPlatformReachRequired, a route declaration error.
var ErrPlatformReachUndeclared = errors.New("ports: platform reach not declared (IDN-19)")

var ErrPlatformLimitInvalid = errors.New("ports: platform page limit out of range")

// Fail closed: an access whose record the trail did not confirm returns and
// writes nothing (DAT-25).
var ErrAuditUnavailable = errors.New("ports: audit trail did not confirm the record (DAT-25)")

// Also the answer to the origin's own tenant and to an absent policy: the
// default is closed (IDN-17).
var ErrProjectionDenied = errors.New("ports: projection denied by the destination policy (IDN-17)")

var ErrProjectionDestinationAbsent = errors.New("ports: projection destination tenant absent")

type platformReachKey struct{}

// Kept apart from the ExecutionContext: CTX-01 fixes its nine fields, and the
// reach is not one.
func WithPlatformReach(ctx context.Context, reach PlatformReach) context.Context {
	return context.WithValue(ctx, platformReachKey{}, reach)
}

func PlatformReachFrom(ctx context.Context) (PlatformReach, bool) {
	reach, ok := ctx.Value(platformReachKey{}).(PlatformReach)
	return reach, ok
}

// The identity alone is unique only within a tenant.
type PlatformKey[ID comparable] struct {
	Tenant TenantID
	ID     ID
}

type PlatformRow[ID comparable, S any] struct {
	Tenant  TenantID
	ID      ID
	State   S
	Version Version
}

// A nil after asks List for the first page.
type PlatformReader[ID comparable, S any] interface {
	LoadIn(ctx context.Context, tenant TenantID, id ID) (PlatformRow[ID, S], error)
	List(ctx context.Context, after *PlatformKey[ID], limit int) ([]PlatformRow[ID, S], error)
}

// Save keeps the optimistic-locking semantics of Repository.Save.
type Projector[ID comparable, S any] interface {
	Load(ctx context.Context, destination TenantID, id ID) (S, Version, error)
	Save(ctx context.Context, destination TenantID, id ID, state S, expected Version) error
}

// origin is nil when the carrier holds no tenant, and the origin's own tenant
// never reaches it, because the Projector refuses it.
type ProjectionPolicy func(ctx context.Context, origin *TenantID, destination TenantID) error

const (
	AuditPlatformRead    = "platform.cross_tenant_read"
	AuditProjectionWrite = "projection.cross_tenant_write"
)

type AuditRecord struct {
	Action     string
	Subject    string
	Object     string
	Outcome    string
	Tenant     string
	DataTenant string
}

// A record it did not confirm must satisfy errors.Is(err, ErrAuditUnavailable),
// so the access fails closed before proceeding (DAT-25).
type AuditTrail interface {
	Record(ctx context.Context, r AuditRecord) error
}
