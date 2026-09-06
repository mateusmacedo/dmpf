package resilience

import (
	"fmt"
	"slices"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
)

// NotApplicable is the declared absence of a policy: the reason is mandatory,
// because "we did not think about it" and "it does not apply here" must not
// look the same in the sheet (RES-21).
type NotApplicable struct {
	Reason string
}

// Field is a value or a declared absence. A field that is neither is a blank in
// the sheet, and Validate refuses it.
type Field[T any] struct {
	value   T
	set     bool
	skipped *NotApplicable
}

// Declare fills the field with a value.
func Declare[T any](value T) Field[T] { return Field[T]{value: value, set: true} }

// Skip marks the field as not applicable, with the reason.
func Skip[T any](reason string) Field[T] {
	return Field[T]{skipped: &NotApplicable{Reason: reason}}
}

// Get is the value and whether one was declared.
func (f Field[T]) Get() (T, bool) { return f.value, f.set }

// Skipped is the declared absence and whether the field carries one.
func (f Field[T]) Skipped() (NotApplicable, bool) {
	if f.skipped == nil {
		return NotApplicable{}, false
	}
	return *f.skipped, true
}

// Declared reports whether the field says anything at all — a value or a
// reason. A field that says nothing is a blank.
func (f Field[T]) Declared() bool { return f.set || f.skipped != nil }

// BreakerPolicy is the circuit breaker of RES-10.
type BreakerPolicy struct {
	Window     time.Duration
	Threshold  float64
	MinSamples int
	Cooldown   time.Duration
	Probes     int
}

// BulkheadPolicy is the pool of RES-13. Queue defaults to the size of the pool.
type BulkheadPolicy struct {
	Pool        int
	Queue       int
	Acquisition time.Duration
}

// BackoffPolicy is the wait between attempts, in the sheet's own terms so the
// sheet does not depend on the evaluator's types.
type BackoffPolicy struct {
	Base   time.Duration
	Factor int
	Cap    time.Duration
}

// ForRetry is the evaluator's backoff.
func (b BackoffPolicy) ForRetry() retry.Backoff {
	return retry.Backoff{Base: b.Base, Factor: b.Factor, Cap: b.Cap}
}

// RateLimitPolicy is outbound rate limiting. This delivery leaves it not
// applicable: admission by route and tenant belongs to another story.
type RateLimitPolicy struct {
	PerSecond int
	Burst     int
}

// CachePolicy is cache-aside. This delivery leaves it not applicable: the
// modelling is still open upstream.
type CachePolicy struct {
	TTL time.Duration
}

// Degradation is the declared behaviour when the dependency cannot answer
// (RES-37).
type Degradation string

const (
	// Fail propagates the failure, which is the platform default.
	Fail Degradation = "fail"
	// Degrade answers with a distinguishable partial result.
	Degrade Degradation = "degrade"
	// Ignore omits the dependency from the answer and counts the omission.
	Ignore Degradation = "ignore"
	// Defer hands the work to the outbox, and is refused in this delivery.
	Defer Degradation = "defer"
)

// Override records a departure from the defaults: what changed, to what, why
// and since when (RES-40). A change nobody can date is a change nobody can
// review.
type Override struct {
	Field  string
	Value  string
	Reason string
	Since  time.Time
}

// Field names of the sheet, used by Validate, Effective and the overrides.
const (
	FieldDeadline    = "deadline"
	FieldRetry       = "retry"
	FieldBudget      = "budget"
	FieldBackoff     = "backoff"
	FieldMaxAttempts = "max_attempts"
	FieldBreaker     = "breaker"
	FieldBulkhead    = "bulkhead"
	FieldRateLimit   = "rate_limit"
	FieldCache       = "cache"
	FieldDegradation = "degradation"
)

// Sheet is the resilience sheet of one dependency: the ten fields of RES-21,
// each a value or a declared absence, plus the overrides that departed from the
// defaults.
type Sheet struct {
	Dependency string

	Deadline    Field[time.Duration]
	Retry       Field[bool]
	Budget      Field[time.Duration]
	Backoff     Field[BackoffPolicy]
	MaxAttempts Field[int]
	Breaker     Field[BreakerPolicy]
	Bulkhead    Field[BulkheadPolicy]
	RateLimit   Field[RateLimitPolicy]
	Cache       Field[CachePolicy]
	Degradation Field[Degradation]

	Overrides []Override
}

// Platform defaults of FND-08 §3.3, §3.4, §4.3 and §4.4.
const (
	DefaultDeadline          = 2 * time.Second
	DefaultBudgetShare       = 2 // the budget is half of the remaining deadline
	DefaultMaxAttempts       = 3
	DefaultAsyncMaxAttempts  = 5
	DefaultBreakerWindow     = 30 * time.Second
	DefaultBreakerThreshold  = 0.5
	DefaultBreakerMinSamples = 20
	DefaultBreakerCooldown   = 30 * time.Second
	DefaultBreakerProbes     = 1
	DefaultBulkheadPool      = 16
	DefaultBulkheadAcquire   = 100 * time.Millisecond
	DefaultHopSlack          = 50 * time.Millisecond
)

// Defaults is the platform sheet for a dependency. Rate limiting and cache are
// declared not applicable with a reason, as RES-21 requires of an empty
// position, and degradation defaults to failing.
func Defaults(dependency string) Sheet {
	return Sheet{
		Dependency:  dependency,
		Deadline:    Declare(DefaultDeadline),
		Retry:       Declare(true),
		Budget:      Declare(DefaultDeadline / DefaultBudgetShare),
		Backoff:     Declare(BackoffPolicy{Base: retry.DefaultBase, Factor: retry.DefaultFactor, Cap: retry.DefaultCap}),
		MaxAttempts: Declare(DefaultMaxAttempts),
		Breaker: Declare(BreakerPolicy{
			Window:     DefaultBreakerWindow,
			Threshold:  DefaultBreakerThreshold,
			MinSamples: DefaultBreakerMinSamples,
			Cooldown:   DefaultBreakerCooldown,
			Probes:     DefaultBreakerProbes,
		}),
		Bulkhead: Declare(BulkheadPolicy{
			Pool:        DefaultBulkheadPool,
			Queue:       DefaultBulkheadPool,
			Acquisition: DefaultBulkheadAcquire,
		}),
		RateLimit:   Skip[RateLimitPolicy]("admissão por rota e tenant é de outra história (KRN-10)"),
		Cache:       Skip[CachePolicy]("modelagem de cache encaminhada em FND-08 §1.4"),
		Degradation: Declare(Fail),
	}
}

// Validate refuses a sheet with a blank: a field that declares neither a value
// nor a reason is a policy nobody decided (RES-21). Every blank is reported at
// once, so a reader fixes the sheet in one pass.
func (s Sheet) Validate() error {
	if s.Dependency == "" {
		return fmt.Errorf("%w: sheet declares no dependency", ErrBlankField)
	}

	blank := make([]string, 0, 10)
	for name, declared := range s.declarations() {
		if !declared {
			blank = append(blank, name)
		}
	}
	if len(blank) > 0 {
		slices.Sort(blank)
		return fmt.Errorf("%w: %s: %v declare neither a value nor a reason", ErrBlankField, s.Dependency, blank)
	}

	if deadline, ok := s.Deadline.Get(); ok && deadline <= 0 {
		return fmt.Errorf("%w: %s: the deadline is not positive (RES-05)", ErrBlankField, s.Dependency)
	}
	if mode, ok := s.Degradation.Get(); ok && mode == Defer {
		return ErrDeferIsOutbox
	}
	return nil
}

func (s Sheet) declarations() map[string]bool {
	return map[string]bool{
		FieldDeadline:    s.Deadline.Declared(),
		FieldRetry:       s.Retry.Declared(),
		FieldBudget:      s.Budget.Declared(),
		FieldBackoff:     s.Backoff.Declared(),
		FieldMaxAttempts: s.MaxAttempts.Declared(),
		FieldBreaker:     s.Breaker.Declared(),
		FieldBulkhead:    s.Bulkhead.Declared(),
		FieldRateLimit:   s.RateLimit.Declared(),
		FieldCache:       s.Cache.Declared(),
		FieldDegradation: s.Degradation.Declared(),
	}
}

// Effective is what is in use, field by field, as strings the bootstrap exposes
// as resource attributes and logs once at start-up. A field not applicable
// reports its reason, so the record says why a position is empty.
func (s Sheet) Effective() map[string]string {
	effective := make(map[string]string, 10)

	effective[FieldDeadline] = describe(s.Deadline)
	effective[FieldRetry] = describe(s.Retry)
	effective[FieldBudget] = describe(s.Budget)
	effective[FieldBackoff] = describe(s.Backoff)
	effective[FieldMaxAttempts] = describe(s.MaxAttempts)
	effective[FieldBreaker] = describe(s.Breaker)
	effective[FieldBulkhead] = describe(s.Bulkhead)
	effective[FieldRateLimit] = describe(s.RateLimit)
	effective[FieldCache] = describe(s.Cache)
	effective[FieldDegradation] = describe(s.Degradation)

	return effective
}

// NotApplicablePrefix marks an effective value that is a declared absence, so a
// reader distinguishes it from a value that happens to read like a sentence.
const NotApplicablePrefix = "não se aplica: "

func describe[T any](field Field[T]) string {
	if absence, skipped := field.Skipped(); skipped {
		return NotApplicablePrefix + absence.Reason
	}
	if value, ok := field.Get(); ok {
		return fmt.Sprintf("%v", value)
	}
	return ""
}
