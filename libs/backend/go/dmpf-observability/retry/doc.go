// Package retry is the pure evaluator of the retry conjunction: every factor
// must hold for one more attempt to be allowed. It owns the retryability
// classification, the budget carried in the context and the backoff.
//
// An absent or indeterminate classification resolves to not retryable, because
// the error taxonomy belongs to FND-07 and is consumed here, never defined.
package retry
