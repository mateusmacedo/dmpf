package resilience_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
)

func TestTheSheetLogsItsEffectiveFieldsUnderItsDependency(t *testing.T) {
	sheet := resilience.Defaults("payments")
	sheet.Deadline = resilience.Declare(3 * time.Second)
	var out bytes.Buffer

	slog.New(slog.NewJSONHandler(&out, nil)).Info("process configured", slog.Any("payments", sheet))

	var record struct {
		Payments map[string]string `json:"payments"`
	}
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("Unmarshal(%s) = %v", out.String(), err)
	}
	want := sheet.Effective()
	want["dependency"] = "payments"
	if len(record.Payments) != len(want) {
		t.Fatalf("logged %v, want the dependency and the ten fields of RES-21 %v", record.Payments, want)
	}
	for field, value := range want {
		if record.Payments[field] != value {
			t.Errorf("%s = %q, want %q", field, record.Payments[field], value)
		}
	}
	if record.Payments[resilience.FieldDeadline] != "3s" {
		t.Errorf("deadline = %q, want the override 3s", record.Payments[resilience.FieldDeadline])
	}
}
