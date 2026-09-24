package admission

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
)

// DefaultMaxKeys is the ceiling on the cardinality of the bucket key, which
// is what keeps a hostile caller from minting one bucket per request
// (RES-16, RES-17).
const DefaultMaxKeys = 64

// NewController builds the controller over the system clock. tenants is the
// allowlist that keeps its own bucket and label (MET-07); every other tenant
// shares "other", and an empty list is a valid declaration, never one to fill.
func NewController(limits map[string]Limit, tenants []string, maxKeys int) (*Controller, error) {
	var declared metrics.Tenants
	if len(tenants) > 0 {
		var err error
		if declared, err = metrics.DeclareTenants(tenants...); err != nil {
			return nil, err
		}
	}
	return New(Config{
		Limits:  limits,
		Tenants: declared,
		MaxKeys: maxKeys,
		Clock:   clock.System(),
	})
}
