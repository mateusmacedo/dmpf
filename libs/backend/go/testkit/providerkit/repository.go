package providerkit

import (
	"context"
	"errors"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// RepositorySubject is what a realization gives the suite so it can observe
// Repository and Reader from the outside: a transaction to write in, the read
// side a query uses, and a state the suite can build and read back.
type RepositorySubject[ID comparable, S any] struct {
	Within func(ctx context.Context, fn func(ctx context.Context, repo ports.Repository[ID, S]) error) error
	Reader ports.Reader[ID, S]

	// NewID mints the nth identifier of a run, so two clauses never collide on
	// a realization the suite does not reset between them.
	NewID func(n int) ID

	// NewState builds a state carrying marker and Marker reads it back, so the
	// suite tells one tenant's row from another's without constraining S.
	NewState func(marker int) S
	Marker   func(S) int

	// TenantUnresolved is the sentinel the realization returns when the carrier
	// resolves no tenant. A realization that declares none leaves it nil and the
	// clause is reported as skipped.
	TenantUnresolved error
}

// The two tenants the suite writes as. They are named for the kit so a shared
// database keeps them apart from whatever the realization's own tests wrote.
const (
	kitTenantA = ports.TenantID("dmpf-kit-a")
	kitTenantB = ports.TenantID("dmpf-kit-b")
)

const kitDeadline = ports.Instant(1_755_432_000_000_000_000)

// Repository exercises the eight observable clauses of Repository and Reader
// over any realization; newSubject must return a subject over a fresh resource
// on every call. Five of them are the tenant scope of IDN-12..IDN-15, which is
// why the suite exists: a realization that isolates by discipline rather than
// by construction passes the other three and fails these.
func Repository[ID comparable, S any](newSubject func() RepositorySubject[ID, S]) Verdict {
	var v Verdict

	acme, err := scopedTo(kitTenantA)
	if err != nil {
		v.fail("mounts the execution context the edge would have mounted", "CTX-01", "NewExecutionContext() = %v", err)
		return v
	}
	globex, err := scopedTo(kitTenantB)
	if err != nil {
		v.fail("mounts the execution context the edge would have mounted", "CTX-01", "NewExecutionContext() = %v", err)
		return v
	}

	{
		const clause = "round-trips the state within the tenant"
		s := newSubject()
		id := s.NewID(1)
		if err := s.save(acme, id, s.NewState(3), 0); err != nil {
			v.fail(clause, "UOW-01", "Save() = %v, want nil", err)
		} else if state, version, err := s.Reader.Load(acme, id); err != nil {
			v.fail(clause, "UOW-11", "Load() = %v, want nil", err)
		} else if got := s.Marker(state); got != 3 || version != 1 {
			v.fail(clause, "UOW-11", "Load() = marker %d, v%d; want marker 3, v1", got, version)
		}
	}

	{
		const clause = "never returns a row of another tenant"
		s := newSubject()
		id := s.NewID(2)
		if err := s.save(acme, id, s.NewState(3), 0); err != nil {
			v.fail(clause, "IDN-12", "Save() = %v, want nil", err)
		} else if _, _, err := s.Reader.Load(globex, id); !errors.Is(err, ports.ErrNotFound) {
			// Telling "not yours" from "does not exist" is an enumeration
			// oracle, so the two answers have to be the same one.
			v.fail(clause, "IDN-13", "Load() from another tenant = %v, want ErrNotFound", err)
		} else if access, ok := crossTenant(err); !ok {
			v.fail(clause, "IDN-12", "Load() from another tenant = %v, want a CrossTenantAccess the security record can name", err)
		} else if access.ContextTenant != kitTenantB || access.DataTenant != kitTenantA {
			v.fail(clause, "IDN-12", "CrossTenantAccess names context %q and data %q, want %q and %q",
				access.ContextTenant, access.DataTenant, kitTenantB, kitTenantA)
		}
	}

	{
		const clause = "does not report an absent identifier as another tenant's"
		s := newSubject()
		id := s.NewID(8)
		if _, _, err := s.Reader.Load(acme, id); !errors.Is(err, ports.ErrNotFound) {
			v.fail(clause, "IDN-13", "Load() of an absent identifier = %v, want ErrNotFound", err)
		} else if _, ok := crossTenant(err); ok {
			v.fail(clause, "IDN-13", "Load() of an absent identifier = %v; the internal record would log an access that never happened", err)
		}
	}

	{
		const clause = "keeps the same identifier apart per tenant"
		s := newSubject()
		id := s.NewID(3)
		first := s.save(acme, id, s.NewState(3), 0)
		second := s.save(globex, id, s.NewState(7), 0)
		switch {
		case first != nil:
			v.fail(clause, "IDN-12", "Save() for the first tenant = %v, want nil", first)
		case second != nil:
			v.fail(clause, "IDN-12", "Save() of the same identifier for a second tenant = %v, want nil — the scope is part of the key", second)
		default:
			expectRow(&v, clause, "IDN-12", s, acme, id, 3, 1, "the first tenant")
			expectRow(&v, clause, "IDN-12", s, globex, id, 7, 1, "the second tenant")
		}
	}

	{
		const clause = "refuses a write over another tenant's row"
		s := newSubject()
		id := s.NewID(4)
		if err := s.save(acme, id, s.NewState(3), 0); err != nil {
			v.fail(clause, "IDN-12", "Save() = %v, want nil", err)
		} else {
			// Version 1 is what the first tenant's row holds: the second must
			// not reach it even knowing the version.
			if err := s.save(globex, id, s.NewState(99), 1); !errors.Is(err, ports.ErrVersionConflict) {
				v.fail(clause, "IDN-12", "Save() over another tenant's row = %v, want ErrVersionConflict", err)
			}
			expectRow(&v, clause, "IDN-12", s, acme, id, 3, 1, "the first tenant's row")
		}
	}

	{
		const clause = "refuses when the carrier resolved no tenant"
		s := newSubject()
		if s.TenantUnresolved == nil {
			v.skip(clause)
		} else if bare, err := tenantless(); err != nil {
			v.fail(clause, "CTX-01", "NewExecutionContext() = %v", err)
		} else {
			id := s.NewID(5)
			if _, _, err := s.Reader.Load(bare, id); !errors.Is(err, s.TenantUnresolved) {
				v.fail(clause, "IDN-15", "Load() without a tenant = %v, want the realization's unresolved-tenant sentinel", err)
			}
			if err := s.save(bare, id, s.NewState(1), 0); !errors.Is(err, s.TenantUnresolved) {
				v.fail(clause, "IDN-15", "Save() without a tenant = %v, want the realization's unresolved-tenant sentinel", err)
			}
		}
	}

	{
		const clause = "refuses when the carrier holds no context"
		s := newSubject()
		id := s.NewID(6)
		if _, _, err := s.Reader.Load(context.Background(), id); !errors.Is(err, ports.ErrContextAbsent) {
			v.fail(clause, "IDN-15", "Load() off a bare context = %v, want ErrContextAbsent", err)
		}
		if err := s.save(context.Background(), id, s.NewState(1), 0); !errors.Is(err, ports.ErrContextAbsent) {
			v.fail(clause, "IDN-15", "Save() off a bare context = %v, want ErrContextAbsent", err)
		}
	}

	{
		const clause = "honours the stored version"
		s := newSubject()
		id := s.NewID(7)
		created := s.save(acme, id, s.NewState(1), 0)
		updated := s.save(acme, id, s.NewState(2), 1)
		switch {
		case created != nil:
			v.fail(clause, "UOW-09", "Save() creating the aggregate = %v, want nil", created)
		case updated != nil:
			v.fail(clause, "UOW-09", "Save() at the stored version = %v, want nil", updated)
		default:
			if err := s.save(acme, id, s.NewState(3), 1); !errors.Is(err, ports.ErrVersionConflict) {
				v.fail(clause, "UOW-09", "Save() at a stale version = %v, want ErrVersionConflict", err)
			}
			// A refused write keeps nothing: the row is still the second one.
			expectRow(&v, clause, "UOW-09", s, acme, id, 2, 2, "the row")
		}
	}

	return v
}

// expectRow reads a row back and names what it found. It reports the load error
// on its own rather than decoding a zero state: S may hold a pointer the
// realization never filled. A wantVersion of 0 leaves the version unchecked.
func expectRow[ID comparable, S any](v *Verdict, clause, rule string, s RepositorySubject[ID, S], ctx context.Context, id ID, wantMarker int, wantVersion ports.Version, whose string) {
	state, version, err := s.Reader.Load(ctx, id)
	if err != nil {
		v.fail(clause, rule, "Load() for %s = %v, want nil", whose, err)
		return
	}
	if got := s.Marker(state); got != wantMarker {
		v.fail(clause, rule, "%s reads marker %d, want %d", whose, got, wantMarker)
	}
	if wantVersion != 0 && version != wantVersion {
		v.fail(clause, rule, "%s is at v%d, want v%d", whose, version, wantVersion)
	}
}

func crossTenant(err error) (ports.CrossTenantAccess, bool) {
	var access ports.CrossTenantAccess
	return access, errors.As(err, &access)
}

func (s RepositorySubject[ID, S]) save(ctx context.Context, id ID, state S, expected ports.Version) error {
	return s.Within(ctx, func(ctx context.Context, repo ports.Repository[ID, S]) error {
		return repo.Save(ctx, id, state, expected)
	})
}

func scopedTo(tenant ports.TenantID) (context.Context, error) {
	subject := ports.SubjectID("dmpf-kit-subject")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID:     "dmpf-kit-request",
		CorrelationID: "dmpf-kit-correlation",
		TraceContext:  "dmpf-kit-trace",
		Subject:       &subject,
		Tenant:        &tenant,
		Permissions:   []ports.Permission{},
		Deadline:      kitDeadline,
		Locale:        "en",
	})
	if err != nil {
		return nil, err
	}
	return ports.WithExecutionContext(context.Background(), execution), nil
}

// tenantless is the caller that authenticated and resolved no tenant, which
// IDN-15 answers with a refusal rather than an unscoped query.
func tenantless() (context.Context, error) {
	subject := ports.SubjectID("dmpf-kit-subject")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID:     "dmpf-kit-request",
		CorrelationID: "dmpf-kit-correlation",
		TraceContext:  "dmpf-kit-trace",
		Subject:       &subject,
		Permissions:   []ports.Permission{},
		Deadline:      kitDeadline,
		Locale:        "en",
	})
	if err != nil {
		return nil, err
	}
	return ports.WithExecutionContext(context.Background(), execution), nil
}
