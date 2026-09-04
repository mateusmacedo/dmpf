package resilience

// Categories of the failures this package produces. They are the platform's
// own, because FND-07 has no realization in the kernel and a decorator must
// still say what kind of failure it is (ERR-11, TRC-12).
const (
	CategoryBreakerOpen       = "breaker_open"
	CategoryBulkheadSaturated = "bulkhead_saturated"
	CategoryDeadlineExceeded  = "deadline_exceeded"
	CategoryCancelled         = "cancelled"
	CategoryConfiguration     = "configuration"
)

// Error is a failure of a decorator. It carries a category and a code so
// redaction can report the shape of the failure without the message, and it is
// comparable by identity through errors.Is.
type Error struct {
	category string
	code     string
	message  string
}

func (e *Error) Error() string { return e.message }

// ErrorCategory satisfies what redaction reads, so the category reaches a log
// or a span and the message does not.
func (e *Error) ErrorCategory() string { return e.category }

// ErrorCode is the stable identifier of this failure.
func (e *Error) ErrorCode() string { return e.code }

// The failures the decorators return. Each one is a distinct value, so a caller
// tells them apart with errors.Is instead of matching on text.
var (
	// ErrBreakerOpen is a call refused because the breaker is open. It consumes
	// neither the timeout nor an attempt (RES-12).
	ErrBreakerOpen = &Error{
		category: CategoryBreakerOpen,
		code:     "RES-12",
		message:  "resilience: the circuit breaker is open and the call was refused",
	}

	// ErrBulkheadSaturated is a call refused because the pool and its queue are
	// full. Saturation is a fast rejection, never a wait (RES-14).
	ErrBulkheadSaturated = &Error{
		category: CategoryBulkheadSaturated,
		code:     "RES-14",
		message:  "resilience: the bulkhead is saturated and the call was refused",
	}

	// ErrDeferIsOutbox refuses the Defer degradation mode in this delivery: the
	// mechanism is the outbox, which belongs to another story.
	ErrDeferIsOutbox = &Error{
		category: CategoryConfiguration,
		code:     "RES-37",
		message:  "resilience: the defer degradation mode is realized by the outbox and is not available here",
	}

	// ErrWrapsUnitOfWork refuses, at construction, a retry decorator around a
	// unit of work: repeating a transaction is the caller's decision, never a
	// policy the composition takes on its own (RES-25, RES-34).
	ErrWrapsUnitOfWork = &Error{
		category: CategoryConfiguration,
		code:     "RES-25",
		message:  "resilience: a retry decorator must not wrap a unit of work",
	}

	// ErrOrderReasonRequired refuses a composition order other than the
	// canonical one without a declared reason (RES-22).
	ErrOrderReasonRequired = &Error{
		category: CategoryConfiguration,
		code:     "RES-22",
		message:  "resilience: an order other than the canonical one requires a declared reason",
	}

	// ErrDeadlineExceeded is a call that ran out of its effective deadline. It
	// is a category of its own, told apart from a cancellation, because one is
	// the platform running late and the other is the caller giving up (CTX-28).
	ErrDeadlineExceeded = &Error{
		category: CategoryDeadlineExceeded,
		code:     "RES-06",
		message:  "resilience: the effective deadline of the call was exceeded",
	}

	// ErrCancelled is a call the caller gave up on.
	ErrCancelled = &Error{
		category: CategoryCancelled,
		code:     "CTX-28",
		message:  "resilience: the caller cancelled the call",
	}

	// ErrBlankField refuses a sheet with a field that declares neither a value
	// nor a reason: a blank is a policy nobody decided (RES-21).
	ErrBlankField = &Error{
		category: CategoryConfiguration,
		code:     "RES-21",
		message:  "resilience: the sheet has a blank field",
	}

	// ErrDeadlineComposition refuses a route whose hops cannot fit in the
	// remaining deadline (RES-07).
	ErrDeadlineComposition = &Error{
		category: CategoryConfiguration,
		code:     "RES-07",
		message:  "resilience: the deadlines of the route do not fit in the remaining time",
	}
)
