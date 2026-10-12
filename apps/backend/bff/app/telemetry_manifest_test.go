package app_test

import (
	"reflect"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

const manifestPath = "../deploy/.env.example"

var edgeTelemetry = obstest.Subject[app.Config, string]{
	Service:     "bff",
	Roles:       []string{"api"},
	API:         "api",
	Defaults:    func(string) app.Config { return app.Defaults() },
	FromEnv:     func(_ string, getenv func(string) string) (app.Config, error) { return app.FromEnv(getenv) },
	TelemetryOf: app.TelemetryOf,
	Scope:       reflect.TypeFor[app.Config]().PkgPath(),
}

func TestTheTelemetryManifest(t *testing.T) {
	obstest.Manifest[app.Config, string]{Subject: edgeTelemetry, EnvFile: manifestPath}.Run(t)
}
