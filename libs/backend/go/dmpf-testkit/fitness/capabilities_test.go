package fitness_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// The domain and contract kits are production code to the checker (FIT-01), so
// they may only import what their block's capability policy allows: pure for
// domain, pure and wire.codec for contract (rule/capability.go). Anything the
// checker classifies as observability (testing), io.filesystem (os), io.clock
// (time) or — for domain — wire.codec (encoding/json) is a forbidden edge. Like
// DMPF-E001 this looks at direct imports: what a declared wire.codec dependency
// reaches on its own is that dependency's contract, not the kit's.
func TestKitPackagesRespectBlockCapabilities(t *testing.T) {
	cases := []struct {
		pkg       string
		forbidden []string
	}{
		{"../domainkit", []string{"testing", "os", "time", "encoding/json"}},
		{"../golden", []string{"testing", "os", "time"}},
	}
	for _, tc := range cases {
		t.Run(strings.TrimPrefix(tc.pkg, "../"), func(t *testing.T) {
			deps := directImports(t, tc.pkg)
			for _, f := range tc.forbidden {
				if slices.Contains(deps, f) {
					t.Errorf("%s reaches %q, which its block does not allow", tc.pkg, f)
				}
			}
		})
	}
}

func directImports(t *testing.T, pkg string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list %s: %v\n%s", pkg, err, out)
	}
	return strings.Fields(string(out))
}
