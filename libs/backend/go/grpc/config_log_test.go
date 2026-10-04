package grpc_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	lognoop "go.opentelemetry.io/otel/log/noop"
	"google.golang.org/grpc/codes"
)

func loggedAs(t *testing.T, value any) map[string]any {
	t.Helper()
	var out bytes.Buffer
	slog.New(slog.NewJSONHandler(&out, nil)).Info("process configured", slog.Any("setting", value))
	var record struct {
		Setting map[string]any `json:"setting"`
	}
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("Unmarshal(%s) = %v", out.String(), err)
	}
	return record.Setting
}

func TestAMethodPolicyLogsItsBudgetAndRetry(t *testing.T) {
	declared := policy(true)
	declared.RetryableCodes = []codes.Code{codes.Unavailable, codes.ResourceExhausted}

	logged := loggedAs(t, declared)

	want := map[string]any{
		"limit":              "1s",
		"slack":              "50ms",
		"estimated_duration": "100ms",
		"idempotent":         true,
		"retryable_codes":    "Unavailable,ResourceExhausted",
	}
	if len(logged) != len(want) {
		t.Fatalf("logged %v, want %v", logged, want)
	}
	for field, value := range want {
		if logged[field] != value {
			t.Errorf("%s = %v, want %v", field, logged[field], value)
		}
	}
}

func TestTheConfigLogsItsTransportSheetAndMethods(t *testing.T) {
	cfg := validConfig()
	cfg.LoggerProvider = lognoop.NewLoggerProvider()

	logged := loggedAs(t, cfg)

	if logged["tls"] != true || logged["insecure_for_development_only"] != false {
		t.Errorf("tls = %v, insecure_for_development_only = %v, want the TLS in effect", logged["tls"], logged["insecure_for_development_only"])
	}
	sheet, _ := logged["sheet"].(map[string]any)
	if sheet["dependency"] != "orders" || sheet["deadline"] != "2s" {
		t.Errorf("sheet = %v, want the effective sheet of orders", logged["sheet"])
	}
	methods, _ := logged["methods"].(map[string]any)
	check, _ := methods[checkMethod].(map[string]any)
	if check["limit"] != "1s" {
		t.Errorf("methods = %v, want the policy of %s", logged["methods"], checkMethod)
	}
	if len(logged) != 4 {
		t.Fatalf("logged %v, want tls, the opt-out, the sheet and the methods only", logged)
	}
}

func TestTheOptOutLogsAsTheTransportInEffect(t *testing.T) {
	cfg := validConfig()
	cfg.TLS = nil
	cfg.InsecureForDevelopmentOnly = true

	logged := loggedAs(t, cfg)

	if logged["tls"] != false || logged["insecure_for_development_only"] != true {
		t.Fatalf("logged %v, want tls false under the development opt-out", logged)
	}
}
