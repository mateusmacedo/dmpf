package boot

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log"
	"google.golang.org/grpc/grpclog"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

const (
	scopeGRPC = "google.golang.org/grpc"
	scopeSDK  = "go.opentelemetry.io/otel"

	envGRPCSeverity  = "GRPC_GO_LOG_SEVERITY_LEVEL"
	envGRPCVerbosity = "GRPC_GO_LOG_VERBOSITY_LEVEL"

	// sdkWarnVerbosity is the V of global.Warn (otel internal/global/internal_logging.go:60-61).
	sdkWarnVerbosity = 1
)

type grpcSeverity int

const (
	grpcInfo grpcSeverity = iota
	grpcWarning
	grpcError
	grpcFatal
	grpcNothing
)

// The LoggerV2 contract writes a FATAL to the ERROR log (grpclog/internal/loggerv2.go:49).
var grpcLevels = [...]slog.Level{grpcInfo: slog.LevelInfo, grpcWarning: slog.LevelWarn, grpcError: slog.LevelError, grpcFatal: slog.LevelError}

type libraryLogs struct {
	grpc, sdk     *slog.Logger
	grpcSeverity  grpcSeverity
	grpcVerbosity int
	flush         func(context.Context) error
}

var (
	discarded = slog.New(slog.DiscardHandler)
	muted     = libraryLogs{grpc: discarded, sdk: discarded, grpcSeverity: grpcNothing, flush: func(context.Context) error { return nil }}
	libraries atomic.Pointer[libraryLogs]
)

// WHY: SetLoggerV2 is not mutex-protected and must precede every gRPC function
// (grpclog/loggerv2.go:33-34); only package init runs before any of their goroutines.
func init() {
	muteLibraries()
	grpclog.SetLoggerV2(grpcLog{})
	otel.SetLogger(logr.New(sdkLog{}))
}

func muteLibraries() { libraries.Store(&muted) }

func hearLibraries(provider log.LoggerProvider, flush func(context.Context) error) {
	verbosity, _ := strconv.Atoi(os.Getenv(envGRPCVerbosity))
	libraries.Store(&libraryLogs{
		grpc:          logging.NewLogger(provider, scopeGRPC),
		sdk:           logging.NewLogger(provider, scopeSDK),
		grpcSeverity:  grpcThreshold(os.Getenv(envGRPCSeverity)),
		grpcVerbosity: verbosity,
		flush:         flush,
	})
}

// grpcThreshold reads the variable as the default grpclog does (grpclog/loggerv2.go:65-73).
func grpcThreshold(declared string) grpcSeverity {
	switch declared {
	case "", "ERROR", "error":
		return grpcError
	case "WARNING", "warning":
		return grpcWarning
	case "INFO", "info":
		return grpcInfo
	}
	return grpcNothing
}

func logGRPC(severity grpcSeverity, format func(...any) string, args []any) {
	logs := libraries.Load()
	if severity < logs.grpcSeverity {
		return
	}
	logs.grpc.LogAttrs(context.Background(), grpcLevels[severity], withoutValues(format(args...)))
}

func logGRPCf(severity grpcSeverity, format string, args []any) {
	logGRPC(severity, func(args ...any) string { return fmt.Sprintf(format, args...) }, args)
}

// WHY: grpclog.Fatal calls os.Exit right after the logger returns
// (grpclog/grpclog.go:88-92), before the batch processor exports.
func flushBeforeExit() {
	ctx, cancel := context.WithTimeout(context.Background(), observability.ShutdownGrace)
	defer cancel()
	_ = libraries.Load().flush(ctx)
}

var grpcStatement = regexp.MustCompile(`^(?:\[[A-Za-z][\w #-]*\] ?)*(?:[a-z]+: )?(?:[A-Za-z][A-Za-z_., -]*)?`)

func withoutValues(message string) string { return redact.WithoutValues(message, grpcStatement) }

type grpcLog struct{}

func (grpcLog) Info(args ...any)                 { logGRPC(grpcInfo, fmt.Sprint, args) }
func (grpcLog) Infoln(args ...any)               { logGRPC(grpcInfo, fmt.Sprintln, args) }
func (grpcLog) Infof(format string, args ...any) { logGRPCf(grpcInfo, format, args) }
func (grpcLog) InfoDepth(_ int, args ...any)     { logGRPC(grpcInfo, fmt.Sprintln, args) }

func (grpcLog) Warning(args ...any)                 { logGRPC(grpcWarning, fmt.Sprint, args) }
func (grpcLog) Warningln(args ...any)               { logGRPC(grpcWarning, fmt.Sprintln, args) }
func (grpcLog) Warningf(format string, args ...any) { logGRPCf(grpcWarning, format, args) }
func (grpcLog) WarningDepth(_ int, args ...any)     { logGRPC(grpcWarning, fmt.Sprintln, args) }

func (grpcLog) Error(args ...any)                 { logGRPC(grpcError, fmt.Sprint, args) }
func (grpcLog) Errorln(args ...any)               { logGRPC(grpcError, fmt.Sprintln, args) }
func (grpcLog) Errorf(format string, args ...any) { logGRPCf(grpcError, format, args) }
func (grpcLog) ErrorDepth(_ int, args ...any)     { logGRPC(grpcError, fmt.Sprintln, args) }

func (grpcLog) Fatal(args ...any) {
	logGRPC(grpcFatal, fmt.Sprint, args)
	flushBeforeExit()
}

func (grpcLog) Fatalln(args ...any) {
	logGRPC(grpcFatal, fmt.Sprintln, args)
	flushBeforeExit()
}

func (grpcLog) Fatalf(format string, args ...any) {
	logGRPCf(grpcFatal, format, args)
	flushBeforeExit()
}

func (grpcLog) FatalDepth(_ int, args ...any) {
	logGRPC(grpcFatal, fmt.Sprintln, args)
	flushBeforeExit()
}

func (grpcLog) V(level int) bool { return level <= libraries.Load().grpcVerbosity }

// sdkLog drops the key-value pairs: the SDK puts the refused value in them
// (sdk/metric env.go:31), and the message is a constant of its code.
type sdkLog struct{}

func (sdkLog) Init(logr.RuntimeInfo)  {}
func (sdkLog) Enabled(level int) bool { return level <= sdkWarnVerbosity }

func (sdkLog) Info(level int, msg string, _ ...any) {
	severity := slog.LevelInfo
	if level >= sdkWarnVerbosity {
		severity = slog.LevelWarn
	}
	libraries.Load().sdk.LogAttrs(context.Background(), severity, msg)
}

func (sdkLog) Error(err error, msg string, _ ...any) {
	libraries.Load().sdk.LogAttrs(context.Background(), slog.LevelError, msg, redact.Error(err))
}

func (s sdkLog) WithValues(...any) logr.LogSink { return s }
func (s sdkLog) WithName(string) logr.LogSink   { return s }

// WHY: otlploggrpc hands a refused setting to otel.Handle while the log exporter
// is built (otlploggrpc@v0.23.0/config.go:441), before the logger exists.
type heldErrors struct {
	mu      sync.Mutex
	errors  []error
	handler otel.ErrorHandler
}

func holdErrors() *heldErrors {
	held := new(heldErrors)
	otel.SetErrorHandler(held)
	return held
}

func (h *heldErrors) Handle(err error) {
	h.mu.Lock()
	handler := h.handler
	if handler == nil {
		h.errors = append(h.errors, err)
	}
	h.mu.Unlock()
	if handler != nil {
		handler.Handle(err)
	}
}

func (h *heldErrors) handOver(handler otel.ErrorHandler) {
	otel.SetErrorHandler(handler)
	h.mu.Lock()
	h.handler = handler
	held := h.errors
	h.errors = nil
	h.mu.Unlock()
	for _, err := range held {
		handler.Handle(err)
	}
}
