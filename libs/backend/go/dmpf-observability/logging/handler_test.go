package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/logging"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
)

func baseConfig() logging.Config {
	return logging.Config{
		Service:  "orders",
		Version:  "1.4.2",
		Instance: "orders-7c9",
		Class:    tracing.ClassMaintenance,
		Rand:     func() float64 { return 0 },
	}
}

func logOnce(t *testing.T, config logging.Config, emit func(logger *slog.Logger)) []map[string]any {
	t.Helper()

	var out bytes.Buffer
	logger := slog.New(logging.NewHandler(&out, config))
	emit(logger)

	records := make([]map[string]any, 0, 2)
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("Unmarshal(%q) = %v, want nil", line, err)
		}
		records = append(records, decoded)
	}
	return records
}

func TestEveryRecordCarriesTheMandatoryFields(t *testing.T) {
	records := logOnce(t, baseConfig(), func(logger *slog.Logger) {
		logger.Info("order accepted")
	})

	if len(records) != 1 {
		t.Fatalf("wrote %d records, want 1", len(records))
	}
	got := records[0]

	for key, want := range map[string]string{
		logging.KeyService:  "orders",
		logging.KeyVersion:  "1.4.2",
		logging.KeyInstance: "orders-7c9",
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %q", key, got[key], want)
		}
	}
	for _, key := range []string{
		logging.KeyTraceID, logging.KeySpanID,
		logging.KeyCorrelationID, logging.KeyRequestID, logging.KeyTenantID,
	} {
		if _, present := got[key]; !present {
			t.Errorf("%s is absent: a mandatory field is recorded as absent, never omitted (LOG-01)", key)
		}
	}
}

func TestWithoutAnActiveSpanTheTraceFieldsAreRecordedAsAbsent(t *testing.T) {
	got := logOnce(t, baseConfig(), func(logger *slog.Logger) {
		logger.Info("order accepted")
	})[0]

	if got[logging.KeyTraceID] != "" || got[logging.KeySpanID] != "" {
		t.Fatalf("trace_id = %v, span_id = %v; want both empty", got[logging.KeyTraceID], got[logging.KeySpanID])
	}
}

func TestTheTraceAndSpanIdsComeFromTheActiveSpan(t *testing.T) {
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})
	ctx, span := provider.Tracer("dmpf-observability").Start(context.Background(), "under.test")
	defer span.End()

	got := logOnce(t, baseConfig(), func(logger *slog.Logger) {
		logger.InfoContext(ctx, "order accepted")
	})[0]

	wantTrace := span.SpanContext().TraceID().String()
	wantSpan := span.SpanContext().SpanID().String()
	if got[logging.KeyTraceID] != wantTrace {
		t.Errorf("trace_id = %v, want %q", got[logging.KeyTraceID], wantTrace)
	}
	if got[logging.KeySpanID] != wantSpan {
		t.Errorf("span_id = %v, want %q", got[logging.KeySpanID], wantSpan)
	}
}

func TestTheExtractorSuppliesTheCorrelation(t *testing.T) {
	config := baseConfig()
	config.Fields = func(context.Context) logging.Fields {
		return logging.Fields{
			logging.KeyCorrelationID: "corr-1",
			logging.KeyRequestID:     "req-2",
			logging.KeyTenantID:      "tenant-3",
		}
	}

	got := logOnce(t, config, func(logger *slog.Logger) { logger.Info("order accepted") })[0]

	for key, want := range map[string]string{
		logging.KeyCorrelationID: "corr-1",
		logging.KeyRequestID:     "req-2",
		logging.KeyTenantID:      "tenant-3",
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %q", key, got[key], want)
		}
	}
}

func TestANilExtractorRecordsTheCorrelationAsAbsent(t *testing.T) {
	got := logOnce(t, baseConfig(), func(logger *slog.Logger) { logger.Info("order accepted") })[0]

	for _, key := range []string{logging.KeyCorrelationID, logging.KeyRequestID, logging.KeyTenantID} {
		if got[key] != "" {
			t.Errorf("%s = %v, want empty — an absent field is never invented", key, got[key])
		}
	}
}

func TestAnExtractorThatOmitsAFieldRecordsItAsAbsent(t *testing.T) {
	config := baseConfig()
	config.Fields = func(context.Context) logging.Fields {
		return logging.Fields{logging.KeyCorrelationID: "corr-1"}
	}

	got := logOnce(t, config, func(logger *slog.Logger) { logger.Info("order accepted") })[0]

	if got[logging.KeyCorrelationID] != "corr-1" {
		t.Errorf("correlation_id = %v, want \"corr-1\"", got[logging.KeyCorrelationID])
	}
	if got[logging.KeyRequestID] != "" || got[logging.KeyTenantID] != "" {
		t.Errorf("request_id = %v, tenant_id = %v; want both empty", got[logging.KeyRequestID], got[logging.KeyTenantID])
	}
}

func TestAKeyOutsideTheAllowlistIsIgnoredWithASingleWarning(t *testing.T) {
	config := baseConfig()
	config.Fields = func(context.Context) logging.Fields {
		return logging.Fields{
			logging.KeyCorrelationID: "corr-1",
			"cpf":                    "123.456.789-00",
			"user_email":             "someone@example.com",
		}
	}

	records := logOnce(t, config, func(logger *slog.Logger) {
		logger.Info("first")
		logger.Info("second")
		logger.Info("third")
	})

	warnings := 0
	for _, r := range records {
		if r[slog.LevelKey] == "WARN" {
			warnings++
			ignored, _ := json.Marshal(r["ignored_keys"])
			if !strings.Contains(string(ignored), "cpf") || !strings.Contains(string(ignored), "user_email") {
				t.Errorf("ignored_keys = %s, want both offending keys", ignored)
			}
		}
	}
	if warnings != 1 {
		t.Fatalf("warnings = %d over three records, want exactly 1 — a misconfigured extractor must not flood", warnings)
	}

	for _, r := range records {
		rendered, _ := json.Marshal(r)
		if strings.Contains(string(rendered), "123.456.789-00") {
			t.Fatalf("record %s carries the value of an ignored key", rendered)
		}
	}
}

func TestTheRedactorFollowsTheConfiguredAllowlist(t *testing.T) {
	config := baseConfig()
	config.AllowedFields = []string{"order_id"}

	redactor, ok := logging.Redactor(logging.NewHandler(&bytes.Buffer{}, config))
	if !ok {
		t.Fatal("Redactor() reported none on a platform handler")
	}
	if !redactor.Allows("order_id") {
		t.Error("Allows(\"order_id\") = false, want true — it is in the configured allowlist")
	}
	if redactor.Allows("cpf") {
		t.Error("Allows(\"cpf\") = true, want false")
	}
}

func TestRedactorReportsFalseForAForeignHandler(t *testing.T) {
	if _, ok := logging.Redactor(slog.NewJSONHandler(&bytes.Buffer{}, nil)); ok {
		t.Fatal("Redactor() reported a redactor for a handler it did not build")
	}
}

func TestWithAttrsAndWithGroupKeepTheMandatoryFields(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(logging.NewHandler(&out, baseConfig())).
		With(slog.String("dependency", "payments")).
		WithGroup("payload")

	logger.Info("order accepted", slog.String("sku", "XYZ"))

	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &got); err != nil {
		t.Fatalf("Unmarshal = %v, want nil", err)
	}
	if got[logging.KeyService] != "orders" {
		t.Fatalf("service = %v, want \"orders\" — WithAttrs must not drop the mandatory fields", got[logging.KeyService])
	}
	if got["dependency"] != "payments" {
		t.Fatalf("dependency = %v, want \"payments\"", got["dependency"])
	}
}
