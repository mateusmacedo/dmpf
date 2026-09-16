package retry

// Retryability is what a classifier reports about an error. The zero value is
// Unknown, so a classifier that says nothing denies the retry instead of
// allowing one by omission.
type Retryability int

const (
	// Unknown is an error the taxonomy did not classify (RES-29).
	Unknown Retryability = iota
	// Retryable is a transient error the caller may attempt again.
	Retryable
	// NotRetryable is a permanent error no attempt will fix.
	NotRetryable
)

func (r Retryability) String() string {
	switch r {
	case Retryable:
		return "retryable"
	case NotRetryable:
		return "not_retryable"
	default:
		return "unknown"
	}
}

// Classifier is the error taxonomy of FND-07 as an injected predicate. The
// kernel consumes it and never defines it: a nil classifier denies the retry.
type Classifier func(err error) Retryability
