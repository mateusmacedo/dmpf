package providerkit_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const (
	clauseSeesEveryTenant = "lists the rows of every tenant with the tenant of each"
	clauseKeyset          = "pages by key without repeating or losing a row"
	clauseUndeclared      = "refuses without a declared reach and records the refusal"
	clausePlatformNoCtx   = "refuses when the carrier holds no context"
	clauseLimit           = "refuses a limit out of range"
	clauseReadEvents      = "records one accepted event per tenant reached"
	clauseReadFailsClosed = "returns nothing when the trail refuses the record"
	clauseEmptyPage       = "records nothing for an empty page"
	clauseLoadIn          = "loads the row of the tenant asked"
	clauseLoadInMiss      = "misses in the tenant asked without probing the owner"
	clauseReaderScoped    = "keeps the scoped Reader filtered under the declared reach"
	clauseReaderNoTrail   = "refuses to be built without a trail"
)

func memoryPlatform() providerkit.PlatformSubject[string, probe] {
	return memoryPlatformWrapped(nil)
}

func memoryPlatformWrapped(wrap func(store *memory.Store, recorder ports.AuditTrail, pageMax int) ports.PlatformReader[string, probe]) providerkit.PlatformSubject[string, probe] {
	store := memory.New()
	s := providerkit.PlatformSubject[string, probe]{
		Within: func(ctx context.Context, fn func(ctx context.Context, repo ports.Repository[string, probe]) error) error {
			uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) probeResources {
				return probeResources{Probes: probeTable.Repository(tx)}
			})
			return uow.Within(ctx, func(ctx context.Context, res probeResources) error { return fn(ctx, res.Probes) })
		},
		Reader: probeTable.Reader(store),
		Platform: func(recorder ports.AuditTrail, pageMax int) ports.PlatformReader[string, probe] {
			return probeTable.PlatformReader(store, recorder, memory.WithPageMax(pageMax))
		},
		NewID:    func(n int) string { return "p-" + strconv.Itoa(n) },
		NewState: func(marker int) probe { return probe{Marker: marker} },
		Marker:   func(p probe) int { return p.Marker },
		Table:    "probes",
	}
	if wrap != nil {
		s.Platform = func(recorder ports.AuditTrail, pageMax int) ports.PlatformReader[string, probe] {
			return wrap(store, recorder, pageMax)
		}
	}
	return s
}

func TestThePlatformKitPassesOverTheMemoryRealization(t *testing.T) {
	v := providerkit.Platform(memoryPlatform)
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped = %v, want none", v.Skipped)
	}
}

func quietRecorder() ports.AuditTrail { return confirmingTrail{} }

type confirmingTrail struct{}

func (confirmingTrail) Record(context.Context, ports.AuditRecord) error { return nil }

type reachForcing struct {
	inner ports.PlatformReader[string, probe]
}

func (r reachForcing) LoadIn(ctx context.Context, tenant ports.TenantID, id string) (ports.PlatformRow[string, probe], error) {
	return r.inner.LoadIn(ports.WithPlatformReach(ctx, ports.ReachAllTenants), tenant, id)
}

func (r reachForcing) List(ctx context.Context, after *ports.PlatformKey[string], limit int) ([]ports.PlatformRow[string, probe], error) {
	return r.inner.List(ports.WithPlatformReach(ctx, ports.ReachAllTenants), after, limit)
}

type emittingAfter struct {
	inner, quiet ports.PlatformReader[string, probe]
	recorder     ports.AuditTrail
}

func (r emittingAfter) LoadIn(ctx context.Context, tenant ports.TenantID, id string) (ports.PlatformRow[string, probe], error) {
	if reach, _ := ports.PlatformReachFrom(ctx); reach != ports.ReachAllTenants || tenant == "" {
		return r.inner.LoadIn(ctx, tenant, id)
	}
	row, err := r.quiet.LoadIn(ctx, tenant, id)
	if err == nil || errors.Is(err, ports.ErrNotFound) {
		_ = r.recorder.Record(ctx, readEvent(ctx, tenant))
	}
	return row, err
}

func (r emittingAfter) List(ctx context.Context, after *ports.PlatformKey[string], limit int) ([]ports.PlatformRow[string, probe], error) {
	if reach, _ := ports.PlatformReachFrom(ctx); reach != ports.ReachAllTenants {
		return r.inner.List(ctx, after, limit)
	}
	rows, err := r.quiet.List(ctx, after, limit)
	var reached []ports.TenantID
	for _, row := range rows {
		if !slices.Contains(reached, row.Tenant) {
			reached = append(reached, row.Tenant)
			_ = r.recorder.Record(ctx, readEvent(ctx, row.Tenant))
		}
	}
	return rows, err
}

func readEvent(ctx context.Context, tenant ports.TenantID) ports.AuditRecord {
	var own string
	if execution, ok := ports.ExecutionContextFrom(ctx); ok {
		if t, ok := execution.Tenant(); ok {
			own = string(t)
		}
	}
	return ports.AuditRecord{Subject: ports.SubjectOf(ctx), Object: "probes", Action: ports.AuditPlatformRead, Outcome: "accepted", Tenant: own, DataTenant: string(tenant)}
}

type swallowing struct {
	inner, quiet ports.PlatformReader[string, probe]
}

func (r swallowing) LoadIn(ctx context.Context, tenant ports.TenantID, id string) (ports.PlatformRow[string, probe], error) {
	row, err := r.inner.LoadIn(ctx, tenant, id)
	if errors.Is(err, ports.ErrAuditUnavailable) {
		return r.quiet.LoadIn(ctx, tenant, id)
	}
	return row, err
}

func (r swallowing) List(ctx context.Context, after *ports.PlatformKey[string], limit int) ([]ports.PlatformRow[string, probe], error) {
	rows, err := r.inner.List(ctx, after, limit)
	if errors.Is(err, ports.ErrAuditUnavailable) {
		return r.quiet.List(ctx, after, limit)
	}
	return rows, err
}

type recordingPerRow struct {
	ports.PlatformReader[string, probe]
	recorder ports.AuditTrail
}

func (r recordingPerRow) List(ctx context.Context, after *ports.PlatformKey[string], limit int) ([]ports.PlatformRow[string, probe], error) {
	rows, err := r.PlatformReader.List(ctx, after, limit)
	var reached []ports.TenantID
	for _, row := range rows {
		if slices.Contains(reached, row.Tenant) {
			_ = r.recorder.Record(ctx, readEvent(ctx, row.Tenant))
		}
		reached = append(reached, row.Tenant)
	}
	return rows, err
}

type cursorIgnoring struct {
	ports.PlatformReader[string, probe]
}

func (r cursorIgnoring) List(ctx context.Context, _ *ports.PlatformKey[string], limit int) ([]ports.PlatformRow[string, probe], error) {
	return r.PlatformReader.List(ctx, nil, limit)
}

type ownerProbing struct {
	ports.PlatformReader[string, probe]
}

func (r ownerProbing) LoadIn(ctx context.Context, tenant ports.TenantID, id string) (ports.PlatformRow[string, probe], error) {
	row, err := r.PlatformReader.LoadIn(ctx, tenant, id)
	if errors.Is(err, ports.ErrNotFound) {
		return row, ports.CrossTenantAccess{Object: "probes/" + id, ContextTenant: tenant, DataTenant: "dmpf-kit-a"}
	}
	return row, err
}

// The widening the IDN-19 carrier must never cause.
type widenedReader struct {
	inner    ports.Reader[string, probe]
	platform ports.PlatformReader[string, probe]
}

func (r widenedReader) Load(ctx context.Context, id string) (probe, ports.Version, error) {
	state, version, err := r.inner.Load(ctx, id)
	if reach, _ := ports.PlatformReachFrom(ctx); reach == ports.ReachAllTenants && errors.Is(err, ports.ErrNotFound) {
		for _, tenant := range []ports.TenantID{"dmpf-kit-a", "dmpf-kit-b"} {
			if row, err := r.platform.LoadIn(ctx, tenant, id); err == nil {
				return row.State, row.Version, nil
			}
		}
	}
	return state, version, err
}

func TestEveryPlatformSabotageIsReprovedOnTheClauseItBreaks(t *testing.T) {
	for _, tc := range []struct {
		name    string
		subject func() providerkit.PlatformSubject[string, probe]
		want    []string
	}{
		{
			name: "declaration ignored",
			subject: func() providerkit.PlatformSubject[string, probe] {
				return memoryPlatformWrapped(func(store *memory.Store, recorder ports.AuditTrail, pageMax int) ports.PlatformReader[string, probe] {
					return reachForcing{inner: probeTable.PlatformReader(store, recorder, memory.WithPageMax(pageMax))}
				})
			},
			want: []string{clauseUndeclared},
		},
		{
			name: "record emitted after the rows are out",
			subject: func() providerkit.PlatformSubject[string, probe] {
				return memoryPlatformWrapped(func(store *memory.Store, recorder ports.AuditTrail, pageMax int) ports.PlatformReader[string, probe] {
					return emittingAfter{
						inner:    probeTable.PlatformReader(store, recorder, memory.WithPageMax(pageMax)),
						quiet:    probeTable.PlatformReader(store, quietRecorder(), memory.WithPageMax(pageMax)),
						recorder: recorder,
					}
				})
			},
			want: []string{clauseReadFailsClosed},
		},
		{
			name: "refused record swallowed",
			subject: func() providerkit.PlatformSubject[string, probe] {
				return memoryPlatformWrapped(func(store *memory.Store, recorder ports.AuditTrail, pageMax int) ports.PlatformReader[string, probe] {
					return swallowing{
						inner: probeTable.PlatformReader(store, recorder, memory.WithPageMax(pageMax)),
						quiet: probeTable.PlatformReader(store, quietRecorder(), memory.WithPageMax(pageMax)),
					}
				})
			},
			want: []string{clauseReadFailsClosed},
		},
		{
			name: "one record per row",
			subject: func() providerkit.PlatformSubject[string, probe] {
				return memoryPlatformWrapped(func(store *memory.Store, recorder ports.AuditTrail, pageMax int) ports.PlatformReader[string, probe] {
					return recordingPerRow{PlatformReader: probeTable.PlatformReader(store, recorder, memory.WithPageMax(pageMax)), recorder: recorder}
				})
			},
			want: []string{clauseReadEvents},
		},
		{
			name: "cursor ignored",
			subject: func() providerkit.PlatformSubject[string, probe] {
				return memoryPlatformWrapped(func(store *memory.Store, recorder ports.AuditTrail, pageMax int) ports.PlatformReader[string, probe] {
					return cursorIgnoring{probeTable.PlatformReader(store, recorder, memory.WithPageMax(pageMax))}
				})
			},
			want: []string{clauseKeyset, clauseEmptyPage},
		},
		{
			name: "page ceiling ignored",
			subject: func() providerkit.PlatformSubject[string, probe] {
				return memoryPlatformWrapped(func(store *memory.Store, recorder ports.AuditTrail, _ int) ports.PlatformReader[string, probe] {
					return probeTable.PlatformReader(store, recorder)
				})
			},
			want: []string{clauseLimit},
		},
		{
			name: "owner probed on a miss",
			subject: func() providerkit.PlatformSubject[string, probe] {
				return memoryPlatformWrapped(func(store *memory.Store, recorder ports.AuditTrail, pageMax int) ports.PlatformReader[string, probe] {
					return ownerProbing{probeTable.PlatformReader(store, recorder, memory.WithPageMax(pageMax))}
				})
			},
			want: []string{clauseLoadInMiss},
		},
		{
			name: "built without a trail",
			subject: func() providerkit.PlatformSubject[string, probe] {
				return memoryPlatformWrapped(func(store *memory.Store, recorder ports.AuditTrail, pageMax int) ports.PlatformReader[string, probe] {
					if recorder == nil {
						recorder = quietRecorder()
					}
					return probeTable.PlatformReader(store, recorder, memory.WithPageMax(pageMax))
				})
			},
			want: []string{clauseReaderNoTrail},
		},
		{
			name: "scoped Reader widened by the reach",
			subject: func() providerkit.PlatformSubject[string, probe] {
				s := memoryPlatform()
				s.Reader = widenedReader{inner: s.Reader, platform: s.Platform(quietRecorder(), ports.PlatformPageMax)}
				return s
			},
			want: []string{clauseReaderScoped},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := providerkit.Platform(tc.subject)
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

// A clause that reproves every deviation must not pass for precise.
func reprovedBeyond(v providerkit.Verdict, want []string) []string {
	var extra []string
	for _, d := range v.Diagnostics {
		if !slices.Contains(want, d.Clause) && !slices.Contains(extra, d.Clause) {
			extra = append(extra, d.Clause)
		}
	}
	return extra
}
