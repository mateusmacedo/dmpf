package boot_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"google.golang.org/grpc/grpclog"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
)

const (
	grpcScope     = "google.golang.org/grpc"
	sdkScope      = "go.opentelemetry.io/otel"
	bootScope     = "github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	refusedValue  = "seventeen"
	emitterOfLibs = "DMPF_TEST_LIBRARY_LOGS"
	scenarioEmit  = "emit"
	scenarioFatal = "fatal"
)

// The default loggers of grpclog and of the SDK hold the os.Stderr of package
// init (grpclog/loggerv2.go:67-68, otel internal/global/internal_logging.go:21),
// so only a process of its own shows what reaches its stderr.
func TestEmittingTheLogsOfTheLibraries(t *testing.T) {
	scenario := os.Getenv(emitterOfLibs)
	if scenario == "" {
		t.Skip("runs only as the process the tests of the library logs start")
	}
	ctx := context.Background()
	runtime, err := boot.StartTelemetry(ctx, sampledTelemetry())
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	core := grpclog.Component("core")
	if scenario == scenarioFatal {
		core.Fatalf("[Server #1] grpc: server failed to start: %v", errors.New("listen tcp 10.0.0.9:50051: bind: address already in use"))
	}
	core.Infof("[Channel #1] Channel Connectivity change to %s", "READY")
	core.Warningf("[Channel #1 SubChannel #2] grpc: addrConn.createTransport failed to connect to {Addr: %q}", "10.0.0.8:50051")
	core.Errorf("[Server #3] grpc: server failed to encode response: %v", errors.New("proto: dial tcp 10.0.0.7:4317: connection refused"))
	runtime.MeterProvider().Meter("")
	if err := runtime.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}
}

func runEmitterOfLibs(t *testing.T, scenario string, environment ...string) (*receiver, string, error) {
	t.Helper()
	cleanOTelEnv(t)
	collector := startReceiver(t)
	command := exec.Command(os.Args[0], "-test.run=^TestEmittingTheLogsOfTheLibraries$", "-test.count=1")
	command.Env = append(os.Environ(),
		emitterOfLibs+"="+scenario,
		"OTEL_EXPORTER_OTLP_PROTOCOL=grpc",
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://"+collector.addr,
		"OTEL_TRACES_EXPORTER=none",
		"OTEL_METRICS_EXPORTER=otlp",
		"OTEL_LOGS_EXPORTER=otlp",
		"OTEL_METRIC_EXPORT_INTERVAL=often",
		"GRPC_GO_LOG_SEVERITY_LEVEL=",
		"GRPC_GO_LOG_VERBOSITY_LEVEL=",
	)
	command.Env = append(command.Env, environment...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err := command.Run()
	return collector, stderr.String(), err
}

func exportedAs(records []exportedLog, scope, severity, body string) (exportedLog, bool) {
	for _, record := range records {
		if record.scope == scope && record.severity == severity && record.body == body {
			return record, true
		}
	}
	return exportedLog{}, false
}

func severitiesOf(records []exportedLog, scope string) []string {
	var severities []string
	for _, record := range records {
		if record.scope == scope {
			severities = append(severities, record.severity)
		}
	}
	return severities
}

func leakedValues(records []exportedLog) []string {
	var leaked []string
	for _, record := range records {
		for _, text := range append([]string{record.body}, values(record.attributes)...) {
			if strings.Contains(text, "10.0.0.") || strings.Contains(text, "often") || strings.Contains(text, refusedValue) {
				leaked = append(leaked, text)
			}
		}
	}
	return leaked
}

func values(attributes map[string]string) []string {
	all := make([]string, 0, len(attributes))
	for _, value := range attributes {
		all = append(all, value)
	}
	return all
}

func TestTheLogsOfGRPCAndOfTheSDKLeaveByOTLPAndNothingByStderr(t *testing.T) {
	collector, stderr, err := runEmitterOfLibs(t, scenarioEmit)
	if err != nil {
		t.Fatalf("emitter = %v, stderr:\n%s", err, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing: the log leaves only by OTLP (RF-A1)", stderr)
	}

	records := collector.logs.exported()
	if _, ok := exportedAs(records, grpcScope, "ERROR", "[core] [Server #3] grpc: server failed to encode response <redacted>"); !ok {
		t.Errorf("exported %+v, want the ERROR of grpclog under the scope %s", records, grpcScope)
	}
	if severities := severitiesOf(records, grpcScope); len(severities) != 1 {
		t.Errorf("severities under %s = %v, want only the ERROR: grpclog lets WARNING and INFO out only when GRPC_GO_LOG_SEVERITY_LEVEL asks", grpcScope, severities)
	}
	parsed, ok := exportedAs(records, sdkScope, "ERROR", "parse duration")
	if !ok || parsed.attributes["error.type"] != "_OTHER" {
		t.Errorf("exported %+v, want the ERROR of the SDK under the scope %s, reduced to error.type", records, sdkScope)
	}
	if _, ok := exportedAs(records, sdkScope, "WARN", "Invalid Meter name."); !ok {
		t.Errorf("exported %+v, want the warning of the SDK under the scope %s at WARN", records, sdkScope)
	}
	if leaked := leakedValues(records); len(leaked) > 0 {
		t.Errorf("exported %q, want the values the libraries format into their messages left out (DAT-23)", leaked)
	}
}

func TestADeclaredGRPCSeverityLetsItsWarningsOut(t *testing.T) {
	collector, stderr, err := runEmitterOfLibs(t, scenarioEmit, "GRPC_GO_LOG_SEVERITY_LEVEL=warning")
	if err != nil {
		t.Fatalf("emitter = %v, stderr:\n%s", err, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing: the log leaves only by OTLP (RF-A1)", stderr)
	}

	records := collector.logs.exported()
	if _, ok := exportedAs(records, grpcScope, "WARN", "[core] [Channel #1 SubChannel #2] grpc: addrConn.createTransport failed to connect to <redacted>"); !ok {
		t.Errorf("exported %+v, want the WARNING of grpclog at WARN under the scope %s", records, grpcScope)
	}
	for _, severity := range severitiesOf(records, grpcScope) {
		if severity == "INFO" {
			t.Errorf("exported %+v, want no INFO of grpclog below GRPC_GO_LOG_SEVERITY_LEVEL=info", records)
		}
	}
}

func TestAFatalOfGRPCLeavesByOTLPBeforeTheProcessExits(t *testing.T) {
	collector, stderr, err := runEmitterOfLibs(t, scenarioFatal)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("emitter = %v, want the exit status 1 of grpclog.Fatal; stderr:\n%s", err, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing: the log leaves only by OTLP (RF-A1)", stderr)
	}
	records := collector.logs.exported()
	if _, ok := exportedAs(records, grpcScope, "ERROR", "[core] [Server #1] grpc: server failed to start <redacted>"); !ok {
		t.Errorf("exported %+v, want the FATAL of grpclog at ERROR under the scope %s", records, grpcScope)
	}
}

func TestTheLogsOfTheLibrariesWithoutALogPipelineAreDiscarded(t *testing.T) {
	for _, environment := range []string{"OTEL_LOGS_EXPORTER=none", "OTEL_SDK_DISABLED=true"} {
		collector, stderr, err := runEmitterOfLibs(t, scenarioEmit, environment)
		if err != nil {
			t.Fatalf("emitter with %s = %v, stderr:\n%s", environment, err, stderr)
		}
		if stderr != "" {
			t.Errorf("stderr with %s = %q, want nothing: the record is discarded (RF-A1)", environment, stderr)
		}
		if records := collector.logs.exported(); len(records) > 0 {
			t.Errorf("exported with %s = %+v, want nothing", environment, records)
		}
	}
}

func TestARefusedSettingOfTheLogExporterLeavesByOTLPAndNothingByStderr(t *testing.T) {
	collector, stderr, err := runEmitterOfLibs(t, scenarioEmit, "OTEL_EXPORTER_OTLP_LOGS_TIMEOUT="+refusedValue)
	if err != nil {
		t.Fatalf("emitter = %v, stderr:\n%s", err, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing: the log leaves only by OTLP (RF-A1)", stderr)
	}

	records := collector.logs.exported()
	refused, ok := exportedAs(records, bootScope, "WARN", "telemetry export failed")
	if !ok || refused.attributes["error.type"] != "_OTHER" {
		t.Errorf("exported %+v, want the refused setting of the log exporter under the scope %s, reduced to error.type", records, bootScope)
	}
	if leaked := leakedValues(records); len(leaked) > 0 {
		t.Errorf("exported %q, want the refused value left out (DAT-23)", leaked)
	}
}
