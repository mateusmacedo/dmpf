package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

const (
	manifestPath = "../deploy/.env.example"
	projectPath  = "../project.json"
)

var legacyVariables = []string{"OTLP_ENDPOINT", "OTLP_INSECURE", "OTLP_LOGS", "SERVICE", "SERVICE_VERSION", "INSTANCE_ID", "TRACE_SAMPLE_RATE"}

type otlpExports struct {
	mu        sync.Mutex
	resources map[string]*resourcepb.Resource
	logs      []*logspb.LogRecord
}

func (e *otlpExports) record(signal string, resource *resourcepb.Resource) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.resources[signal] = resource
}

func (e *otlpExports) keep(records []*logspb.LogRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logs = append(e.logs, records...)
}

func (e *otlpExports) logRecords() []*logspb.LogRecord {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]*logspb.LogRecord(nil), e.logs...)
}

func (e *otlpExports) identity(signal string) (map[string]string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	resource, ok := e.resources[signal]
	if !ok {
		return nil, false
	}
	identity := map[string]string{}
	for _, attribute := range resource.GetAttributes() {
		identity[attribute.GetKey()] = attribute.GetValue().GetStringValue()
	}
	return identity, true
}

type otlpTraceSink struct {
	coltracepb.UnimplementedTraceServiceServer
	got *otlpExports
}

func (s otlpTraceSink) Export(_ context.Context, request *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	for _, spans := range request.GetResourceSpans() {
		s.got.record("traces", spans.GetResource())
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

type otlpMetricSink struct {
	colmetricpb.UnimplementedMetricsServiceServer
	got *otlpExports
}

func (s otlpMetricSink) Export(_ context.Context, request *colmetricpb.ExportMetricsServiceRequest) (*colmetricpb.ExportMetricsServiceResponse, error) {
	for _, metrics := range request.GetResourceMetrics() {
		s.got.record("metrics", metrics.GetResource())
	}
	return &colmetricpb.ExportMetricsServiceResponse{}, nil
}

type otlpLogSink struct {
	collogspb.UnimplementedLogsServiceServer
	got *otlpExports
}

func (s otlpLogSink) Export(_ context.Context, request *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	for _, logs := range request.GetResourceLogs() {
		s.got.record("logs", logs.GetResource())
		for _, scope := range logs.GetScopeLogs() {
			s.got.keep(scope.GetLogRecords())
		}
	}
	return &collogspb.ExportLogsServiceResponse{}, nil
}

func startCollector(t *testing.T) (string, *otlpExports) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	got := &otlpExports{resources: map[string]*resourcepb.Resource{}}
	server := grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(server, otlpTraceSink{got: got})
	colmetricpb.RegisterMetricsServiceServer(server, otlpMetricSink{got: got})
	collogspb.RegisterLogsServiceServer(server, otlpLogSink{got: got})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return listener.Addr().String(), got
}

func readManifest(t *testing.T) map[string]string {
	t.Helper()
	return readEnvFile(t, manifestPath)
}

func readEnvFile(t *testing.T, path string) map[string]string {
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

func useOnlyTheOTelEnvironmentOf(t *testing.T, env map[string]string) {
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

func exportTheProbesOver(t *testing.T, role app.Role, environment map[string]string) *otlpExports {
	t.Helper()
	addr, collector := startCollector(t)
	env := maps.Clone(environment)
	env["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://" + addr
	useOnlyTheOTelEnvironmentOf(t, env)
	cfg, err := app.FromEnv(role, func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("FromEnv() over %s = %v", manifestPath, err)
	}

	ctx := context.Background()
	runtime, err := boot.StartTelemetry(ctx, app.TelemetryOf(cfg))
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	_, span := runtime.Tracer().Start(ctx, "manifest.probe")
	span.End()
	runtime.LoggerFor(reflect.TypeFor[app.Config]().PkgPath()).InfoContext(ctx, "manifest probe")
	grace, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := runtime.Shutdown(grace); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}
	return collector
}

type commandTarget struct {
	Options struct {
		Command string `json:"command"`
		EnvFile string `json:"envFile"`
	} `json:"options"`
}

func targetOf(t *testing.T, name string) commandTarget {
	t.Helper()
	content, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatalf("read %s: %v", projectPath, err)
	}
	var project struct {
		Targets map[string]commandTarget `json:"targets"`
	}
	if err := json.Unmarshal(content, &project); err != nil {
		t.Fatalf("parse %s: %v", projectPath, err)
	}
	target, ok := project.Targets[name]
	if !ok {
		t.Fatalf("%s declares no %s target", projectPath, name)
	}
	return target
}

func hostEnvironmentOf(t *testing.T, role app.Role, host map[string]string, deployEnv string) map[string]string {
	t.Helper()
	target := targetOf(t, "serve-"+string(role))
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

func TestTheManifestDeclaresTheCanonicalOTelEnvironment(t *testing.T) {
	manifest := readManifest(t)

	for key, want := range map[string]string{
		"OTEL_SERVICE_NAME":           "orders",
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

func TestTheManifestEnvironmentExportsEverySignalOverGRPC(t *testing.T) {
	for _, role := range app.Roles {
		t.Run(string(role), func(t *testing.T) {
			collector := exportTheProbesOver(t, role, readManifest(t))

			for _, signal := range []string{"traces", "metrics", "logs"} {
				identity, ok := collector.identity(signal)
				if !ok {
					t.Errorf("the OTLP/gRPC receiver got no %s (RF-E1)", signal)
					continue
				}
				if identity["service.name"] != "orders" || identity["dmpf.process.role"] != string(role) {
					t.Errorf("%s resource = %v, want service.name=orders and dmpf.process.role=%s (RF-E3)", signal, identity, role)
				}
			}
		})
	}
}

func TestAStartWithoutTheVersionOrTheInstanceInTheEnvironmentIsRefused(t *testing.T) {
	for _, missing := range []string{"service.version", "service.instance.id"} {
		t.Run(missing, func(t *testing.T) {
			addr, _ := startCollector(t)
			env := maps.Clone(readManifest(t))
			env["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://" + addr
			var kept []string
			for _, pair := range strings.Split(env["OTEL_RESOURCE_ATTRIBUTES"], ",") {
				if !strings.HasPrefix(pair, missing+"=") {
					kept = append(kept, pair)
				}
			}
			env["OTEL_RESOURCE_ATTRIBUTES"] = strings.Join(kept, ",")
			useOnlyTheOTelEnvironmentOf(t, env)
			cfg, err := app.FromEnv(app.RoleAPI, func(key string) string { return env[key] })
			if err != nil {
				t.Fatalf("FromEnv() over %s = %v", manifestPath, err)
			}

			runtime, err := boot.StartTelemetry(context.Background(), app.TelemetryOf(cfg))
			if err == nil {
				_ = runtime.Shutdown(context.Background())
			}

			if !errors.Is(err, otelboot.ErrResourceIncomplete) {
				t.Fatalf("StartTelemetry() without %s = %v, want %v (RF-E3)", missing, err, otelboot.ErrResourceIncomplete)
			}
		})
	}
}

func TestTheSharedManifestLeavesTheRoleToTheServeTargets(t *testing.T) {
	if attributes := readManifest(t)["OTEL_RESOURCE_ATTRIBUTES"]; strings.Contains(attributes, "dmpf.process.role=") {
		t.Errorf("OTEL_RESOURCE_ATTRIBUTES = %q, want no dmpf.process.role in the env file that every role shares (RF-E3)", attributes)
	}
}

func TestEveryServedRoleStartsWithItsOwnInstanceAndRole(t *testing.T) {
	owners := map[string]app.Role{}
	for _, role := range app.Roles {
		t.Run(string(role), func(t *testing.T) {
			identity, ok := exportTheProbesOver(t, role, hostEnvironmentOf(t, role, readManifest(t), "")).identity("traces")
			if !ok {
				t.Fatalf("the OTLP/gRPC receiver got no traces from serve-%s (RF-E1)", role)
			}
			want := "orders-local-" + string(role)
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

func TestAServedRoleKeepsTheResourceOfItsOwnDeployEnv(t *testing.T) {
	const own = "OTEL_RESOURCE_ATTRIBUTES=service.version=1.2.3,service.instance.id=workstation,deployment.environment.name=dev\n"
	for _, role := range app.Roles {
		t.Run(string(role), func(t *testing.T) {
			identity, ok := exportTheProbesOver(t, role, hostEnvironmentOf(t, role, readManifest(t), own)).identity("traces")
			if !ok {
				t.Fatalf("the OTLP/gRPC receiver got no traces from serve-%s (RF-E1)", role)
			}
			for key, want := range map[string]string{
				"service.version":             "1.2.3",
				"deployment.environment.name": "dev",
				"service.instance.id":         "orders-local-" + string(role),
				"dmpf.process.role":           string(role),
			} {
				if identity[key] != want {
					t.Errorf("serve-%s over its own deploy/.env: %s = %q, want %q (RF-E3)", role, key, identity[key], want)
				}
			}
		})
	}
}

func manifestCopy(t *testing.T, resource string) string {
	t.Helper()
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read %s: %v", manifestPath, err)
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

func containerEnvironmentOf(t *testing.T, role app.Role, deployEnv string) map[string]string {
	t.Helper()
	target := targetOf(t, "docker:run-"+string(role))
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
	launch := []string{"apps-backend-orders", "--role", string(role)}
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
	env := readEnvFile(t, filepath.Join(root, "deploy", ".env"))
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

func startTelemetryOver(t *testing.T, role app.Role, environment map[string]string) error {
	t.Helper()
	addr, _ := startCollector(t)
	env := maps.Clone(environment)
	env["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://" + addr
	useOnlyTheOTelEnvironmentOf(t, env)
	cfg, err := app.FromEnv(role, func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	runtime, err := boot.StartTelemetry(context.Background(), app.TelemetryOf(cfg))
	if err == nil {
		_ = runtime.Shutdown(context.Background())
	}
	return err
}

func TestEveryContainerRoleStartsWithItsOwnInstanceAndRole(t *testing.T) {
	manifest := readManifest(t)
	owners := map[string]app.Role{instanceOf(manifest["OTEL_RESOURCE_ATTRIBUTES"]): app.RoleAPI}
	for _, role := range app.Roles {
		if role == app.RoleAPI {
			continue
		}
		t.Run(string(role), func(t *testing.T) {
			env := containerEnvironmentOf(t, role, manifestCopy(t, manifest["OTEL_RESOURCE_ATTRIBUTES"]))
			identity, ok := exportTheProbesOver(t, role, env).identity("traces")
			if !ok {
				t.Fatalf("the OTLP/gRPC receiver got no traces from docker:run-%s (RF-E1)", role)
			}
			want := "orders-local-" + string(role)
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

func TestAContainerRoleKeepsTheResourceOfItsOwnDeployEnv(t *testing.T) {
	const own = "service.version=1.2.3,service.instance.id=workstation,deployment.environment.name=dev"
	for _, role := range app.Roles {
		if role == app.RoleAPI {
			continue
		}
		t.Run(string(role), func(t *testing.T) {
			identity, ok := exportTheProbesOver(t, role, containerEnvironmentOf(t, role, manifestCopy(t, own))).identity("traces")
			if !ok {
				t.Fatalf("the OTLP/gRPC receiver got no traces from docker:run-%s (RF-E1)", role)
			}
			for key, want := range map[string]string{
				"service.version":             "1.2.3",
				"deployment.environment.name": "dev",
				"service.instance.id":         "orders-local-" + string(role),
				"dmpf.process.role":           string(role),
			} {
				if identity[key] != want {
					t.Errorf("docker:run-%s over its own deploy/.env: %s = %q, want %q (RF-E3)", role, key, identity[key], want)
				}
			}
		})
	}
}

func TestARoleLaunchedWithoutAResourceNamesTheBlankVersion(t *testing.T) {
	host := readManifest(t)
	delete(host, "OTEL_RESOURCE_ATTRIBUTES")
	launches := []struct {
		target        string
		environmentOf func(*testing.T, app.Role) map[string]string
	}{
		{"serve", func(t *testing.T, role app.Role) map[string]string { return hostEnvironmentOf(t, role, host, "") }},
		{"docker:run", func(t *testing.T, role app.Role) map[string]string {
			return containerEnvironmentOf(t, role, manifestCopy(t, ""))
		}},
	}
	for _, launch := range launches {
		for _, role := range app.Roles {
			if launch.target == "docker:run" && role == app.RoleAPI {
				continue
			}
			t.Run(launch.target+"-"+string(role), func(t *testing.T) {
				err := startTelemetryOver(t, role, launch.environmentOf(t, role))

				if !errors.Is(err, otelboot.ErrResourceIncomplete) || !strings.Contains(err.Error(), "[service.version] are blank") {
					t.Fatalf("%s-%s without OTEL_RESOURCE_ATTRIBUTES: StartTelemetry() = %v, want %v naming only [service.version] (RF-E3)", launch.target, role, err, otelboot.ErrResourceIncomplete)
				}
			})
		}
	}
}
