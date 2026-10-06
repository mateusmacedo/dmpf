package relay

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	lognoop "go.opentelemetry.io/otel/log/noop"
)

func TestTheConfigLogsTheValuesThatVaryByEnvironment(t *testing.T) {
	config := validConfig()
	config.System = "kafka"
	config.LoggerProvider = lognoop.NewLoggerProvider()
	var out bytes.Buffer

	slog.New(slog.NewJSONHandler(&out, nil)).Info("process configured", slog.Any("relay", config))

	var record struct {
		Relay map[string]any `json:"relay"`
	}
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("Unmarshal(%s) = %v", out.String(), err)
	}
	want := map[string]any{
		"source":          testSource,
		"interval":        "1s",
		"batch_size":      float64(50),
		"lease":           "1m0s",
		"concurrency":     float64(4),
		"max_attempts":    float64(5),
		"backoff_base":    "1s",
		"backoff_ceiling": "1m0s",
		"shutdown_grace":  "5s",
	}
	if len(record.Relay) != len(want) {
		t.Fatalf("logged %v, want only the operational values %v: tracer, meter, system, address and logger are code", record.Relay, want)
	}
	for field, value := range want {
		if record.Relay[field] != value {
			t.Errorf("%s = %v, want %v", field, record.Relay[field], value)
		}
	}
}
