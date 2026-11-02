// comment-discipline-ok-file: regras da porta de object storage replicadas do fake (memory/objectstore.go); cada godoc cita a norma (DAT-14, DAT-17, DAT-18, IDN-14 a IDN-20) que a regra realiza, e o kit providerkit.ObjectStore prova a equivalência.

// Package worm holds the rules of ports.ObjectStore that the S3 store and the
// administrative path share and that S3 does not enforce for the caller: the
// physical key of the declared scope, the dated retention under the ceiling,
// extension and promotion, and when a version may be purged.
package worm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ErrTenantUnresolved reports an operation of a tenant store whose carrier
// resolved no tenant, or one with a slash (IDN-15).
var ErrTenantUnresolved = errors.New("s3: the execution context resolved no tenant")

const resolution = ports.Instant(1_000_000_000)

// Physical is the key the bucket stores, built only from the carrier, so the
// reference of another tenant or scope cannot resolve (IDN-14); the tenant/
// segment keeps every tenant id, "platform" included, out of platform/ (IDN-19,
// IDN-20), and a tenant that is a dot segment is refused like one with a slash.
func Physical(ctx context.Context, scope ports.Scope, key ports.ObjectKey) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	execution, err := ports.RequireExecutionContext(ctx)
	if err != nil {
		return "", err
	}
	tenant, scoped := execution.Tenant()
	if scope == ports.ScopePlatform {
		if scoped {
			return "", ports.ErrScopeMismatch
		}
		return "platform/" + string(key), nil
	}
	if !scoped || strings.Contains(string(tenant), "/") || tenant == "." || tenant == ".." {
		return "", ErrTenantUnresolved
	}
	return "tenant/" + string(tenant) + "/" + string(key), nil
}

// Locate refuses, in the order the kit observes, a reference without version,
// an invalid key and a carrier the scope does not accept, before any I/O.
func Locate(ctx context.Context, scope ports.Scope, ref ports.ObjectRef) (string, error) {
	if ref.Version == "" {
		return "", ports.ErrVersionRequired
	}
	if err := ref.Key.Validate(); err != nil {
		return "", err
	}
	return Physical(ctx, scope, ref.Key)
}

// WholeSeconds drops the fraction S3 does not keep, so a caller comparing what
// it wrote compares the same instant on every store.
func WholeSeconds(r ports.Retention) ports.Retention {
	r.Until -= r.Until % resolution
	return r
}

// CheckRetention refuses a retention on a store without ceiling (DAT-17), of an
// unknown mode, not in the future, or above the ceiling, which is inclusive (DAT-14).
func CheckRetention(now ports.Instant, ceiling *ports.RetentionCeiling, r ports.Retention) error {
	if ceiling == nil {
		return ports.ErrRetentionCeilingUndeclared
	}
	if !r.Mode.Known() {
		return fmt.Errorf("%w: %q", ports.ErrRetentionModeUnknown, r.Mode)
	}
	if r.Until <= now {
		return ports.ErrRetentionUnbounded
	}
	if limit, ok := ceiling.Span.Add(now); ok && r.Until > limit {
		return ports.ErrRetentionAboveCeiling
	}
	return nil
}

// CheckChange allows, over a retention still in force, only extension in the
// same mode and promotion of GOVERNANCE to COMPLIANCE on a date no earlier; an
// expired retention leaves the version free to take any new one.
func CheckChange(now ports.Instant, current *ports.Retention, to ports.Retention) error {
	if current == nil || current.Until <= now {
		return nil
	}
	if current.Mode == ports.RetentionCompliance && to.Mode == ports.RetentionGovernance {
		return ports.ErrRetentionModeChange
	}
	promoted := current.Mode == ports.RetentionGovernance && to.Mode == ports.RetentionCompliance
	if to.Until < current.Until || (to.Until == current.Until && !promoted) {
		return ports.ErrRetentionNotExtended
	}
	return nil
}

// Purgeable refuses a version under legal hold or retention in force and gives
// the date its retention held it until, zero without one (DAT-18).
func Purgeable(now ports.Instant, current *ports.Retention, hold bool) (ports.Instant, error) {
	if hold {
		return 0, ports.ErrObjectLocked
	}
	if current == nil {
		return 0, nil
	}
	if current.Until > now {
		return 0, ports.ErrObjectLocked
	}
	return current.Until, nil
}
