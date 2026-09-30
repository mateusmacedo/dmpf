package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
)

func TestASinkReceivesTheRecordWithTheMandatoryFieldsAndTheAuthorGroup(t *testing.T) {
	var out, sink bytes.Buffer
	config := baseConfig()
	config.Sinks = []slog.Handler{slog.NewJSONHandler(&sink, nil)}

	slog.New(logging.NewHandler(&out, config)).WithGroup("order").Info("placed", "id", "o-1")

	for name, written := range map[string]*bytes.Buffer{"writer": &out, "sink": &sink} {
		var record map[string]any
		if err := json.Unmarshal(written.Bytes(), &record); err != nil {
			t.Fatalf("%s: %v in %q", name, err, written.String())
		}
		if record[logging.KeyService] != "orders" {
			t.Fatalf("%s: service = %v, want orders", name, record[logging.KeyService])
		}
		if group, _ := record["order"].(map[string]any); group["id"] != "o-1" {
			t.Fatalf("%s: order = %v, want the author group", name, record["order"])
		}
	}
}

func TestASinkBelowItsLevelIsSkippedWithoutSilencingTheWriter(t *testing.T) {
	var out, sink bytes.Buffer
	config := baseConfig()
	config.Level = slog.LevelDebug
	config.Sinks = []slog.Handler{slog.NewJSONHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn})}

	slog.New(logging.NewHandler(&out, config)).Debug("detail")

	if out.Len() == 0 || sink.Len() != 0 {
		t.Fatalf("writer %q, sink %q; want the record only on the writer", out.String(), sink.String())
	}
}
