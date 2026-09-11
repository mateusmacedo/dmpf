package dmpfobservability_test

import (
	"os"
	"strings"
	"testing"

	dmpfobservability "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	_ "go.opentelemetry.io/otel"
	_ "go.opentelemetry.io/otel/metric"
	_ "go.opentelemetry.io/otel/sdk"
	_ "go.opentelemetry.io/otel/sdk/metric"
	_ "go.opentelemetry.io/otel/trace"
)

// WHY: the pin is read from go.mod, not from debug.ReadBuildInfo. Under a
// go.work the Go toolchain builds the test binary in workspace mode and leaves
// BuildInfo.Deps empty (measured: deps=0), so ReadBuildInfo proves nothing here.
func TestEveryOTelRequirementMatchesTheDeclaredVersion(t *testing.T) {
	manifest, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("ReadFile(go.mod) = %v, want nil", err)
	}

	want := "v" + dmpfobservability.OTelVersion
	found := 0
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasPrefix(fields[0], "go.opentelemetry.io/otel") {
			continue
		}
		found++
		if fields[1] != want {
			t.Errorf("%s = %s, want %s: the platform pins one OTel version", fields[0], fields[1], want)
		}
	}
	if found == 0 {
		t.Fatal("go.mod requires no go.opentelemetry.io/otel module, so the pin proves nothing")
	}
}

func TestSemconvVersionMatchesTheImportedSchema(t *testing.T) {
	want := "https://opentelemetry.io/schemas/" + dmpfobservability.SemconvVersion

	if semconv.SchemaURL != want {
		t.Fatalf("semconv.SchemaURL = %q, want %q: the declared version must be the imported one", semconv.SchemaURL, want)
	}
}
