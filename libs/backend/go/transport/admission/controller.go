package admission

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
)

// DefaultMaxKeys is the ceiling on the cardinality of the bucket key, which
// is what keeps a hostile caller from minting one bucket per request
// (RES-16, RES-17).
const DefaultMaxKeys = 64

// NewController declares the tenancy of the process and builds the controller
// over the system clock. The tenant is a parameter because the bucket is per
// route and per tenant, and a process states its tenancy at startup.
func NewController(limits map[string]Limit, tenant string, maxKeys int) (*Controller, error) {
	tenants, err := metrics.DeclareTenants(tenant)
	if err != nil {
		return nil, err
	}
	return New(Config{
		Limits:  limits,
		Tenants: tenants,
		MaxKeys: maxKeys,
		Clock:   clock.System(),
	})
}
