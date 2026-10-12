package observe_test

import (
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

const observeScope = "github.com/mateusmacedo/dmpf/libs/backend/go/transport/observe"

func textOf(logs *obstest.MemoryLogs) string {
	var out strings.Builder
	for _, record := range logs.Records() {
		out.WriteString(record.Body().String())
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			out.WriteString(" " + string(kv.Key) + "=" + kv.Value.String())
			return true
		})
		out.WriteString("\n")
	}
	return out.String()
}

func memoryLogs(level slog.Level) (log.LoggerProvider, *obstest.MemoryLogs) {
	exporter := &obstest.MemoryLogs{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	return otelboot.Leveled(provider, level), exporter
}
