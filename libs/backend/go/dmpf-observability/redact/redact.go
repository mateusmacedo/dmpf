package redact

import (
	"errors"
	"log/slog"
)

// Placeholder replaces the value of a field outside the allowlist. The field
// stays present, so a reader sees that something was said and not that nothing
// was (LOG-06, DAT-22).
const Placeholder = "<redacted>"

// Keys of what an error is allowed to contribute to a record.
const (
	KeyErrorCategory = "error_category"
	KeyErrorCode     = "error_code"
)

// CategoryUnclassified is what an error that the taxonomy did not classify
// reports. It is never the error message: an unclassified failure is still a
// category, and FND-07 owns the taxonomy this package consumes.
const CategoryUnclassified = "unclassified"

// Categorized is what an error implements to contribute its category and code.
// The taxonomy lives in FND-07; this package only reads it.
type Categorized interface {
	ErrorCategory() string
	ErrorCode() string
}

// Redactor holds the allowlist of the service. It is a value built at boot, not
// package state, so two services in one process do not share an allowlist.
type Redactor struct {
	allowed map[string]struct{}
}

// New builds a redactor for the given fields. Everything not named here is
// redacted, which is the fail-closed reading: a field nobody declared is a
// field nobody reviewed.
func New(allowedFields ...string) Redactor {
	allowed := make(map[string]struct{}, len(allowedFields))
	for _, field := range allowedFields {
		if field != "" {
			allowed[field] = struct{}{}
		}
	}
	return Redactor{allowed: allowed}
}

// Allows reports whether the field is in the allowlist.
func (r Redactor) Allows(key string) bool {
	_, ok := r.allowed[key]
	return ok
}

// Attr keeps the value of an allowed field and replaces every other one with
// the placeholder, so the shape of the record survives and the content does not.
func (r Redactor) Attr(key string, value any) slog.Attr {
	if r.Allows(key) {
		return slog.Any(key, value)
	}
	return slog.String(key, Placeholder)
}

// Error reduces a failure to its category and code. The message never comes
// out: it carries whatever the failing call put in it, which redaction cannot
// inspect (LOG-07, DAT-23).
func Error(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}

	var categorized Categorized
	if !errors.As(err, &categorized) {
		return slog.String(KeyErrorCategory, CategoryUnclassified)
	}

	category := categorized.ErrorCategory()
	if category == "" {
		category = CategoryUnclassified
	}

	if code := categorized.ErrorCode(); code != "" {
		return slog.Group("", slog.String(KeyErrorCategory, category), slog.String(KeyErrorCode, code))
	}
	return slog.String(KeyErrorCategory, category)
}
