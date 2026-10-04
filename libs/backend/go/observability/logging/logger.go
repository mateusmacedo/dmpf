package logging

import (
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/log"
)

// NewLogger records under scope, the import path of the emitting package
// (RF-A1). A nil provider stands for the global one of OpenTelemetry.
func NewLogger(provider log.LoggerProvider, scope string) *slog.Logger {
	return otelslog.NewLogger(scope, otelslog.WithLoggerProvider(provider))
}
