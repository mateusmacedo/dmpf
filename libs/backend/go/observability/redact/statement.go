package redact

import (
	"regexp"
	"strings"
)

// WithoutValues keeps the text a library wrote before the first value it
// formatted in, as statement matches it, and replaces the rest with Placeholder
// (DAT-02, DAT-23). statement must not be nil.
func WithoutValues(message string, statement *regexp.Regexp) string {
	message = strings.TrimRight(message, "\n")
	kept := statement.FindString(message)
	if len(kept) == len(message) {
		return message
	}
	return strings.TrimSpace(strings.TrimRight(kept, " ,.-") + " " + Placeholder)
}
