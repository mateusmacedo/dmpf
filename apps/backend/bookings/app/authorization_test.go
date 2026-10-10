package app

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

// IDN-16: every operation of this context declares the permission it requires;
// an operation added without one would be denied (IDN-17), and this table is
// what names the omission first.
func TestEveryOperationDeclaresItsPermission(t *testing.T) {
	cases := map[string]struct {
		op   application.Operation
		want ports.Permission
	}{
		"Reserve":               {application.ReserveBooking{}, "bookings:write"},
		"Cancel":                {application.CancelBooking{}, "bookings:write"},
		"Register":              {application.RegisterResource{}, "bookings:write"},
		"FindBooking":           {application.FindBooking{}, "bookings:read"},
		"FindBookingByResource": {application.FindBookingByResource{}, "bookings:read"},
	}
	for name, c := range cases {
		if got := permissionOf(c.op); got != c.want {
			t.Errorf("%s: permissionOf() = %q, want %q", name, got, c.want)
		}
	}
}

func TestAuthorizationDeniesASubjectWithoutThePermission(t *testing.T) {
	ctx := execctx.WithExecution(t, context.Background())

	if err := Authorization()(ctx, application.FindBooking{}); !errors.Is(err, ports.ErrDenied) {
		t.Fatalf("Authorization() = %v, want ErrDenied (IDN-08)", err)
	}
}
