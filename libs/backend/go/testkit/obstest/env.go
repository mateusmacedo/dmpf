package obstest

import (
	"os"
	"strings"
	"testing"
)

func ReadEnvFile(t testing.TB, path string) map[string]string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	values := map[string]string{}
	for line := range strings.Lines(string(content)) {
		line = strings.TrimSpace(line)
		if key, value, ok := strings.Cut(line, "="); ok && !strings.HasPrefix(line, "#") {
			values[key] = value
		}
	}
	return values
}

// UseOnlyTheOTelEnvironmentOf clears every OTEL_* the process inherited before
// setting those of env, so a variable of the machine never reaches the SDK.
func UseOnlyTheOTelEnvironmentOf(t *testing.T, env map[string]string) {
	t.Helper()
	for _, pair := range os.Environ() {
		if key, _, _ := strings.Cut(pair, "="); strings.HasPrefix(key, "OTEL_") {
			t.Setenv(key, "")
			_ = os.Unsetenv(key)
		}
	}
	for key, value := range env {
		if strings.HasPrefix(key, "OTEL_") {
			t.Setenv(key, value)
		}
	}
}
