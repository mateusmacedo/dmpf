package admission_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

func TestAControllerWithoutDeclaredTenantsLabelsEveryTenantOther(t *testing.T) {
	ctrl, err := admission.NewController(map[string]admission.Limit{"r": {PerSecond: 1, Burst: 1, Concurrency: 1}}, nil, admission.DefaultMaxKeys)
	if err != nil {
		t.Fatalf("NewController() without tenants = %v, want nil: no declaration is a valid tenancy, never a reason to invent one (IDN-20)", err)
	}
	if got := ctrl.Tenants().Resolve("acme"); got != metrics.OtherTenant {
		t.Fatalf("Resolve(acme) = %q, want %q", got, metrics.OtherTenant)
	}
}

func TestAControllerLabelsTheTenantsItDeclares(t *testing.T) {
	ctrl, err := admission.NewController(map[string]admission.Limit{"r": {PerSecond: 1, Burst: 1, Concurrency: 1}}, []string{"acme"}, admission.DefaultMaxKeys)
	if err != nil {
		t.Fatalf("NewController() = %v", err)
	}
	if got := ctrl.Tenants().Resolve("acme"); got != "acme" {
		t.Fatalf("Resolve(acme) = %q, want acme", got)
	}
	if got := ctrl.Tenants().Resolve("globex"); got != metrics.OtherTenant {
		t.Fatalf("Resolve(globex) = %q, want %q", got, metrics.OtherTenant)
	}
}
