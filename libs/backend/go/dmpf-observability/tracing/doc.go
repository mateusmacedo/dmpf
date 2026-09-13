// Package tracing is the closed attribute builder for spans and the recording of
// errors by category.
//
// An error is recorded with its category and never with its message, so a span
// cannot become a channel for content that redaction would have removed.
package tracing
