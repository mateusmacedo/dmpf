package evidence

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const Schema = "dmpf/evidence@1"

type Header struct {
	Schema    string            `json:"schema"`
	Release   string            `json:"release"`
	Subject   string            `json:"subject"`
	Commit    string            `json:"commit"`
	GoVersion string            `json:"goversion"`
	Tags      []string          `json:"tags"`
	Packages  []string          `json:"packages"`
	Modules   []Module          `json:"modules"`
	Externals []External        `json:"externals"`
	Infra     map[string]string `json:"infra,omitempty"`
	Tools     []Tool            `json:"tools,omitempty"`
}

type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type External struct {
	Package string `json:"package"`
	Version string `json:"version"`
}

type Tool struct {
	Identity string `json:"identity"`
	Version  string `json:"version"`
}

var goLine = regexp.MustCompile(`(?m)^go\s+(\S+)\s*$`)

func GoVersion(goWork []byte, toolchain string) (string, error) {
	m := goLine.FindSubmatch(goWork)
	if m == nil {
		return "", errors.New("go.work declares no go line")
	}
	declared := string(m[1])
	running := strings.TrimPrefix(strings.Fields(toolchain + " ")[0], "go")
	if running != declared {
		return "", fmt.Errorf("toolchain go%s differs from go.work go %s: run with GOTOOLCHAIN=go%s", running, declared, declared)
	}
	return declared, nil
}

type listedPackage struct {
	Standard bool
	Module   *listedModule
}

type listedModule struct {
	Path    string
	Version string
	Main    bool
	Dir     string
	Replace *listedModule
}

// Reach reads what a subject's test binaries link. A workspace module has no Go
// version of its own, so its version is the package.json one, the same the BOM
// declares for it.
func Reach(ctx context.Context, root string, tags, packages []string) ([]Module, []External, error) {
	args := []string{"list", "-deps", "-test", "-json=Standard,Module"}
	if len(tags) > 0 {
		args = append(args, "-tags="+strings.Join(tags, ","))
	}
	args = append(args, packages...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("go %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	modules := map[string]Module{}
	externals := map[string]External{}
	dec := json.NewDecoder(&stdout)
	for {
		var p listedPackage
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, nil, fmt.Errorf("go list -json: %w", err)
		}
		if p.Standard || p.Module == nil {
			continue
		}
		m := p.Module
		switch {
		case m.Main:
			if _, seen := modules[m.Path]; seen {
				continue
			}
			version, err := packageVersion(m.Dir)
			if err != nil {
				return nil, nil, fmt.Errorf("module %s: %w", m.Path, err)
			}
			modules[m.Path] = Module{Path: m.Path, Version: version}
		default:
			version := m.Version
			if m.Replace != nil && m.Replace.Version != "" {
				version = m.Replace.Version
			}
			if version == "" {
				return nil, nil, fmt.Errorf("external module %s resolved without a version", m.Path)
			}
			externals[m.Path] = External{Package: m.Path, Version: version}
		}
	}

	outModules := slices.SortedFunc(maps.Values(modules), func(a, b Module) int { return cmp.Compare(a.Path, b.Path) })
	outExternals := slices.SortedFunc(maps.Values(externals), func(a, b External) int { return cmp.Compare(a.Package, b.Package) })
	return outModules, outExternals, nil
}

func packageVersion(dir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", err
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return "", fmt.Errorf("package.json: %w", err)
	}
	if pkg.Version == "" {
		return "", errors.New("package.json declares no version")
	}
	return pkg.Version, nil
}

func ParseRedpandaBrokers(raw []byte) (string, error) {
	var brokers []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &brokers); err != nil {
		return "", fmt.Errorf("redpanda /v1/brokers: %w", err)
	}
	if len(brokers) == 0 {
		return "", errors.New("redpanda /v1/brokers lists no broker")
	}
	release, _, _ := strings.Cut(brokers[0].Version, " - ")
	release = strings.TrimSpace(release)
	if release == "" {
		return "", errors.New("redpanda /v1/brokers carries no version")
	}
	return release, nil
}

const (
	BufIdentity         = "github.com/bufbuild/buf/cmd/buf"
	ProtocGenGoIdentity = "google.golang.org/protobuf/cmd/protoc-gen-go"
)

var (
	bufPin         = regexp.MustCompile(`github\.com/bufbuild/buf/cmd/buf@(\S+)`)
	protocGenGoPin = regexp.MustCompile(`(?m)(?:^|[\s/"'])protoc-gen-go@([^\s"']+)`)
)

func ToolVersions(bufScript, bufGen []byte) ([]Tool, error) {
	buf := bufPin.FindSubmatch(bufScript)
	if buf == nil {
		return nil, errors.New("tools/buf.sh pins no buf version")
	}
	plugin := protocGenGoPin.FindSubmatch(bufGen)
	if plugin == nil {
		return nil, errors.New("contracts/buf.gen.yaml pins no protoc-gen-go version")
	}
	return []Tool{
		{Identity: BufIdentity, Version: string(buf[1])},
		{Identity: ProtocGenGoIdentity, Version: string(plugin[1])},
	}, nil
}
