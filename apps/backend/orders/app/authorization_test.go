package app

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// IDN-16: every operation of this context declares the permission it requires;
// an operation added without one would be denied (IDN-17), and this table is
// what names the omission first.
func TestEveryOperationDeclaresItsPermission(t *testing.T) {
	cases := map[string]struct {
		op   application.Operation
		want ports.Permission
	}{
		"AddItem":    {application.AddItem{}, "orders:write"},
		"PlaceOrder": {application.PlaceOrder{}, "orders:write"},
		"FindOrder":  {application.FindOrder{}, "orders:read"},
	}
	for name, c := range cases {
		if got := permissionOf(c.op); got != c.want {
			t.Errorf("%s: permissionOf() = %q, want %q", name, got, c.want)
		}
	}
}

func TestAuthorizationDeniesASubjectWithoutThePermission(t *testing.T) {
	subject, tenant := ports.SubjectID("s-1"), ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1",
		Subject: &subject, Tenant: &tenant, Permissions: []ports.Permission{},
		Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	ctx := ports.WithExecutionContext(context.Background(), execution)

	if err := Authorization()(ctx, application.FindOrder{}); !errors.Is(err, ports.ErrDenied) {
		t.Fatalf("Authorization() = %v, want ErrDenied (IDN-08)", err)
	}
}
