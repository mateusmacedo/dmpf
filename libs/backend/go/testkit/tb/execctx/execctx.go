package execctx

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var keys atomic.Int64

type Option func(*setup)

type setup struct {
	spec    ports.ExecutionContextSpec
	slot    bool
	autoKey bool
}

func WithSlot() Option { return func(s *setup) { s.slot = true } }

func AutoKey() Option { return func(s *setup) { s.autoKey = true } }

func WithTenant(tenant ports.TenantID) Option { return func(s *setup) { s.spec.Tenant = &tenant } }

// WithoutSubject drops the permission set with the subject: CTX-01 refuses a
// permission set that no resolved subject owns.
func WithoutSubject() Option {
	return func(s *setup) { s.spec.Subject, s.spec.Permissions = nil, nil }
}

func WithDeadline(deadline ports.Instant) Option {
	return func(s *setup) { s.spec.Deadline = deadline }
}

func newSetup(opts []Option) setup {
	subject, tenant := ports.SubjectID("s-test"), ports.TenantID("acme")
	s := setup{spec: ports.ExecutionContextSpec{
		RequestID:     "r-test",
		CorrelationID: "c-test",
		TraceContext:  "t-test",
		Subject:       &subject,
		Tenant:        &tenant,
		Permissions:   []ports.Permission{},
		Deadline:      ports.Instant(1_755_432_000_000_000_000),
		Locale:        "en",
	}}
	for _, opt := range opts {
		opt(&s)
	}
	return s
}

func Execution(t testing.TB, opts ...Option) ports.ExecutionContext {
	t.Helper()
	execution, err := ports.NewExecutionContext(newSetup(opts).spec)
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	return execution
}

// WithExecution deposits on the carrier what the edge would have mounted, so a
// test exercises the path production does (CTX-03, ADR-049).
func WithExecution(t testing.TB, ctx context.Context, opts ...Option) context.Context {
	t.Helper()
	s := newSetup(opts)
	ctx = ports.WithExecutionContext(ctx, Execution(t, opts...))
	if s.slot {
		ctx = ports.WithIdempotencySlot(ctx)
	}
	if s.autoKey {
		ctx = ports.WithIdempotencyKey(ctx, fmt.Sprintf("k-%d", keys.Add(1)))
	}
	return ctx
}

func WithKey(t testing.TB, ctx context.Context, key string, opts ...Option) context.Context {
	t.Helper()
	return ports.WithIdempotencyKey(WithExecution(t, ctx, opts...), key)
}
