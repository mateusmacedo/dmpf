//go:build integration && distributed

package dist_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
)

var externalSpec = dist.Spec{
	Name:  "external",
	Pool:  dist.ExternalAttemptPool,
	Roles: map[dist.Role]dist.RoleFunc{dist.RoleExternalCrash: dist.SubmitThatNeverReturns},
}

func TestDistkitRole(t *testing.T) { dist.RunRole(t, externalSpec) }

func TestACrashBetweenBeginAndFinishIsSweptAsUnknownSentAndNeverResent(t *testing.T) {
	h := dist.NewPostgres(t, externalSpec)

	tb.Require(t, dist.CrashBetweenBeginAndFinishVector(t, h))
}
