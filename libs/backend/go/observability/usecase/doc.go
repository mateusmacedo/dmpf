// Package usecase realizes the instrumentation hook the application service
// calls: it opens the use case span, records the service metrics and forwards
// the audit event to a sink.
//
// It satisfies the interface structurally and does not import application,
// because provider → application is a forbidden cell. The composition root is
// what binds the two.
package usecase
