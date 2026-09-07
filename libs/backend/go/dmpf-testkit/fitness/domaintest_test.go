package fitness_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	conffit "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/fitness"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
)

// V29/V30 (PIR-17): a test of a domain unit that needs an infrastructure double
// names the coupling of the SUT. The closure of the unit's test packages may
// not reach a port or provider unit, and their direct imports may not carry a
// capability the domain block lacks; testing itself is the instrument and is
// exempt. Test files are outside the checked universe, which is exactly why the
// suite, not the gate, has to look at them.
func TestDomainTestsNeedNoInfrastructureDouble(t *testing.T) {
	root := tb.RepoRoot(t)
	in, err := conffit.Workspace(root, "")
	if err != nil {
		t.Fatalf("Workspace: %v", err)
	}
	units, err := conffit.Units(in)
	if err != nil {
		t.Fatalf("Units: %v", err)
	}
	for _, v := range domainTestViolations(t, root, nil, units) {
		t.Errorf("%s", v)
	}
}

// The negative vector: a synthetic module whose domain unit has a test that
// imports the module's own port unit.
func TestDomainTestImportingAPortIsNamed(t *testing.T) {
	dir, err := filepath.Abs("testdata/domaintest")
	if err != nil {
		t.Fatal(err)
	}
	const mod = "exemplo.test/domaintest"
	units := []conffit.Unit{
		{ID: "dt/domain", Block: conffit.BlockDomain, BoundedContext: "dt", Include: []string{mod + "/domain"}, Module: mod},
		{ID: "dt/port", Block: conffit.BlockPort, BoundedContext: "dt", Include: []string{mod + "/port"}, Module: mod},
	}
	got := domainTestViolations(t, dir, []string{"GOWORK=off"}, units)
	if len(got) != 1 {
		t.Fatalf("violations = %v, want exactly one", got)
	}
	if !strings.Contains(got[0].String(), mod+"/port") || got[0].Unit != "dt/domain" {
		t.Fatalf("violation does not name the port import against the domain unit: %v", got[0])
	}
}

type violation struct {
	Unit    string
	Package string
	Import  string
	Reason  string
}

func (v violation) String() string {
	return fmt.Sprintf("unidade %s: teste de %s importa %s (%s)", v.Unit, v.Package, v.Import, v.Reason)
}

type listedPackage struct {
	ImportPath string
	ForTest    string
	Imports    []string
	Deps       []string
}

func domainTestViolations(t *testing.T, dir string, env []string, units []conffit.Unit) []violation {
	t.Helper()
	blockOf := map[string]conffit.Unit{}
	for _, u := range units {
		for _, inc := range u.Include {
			blockOf[inc] = u
		}
	}
	var out []violation
	for _, u := range units {
		if u.Block != conffit.BlockDomain {
			continue
		}
		for _, pkg := range goListTest(t, dir, env, u.Include) {
			if pkg.ForTest == "" {
				continue
			}
			for _, imp := range pkg.Imports {
				imp = canonical(imp)
				if imp == "testing" || strings.HasPrefix(imp, "testing/") {
					continue
				}
				if owner, ok := blockOf[imp]; ok {
					if owner.Block == conffit.BlockPort || owner.Block == conffit.BlockProvider {
						out = append(out, violation{u.ID, pkg.ForTest, imp, "duplo de infraestrutura: unidade " + owner.ID + " é " + string(owner.Block)})
					}
					continue
				}
				if c, known := conffit.StandardCapability(imp); known && c != conffit.CapPure {
					out = append(out, violation{u.ID, pkg.ForTest, imp, "capability " + string(c) + " fora do bloco domain"})
				}
			}
			for _, dep := range pkg.Deps {
				dep = canonical(dep)
				owner, ok := blockOf[dep]
				if !ok || slices.ContainsFunc(pkg.Imports, func(i string) bool { return canonical(i) == dep }) {
					continue
				}
				if owner.Block == conffit.BlockPort || owner.Block == conffit.BlockProvider {
					out = append(out, violation{u.ID, pkg.ForTest, dep, "alcançado transitivamente: unidade " + owner.ID + " é " + string(owner.Block)})
				}
			}
		}
	}
	slices.SortFunc(out, func(a, b violation) int { return strings.Compare(a.String(), b.String()) })
	return slices.Compact(out)
}

// A package recompiled for a test binary is listed as "path [pkg.test]"; the
// suffix is build identity, not import identity.
func canonical(importPath string) string {
	if i := strings.Index(importPath, " ["); i >= 0 {
		return importPath[:i]
	}
	return importPath
}

// go list -test lists, next to each package, its test variants (ForTest set):
// the internal test build of the package and the external _test package. Their
// Imports are what the test files add; Deps is the transitive closure.
func goListTest(t *testing.T, dir string, env []string, patterns []string) []listedPackage {
	t.Helper()
	args := append([]string{"list", "-test", "-json=ImportPath,ForTest,Imports,Deps"}, patterns...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(cmd.Environ(), env...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -test %v in %s: %v\n%s", patterns, dir, err, stderr.String())
	}
	var out []listedPackage
	dec := json.NewDecoder(bytes.NewReader(raw))
	for {
		var p listedPackage
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("decoding go list output: %v", err)
		}
		out = append(out, p)
	}
	return out
}
