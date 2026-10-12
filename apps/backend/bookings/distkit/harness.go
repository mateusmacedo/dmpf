//go:build integration && distributed

package distkit

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/appkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
)

var Spec = dist.Spec{
	Name: "bookings",
	Pool: appkit.PoolOptions,
	Roles: map[dist.Role]dist.RoleFunc{dist.RoleRelay: dist.Relay(func(ctx context.Context, getenv func(string) string) error {
		cfg, err := app.FromEnv(app.RoleRelay, getenv)
		if err != nil {
			return err
		}
		return app.Run(ctx, cfg)
	})},
}

func RunRole(t *testing.T) { dist.RunRole(t, Spec) }
