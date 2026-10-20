package providerkit_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const (
	clauseIntoDestination  = "writes into the destination, out of the origin's sight"
	clauseDenied           = "denies without an accepting policy and writes nothing"
	clauseNoDestination    = "refuses an empty destination before the policy"
	clauseAbsentOrigin     = "hands an absent origin to the policy as nil"
	clauseWriteFailsClosed = "writes and reads nothing when the trail refuses the record"
	clauseProjectionLoad   = "loads the destination under the policy"
	clauseProjectionMiss   = "misses in the destination without probing the owner"
	clauseProjectionVer    = "honours the stored version"
	clauseProjectorNoTrail = "refuses to be built without a trail"
	clauseProjectionNoCtx  = "refuses when the carrier holds no context"
	clauseOwnTenant        = "refuses to project into the origin's own tenant"
	clauseRollback         = "rolls the destination back with the unit of work"
)

type projectorFor func(tx *memory.Tx, policy ports.ProjectionPolicy, recorder ports.AuditTrail) ports.Projector[string, probe]

func memoryProjection() providerkit.ProjectionSubject[string, probe] {
	return memoryProjectionOver(probeTable.Projector)
}

func memoryProjectionOver(build projectorFor) providerkit.ProjectionSubject[string, probe] {
	store := memory.New()
	return providerkit.ProjectionSubject[string, probe]{
		Within: func(ctx context.Context, policy ports.ProjectionPolicy, recorder ports.AuditTrail, fn func(ctx context.Context, p ports.Projector[string, probe]) error) error {
			uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) ports.Projector[string, probe] {
				return build(tx, policy, recorder)
			})
			return uow.Within(ctx, fn)
		},
		Reader:   probeTable.Reader(store),
		NewID:    func(n int) string { return "p-" + strconv.Itoa(n) },
		NewState: func(marker int) probe { return probe{Marker: marker} },
		Marker:   func(p probe) int { return p.Marker },
		Table:    "probes",
	}
}

func TestTheProjectionKitPassesOverTheMemoryRealization(t *testing.T) {
	v := providerkit.Projection(memoryProjection)
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped = %v, want none", v.Skipped)
	}
}

type carrierBound struct {
	ports.Projector[string, probe]
	scoped ports.Repository[string, probe]
}

func (p carrierBound) Load(ctx context.Context, destination ports.TenantID, id string) (probe, ports.Version, error) {
	if _, _, err := p.Projector.Load(ctx, destination, id); err != nil && !errors.Is(err, ports.ErrNotFound) {
		return probe{}, 0, err
	}
	return p.scoped.Load(ctx, id)
}

func (p carrierBound) Save(ctx context.Context, destination ports.TenantID, id string, state probe, expected ports.Version) error {
	if _, _, err := p.Projector.Load(ctx, destination, id); err != nil && !errors.Is(err, ports.ErrNotFound) {
		return err
	}
	return p.scoped.Save(ctx, id, state, expected)
}

type writingFirst struct {
	inner, quiet ports.Projector[string, probe]
	policy       ports.ProjectionPolicy
	recorder     ports.AuditTrail
}

func (p writingFirst) refused(ctx context.Context, destination ports.TenantID) bool {
	if _, ok := ports.ExecutionContextFrom(ctx); !ok {
		return true
	}
	origin := originOf(ctx)
	return destination == "" || (origin != nil && *origin == destination) || p.policy == nil || p.policy(ctx, origin, destination) != nil
}

func (p writingFirst) Load(ctx context.Context, destination ports.TenantID, id string) (probe, ports.Version, error) {
	if p.refused(ctx, destination) {
		return p.inner.Load(ctx, destination, id)
	}
	state, version, err := p.quiet.Load(ctx, destination, id)
	if recorded := p.recorder.Record(ctx, writeEvent(ctx, destination)); recorded != nil {
		return state, version, recorded
	}
	return state, version, err
}

func (p writingFirst) Save(ctx context.Context, destination ports.TenantID, id string, state probe, expected ports.Version) error {
	if p.refused(ctx, destination) {
		return p.inner.Save(ctx, destination, id, state, expected)
	}
	if err := p.quiet.Save(ctx, destination, id, state, expected); err != nil {
		return err
	}
	return p.recorder.Record(ctx, writeEvent(ctx, destination))
}

func originOf(ctx context.Context) *ports.TenantID {
	if execution, ok := ports.ExecutionContextFrom(ctx); ok {
		if tenant, ok := execution.Tenant(); ok {
			return &tenant
		}
	}
	return nil
}

func writeEvent(ctx context.Context, destination ports.TenantID) ports.AuditRecord {
	event := readEvent(ctx, destination)
	event.Action = ports.AuditProjectionWrite
	return event
}

type probingProjector struct{ ports.Projector[string, probe] }

func (p probingProjector) Load(ctx context.Context, destination ports.TenantID, id string) (probe, ports.Version, error) {
	state, version, err := p.Projector.Load(ctx, destination, id)
	if errors.Is(err, ports.ErrNotFound) {
		return state, version, ports.CrossTenantAccess{Object: "probes/" + id, ContextTenant: destination, DataTenant: "dmpf-kit-a"}
	}
	return state, version, err
}

// The lost update UOW-09 refuses; it peeks over a trail of its own so only the
// version is sabotaged.
type versionBlind struct {
	ports.Projector[string, probe]
	peek ports.Projector[string, probe]
}

func (p versionBlind) Save(ctx context.Context, destination ports.TenantID, id string, state probe, expected ports.Version) error {
	if _, found, err := p.peek.Load(ctx, destination, id); err == nil || errors.Is(err, ports.ErrNotFound) {
		expected = found
	}
	return p.Projector.Save(ctx, destination, id, state, expected)
}

type contextMounting struct{ ports.Projector[string, probe] }

func (p contextMounting) mounted(ctx context.Context) context.Context {
	if _, ok := ports.ExecutionContextFrom(ctx); ok {
		return ctx
	}
	return withoutTenant(ctx)
}

func (p contextMounting) Load(ctx context.Context, destination ports.TenantID, id string) (probe, ports.Version, error) {
	return p.Projector.Load(p.mounted(ctx), destination, id)
}

func (p contextMounting) Save(ctx context.Context, destination ports.TenantID, id string, state probe, expected ports.Version) error {
	return p.Projector.Save(p.mounted(ctx), destination, id, state, expected)
}

type ownTenantOpen struct{ ports.Projector[string, probe] }

func (p ownTenantOpen) opened(ctx context.Context, destination ports.TenantID) context.Context {
	if origin := originOf(ctx); origin != nil && *origin == destination {
		return withoutTenant(ctx)
	}
	return ctx
}

func (p ownTenantOpen) Load(ctx context.Context, destination ports.TenantID, id string) (probe, ports.Version, error) {
	return p.Projector.Load(p.opened(ctx, destination), destination, id)
}

func (p ownTenantOpen) Save(ctx context.Context, destination ports.TenantID, id string, state probe, expected ports.Version) error {
	return p.Projector.Save(p.opened(ctx, destination), destination, id, state, expected)
}

func committingAnyway() providerkit.ProjectionSubject[string, probe] {
	s := memoryProjection()
	within := s.Within
	s.Within = func(ctx context.Context, policy ports.ProjectionPolicy, recorder ports.AuditTrail, fn func(ctx context.Context, p ports.Projector[string, probe]) error) error {
		var failed error
		committed := within(ctx, policy, recorder, func(ctx context.Context, p ports.Projector[string, probe]) error {
			failed = fn(ctx, p)
			return nil
		})
		return errors.Join(committed, failed)
	}
	return s
}

func withoutTenant(ctx context.Context) context.Context {
	subject := ports.SubjectID("dmpf-kit-subject")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "dmpf-kit-request", CorrelationID: "dmpf-kit-correlation", TraceContext: "dmpf-kit-trace",
		Subject: &subject, Permissions: []ports.Permission{}, Deadline: 1_755_432_000_000_000_000, Locale: "en",
	})
	if err != nil {
		panic(err)
	}
	return ports.WithExecutionContext(ctx, execution)
}

func allowAll(context.Context, *ports.TenantID, ports.TenantID) error { return nil }

func TestEveryProjectionSabotageIsReprovedOnTheClauseItBreaks(t *testing.T) {
	for _, tc := range []struct {
		name    string
		subject func() providerkit.ProjectionSubject[string, probe]
		want    []string
	}{
		{
			name: "written into the carrier's tenant",
			subject: func() providerkit.ProjectionSubject[string, probe] {
				return memoryProjectionOver(func(tx *memory.Tx, policy ports.ProjectionPolicy, recorder ports.AuditTrail) ports.Projector[string, probe] {
					return carrierBound{Projector: probeTable.Projector(tx, policy, recorder), scoped: probeTable.Repository(tx)}
				})
			},
			want: []string{clauseIntoDestination, clauseAbsentOrigin, clauseProjectionMiss, clauseProjectionVer},
		},
		{
			name: "policy not in effect",
			subject: func() providerkit.ProjectionSubject[string, probe] {
				return memoryProjectionOver(func(tx *memory.Tx, policy ports.ProjectionPolicy, recorder ports.AuditTrail) ports.Projector[string, probe] {
					consulted := func(ctx context.Context, origin *ports.TenantID, destination ports.TenantID) error {
						if policy != nil {
							_ = policy(ctx, origin, destination)
						}
						return nil
					}
					return probeTable.Projector(tx, consulted, recorder)
				})
			},
			want: []string{clauseDenied},
		},
		{
			name: "record emitted after the write",
			subject: func() providerkit.ProjectionSubject[string, probe] {
				return memoryProjectionOver(func(tx *memory.Tx, policy ports.ProjectionPolicy, recorder ports.AuditTrail) ports.Projector[string, probe] {
					if recorder == nil {
						return probeTable.Projector(tx, policy, recorder)
					}
					return writingFirst{
						inner:    probeTable.Projector(tx, policy, recorder),
						quiet:    probeTable.Projector(tx, allowAll, quietRecorder()),
						policy:   policy,
						recorder: recorder,
					}
				})
			},
			want: []string{clauseWriteFailsClosed},
		},
		{
			name: "origin invented for an absent tenant",
			subject: func() providerkit.ProjectionSubject[string, probe] {
				return memoryProjectionOver(func(tx *memory.Tx, policy ports.ProjectionPolicy, recorder ports.AuditTrail) ports.Projector[string, probe] {
					if policy == nil {
						return probeTable.Projector(tx, nil, recorder)
					}
					invented := func(ctx context.Context, origin *ports.TenantID, destination ports.TenantID) error {
						if origin == nil {
							system := ports.TenantID("system")
							origin = &system
						}
						return policy(ctx, origin, destination)
					}
					return probeTable.Projector(tx, invented, recorder)
				})
			},
			want: []string{clauseAbsentOrigin},
		},
		{
			name: "owner probed on a miss",
			subject: func() providerkit.ProjectionSubject[string, probe] {
				return memoryProjectionOver(func(tx *memory.Tx, policy ports.ProjectionPolicy, recorder ports.AuditTrail) ports.Projector[string, probe] {
					return probingProjector{probeTable.Projector(tx, policy, recorder)}
				})
			},
			want: []string{clauseProjectionMiss},
		},
		{
			name: "stored version ignored",
			subject: func() providerkit.ProjectionSubject[string, probe] {
				return memoryProjectionOver(func(tx *memory.Tx, policy ports.ProjectionPolicy, recorder ports.AuditTrail) ports.Projector[string, probe] {
					return versionBlind{Projector: probeTable.Projector(tx, policy, recorder), peek: probeTable.Projector(tx, allowAll, quietRecorder())}
				})
			},
			want: []string{clauseProjectionVer},
		},
		{
			name: "context mounted for a bare carrier",
			subject: func() providerkit.ProjectionSubject[string, probe] {
				return memoryProjectionOver(func(tx *memory.Tx, policy ports.ProjectionPolicy, recorder ports.AuditTrail) ports.Projector[string, probe] {
					return contextMounting{probeTable.Projector(tx, policy, recorder)}
				})
			},
			want: []string{clauseProjectionNoCtx},
		},
		{
			name: "own tenant opened to the projection",
			subject: func() providerkit.ProjectionSubject[string, probe] {
				return memoryProjectionOver(func(tx *memory.Tx, policy ports.ProjectionPolicy, recorder ports.AuditTrail) ports.Projector[string, probe] {
					return ownTenantOpen{probeTable.Projector(tx, policy, recorder)}
				})
			},
			want: []string{clauseOwnTenant},
		},
		{
			name:    "written outside the unit of work",
			subject: committingAnyway,
			want:    []string{clauseRollback},
		},
		{
			name: "built without a trail",
			subject: func() providerkit.ProjectionSubject[string, probe] {
				return memoryProjectionOver(func(tx *memory.Tx, policy ports.ProjectionPolicy, recorder ports.AuditTrail) ports.Projector[string, probe] {
					if recorder == nil {
						recorder = quietRecorder()
					}
					return probeTable.Projector(tx, policy, recorder)
				})
			},
			want: []string{clauseProjectorNoTrail},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := providerkit.Projection(tc.subject)
			for _, clause := range tc.want {
				if !reprovedOn(v, clause) {
					t.Errorf("clause %q not reproved: %v", clause, v.Failures())
				}
			}
			if extra := reprovedBeyond(v, tc.want); len(extra) > 0 {
				t.Errorf("reproved beyond %v: %v", tc.want, extra)
			}
		})
	}
}
