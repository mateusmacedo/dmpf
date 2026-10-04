package redact_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

type categorizedError struct {
	category string
	code     string
}

func (e categorizedError) Error() string         { return "dial tcp 10.0.0.7:5432: connection refused" }
func (e categorizedError) ErrorCategory() string { return e.category }
func (e categorizedError) ErrorCode() string     { return e.code }

// record renders through a handler: only a handler inlines the empty-key group
// of redact.Error, as the OTLP bridge does (otelslog@v0.21.0/handler.go:481-483).
func record(t *testing.T, attr slog.Attr) map[string]any {
	t.Helper()

	var out bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&out, nil))
	logger.LogAttrs(context.Background(), slog.LevelInfo, "under.test", attr)

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal(%q) = %v, want nil", out.String(), err)
	}
	delete(decoded, slog.TimeKey)
	delete(decoded, slog.LevelKey)
	delete(decoded, slog.MessageKey)
	return decoded
}

func TestAnAllowedFieldKeepsItsValue(t *testing.T) {
	redactor := redact.New("order_id", "sku")

	got := record(t, redactor.Attr("sku", "XYZ-1"))

	if got["sku"] != "XYZ-1" {
		t.Fatalf("record = %v, want sku kept", got)
	}
}

func TestAFieldOutsideTheAllowlistIsPresentAndRedacted(t *testing.T) {
	redactor := redact.New("order_id")

	got := record(t, redactor.Attr("cpf", "123.456.789-00"))

	value, present := got["cpf"]
	if !present {
		t.Fatalf("record = %v, want the field present: absence would hide that something was said (LOG-06)", got)
	}
	if value != redact.Placeholder {
		t.Fatalf("cpf = %v, want %q", value, redact.Placeholder)
	}
}

func TestAnEmptyAllowlistRedactsEverything(t *testing.T) {
	redactor := redact.New()

	for _, key := range []string{"cpf", "order_id", "email"} {
		if got := record(t, redactor.Attr(key, "secret"))[key]; got != redact.Placeholder {
			t.Errorf("%s = %v, want %q — an undeclared field is redacted (fail-closed)", key, got, redact.Placeholder)
		}
	}
}

func TestAnEmptyFieldNameIsNeverAllowed(t *testing.T) {
	redactor := redact.New("")

	if redactor.Allows("") {
		t.Fatal("Allows(\"\") = true: the empty field name must not enter the allowlist")
	}
}

func TestTheRedactedValueNeverCarriesTheOriginal(t *testing.T) {
	redactor := redact.New("order_id")
	secret := "123.456.789-00"

	rendered := fmt.Sprint(record(t, redactor.Attr("cpf", secret)))

	if strings.Contains(rendered, secret) {
		t.Fatalf("the record %q still carries the original value", rendered)
	}
}

func TestErrorEmitsTheCategoryAndTheCode(t *testing.T) {
	cause := categorizedError{category: "timeout", code: "PAY-504"}

	got := record(t, redact.Error(cause))

	if got[redact.KeyErrorType] != "timeout" {
		t.Errorf("%s = %v, want \"timeout\"", redact.KeyErrorType, got[redact.KeyErrorType])
	}
	if got[redact.KeyErrorCode] != "PAY-504" {
		t.Errorf("%s = %v, want \"PAY-504\"", redact.KeyErrorCode, got[redact.KeyErrorCode])
	}
}

func TestErrorNeverEmitsTheMessage(t *testing.T) {
	cause := categorizedError{category: "timeout", code: "PAY-504"}

	rendered := fmt.Sprint(record(t, redact.Error(cause)))

	if strings.Contains(rendered, cause.Error()) {
		t.Fatalf("the record %q carries the error message (LOG-07, DAT-23)", rendered)
	}
	if strings.Contains(rendered, "10.0.0.7") {
		t.Fatalf("the record %q carries an address from the error message", rendered)
	}
}

func TestErrorFindsTheCategoryThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("payments: authorize: %w", categorizedError{category: "timeout", code: "PAY-504"})

	got := record(t, redact.Error(wrapped))

	if got[redact.KeyErrorType] != "timeout" {
		t.Fatalf("%s = %v, want the category found through the wrap", redact.KeyErrorType, got[redact.KeyErrorType])
	}
}

func TestAnUnclassifiedErrorReportsTheTypeOther(t *testing.T) {
	got := record(t, redact.Error(errors.New("connection reset by peer")))

	if want := semconv.ErrorTypeOther.Value.AsString(); got[redact.KeyErrorType] != want {
		t.Fatalf("%s = %v, want %q, the fallback of the semantic conventions (RF-B1)", redact.KeyErrorType, got[redact.KeyErrorType], want)
	}
	if _, present := got[redact.KeyErrorCode]; present {
		t.Fatalf("record = %v, want no code for an unclassified error", got)
	}
}

func TestAnErrorWithABlankCategoryReportsTheTypeOther(t *testing.T) {
	got := record(t, redact.Error(categorizedError{category: "", code: "PAY-504"}))

	if want := semconv.ErrorTypeOther.Value.AsString(); got[redact.KeyErrorType] != want {
		t.Fatalf("%s = %v, want %q — a blank category is no category (RF-B1)", redact.KeyErrorType, got[redact.KeyErrorType], want)
	}
}

func TestAnErrorWithoutACodeEmitsOnlyTheCategory(t *testing.T) {
	got := record(t, redact.Error(categorizedError{category: "timeout"}))

	if got[redact.KeyErrorType] != "timeout" {
		t.Errorf("%s = %v, want \"timeout\"", redact.KeyErrorType, got[redact.KeyErrorType])
	}
	if _, present := got[redact.KeyErrorCode]; present {
		t.Errorf("record = %v, want no code when the error declares none", got)
	}
}

func TestANilErrorEmitsNothing(t *testing.T) {
	got := record(t, redact.Error(nil))

	if len(got) != 0 {
		t.Fatalf("record = %v, want empty for a nil error", got)
	}
}

func TestErrorWritesTheCanonicalKeys(t *testing.T) {
	got := record(t, redact.Error(categorizedError{category: "timeout", code: "PAY-504"}))

	if got["error.type"] != "timeout" {
		t.Errorf("error.type = %v, want \"timeout\" (RF-A3)", got["error.type"])
	}
	if got["dmpf.error.code"] != "PAY-504" {
		t.Errorf("dmpf.error.code = %v, want \"PAY-504\" (RF-A3, LOG-03)", got["dmpf.error.code"])
	}
	for _, legacy := range []string{"error_category", "error_code"} {
		if _, present := got[legacy]; present {
			t.Errorf("record = %v, want no %s", got, legacy)
		}
	}
}
