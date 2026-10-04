// Package audit is the audit trail of LOG-14: the event, the Logs API sink that
// emits it as dmpf.audit, the JSON sink and the recording sink that tests read
// back.
//
// No constructor accepts a slog.Handler and no sink is sampled: an audit record
// that could be dropped is not an audit record.
package audit
