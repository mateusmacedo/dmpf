// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-08 (RES-16, RES-17, MET-07) que o símbolo realiza, dentro do limite de 3 linhas.

package admission

import (
	"container/list"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
)

// Reason is the category of an admission decision (RES-17). Admitted is the
// only one that grants entry.
type Reason string

const (
	Admitted        Reason = "admitted"
	RateLimited     Reason = "rate_limited"
	Saturated       Reason = "saturated"
	UndeclaredRoute Reason = "undeclared_route"
)

var (
	// ErrInvalidLimit is a limit with a rate, burst or concurrency that is not
	// positive: a route without a declared limit is not exposed (RES-16).
	ErrInvalidLimit = errors.New("admission: rate, burst and concurrency must be positive (RES-16)")

	// ErrNoRoutes is a configuration that declares no route at all.
	ErrNoRoutes = errors.New("admission: at least one route must declare a limit (RES-16)")

	// ErrNoClock is a configuration without a clock.
	ErrNoClock = errors.New("admission: a clock is required")

	// ErrNoKeyCeiling is a configuration without a ceiling on the number of
	// (route, tenant) buckets, which is the limit MET-07 imposes.
	ErrNoKeyCeiling = errors.New("admission: MaxKeys must be positive (MET-07)")
)

// Limit is what one route admits: PerSecond tokens refilled up to Burst, and
// at most Concurrency requests in flight at once (RES-16).
type Limit struct {
	PerSecond   float64
	Burst       int
	Concurrency int
}

// Validate refuses a limit any of whose three terms is not positive.
func (l Limit) Validate() error {
	if l.PerSecond <= 0 || l.Burst <= 0 || l.Concurrency <= 0 {
		return fmt.Errorf("%w: %+v", ErrInvalidLimit, l)
	}
	return nil
}

// Config declares the limits per route, the tenant allowlist the metric label
// is resolved against, the ceiling on the number of live buckets and the clock.
type Config struct {
	Limits  map[string]Limit
	Tenants metrics.Tenants
	MaxKeys int
	Clock   clock.Clock
}

// Validate refuses a configuration the controller could not decide with.
func (c Config) Validate() error {
	switch {
	case len(c.Limits) == 0:
		return ErrNoRoutes
	case c.Clock == nil:
		return ErrNoClock
	case c.MaxKeys <= 0:
		return ErrNoKeyCeiling
	}
	for route, limit := range c.Limits {
		if err := limit.Validate(); err != nil {
			return fmt.Errorf("%s: %w", route, err)
		}
	}
	return nil
}

// Controller admits or refuses per (route, tenant). It is safe for concurrent
// use; every decision holds one lock for a bounded amount of work.
type Controller struct {
	cfg Config

	mu      sync.Mutex
	buckets map[key]*list.Element
	lru     *list.List
}

type key struct {
	route  string
	tenant string
}

type bucket struct {
	key      key
	tokens   float64
	refilled time.Time
	inflight int
}

// New validates the configuration and returns a controller with no buckets:
// each (route, tenant) is created on first sight and evicted when idle.
func New(cfg Config) (*Controller, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	limits := make(map[string]Limit, len(cfg.Limits))
	for route, limit := range cfg.Limits {
		limits[route] = limit
	}
	cfg.Limits = limits
	return &Controller{cfg: cfg, buckets: map[key]*list.Element{}, lru: list.New()}, nil
}

// Admit decides for one request. The returned release is never nil: it frees
// the concurrency slot exactly once and is a no-op on a refusal, so the caller
// may always defer it. The bucket is the tenant's own; MaxKeys bounds them.
func (c *Controller) Admit(route, tenant string) (release func(), reason Reason) {
	noop := func() {}

	limit, declared := c.cfg.Limits[route]
	if !declared {
		return noop, UndeclaredRoute
	}
	k := key{route: route, tenant: tenant}
	now := c.cfg.Clock.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	b, ok := c.lookup(k, limit, now)
	if !ok {
		return noop, Saturated
	}
	if b.inflight >= limit.Concurrency {
		return noop, Saturated
	}

	elapsed := now.Sub(b.refilled)
	if elapsed > 0 {
		b.tokens = min(float64(limit.Burst), b.tokens+elapsed.Seconds()*limit.PerSecond)
		b.refilled = now
	}
	if b.tokens < 1 {
		return noop, RateLimited
	}

	b.tokens--
	b.inflight++
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			b.inflight--
		})
	}, Admitted
}

// Tenants is the allowlist the controller resolves tenants against, so the
// caller labels its refusal metric with the same collapse (MET-12).
func (c *Controller) Tenants() metrics.Tenants { return c.cfg.Tenants }

// Keys reports how many (route, tenant) buckets are live.
func (c *Controller) Keys() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lru.Len()
}

// lookup returns the bucket for the key, creating it under the ceiling. When
// the ceiling is reached it evicts the least recently used idle bucket; with
// no idle bucket to evict it reports false and the request is refused.
func (c *Controller) lookup(k key, limit Limit, now time.Time) (*bucket, bool) {
	if element, found := c.buckets[k]; found {
		c.lru.MoveToFront(element)
		return element.Value.(*bucket), true
	}

	if c.lru.Len() >= c.cfg.MaxKeys && !c.evictIdle() {
		return nil, false
	}

	b := &bucket{key: k, tokens: float64(limit.Burst), refilled: now}
	c.buckets[k] = c.lru.PushFront(b)
	return b, true
}

func (c *Controller) evictIdle() bool {
	for element := c.lru.Back(); element != nil; element = element.Prev() {
		b := element.Value.(*bucket)
		if b.inflight == 0 {
			c.lru.Remove(element)
			delete(c.buckets, b.key)
			return true
		}
	}
	return false
}
