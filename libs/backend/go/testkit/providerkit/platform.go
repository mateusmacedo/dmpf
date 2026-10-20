package providerkit

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type PlatformSubject[ID comparable, S any] struct {
	Within   func(ctx context.Context, fn func(ctx context.Context, repo ports.Repository[ID, S]) error) error
	Reader   ports.Reader[ID, S]
	Platform func(trail ports.AuditTrail, pageMax int) ports.PlatformReader[ID, S]
	NewID    func(n int) ID
	NewState func(marker int) S
	Marker   func(S) int
	Table    string
}

// Platform exercises the platform read of IDN-19; newSubject must return a
// subject over a fresh, empty table on every call, because List reaches every
// row. The page order is the realization's: only a stable total order is asserted.
func Platform[ID comparable, S any](newSubject func() PlatformSubject[ID, S]) Verdict {
	var v Verdict

	acme, err := scopedTo(kitTenantA)
	if err != nil {
		v.fail("mounts the execution context the edge would have mounted", "CTX-01", "NewExecutionContext() = %v", err)
		return v
	}
	bare, err := tenantless()
	if err != nil {
		v.fail("mounts the execution context the edge would have mounted", "CTX-01", "NewExecutionContext() = %v", err)
		return v
	}
	reach := func(ctx context.Context) context.Context { return ports.WithPlatformReach(ctx, ports.ReachAllTenants) }

	{
		const clause = "lists the rows of every tenant with the tenant of each"
		s := newSubject()
		seeded := s.seed(&v, clause, []kitRow{{kitTenantA, 1, 11}, {kitTenantA, 2, 12}, {kitTenantB, 1, 21}, {kitTenantB, 3, 23}})
		if seeded != nil {
			rows, err := s.Platform(&recordingTrail{}, ports.PlatformPageMax).List(reach(acme), nil, ports.PlatformPageMax)
			if err != nil {
				v.fail(clause, "IDN-19", "List() = %v, want nil", err)
			} else {
				s.expectRows(&v, clause, rows, seeded)
			}
		}
	}

	{
		const clause = "pages by key without repeating or losing a row"
		s := newSubject()
		seeded := s.seed(&v, clause, []kitRow{{kitTenantA, 1, 11}, {kitTenantA, 2, 12}, {kitTenantA, 3, 13}, {kitTenantB, 1, 21}, {kitTenantB, 4, 24}})
		if seeded != nil {
			reader := s.Platform(&recordingTrail{}, 3)
			byTwo, twoErr := walk(reader, reach(acme), 2)
			byThree, threeErr := walk(reader, reach(acme), 3)
			switch {
			case twoErr != nil:
				v.fail(clause, "IDN-19", "paging by 2: %v", twoErr)
			case threeErr != nil:
				v.fail(clause, "IDN-19", "paging by 3: %v", threeErr)
			case !slices.Equal(keysOf(byTwo), keysOf(byThree)):
				v.fail(clause, "IDN-19", "paging by 2 yields %v and by 3 yields %v, want one stable order", keysOf(byTwo), keysOf(byThree))
			default:
				s.expectRows(&v, clause, byTwo, seeded)
			}
		}
	}

	{
		const clause = "refuses without a declared reach and records the refusal"
		s := newSubject()
		if s.seed(&v, clause, []kitRow{{kitTenantA, 1, 11}}) != nil {
			for _, caller := range []struct {
				name            string
				ctx             context.Context
				subject, tenant string
			}{
				{"a scoped carrier", acme, kitSubject, string(kitTenantA)},
				{"a carrier declaring ReachUndeclared", ports.WithPlatformReach(acme, ports.ReachUndeclared), kitSubject, string(kitTenantA)},
				{"a carrier without a tenant", bare, kitSubject, ""},
				{"a bare carrier", context.Background(), "", ""},
			} {
				trail := &recordingTrail{}
				reader := s.Platform(trail, ports.PlatformPageMax)
				rows, listed := reader.List(caller.ctx, nil, ports.PlatformPageMax)
				_, loaded := reader.LoadIn(caller.ctx, kitTenantA, s.NewID(1))
				if !errors.Is(listed, ports.ErrPlatformReachUndeclared) || len(rows) != 0 {
					v.fail(clause, "IDN-19", "List() from %s = %d rows, %v; want none and ErrPlatformReachUndeclared", caller.name, len(rows), listed)
				}
				if !errors.Is(loaded, ports.ErrPlatformReachUndeclared) {
					v.fail(clause, "IDN-19", "LoadIn() from %s = %v, want ErrPlatformReachUndeclared", caller.name, loaded)
				}
				listDenied := ports.AuditRecord{Subject: caller.subject, Object: s.Table, Action: ports.AuditPlatformRead, Outcome: string(ports.OutcomeDenied), Tenant: caller.tenant}
				loadDenied := listDenied
				loadDenied.DataTenant = string(kitTenantA)
				expectTrail(&v, clause, "LOG-14", "the refusals from "+caller.name, trail, listDenied, loadDenied)

				refusing := s.Platform(refusingTrail{}, ports.PlatformPageMax)
				if _, err := refusing.List(caller.ctx, nil, ports.PlatformPageMax); !errors.Is(err, ports.ErrPlatformReachUndeclared) {
					v.fail(clause, "IDN-19", "List() from %s over a refusing trail = %v, want ErrPlatformReachUndeclared: the failed record leaves the refusal as it is", caller.name, err)
				}
			}
		}
	}

	{
		const clause = "refuses when the carrier holds no context"
		s := newSubject()
		trail := &recordingTrail{}
		reader := s.Platform(trail, ports.PlatformPageMax)
		ctx := reach(context.Background())
		if _, err := reader.List(ctx, nil, ports.PlatformPageMax); !errors.Is(err, ports.ErrContextAbsent) {
			v.fail(clause, "IDN-15", "List() off a bare context = %v, want ErrContextAbsent", err)
		}
		if _, err := reader.LoadIn(ctx, kitTenantA, s.NewID(1)); !errors.Is(err, ports.ErrContextAbsent) {
			v.fail(clause, "IDN-15", "LoadIn() off a bare context = %v, want ErrContextAbsent", err)
		}
		expectNoAccepted(&v, clause, "the refusals", trail)
	}

	{
		const clause = "refuses a limit out of range"
		s := newSubject()
		if s.seed(&v, clause, []kitRow{{kitTenantA, 1, 11}}) != nil {
			for _, page := range []struct {
				ceiling, limit int
				ok             bool
			}{
				{ports.PlatformPageMax, 0, false},
				{ports.PlatformPageMax, -1, false},
				{ports.PlatformPageMax, ports.PlatformPageMax + 1, false},
				{ports.PlatformPageMax, ports.PlatformPageMax, true},
				{2, 3, false},
				{2, 2, true},
			} {
				trail := &recordingTrail{}
				rows, err := s.Platform(trail, page.ceiling).List(reach(acme), nil, page.limit)
				switch {
				case page.ok && (err != nil || len(rows) != 1):
					v.fail(clause, "IDN-19", "List(limit %d) under ceiling %d = %d rows, %v; want the one row", page.limit, page.ceiling, len(rows), err)
				case !page.ok && (!errors.Is(err, ports.ErrPlatformLimitInvalid) || len(rows) != 0):
					v.fail(clause, "IDN-19", "List(limit %d) under ceiling %d = %d rows, %v; want ErrPlatformLimitInvalid", page.limit, page.ceiling, len(rows), err)
				case !page.ok:
					expectNoAccepted(&v, clause, fmt.Sprintf("List(limit %d)", page.limit), trail)
				}
			}
		}
	}

	{
		const clause = "records one accepted event per tenant reached"
		s := newSubject()
		if s.seed(&v, clause, []kitRow{{kitTenantA, 1, 11}, {kitTenantA, 2, 12}, {kitTenantB, 1, 21}}) != nil {
			for _, caller := range []struct {
				name   string
				ctx    context.Context
				tenant string
			}{
				{"a scoped carrier", reach(acme), string(kitTenantA)},
				{"a carrier without a tenant", reach(bare), ""},
			} {
				trail := &recordingTrail{}
				if _, err := s.Platform(trail, ports.PlatformPageMax).List(caller.ctx, nil, ports.PlatformPageMax); err != nil {
					v.fail(clause, "IDN-19", "List() from %s = %v, want nil", caller.name, err)
					continue
				}
				expectTrail(&v, clause, "LOG-14", "List() from "+caller.name, trail,
					s.readEvent(caller.tenant, kitTenantA), s.readEvent(caller.tenant, kitTenantB))
			}

			trail := &recordingTrail{}
			if rows, err := s.Platform(trail, ports.PlatformPageMax).List(reach(acme), nil, 1); err != nil || len(rows) != 1 {
				v.fail(clause, "IDN-19", "List(limit 1) = %d rows, %v; want one", len(rows), err)
			} else {
				expectTrail(&v, clause, "LOG-14", "a page of one row", trail, s.readEvent(string(kitTenantA), rows[0].Tenant))
			}

			trail = &recordingTrail{}
			if row, err := s.Platform(trail, ports.PlatformPageMax).LoadIn(reach(acme), kitTenantB, s.NewID(1)); err != nil || row.Tenant != kitTenantB {
				v.fail(clause, "IDN-19", "LoadIn() = tenant %q, %v; want the row of %q", row.Tenant, err, kitTenantB)
			} else {
				expectTrail(&v, clause, "LOG-14", "LoadIn()", trail, s.readEvent(string(kitTenantA), kitTenantB))
			}
		}
	}

	{
		const clause = "returns nothing when the trail refuses the record"
		s := newSubject()
		if s.seed(&v, clause, []kitRow{{kitTenantA, 1, 11}, {kitTenantB, 1, 21}}) != nil {
			reader := s.Platform(refusingTrail{}, ports.PlatformPageMax)
			if rows, err := reader.List(reach(acme), nil, ports.PlatformPageMax); !errors.Is(err, ports.ErrAuditUnavailable) || len(rows) != 0 {
				v.fail(clause, "DAT-25", "List() over a refusing trail = %d rows, %v; want none and ErrAuditUnavailable", len(rows), err)
			}
			if row, err := reader.LoadIn(reach(acme), kitTenantB, s.NewID(1)); !errors.Is(err, ports.ErrAuditUnavailable) || row.Version != 0 {
				v.fail(clause, "DAT-25", "LoadIn() over a refusing trail = v%d, %v; want nothing and ErrAuditUnavailable", row.Version, err)
			}
		}
	}

	{
		const clause = "records nothing for an empty page"
		s := newSubject()
		trail := &recordingTrail{}
		reader := s.Platform(trail, ports.PlatformPageMax)
		if rows, err := reader.List(reach(acme), nil, ports.PlatformPageMax); err != nil || len(rows) != 0 {
			v.fail(clause, "IDN-19", "List() over an empty table = %d rows, %v; want an empty page", len(rows), err)
		} else if seeded := s.seed(&v, clause, []kitRow{{kitTenantA, 1, 11}}); seeded != nil {
			last := ports.PlatformKey[ID]{Tenant: kitTenantA, ID: s.NewID(1)}
			if rows, err := reader.List(reach(acme), &last, ports.PlatformPageMax); err != nil || len(rows) != 0 {
				v.fail(clause, "IDN-19", "List() after the last key = %v, %v; want an empty page", keysOf(rows), err)
			}
			if got := trail.Events(); len(got) != 0 {
				v.fail(clause, "LOG-14", "an empty page recorded %+v, want nothing: it reached no tenant", got)
			}
		}
	}

	{
		const clause = "loads the row of the tenant asked"
		s := newSubject()
		if s.seed(&v, clause, []kitRow{{kitTenantB, 1, 21}}) != nil {
			row, err := s.Platform(&recordingTrail{}, ports.PlatformPageMax).LoadIn(reach(acme), kitTenantB, s.NewID(1))
			switch {
			case err != nil:
				v.fail(clause, "IDN-19", "LoadIn() = %v, want nil", err)
			case row.Tenant != kitTenantB || row.ID != s.NewID(1) || s.Marker(row.State) != 21 || row.Version != 1:
				v.fail(clause, "IDN-19", "LoadIn() = %q/%v marker %d v%d; want %q/%v marker 21 v1",
					row.Tenant, row.ID, s.Marker(row.State), row.Version, kitTenantB, s.NewID(1))
			}
		}
	}

	{
		const clause = "misses in the tenant asked without probing the owner"
		s := newSubject()
		if s.seed(&v, clause, []kitRow{{kitTenantA, 1, 11}}) != nil {
			trail := &recordingTrail{}
			reader := s.Platform(trail, ports.PlatformPageMax)
			_, err := reader.LoadIn(reach(acme), kitTenantB, s.NewID(1))
			if !errors.Is(err, ports.ErrNotFound) {
				v.fail(clause, "IDN-13", "LoadIn() of an identifier another tenant holds = %v, want ErrNotFound", err)
			} else if _, probed := crossTenant(err); probed {
				v.fail(clause, "IDN-12", "LoadIn() = %v; a declared read is no mistaken access, and the probe would reveal the owner", err)
			}
			expectTrail(&v, clause, "LOG-14", "the miss", trail, s.readEvent(string(kitTenantA), kitTenantB))

			if _, err := reader.LoadIn(reach(acme), "", s.NewID(1)); !errors.Is(err, ports.ErrNotFound) {
				v.fail(clause, "IDN-20", "LoadIn() of an empty tenant = %v, want ErrNotFound", err)
			}
		}
	}

	{
		const clause = "keeps the scoped Reader filtered under the declared reach"
		s := newSubject()
		if s.seed(&v, clause, []kitRow{{kitTenantB, 1, 21}}) != nil {
			_, _, err := s.Reader.Load(reach(acme), s.NewID(1))
			if access, ok := crossTenant(err); !ok {
				v.fail(clause, "IDN-12", "Reader.Load() of another tenant's row under ReachAllTenants = %v, want the CrossTenantAccess of a scoped read", err)
			} else if access.ContextTenant != kitTenantA || access.DataTenant != kitTenantB {
				v.fail(clause, "IDN-12", "CrossTenantAccess names context %q and data %q, want %q and %q", access.ContextTenant, access.DataTenant, kitTenantA, kitTenantB)
			}
		}
	}

	{
		const clause = "refuses to be built without a trail"
		s := newSubject()
		if recovered := panicOf(func() { s.Platform(nil, ports.PlatformPageMax) }); recovered == nil {
			v.fail(clause, "DAT-25", "Platform(nil trail) built a reader, want a panic: an access across tenants without its trail is a composition error")
		} else if !strings.Contains(fmt.Sprint(recovered), s.Table) {
			v.fail(clause, "DAT-25", "Platform(nil trail) panicked with %q, want the table %q named", recovered, s.Table)
		}
	}

	return v
}

type kitRow struct {
	tenant     ports.TenantID
	row, value int
}

func (s PlatformSubject[ID, S]) seed(v *Verdict, clause string, rows []kitRow) map[ports.PlatformKey[ID]]int {
	seeded := make(map[ports.PlatformKey[ID]]int, len(rows))
	for _, r := range rows {
		ctx, err := scopedTo(r.tenant)
		if err == nil {
			err = s.Within(ctx, func(ctx context.Context, repo ports.Repository[ID, S]) error {
				return repo.Save(ctx, s.NewID(r.row), s.NewState(r.value), 0)
			})
		}
		if err != nil {
			v.fail(clause, "UOW-01", "Save() of %s/%v = %v, want nil", r.tenant, s.NewID(r.row), err)
			return nil
		}
		seeded[ports.PlatformKey[ID]{Tenant: r.tenant, ID: s.NewID(r.row)}] = r.value
	}
	return seeded
}

func (s PlatformSubject[ID, S]) expectRows(v *Verdict, clause string, rows []ports.PlatformRow[ID, S], seeded map[ports.PlatformKey[ID]]int) {
	seen := make(map[ports.PlatformKey[ID]]bool, len(rows))
	for _, row := range rows {
		key := ports.PlatformKey[ID]{Tenant: row.Tenant, ID: row.ID}
		want, ok := seeded[key]
		switch {
		case seen[key]:
			v.fail(clause, "IDN-19", "%s/%v came back twice", row.Tenant, row.ID)
		case !ok:
			v.fail(clause, "IDN-19", "%s/%v came back and was never written", row.Tenant, row.ID)
		case s.Marker(row.State) != want || row.Version != 1:
			v.fail(clause, "IDN-19", "%s/%v reads marker %d v%d, want marker %d v1 — the row and its tenant must agree", row.Tenant, row.ID, s.Marker(row.State), row.Version, want)
		}
		seen[key] = true
	}
	for key := range seeded {
		if !seen[key] {
			v.fail(clause, "IDN-19", "%s/%v never came back", key.Tenant, key.ID)
		}
	}
}

func (s PlatformSubject[ID, S]) readEvent(tenant string, data ports.TenantID) ports.AuditRecord {
	return ports.AuditRecord{Subject: kitSubject, Object: s.Table, Action: ports.AuditPlatformRead, Outcome: string(ports.OutcomeAccepted), Tenant: tenant, DataTenant: string(data)}
}

// walkPages bounds a walk, so a cursor the realization ignores fails instead of
// looping.
const walkPages = 16

func walk[ID comparable, S any](reader ports.PlatformReader[ID, S], ctx context.Context, limit int) ([]ports.PlatformRow[ID, S], error) {
	var all []ports.PlatformRow[ID, S]
	var after *ports.PlatformKey[ID]
	for range walkPages {
		page, err := reader.List(ctx, after, limit)
		if err != nil {
			return nil, err
		}
		if len(page) > limit {
			return nil, fmt.Errorf("a page of %d rows under limit %d", len(page), limit)
		}
		if len(page) == 0 {
			return all, nil
		}
		all = append(all, page...)
		last := page[len(page)-1]
		after = &ports.PlatformKey[ID]{Tenant: last.Tenant, ID: last.ID}
	}
	return nil, fmt.Errorf("no empty page after %d pages: the cursor does not advance", walkPages)
}

func keysOf[ID comparable, S any](rows []ports.PlatformRow[ID, S]) []ports.PlatformKey[ID] {
	keys := make([]ports.PlatformKey[ID], 0, len(rows))
	for _, row := range rows {
		keys = append(keys, ports.PlatformKey[ID]{Tenant: row.Tenant, ID: row.ID})
	}
	return keys
}

const kitSubject = "dmpf-kit-subject"

// recordingTrail keeps the records apart from observability/audit, so a
// provider's conformance test does not pull the observability module.
type recordingTrail struct {
	mu      sync.Mutex
	records []ports.AuditRecord
}

func (t *recordingTrail) Record(_ context.Context, r ports.AuditRecord) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, r)
	return nil
}

func (t *recordingTrail) Events() []ports.AuditRecord {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.records)
}

type refusingTrail struct{}

func (refusingTrail) Record(context.Context, ports.AuditRecord) error {
	return fmt.Errorf("providerkit: the trail refused the record: %w", ports.ErrAuditUnavailable)
}

// expectTrail compares the trail with want regardless of order: the
// order of the events of one page is the realization's.
func expectTrail(v *Verdict, clause, rule, what string, trail *recordingTrail, want ...ports.AuditRecord) {
	got := trail.Events()
	byTenant := func(a, b ports.AuditRecord) int {
		return cmp.Or(cmp.Compare(a.DataTenant, b.DataTenant), cmp.Compare(a.Outcome, b.Outcome))
	}
	slices.SortFunc(got, byTenant)
	slices.SortFunc(want, byTenant)
	if !slices.Equal(got, want) {
		v.fail(clause, rule, "%s recorded %+v, want %+v", what, got, want)
	}
}

func expectNoAccepted(v *Verdict, clause, what string, trail *recordingTrail) {
	for _, event := range trail.Events() {
		if event.Outcome == string(ports.OutcomeAccepted) {
			v.fail(clause, "LOG-14", "%s recorded an accepted access %+v", what, event)
		}
	}
}

func panicOf(fn func()) (recovered any) {
	defer func() { recovered = recover() }()
	fn()
	return nil
}
