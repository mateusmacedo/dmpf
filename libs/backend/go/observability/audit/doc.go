// Package audit is the audit trail of LOG-14: the event, the JSON sink that
// writes it and the recording sink that tests read back.
//
// No constructor accepts a slog.Handler and no sink is sampled: an audit record
// that could be dropped is not an audit record.
package audit
