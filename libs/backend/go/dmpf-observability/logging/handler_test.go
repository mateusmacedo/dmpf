package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/tracing"
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

// countKey is how many times a key appears at the root of the rendered line.
// It counts on the raw JSON, not on a decoded map, because a decoded map keeps
// only the last value of a duplicated key — which is exactly the failure these
// tests guard against.
func countKey(line, key string) int {
	needle := `"` + key + `":`
	found, from := 0, 0
	for {
		at := strings.Index(line[from:], needle)
		if at < 0 {
			return found
		}
		found++
		from += at + len(needle)
	}
}

func renderRaw(t *testing.T, config logging.Config, emit func(logger *slog.Logger)) []string {
	t.Helper()

	var out bytes.Buffer
	emit(slog.New(logging.NewHandler(&out, config)))

	written := strings.TrimSpace(out.String())
	if written == "" {
		return nil
	}
	return strings.Split(written, "\n")
}

func TestAnAuthorAttributeCannotOccupyAMandatoryKey(t *testing.T) {
	rendered := renderRaw(t, baseConfig(), func(logger *slog.Logger) {
		logger.With(slog.String(logging.KeyService, "impostor")).Info("order accepted")
	})

	record := rendered[len(rendered)-1]
	if got := countKey(record, logging.KeyService); got != 1 {
		t.Fatalf("service appears %d times in %s, want exactly 1 — a duplicated key lets the author's value win", got, record)
	}
	if !strings.Contains(record, `"service":"orders"`) {
		t.Errorf("record = %s, want the platform service to survive", record)
	}
	if !strings.Contains(record, `"`+logging.AuthorPrefix+`service":"impostor"`) {
		t.Errorf("record = %s, want the author's value kept under the %q prefix", record, logging.AuthorPrefix)
	}
}

func TestAnAuthorGroupCannotOccupyAMandatoryKey(t *testing.T) {
	rendered := renderRaw(t, baseConfig(), func(logger *slog.Logger) {
		logger.WithGroup(logging.KeyService).Info("order accepted", slog.String("inner", "1"))
	})

	record := rendered[len(rendered)-1]
	if got := countKey(record, logging.KeyService); got != 1 {
		t.Fatalf("service appears %d times in %s, want exactly 1", got, record)
	}
	if !strings.Contains(record, `"`+logging.AuthorPrefix+`service":{"inner":"1"}`) {
		t.Errorf("record = %s, want the author's group under the %q prefix", record, logging.AuthorPrefix)
	}
}

func TestARecordAttributeCannotOccupyAMandatoryKey(t *testing.T) {
	rendered := renderRaw(t, baseConfig(), func(logger *slog.Logger) {
		logger.Info("order accepted", slog.String(logging.KeyTraceID, "forjado"))
	})

	record := rendered[len(rendered)-1]
	if got := countKey(record, logging.KeyTraceID); got != 1 {
		t.Fatalf("trace_id appears %d times in %s, want exactly 1", got, record)
	}
	if !strings.Contains(record, `"`+logging.AuthorPrefix+`trace_id":"forjado"`) {
		t.Errorf("record = %s, want the author's value under the %q prefix", record, logging.AuthorPrefix)
	}
}

func TestEveryMandatoryKeyIsProtected(t *testing.T) {
	protected := []string{
		logging.KeyTraceID, logging.KeySpanID, logging.KeyService, logging.KeyVersion,
		logging.KeyInstance, logging.KeyCorrelationID, logging.KeyRequestID, logging.KeyTenantID,
	}

	for _, key := range protected {
		t.Run(key, func(t *testing.T) {
			rendered := renderRaw(t, baseConfig(), func(logger *slog.Logger) {
				logger.Info("order accepted", slog.String(key, "impostor"))
			})

			record := rendered[len(rendered)-1]
			if got := countKey(record, key); got != 1 {
				t.Fatalf("%s appears %d times in %s, want exactly 1", key, got, record)
			}
		})
	}
}

func TestInsideAnAuthorGroupTheKeysAreFreeAgain(t *testing.T) {
	rendered := renderRaw(t, baseConfig(), func(logger *slog.Logger) {
		logger.WithGroup("payload").Info("order accepted", slog.String(logging.KeyService, "inner"))
	})

	record := rendered[len(rendered)-1]
	if !strings.Contains(record, `"payload":{"service":"inner"}`) {
		t.Fatalf("record = %s, want service kept inside the group: payload.service collides with nothing", record)
	}
	if !strings.Contains(record, `"service":"orders"`) {
		t.Errorf("record = %s, want the platform service at the root", record)
	}
}

func TestTheRenamingIsWarnedOnce(t *testing.T) {
	rendered := renderRaw(t, baseConfig(), func(logger *slog.Logger) {
		guarded := logger.With(slog.String(logging.KeyService, "impostor"))
		guarded.Info("first")
		guarded.Info("second")
		guarded.Info("third")
	})

	warnings := 0
	for _, line := range rendered {
		if strings.Contains(line, `"level":"WARN"`) && strings.Contains(line, "renamed_keys") {
			warnings++
			if !strings.Contains(line, logging.KeyService) {
				t.Errorf("warning = %s, want it to name the renamed key", line)
			}
		}
	}
	if warnings != 1 {
		t.Fatalf("warnings = %d, want exactly 1 — one bad call site must not flood the log", warnings)
	}
}
