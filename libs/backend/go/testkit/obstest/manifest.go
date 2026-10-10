package obstest

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

var legacyVariables = []string{"OTLP_ENDPOINT", "OTLP_INSECURE", "OTLP_LOGS", "SERVICE", "SERVICE_VERSION", "INSTANCE_ID", "TRACE_SAMPLE_RATE"}

// Manifest is the OTel environment an app ships in EnvFile. Project names its
// project.json, whose serve-* and docker:run-* targets launch every role; an app
// without per-role targets leaves it empty. Image is what docker:run-* runs.
type Manifest[C any, R ~string] struct {
	Subject[C, R]
	EnvFile string
	Project string
	Image   string
}

func (m Manifest[C, R]) Run(t *testing.T) {
	t.Run("TheManifestDeclaresTheCanonicalOTelEnvironment", m.declaresTheCanonicalEnvironment)
	t.Run("TheManifestEnvironmentExportsEverySignalOverGRPC", m.exportsEverySignal)
	t.Run("AStartWithoutTheVersionOrTheInstanceInTheEnvironmentIsRefused", m.refusesAnIncompleteResource)
	if m.Project == "" {
		return
	}
	t.Run("TheSharedManifestLeavesTheRoleToTheServeTargets", m.leavesTheRoleToTheServeTargets)
	t.Run("EveryServedRoleStartsWithItsOwnInstanceAndRole", m.servedRolesOwnTheirInstance)
	t.Run("AServedRoleKeepsTheResourceOfItsOwnDeployEnv", m.servedRoleKeepsItsDeployEnv)
	t.Run("EveryContainerRoleStartsWithItsOwnInstanceAndRole", m.containerRolesOwnTheirInstance)
	t.Run("AContainerRoleKeepsTheResourceOfItsOwnDeployEnv", m.containerRoleKeepsItsDeployEnv)
	t.Run("ARoleLaunchedWithoutAResourceNamesTheBlankVersion", m.launchWithoutAResourceNamesTheVersion)
}

func (m Manifest[C, R]) manifest(t *testing.T) map[string]string {
	t.Helper()
	return ReadEnvFile(t, m.EnvFile)
}

func (m Manifest[C, R]) declaresTheCanonicalEnvironment(t *testing.T) {
	manifest := m.manifest(t)

	for key, want := range map[string]string{
		"OTEL_SERVICE_NAME":           m.Service,
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		"OTEL_TRACES_SAMPLER_ARG":     "1.0",
		"OTEL_LOGS_EXPORTER":          "otlp",
		"OTEL_PROPAGATORS":            "tracecontext",
		"OTEL_GO_X_OBSERVABILITY":     "true",
	} {
		if got := manifest[key]; got != want {
			t.Errorf("%s = %q, want %q (RF-E1, RF-E5, RF-D7)", key, got, want)
		}
	}
	if endpoint, err := url.Parse(manifest["OTEL_EXPORTER_OTLP_ENDPOINT"]); err != nil || endpoint.Scheme != "http" || endpoint.Port() != "4317" {
		t.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT = %q, want http:// on the gRPC port 4317 of the Collector (RF-E1)", manifest["OTEL_EXPORTER_OTLP_ENDPOINT"])
	}
	for _, attribute := range []string{"service.version=", "service.instance.id=", "deployment.environment.name="} {
		if !strings.Contains(manifest["OTEL_RESOURCE_ATTRIBUTES"], attribute) {
			t.Errorf("OTEL_RESOURCE_ATTRIBUTES = %q, want %s (RF-E3)", manifest["OTEL_RESOURCE_ATTRIBUTES"], attribute)
		}
	}
	for _, legacy := range legacyVariables {
		if _, declared := manifest[legacy]; declared {
			t.Errorf("%s is declared, want it gone in favor of OTEL_* (RF-E1)", legacy)
		}
	}
}

func (m Manifest[C, R]) configOver(t *testing.T, role R, env map[string]string) C {
	t.Helper()
	UseOnlyTheOTelEnvironmentOf(t, env)
	cfg, err := m.FromEnv(role, func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("FromEnv() over %s = %v", m.EnvFile, err)
	}
	return cfg
}

func (m Manifest[C, R]) exportTheProbesOver(t *testing.T, role R, environment map[string]string) *Exports {
	t.Helper()
	addr, collector := StartCollector(t)
	env := maps.Clone(environment)
	env["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://" + addr
	cfg := m.configOver(t, role, env)

	ctx := context.Background()
	runtime, err := boot.StartTelemetry(ctx, m.TelemetryOf(cfg))
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	_, span := runtime.Tracer().Start(ctx, "manifest.probe")
	span.End()
	runtime.LoggerFor(m.Scope).InfoContext(ctx, "manifest probe")
	grace, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := runtime.Shutdown(grace); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}
	return collector
}

func (m Manifest[C, R]) startTelemetryOver(t *testing.T, role R, environment map[string]string) error {
	t.Helper()
	addr, _ := StartCollector(t)
	env := maps.Clone(environment)
	env["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://" + addr
	cfg := m.configOver(t, role, env)
	runtime, err := boot.StartTelemetry(context.Background(), m.TelemetryOf(cfg))
	if err == nil {
		_ = runtime.Shutdown(context.Background())
	}
	return err
}

func (m Manifest[C, R]) exportsEverySignal(t *testing.T) {
	for _, role := range m.Roles {
		t.Run(string(role), func(t *testing.T) {
			collector := m.exportTheProbesOver(t, role, m.manifest(t))

			for _, signal := range []string{"traces", "metrics", "logs"} {
				identity, ok := collector.Identity(signal)
				if !ok {
					t.Errorf("the OTLP/gRPC receiver got no %s (RF-E1)", signal)
					continue
				}
				if identity["service.name"] != m.Service || identity["dmpf.process.role"] != string(role) {
					t.Errorf("%s resource = %v, want service.name=%s and dmpf.process.role=%s (RF-E3)", signal, identity, m.Service, role)
				}
			}
		})
	}
}

func (m Manifest[C, R]) refusesAnIncompleteResource(t *testing.T) {
	for _, missing := range []string{"service.version", "service.instance.id"} {
		t.Run(missing, func(t *testing.T) {
			env := maps.Clone(m.manifest(t))
			var kept []string
			for pair := range strings.SplitSeq(env["OTEL_RESOURCE_ATTRIBUTES"], ",") {
				if !strings.HasPrefix(pair, missing+"=") {
					kept = append(kept, pair)
				}
			}
			env["OTEL_RESOURCE_ATTRIBUTES"] = strings.Join(kept, ",")

			if err := m.startTelemetryOver(t, m.API, env); !errors.Is(err, otelboot.ErrResourceIncomplete) {
				t.Fatalf("StartTelemetry() without %s = %v, want %v (RF-E3)", missing, err, otelboot.ErrResourceIncomplete)
			}
		})
	}
}

func (m Manifest[C, R]) leavesTheRoleToTheServeTargets(t *testing.T) {
	if attributes := m.manifest(t)["OTEL_RESOURCE_ATTRIBUTES"]; strings.Contains(attributes, "dmpf.process.role=") {
		t.Errorf("OTEL_RESOURCE_ATTRIBUTES = %q, want no dmpf.process.role in the env file that every role shares (RF-E3)", attributes)
	}
}

type commandTarget struct {
	Options struct {
		Command string `json:"command"`
		EnvFile string `json:"envFile"`
	} `json:"options"`
}

func (m Manifest[C, R]) targetOf(t *testing.T, name string) commandTarget {
	t.Helper()
	content, err := os.ReadFile(m.Project)
	if err != nil {
		t.Fatalf("read %s: %v", m.Project, err)
	}
	var project struct {
		Targets map[string]commandTarget `json:"targets"`
	}
	if err := json.Unmarshal(content, &project); err != nil {
		t.Fatalf("parse %s: %v", m.Project, err)
	}
	target, ok := project.Targets[name]
	if !ok {
		t.Fatalf("%s declares no %s target", m.Project, name)
	}
	return target
}

func (m Manifest[C, R]) hostEnvironmentOf(t *testing.T, role R, host map[string]string, deployEnv string) map[string]string {
	t.Helper()
	target := m.targetOf(t, "serve-"+string(role))
	if target.Options.EnvFile != "{projectRoot}/deploy/.env.example" {
		t.Fatalf("serve-%s envFile = %q, want the shared {projectRoot}/deploy/.env.example", role, target.Options.EnvFile)
	}
	launch := "exec go run ./cmd --role " + string(role)
	setup, found := strings.CutSuffix(target.Options.Command, launch)
	if !found {
		t.Fatalf("serve-%s command = %q, want it to end with %q", role, target.Options.Command, launch)
	}
	root := t.TempDir()
	if deployEnv != "" {
		if err := os.Mkdir(filepath.Join(root, "deploy"), 0o750); err != nil {
			t.Fatalf("Mkdir() = %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "deploy", ".env"), []byte(deployEnv), 0o600); err != nil {
			t.Fatalf("WriteFile() = %v", err)
		}
	}
	shell := exec.Command("sh", "-c", setup+"exec env")
	shell.Dir = root
	shell.Env = []string{"PATH=" + os.Getenv("PATH")}
	for key, value := range host {
		shell.Env = append(shell.Env, key+"="+value)
	}
	output, err := shell.Output()
	if err != nil {
		t.Fatalf("sh -c over the setup of serve-%s = %v", role, err)
	}
	env := map[string]string{}
	for line := range strings.Lines(string(output)) {
		if key, value, ok := strings.Cut(strings.TrimSuffix(line, "\n"), "="); ok {
			env[key] = value
		}
	}
	return env
}

func (m Manifest[C, R]) servedRolesOwnTheirInstance(t *testing.T) {
	owners := map[string]R{}
	for _, role := range m.Roles {
		t.Run(string(role), func(t *testing.T) {
			identity, ok := m.exportTheProbesOver(t, role, m.hostEnvironmentOf(t, role, m.manifest(t), "")).Identity("traces")
			if !ok {
				t.Fatalf("the OTLP/gRPC receiver got no traces from serve-%s (RF-E1)", role)
			}
			want := m.Service + "-local-" + string(role)
			if identity["service.instance.id"] != want || identity["dmpf.process.role"] != string(role) {
				t.Errorf("serve-%s resource = %v, want service.instance.id=%s and dmpf.process.role=%s (RF-E3)", role, identity, want, role)
			}
			if owner, taken := owners[identity["service.instance.id"]]; taken {
				t.Errorf("serve-%s shares service.instance.id=%s with serve-%s (RF-E3)", role, identity["service.instance.id"], owner)
			}
			owners[identity["service.instance.id"]] = role
		})
	}
}

func (m Manifest[C, R]) requireOwnResource(t *testing.T, launch string, role R, identity map[string]string) {
	t.Helper()
	for key, want := range map[string]string{
		"service.version":             "1.2.3",
		"deployment.environment.name": "dev",
		"service.instance.id":         m.Service + "-local-" + string(role),
		"dmpf.process.role":           string(role),
	} {
		if identity[key] != want {
			t.Errorf("%s over its own deploy/.env: %s = %q, want %q (RF-E3)", launch, key, identity[key], want)
		}
	}
}

func (m Manifest[C, R]) servedRoleKeepsItsDeployEnv(t *testing.T) {
	const own = "OTEL_RESOURCE_ATTRIBUTES=service.version=1.2.3,service.instance.id=workstation,deployment.environment.name=dev\n"
	for _, role := range m.Roles {
		t.Run(string(role), func(t *testing.T) {
			identity, ok := m.exportTheProbesOver(t, role, m.hostEnvironmentOf(t, role, m.manifest(t), own)).Identity("traces")
			if !ok {
				t.Fatalf("the OTLP/gRPC receiver got no traces from serve-%s (RF-E1)", role)
			}
			m.requireOwnResource(t, "serve-"+string(role), role, identity)
		})
	}
}

func (m Manifest[C, R]) manifestCopy(t *testing.T, resource string) string {
	t.Helper()
	content, err := os.ReadFile(m.EnvFile)
	if err != nil {
		t.Fatalf("read %s: %v", m.EnvFile, err)
	}
	var copied strings.Builder
	for line := range strings.Lines(string(content)) {
		if strings.HasPrefix(line, "OTEL_RESOURCE_ATTRIBUTES=") {
			if resource == "" {
				continue
			}
			line = "OTEL_RESOURCE_ATTRIBUTES=" + resource + "\n"
		}
		copied.WriteString(line)
	}
	return copied.String()
}

func (m Manifest[C, R]) containerEnvironmentOf(t *testing.T, role R, deployEnv string) map[string]string {
	t.Helper()
	target := m.targetOf(t, "docker:run-"+string(role))
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	for _, dir := range []string{bin, filepath.Join(root, "deploy")} {
		if err := os.Mkdir(dir, 0o750); err != nil {
			t.Fatalf("Mkdir() = %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "deploy", ".env"), []byte(deployEnv), 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\nprintf '%s\\0' \"$@\"\n"), 0o700); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	shell := exec.Command("sh", "-c", target.Options.Command)
	shell.Dir = root
	shell.Env = []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")}
	output, err := shell.Output()
	if err != nil {
		t.Fatalf("sh -c over docker:run-%s = %v", role, err)
	}
	args := strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00")
	launch := []string{m.Image, "--role", string(role)}
	if len(args) <= len(launch) || args[0] != "run" || !slices.Equal(args[len(args)-len(launch):], launch) {
		t.Fatalf("docker:run-%s runs docker %q, want docker run ... %s", role, args, strings.Join(launch, " "))
	}
	var files []string
	overrides := map[string]string{}
	for i := 1; i < len(args)-len(launch); i++ {
		switch args[i] {
		case "--env-file":
			i++
			files = append(files, args[i])
		case "-e", "--env":
			i++
			key, value, _ := strings.Cut(args[i], "=")
			overrides[key] = value
		}
	}
	if !slices.Equal(files, []string{"deploy/.env"}) {
		t.Fatalf("docker:run-%s env files = %q, want only deploy/.env", role, files)
	}
	env := ReadEnvFile(t, filepath.Join(root, "deploy", ".env"))
	maps.Copy(env, overrides)
	return env
}

func instanceOf(attributes string) string {
	instance := ""
	for pair := range strings.SplitSeq(attributes, ",") {
		if value, ok := strings.CutPrefix(pair, "service.instance.id="); ok {
			instance = value
		}
	}
	return instance
}

// containerRoles are the roles with a docker:run-<role> target; the API runs
// under the plain docker:run.
func (m Manifest[C, R]) containerRoles() []R {
	return slices.DeleteFunc(slices.Clone(m.Roles), func(role R) bool { return role == m.API })
}

func (m Manifest[C, R]) containerRolesOwnTheirInstance(t *testing.T) {
	manifest := m.manifest(t)
	owners := map[string]R{instanceOf(manifest["OTEL_RESOURCE_ATTRIBUTES"]): m.API}
	for _, role := range m.containerRoles() {
		t.Run(string(role), func(t *testing.T) {
			env := m.containerEnvironmentOf(t, role, m.manifestCopy(t, manifest["OTEL_RESOURCE_ATTRIBUTES"]))
			identity, ok := m.exportTheProbesOver(t, role, env).Identity("traces")
			if !ok {
				t.Fatalf("the OTLP/gRPC receiver got no traces from docker:run-%s (RF-E1)", role)
			}
			want := m.Service + "-local-" + string(role)
			if identity["service.instance.id"] != want || identity["dmpf.process.role"] != string(role) {
				t.Errorf("docker:run-%s resource = %v, want service.instance.id=%s and dmpf.process.role=%s (RF-E3)", role, identity, want, role)
			}
			if owner, taken := owners[identity["service.instance.id"]]; taken {
				t.Errorf("docker:run-%s shares service.instance.id=%s with the container of %s (RF-E3)", role, identity["service.instance.id"], owner)
			}
			owners[identity["service.instance.id"]] = role
		})
	}
}

func (m Manifest[C, R]) containerRoleKeepsItsDeployEnv(t *testing.T) {
	const own = "service.version=1.2.3,service.instance.id=workstation,deployment.environment.name=dev"
	for _, role := range m.containerRoles() {
		t.Run(string(role), func(t *testing.T) {
			identity, ok := m.exportTheProbesOver(t, role, m.containerEnvironmentOf(t, role, m.manifestCopy(t, own))).Identity("traces")
			if !ok {
				t.Fatalf("the OTLP/gRPC receiver got no traces from docker:run-%s (RF-E1)", role)
			}
			m.requireOwnResource(t, "docker:run-"+string(role), role, identity)
		})
	}
}

func (m Manifest[C, R]) launchWithoutAResourceNamesTheVersion(t *testing.T) {
	host := m.manifest(t)
	delete(host, "OTEL_RESOURCE_ATTRIBUTES")
	launches := []struct {
		target        string
		roles         []R
		environmentOf func(*testing.T, R) map[string]string
	}{
		{"serve", m.Roles, func(t *testing.T, role R) map[string]string { return m.hostEnvironmentOf(t, role, host, "") }},
		{"docker:run", m.containerRoles(), func(t *testing.T, role R) map[string]string {
			return m.containerEnvironmentOf(t, role, m.manifestCopy(t, ""))
		}},
	}
	for _, launch := range launches {
		for _, role := range launch.roles {
			t.Run(launch.target+"-"+string(role), func(t *testing.T) {
				err := m.startTelemetryOver(t, role, launch.environmentOf(t, role))

				if !errors.Is(err, otelboot.ErrResourceIncomplete) || !strings.Contains(err.Error(), "[service.version] are blank") {
					t.Fatalf("%s-%s without OTEL_RESOURCE_ATTRIBUTES: StartTelemetry() = %v, want %v naming only [service.version] (RF-E3)", launch.target, role, err, otelboot.ErrResourceIncomplete)
				}
			})
		}
	}
}
