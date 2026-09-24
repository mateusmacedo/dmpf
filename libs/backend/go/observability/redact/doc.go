// Package redact removes sensitive content from attributes and errors before
// they leave the process, so telemetry carries the shape of a failure and never
// the data that caused it.
package redact
