package app_test

import (
	"reflect"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

var telemetrySubject = obstest.Subject[app.Config, app.Role]{
	Service:     "orders",
	Roles:       app.Roles,
	API:         app.RoleAPI,
	Defaults:    app.Defaults,
	FromEnv:     app.FromEnv,
	TelemetryOf: app.TelemetryOf,
	Scope:       reflect.TypeFor[app.Config]().PkgPath(),
}

func TestTheTelemetryManifest(t *testing.T) {
	obstest.Manifest[app.Config, app.Role]{
		Subject: telemetrySubject,
		EnvFile: "../deploy/.env.example",
		Project: "../project.json",
		Image:   "apps-backend-orders",
	}.Run(t)
}
