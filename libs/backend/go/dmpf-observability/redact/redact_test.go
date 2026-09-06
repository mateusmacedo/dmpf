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

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/redact"
)

type categorizedError struct {
	category string
	code     string
}

func (e categorizedError) Error() string         { return "dial tcp 10.0.0.7:5432: connection refused" }
func (e categorizedError) ErrorCategory() string { return e.category }
func (e categorizedError) ErrorCode() string     { return e.code }

// record renders one attribute through a JSON handler, which is how a real
// record reaches a sink and the only way to see what a group actually emits.
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

	if got[redact.KeyErrorCategory] != "timeout" {
		t.Errorf("%s = %v, want \"timeout\"", redact.KeyErrorCategory, got[redact.KeyErrorCategory])
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

	if got[redact.KeyErrorCategory] != "timeout" {
		t.Fatalf("%s = %v, want the category found through the wrap", redact.KeyErrorCategory, got[redact.KeyErrorCategory])
	}
}

func TestAnUnclassifiedErrorReportsTheCategoryAsUnclassified(t *testing.T) {
	got := record(t, redact.Error(errors.New("connection reset by peer")))

	if got[redact.KeyErrorCategory] != redact.CategoryUnclassified {
		t.Fatalf("%s = %v, want %q", redact.KeyErrorCategory, got[redact.KeyErrorCategory], redact.CategoryUnclassified)
	}
	if _, present := got[redact.KeyErrorCode]; present {
		t.Fatalf("record = %v, want no code for an unclassified error", got)
	}
}

func TestAnErrorWithABlankCategoryReportsUnclassified(t *testing.T) {
	got := record(t, redact.Error(categorizedError{category: "", code: "PAY-504"}))

	if got[redact.KeyErrorCategory] != redact.CategoryUnclassified {
		t.Fatalf("%s = %v, want %q — a blank category is no category", redact.KeyErrorCategory, got[redact.KeyErrorCategory], redact.CategoryUnclassified)
	}
}

func TestAnErrorWithoutACodeEmitsOnlyTheCategory(t *testing.T) {
	got := record(t, redact.Error(categorizedError{category: "timeout"}))

	if got[redact.KeyErrorCategory] != "timeout" {
		t.Errorf("%s = %v, want \"timeout\"", redact.KeyErrorCategory, got[redact.KeyErrorCategory])
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
