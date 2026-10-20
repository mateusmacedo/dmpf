//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

var errSinkDown = errors.New("audit sink down")

type refusingSink struct{}

func (refusingSink) Emit(context.Context, audit.Event) error { return errSinkDown }

func recorderOver(sink audit.Sink) ports.AuditTrail {
	return audit.NewRecorder(sink, slog.New(slog.DiscardHandler), fixedClock(1_755_432_000_000_000_000)).Trail()
}

func withReach(ctx context.Context) context.Context {
	return ports.WithPlatformReach(ctx, ports.ReachAllTenants)
}

func seedProbes(t *testing.T, pool *pgxpool.Pool, rows map[string][]string) {
	t.Helper()
	for tenant, ids := range rows {
		for i, id := range ids {
			if err := saveProbe(t, scopedTo(t, tenant), pool, id, probe{Items: i + 1}, 0); err != nil {
				t.Fatalf("Save(%s/%s) = %v, want nil", tenant, id, err)
			}
		}
	}
}

func dataTenants(events []audit.Event) []string {
	tenants := make([]string, len(events))
	for i, event := range events {
		tenants[i] = event.DataTenant
	}
	return tenants
}

func TestPlatformReaderSeesEveryTenantWithTheTenantOfEachRow(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"acme": {"p-1", "p-2"}, "globex": {"p-1"}})
	trail := &audit.Recording{}
	reader := probeTable.PlatformReader(postgres.NewReadPool(pool), recorderOver(trail))

	rows, err := reader.List(withReach(scopedTo(t, "acme")), nil, 10)
	if err != nil {
		t.Fatalf("List() = %v, want nil", err)
	}
	want := []ports.PlatformRow[string, probe]{
		{Tenant: "acme", ID: "p-1", State: probe{Items: 1}, Version: 1},
		{Tenant: "acme", ID: "p-2", State: probe{Items: 2}, Version: 1},
		{Tenant: "globex", ID: "p-1", State: probe{Items: 1}, Version: 1},
	}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("List() = %+v, want %+v", rows, want)
	}

	row, err := reader.LoadIn(withReach(scopedTo(t, "acme")), "globex", "p-1")
	if err != nil || row != want[2] {
		t.Fatalf("LoadIn(globex, p-1) = %+v, %v; want %+v, nil", row, err, want[2])
	}

	events := trail.Events()
	if got := dataTenants(events); fmt.Sprint(got) != "[acme globex globex]" {
		t.Fatalf("data tenants of the trail = %v, want one per tenant of the page, then the LoadIn", got)
	}
	for _, event := range events {
		if event.Action != audit.ActionPlatformRead || event.Outcome != "accepted" ||
			event.Object != "probes" || event.Subject != "s-1" || event.Tenant != "acme" {
			t.Fatalf("event = %+v, want an accepted platform read of probes by s-1 from acme", event)
		}
	}
}

func TestPlatformReaderPagesCoverEveryRowOnce(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{
		"acme": {"p-1", "p-2", "p-3"}, "globex": {"p-1", "p-4"}, "initech": {"p-2"},
	})
	reader := probeTable.PlatformReader(postgres.NewReadPool(pool), recorderOver(&audit.Recording{}))
	ctx := withReach(scopedTo(t, "acme"))

	seen := map[ports.PlatformKey[string]]bool{}
	var after *ports.PlatformKey[string]
	for pages := 0; ; pages++ {
		if pages > 6 {
			t.Fatalf("paging did not end after %d pages", pages)
		}
		rows, err := reader.List(ctx, after, 2)
		if err != nil {
			t.Fatalf("List(after %v) = %v, want nil", after, err)
		}
		if len(rows) > 2 {
			t.Fatalf("List() returned %d rows, want at most the limit 2", len(rows))
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			key := ports.PlatformKey[string]{Tenant: row.Tenant, ID: row.ID}
			if seen[key] {
				t.Fatalf("row %v came back on two pages", key)
			}
			seen[key] = true
		}
		last := rows[len(rows)-1]
		after = &ports.PlatformKey[string]{Tenant: last.Tenant, ID: last.ID}
	}
	if len(seen) != 6 {
		t.Fatalf("pages covered %d rows %v, want all 6", len(seen), seen)
	}
}

func TestPlatformReaderRefusesAnUndeclaredReachWithADeniedEvent(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"acme": {"p-1"}})

	cases := map[string]struct {
		ctx    context.Context
		tenant string
	}{
		"scoped carrier without reach":    {ctx: scopedTo(t, "acme")},
		"reach declared as undeclared":    {ctx: ports.WithPlatformReach(scopedTo(t, "acme"), ports.ReachUndeclared)},
		"tenantless carrier":              {ctx: tenantless(t)},
		"carrier without context":         {ctx: context.Background()},
		"refusing sink keeps the refusal": {ctx: scopedTo(t, "acme")},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			trail := &audit.Recording{}
			var sink audit.Sink = trail
			if strings.HasPrefix(name, "refusing") {
				sink = refusingSink{}
			}
			reader := probeTable.PlatformReader(postgres.NewReadPool(pool), recorderOver(sink))

			rows, err := reader.List(tc.ctx, nil, 10)
			if !errors.Is(err, ports.ErrPlatformReachUndeclared) || rows != nil {
				t.Fatalf("List() = %v, %v; want nil, ErrPlatformReachUndeclared", rows, err)
			}
			if _, err := reader.LoadIn(tc.ctx, "acme", "p-1"); !errors.Is(err, ports.ErrPlatformReachUndeclared) {
				t.Fatalf("LoadIn() = %v, want ErrPlatformReachUndeclared", err)
			}
			if sink != trail {
				return
			}
			events := trail.Events()
			if len(events) != 2 {
				t.Fatalf("trail = %+v, want one denied event per refused call", events)
			}
			wantTenant := ""
			if execution, ok := ports.ExecutionContextFrom(tc.ctx); ok {
				if tenant, ok := execution.Tenant(); ok {
					wantTenant = string(tenant)
				}
			}
			for i, wantData := range []string{"", "acme"} {
				event := events[i]
				if event.Outcome != "denied" || event.DataTenant != wantData || event.Tenant != wantTenant ||
					event.Action != audit.ActionPlatformRead || event.Object != "probes" {
					t.Fatalf("event %d = %+v, want denied with data tenant %q and tenant %q", i, event, wantData, wantTenant)
				}
			}
		})
	}
}

func TestPlatformReaderRefusesACarrierWithoutContext(t *testing.T) {
	reader := probeTable.PlatformReader(postgres.NewReadPool(openPool(t)), recorderOver(&audit.Recording{}))
	ctx := withReach(context.Background())

	if _, err := reader.List(ctx, nil, 10); !errors.Is(err, ports.ErrContextAbsent) {
		t.Fatalf("List() = %v, want ErrContextAbsent", err)
	}
	if _, err := reader.LoadIn(ctx, "acme", "p-1"); !errors.Is(err, ports.ErrContextAbsent) {
		t.Fatalf("LoadIn() = %v, want ErrContextAbsent", err)
	}
}

func TestPlatformReaderRefusesALimitOutOfRange(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"acme": {"p-1", "p-2", "p-3"}})
	ctx := withReach(scopedTo(t, "acme"))

	cases := map[string]struct {
		opts  []postgres.PlatformOption
		limit int
		valid bool
	}{
		"zero":                  {limit: 0},
		"negative":              {limit: -1},
		"above the default cap": {limit: ports.PlatformPageMax + 1},
		"at the default cap":    {limit: ports.PlatformPageMax, valid: true},
		"above a declared cap":  {opts: []postgres.PlatformOption{postgres.WithPageMax(2)}, limit: 3},
		"at a declared cap":     {opts: []postgres.PlatformOption{postgres.WithPageMax(2)}, limit: 2, valid: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			trail := &audit.Recording{}
			reader := probeTable.PlatformReader(postgres.NewReadPool(pool), recorderOver(trail), tc.opts...)

			rows, err := reader.List(ctx, nil, tc.limit)
			if tc.valid {
				if err != nil || len(rows) == 0 || len(rows) > tc.limit {
					t.Fatalf("List(%d) = %d rows, %v; want a page within the limit", tc.limit, len(rows), err)
				}
				return
			}
			if !errors.Is(err, ports.ErrPlatformLimitInvalid) || rows != nil {
				t.Fatalf("List(%d) = %v, %v; want nil, ErrPlatformLimitInvalid", tc.limit, rows, err)
			}
			if events := trail.Events(); len(events) != 0 {
				t.Fatalf("trail = %+v, want nothing for a refused limit", events)
			}
		})
	}
}

func TestWithPageMaxRefusesANonPositiveCap(t *testing.T) {
	for _, n := range []int{0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("WithPageMax(%d) did not panic", n)
				}
			}()
			postgres.WithPageMax(n)
		}()
	}
}

func TestPlatformReaderNamesTheCarrierTenantOrNone(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"acme": {"p-1"}})

	cases := map[string]struct {
		ctx    context.Context
		tenant string
	}{
		"carrier with tenant":    {ctx: withReach(scopedTo(t, "globex")), tenant: "globex"},
		"carrier without tenant": {ctx: withReach(tenantless(t))},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			trail := &audit.Recording{}
			reader := probeTable.PlatformReader(postgres.NewReadPool(pool), recorderOver(trail))

			if _, err := reader.List(tc.ctx, nil, 10); err != nil {
				t.Fatalf("List() = %v, want nil", err)
			}
			if _, err := reader.LoadIn(tc.ctx, "acme", "p-1"); err != nil {
				t.Fatalf("LoadIn() = %v, want nil", err)
			}
			for _, event := range trail.Events() {
				if event.Tenant != tc.tenant || event.DataTenant != "acme" || event.Outcome != "accepted" {
					t.Fatalf("event = %+v, want accepted with tenant %q over acme", event, tc.tenant)
				}
			}
		})
	}
}

func TestPlatformReaderFailsClosedWhenTheTrailRefuses(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"acme": {"p-1"}})
	recorder := &sqlRecorder{}
	cfg := pool.Config()
	cfg.ConnConfig.Tracer = recorder
	traced, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() = %v, want nil", err)
	}
	t.Cleanup(traced.Close)
	reader := probeTable.PlatformReader(postgres.NewReadPool(traced), recorderOver(refusingSink{}))
	ctx := withReach(scopedTo(t, "acme"))

	if _, err := reader.LoadIn(ctx, "acme", "p-1"); !errors.Is(err, ports.ErrAuditUnavailable) {
		t.Fatalf("LoadIn() = %v, want ErrAuditUnavailable", err)
	}
	if sent := recorder.statements(); len(sent) != 0 {
		t.Fatalf("LoadIn() sent %q, want no read before the trail confirms", sent)
	}

	rows, err := reader.List(ctx, nil, 10)
	if !errors.Is(err, ports.ErrAuditUnavailable) || rows != nil {
		t.Fatalf("List() = %v, %v; want nil, ErrAuditUnavailable", rows, err)
	}
}

func TestPlatformReaderRecordsNothingForAnEmptyPage(t *testing.T) {
	trail := &audit.Recording{}
	reader := probeTable.PlatformReader(postgres.NewReadPool(openPool(t)), recorderOver(trail))

	rows, err := reader.List(withReach(scopedTo(t, "acme")), nil, 10)
	if err != nil || len(rows) != 0 {
		t.Fatalf("List() over an empty table = %v, %v; want an empty page", rows, err)
	}
	if events := trail.Events(); len(events) != 0 {
		t.Fatalf("trail = %+v, want nothing: no tenant was reached", events)
	}
}

func TestPlatformReaderLoadInAnswersAbsenceWithoutProbingTheOwner(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"acme": {"p-1"}})
	trail := &audit.Recording{}
	reader := probeTable.PlatformReader(postgres.NewReadPool(pool), recorderOver(trail))
	ctx := withReach(scopedTo(t, "acme"))

	_, err := reader.LoadIn(ctx, "globex", "p-1")
	var access ports.CrossTenantAccess
	if !errors.Is(err, ports.ErrNotFound) || errors.As(err, &access) {
		t.Fatalf("LoadIn(globex, p-1) = %v, want a plain ErrNotFound", err)
	}
	if got := dataTenants(trail.Events()); fmt.Sprint(got) != "[globex]" {
		t.Fatalf("data tenants of the trail = %v, want the requested tenant recorded before the read", got)
	}

	if _, err := reader.LoadIn(ctx, "", "p-1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LoadIn() into an empty tenant = %v, want ErrNotFound", err)
	}
	if events := trail.Events(); len(events) != 1 {
		t.Fatalf("trail = %+v, want nothing for an empty tenant", events)
	}
}

func TestPlatformAccessPanicsWithoutATrail(t *testing.T) {
	cases := map[string]func(){
		"PlatformReader": func() { probeTable.PlatformReader(postgres.ReadPool{}, nil) },
		"Projector":      func() { probeTable.Projector(nil, allowProjection, nil) },
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "probes") {
					t.Fatalf("%s without a recorder recovered %v, want a panic naming the table", name, r)
				}
			}()
			build()
		})
	}
}

func TestReachAllTenantsLeavesTheReaderAndTheRelationScoped(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"acme": {"p-1"}})
	read := postgres.NewReadPool(pool)
	ctx := withReach(scopedTo(t, "globex"))

	_, _, err := probeTable.Reader(read).Load(ctx, "p-1")
	var access ports.CrossTenantAccess
	if !errors.As(err, &access) || access.ContextTenant != "globex" || access.DataTenant != "acme" {
		t.Fatalf("Reader.Load() = %v, want CrossTenantAccess globex over acme", err)
	}

	_, err = probeTable.Relation("probe_id").Query(ctx, read, "p-1")
	access = ports.CrossTenantAccess{}
	if !errors.As(err, &access) || access.ContextTenant != "globex" || access.DataTenant != "acme" {
		t.Fatalf("Relation.Query() = %v, want CrossTenantAccess globex over acme", err)
	}
}

var errPolicyRefused = errors.New("policy refused the destination")

func allowProjection(context.Context, *ports.TenantID, ports.TenantID) error { return nil }

func refuseProjection(context.Context, *ports.TenantID, ports.TenantID) error {
	return errPolicyRefused
}

func project(t *testing.T, ctx context.Context, pool *pgxpool.Pool, policy ports.ProjectionPolicy, trail ports.AuditTrail,
	fn func(context.Context, ports.Projector[string, probe]) error,
) error {
	t.Helper()
	uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) ports.Projector[string, probe] {
		return probeTable.Projector(tx, policy, trail)
	})
	return uow.Within(ctx, fn)
}

func loadIn(t *testing.T, pool *pgxpool.Pool, tenant, id string) (probe, ports.Version, error) {
	t.Helper()
	return probeTable.Reader(postgres.NewReadPool(pool)).Load(scopedTo(t, tenant), id)
}

func TestProjectorWritesIntoTheDestinationTenant(t *testing.T) {
	pool := openPool(t)
	trail := &audit.Recording{}

	err := project(t, scopedTo(t, "acme"), pool, allowProjection, recorderOver(trail),
		func(ctx context.Context, p ports.Projector[string, probe]) error {
			return p.Save(ctx, "globex", "p-1", probe{Items: 4}, 0)
		})
	if err != nil {
		t.Fatalf("Save() = %v, want nil", err)
	}

	if state, version, err := loadIn(t, pool, "globex", "p-1"); err != nil || state.Items != 4 || version != 1 {
		t.Fatalf("globex Load() = %+v, v%d, %v; want {Items:4}, v1, nil", state, version, err)
	}
	if _, _, err := loadIn(t, pool, "acme", "p-1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("acme Load() = %v, want ErrNotFound: the origin does not see the projection", err)
	}
	events := trail.Events()
	want := audit.Event{
		Subject: "s-1", Object: "probes", Action: audit.ActionProjectionWrite, Outcome: "accepted",
		At: 1_755_432_000_000_000_000, Tenant: "acme", DataTenant: "globex",
	}
	if len(events) != 1 || events[0] != want {
		t.Fatalf("trail = %+v, want [%+v]", events, want)
	}
}

func TestProjectorRefusesWithoutAnAcceptingPolicy(t *testing.T) {
	cases := map[string]struct {
		policy ports.ProjectionPolicy
		cause  error
	}{
		"no policy":       {},
		"refusing policy": {policy: refuseProjection, cause: errPolicyRefused},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pool := openPool(t)
			seedProbes(t, pool, map[string][]string{"globex": {"p-1"}})
			trail := &audit.Recording{}

			err := project(t, scopedTo(t, "acme"), pool, tc.policy, recorderOver(trail),
				func(ctx context.Context, p ports.Projector[string, probe]) error {
					if _, _, err := p.Load(ctx, "globex", "p-1"); !errors.Is(err, ports.ErrProjectionDenied) {
						t.Errorf("Load() = %v, want ErrProjectionDenied", err)
					}
					return p.Save(ctx, "globex", "p-1", probe{Items: 9}, 1)
				})
			if !errors.Is(err, ports.ErrProjectionDenied) {
				t.Fatalf("Save() = %v, want ErrProjectionDenied", err)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("Save() = %v, want the policy's cause kept", err)
			}
			if state, version, err := loadIn(t, pool, "globex", "p-1"); err != nil || state.Items != 1 || version != 1 {
				t.Fatalf("globex row = %+v, v%d, %v; want it untouched", state, version, err)
			}
			for _, event := range trail.Events() {
				if event.Outcome != "denied" || event.DataTenant != "globex" || event.Action != audit.ActionProjectionWrite {
					t.Fatalf("event = %+v, want a denied projection into globex", event)
				}
			}
			if events := trail.Events(); len(events) != 2 {
				t.Fatalf("trail = %+v, want one denied event per call", events)
			}
		})
	}
}

func TestProjectorRefusesAnEmptyDestination(t *testing.T) {
	trail := &audit.Recording{}
	called := false
	policy := func(context.Context, *ports.TenantID, ports.TenantID) error {
		called = true
		return nil
	}

	err := project(t, scopedTo(t, "acme"), openPool(t), policy, recorderOver(trail),
		func(ctx context.Context, p ports.Projector[string, probe]) error {
			if _, _, err := p.Load(ctx, "", "p-1"); !errors.Is(err, ports.ErrProjectionDestinationAbsent) {
				t.Errorf("Load() = %v, want ErrProjectionDestinationAbsent", err)
			}
			return p.Save(ctx, "", "p-1", probe{Items: 1}, 0)
		})
	if !errors.Is(err, ports.ErrProjectionDestinationAbsent) {
		t.Fatalf("Save() = %v, want ErrProjectionDestinationAbsent", err)
	}
	if called {
		t.Fatalf("the policy was asked about an absent destination")
	}
	events := trail.Events()
	if len(events) != 2 || events[0].Outcome != "denied" || events[0].DataTenant != "" {
		t.Fatalf("trail = %+v, want denied events without a data tenant", events)
	}
}

func TestProjectorRefusesTheOriginsOwnTenantBeforeThePolicy(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"acme": {"p-1"}})
	trail := &audit.Recording{}
	called := false
	policy := func(context.Context, *ports.TenantID, ports.TenantID) error {
		called = true
		return nil
	}

	err := project(t, scopedTo(t, "acme"), pool, policy, recorderOver(trail),
		func(ctx context.Context, p ports.Projector[string, probe]) error {
			if _, _, err := p.Load(ctx, "acme", "p-1"); !errors.Is(err, ports.ErrProjectionDenied) {
				t.Errorf("Load() = %v, want ErrProjectionDenied", err)
			}
			return p.Save(ctx, "acme", "p-1", probe{Items: 9}, 1)
		})
	if !errors.Is(err, ports.ErrProjectionDenied) {
		t.Fatalf("Save() = %v, want ErrProjectionDenied", err)
	}
	if called {
		t.Fatalf("the policy was asked about the origin's own tenant")
	}
	if state, version, err := loadIn(t, pool, "acme", "p-1"); err != nil || state.Items != 1 || version != 1 {
		t.Fatalf("acme row = %+v, v%d, %v; want it untouched", state, version, err)
	}
	events := trail.Events()
	if len(events) != 2 {
		t.Fatalf("trail = %+v, want one denied event per call", events)
	}
	for _, event := range events {
		if event.Outcome != "denied" || event.Tenant != "acme" || event.DataTenant != "acme" {
			t.Fatalf("event = %+v, want a denied projection from acme into acme", event)
		}
	}
}

func TestProjectorHandsThePolicyTheCarrierTenantOrNone(t *testing.T) {
	cases := map[string]struct {
		ctx    context.Context
		origin string
	}{
		"carrier with tenant":    {ctx: scopedTo(t, "acme"), origin: "acme"},
		"carrier without tenant": {ctx: tenantless(t)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			trail := &audit.Recording{}
			var origins []*ports.TenantID
			policy := func(_ context.Context, origin *ports.TenantID, destination ports.TenantID) error {
				if destination != "globex" {
					t.Errorf("policy destination = %q, want globex", destination)
				}
				origins = append(origins, origin)
				return nil
			}

			err := project(t, tc.ctx, openPool(t), policy, recorderOver(trail),
				func(ctx context.Context, p ports.Projector[string, probe]) error {
					return p.Save(ctx, "globex", "p-1", probe{Items: 1}, 0)
				})
			if err != nil {
				t.Fatalf("Save() = %v, want nil", err)
			}
			if len(origins) != 1 {
				t.Fatalf("policy asked %d times, want once", len(origins))
			}
			if tc.origin == "" && origins[0] != nil {
				t.Fatalf("policy origin = %q, want nil: no tenant is invented (IDN-20)", *origins[0])
			}
			if tc.origin != "" && (origins[0] == nil || string(*origins[0]) != tc.origin) {
				t.Fatalf("policy origin = %v, want %q", origins[0], tc.origin)
			}
			if events := trail.Events(); len(events) != 1 || events[0].Tenant != tc.origin {
				t.Fatalf("trail = %+v, want the event tenant %q", events, tc.origin)
			}
		})
	}
}

func TestProjectorLoadReadsTheDestinationOnly(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"acme": {"p-1"}, "globex": {"p-2"}})
	trail := &audit.Recording{}

	err := project(t, scopedTo(t, "acme"), pool, allowProjection, recorderOver(trail),
		func(ctx context.Context, p ports.Projector[string, probe]) error {
			if state, version, err := p.Load(ctx, "globex", "p-2"); err != nil || state.Items != 1 || version != 1 {
				t.Errorf("Load(globex, p-2) = %+v, v%d, %v; want {Items:1}, v1, nil", state, version, err)
			}
			_, _, err := p.Load(ctx, "globex", "p-1")
			var access ports.CrossTenantAccess
			if !errors.Is(err, ports.ErrNotFound) || errors.As(err, &access) {
				t.Errorf("Load(globex, p-1) = %v, want a plain ErrNotFound", err)
			}
			return nil
		})
	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}
	if got := dataTenants(trail.Events()); fmt.Sprint(got) != "[globex globex]" {
		t.Fatalf("data tenants of the trail = %v, want one accepted event per Load", got)
	}
}

func TestProjectorFailsClosedWhenTheTrailRefuses(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"globex": {"p-1"}})
	recorder := &sqlRecorder{}
	cfg := pool.Config()
	cfg.ConnConfig.Tracer = recorder
	traced, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() = %v, want nil", err)
	}
	t.Cleanup(traced.Close)

	err = project(t, scopedTo(t, "acme"), traced, allowProjection, recorderOver(refusingSink{}),
		func(ctx context.Context, p ports.Projector[string, probe]) error {
			if _, _, err := p.Load(ctx, "globex", "p-1"); !errors.Is(err, ports.ErrAuditUnavailable) {
				t.Errorf("Load() = %v, want ErrAuditUnavailable", err)
			}
			return p.Save(ctx, "globex", "p-1", probe{Items: 9}, 1)
		})
	if !errors.Is(err, ports.ErrAuditUnavailable) {
		t.Fatalf("Save() = %v, want ErrAuditUnavailable", err)
	}
	for _, statement := range recorder.statements() {
		if strings.Contains(statement, "probes") {
			t.Fatalf("the projection sent %q, want no read or write before the trail confirms", statement)
		}
	}
}

func TestProjectorKeepsTheVersionSemanticsOfTheRepository(t *testing.T) {
	pool := openPool(t)
	recorder := recorderOver(&audit.Recording{})
	save := func(state probe, expected ports.Version) error {
		return project(t, scopedTo(t, "acme"), pool, allowProjection, recorder,
			func(ctx context.Context, p ports.Projector[string, probe]) error {
				return p.Save(ctx, "globex", "p-1", state, expected)
			})
	}

	if err := save(probe{Items: 1}, 0); err != nil {
		t.Fatalf("create = %v, want nil", err)
	}
	if err := save(probe{Items: 2}, 0); !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("create over an existing aggregate = %v, want ErrVersionConflict", err)
	}
	if err := save(probe{Items: 2}, 1); err != nil {
		t.Fatalf("update at v1 = %v, want nil", err)
	}
	if err := save(probe{Items: 3}, 1); !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("update at a stale version = %v, want ErrVersionConflict", err)
	}
	if state, version, err := loadIn(t, pool, "globex", "p-1"); err != nil || state.Items != 2 || version != 2 {
		t.Fatalf("globex row = %+v, v%d, %v; want {Items:2}, v2, nil", state, version, err)
	}
}

func TestProjectorCommitsWithTheUnitOfWork(t *testing.T) {
	pool := openPool(t)
	errRollback := errors.New("the callback failed after projecting")

	err := project(t, scopedTo(t, "acme"), pool, allowProjection, recorderOver(&audit.Recording{}),
		func(ctx context.Context, p ports.Projector[string, probe]) error {
			if err := p.Save(ctx, "globex", "p-1", probe{Items: 1}, 0); err != nil {
				return err
			}
			return errRollback
		})
	if !errors.Is(err, errRollback) {
		t.Fatalf("Within() = %v, want the callback's error", err)
	}
	if _, _, err := loadIn(t, pool, "globex", "p-1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("globex Load() after rollback = %v, want ErrNotFound", err)
	}
}

func projectUnique(t *testing.T, pool *pgxpool.Pool, destination ports.TenantID, id string, state uniqueProbe) error {
	t.Helper()
	uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) ports.Projector[string, uniqueProbe] {
		return uniqueTable.Projector(tx, allowProjection, recorderOver(&audit.Recording{}))
	})
	return uow.Within(scopedTo(t, "acme"), func(ctx context.Context, p ports.Projector[string, uniqueProbe]) error {
		return p.Save(ctx, destination, id, state, 0)
	})
}

func TestProjectorRejectsAUniqueViolationScopedToTheDestination(t *testing.T) {
	pool := openPool(t)
	taken := uniqueProbe{Name: "alpha", Code: "A", Label: "red", RefA: "A", RefB: "red"}
	if err := saveUnique(t, scopedTo(t, "globex"), pool, "U-1", taken, 0); err != nil {
		t.Fatalf("globex Save() = %v, want nil", err)
	}
	if err := saveUnique(t, scopedTo(t, "acme"), pool, "U-9", uniqueProbe{Name: "beta", Code: "B", Label: "blue", RefA: "B", RefB: "blue"}, 0); err != nil {
		t.Fatalf("acme Save() = %v, want nil", err)
	}

	err := projectUnique(t, pool, "globex", "U-2", uniqueProbe{Name: "alpha", Code: "C", Label: "green", RefA: "C", RefB: "green"})
	requireViolation(t, err, "unique_probes_tenant_id_name_key", nameTaken)
	if _, _, err := uniqueTable.Reader(postgres.NewReadPool(pool)).Load(scopedTo(t, "globex"), "U-2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("globex Load(U-2) after the rejection = %v, want ErrNotFound", err)
	}

	for destination, state := range map[ports.TenantID]uniqueProbe{
		"initech": taken,
		"globex":  {Name: "beta", Code: "B", Label: "blue", RefA: "B", RefB: "blue"},
	} {
		if err := projectUnique(t, pool, destination, "U-3", state); err != nil {
			t.Fatalf("Save() into %s of values another tenant holds = %v, want nil", destination, err)
		}
	}
}

var errDecodeMissing = fmt.Errorf("decode refused the row: %w", ports.ErrNotFound)

func TestTheCommonReadReturnsADecodeFailureWithoutProbingTheOwner(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"acme": {"p-1"}, "globex": {"p-1"}})
	refusing := probeTable
	refusing.Decode = func(scan func(dest ...any) error) (probe, error) {
		var raw []byte
		if err := scan(&raw); err != nil {
			return probe{}, err
		}
		return probe{}, errDecodeMissing
	}

	_, _, err := refusing.Reader(postgres.NewReadPool(pool)).Load(scopedTo(t, "acme"), "p-1")
	var access ports.CrossTenantAccess
	if !errors.Is(err, errDecodeMissing) || errors.As(err, &access) ||
		err.Error() != "postgres: load probes p-1: "+errDecodeMissing.Error() {
		t.Fatalf("Load() = %v, want the decode failure as it was, without CrossTenantAccess", err)
	}
}

func TestProjectorRefusesACarrierWithoutContext(t *testing.T) {
	pool := openPool(t)
	seedProbes(t, pool, map[string][]string{"globex": {"p-1"}})
	recorder := &sqlRecorder{}
	cfg := pool.Config()
	cfg.ConnConfig.Tracer = recorder
	traced, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() = %v, want nil", err)
	}
	t.Cleanup(traced.Close)
	trail := &audit.Recording{}
	called := false
	policy := func(context.Context, *ports.TenantID, ports.TenantID) error {
		called = true
		return nil
	}

	err = project(t, context.Background(), traced, policy, recorderOver(trail),
		func(ctx context.Context, p ports.Projector[string, probe]) error {
			if _, _, err := p.Load(ctx, "globex", "p-1"); !errors.Is(err, ports.ErrContextAbsent) {
				t.Errorf("Load() = %v, want ErrContextAbsent", err)
			}
			return p.Save(ctx, "globex", "p-2", probe{Items: 1}, 0)
		})
	if !errors.Is(err, ports.ErrContextAbsent) {
		t.Fatalf("Save() = %v, want ErrContextAbsent", err)
	}
	if called {
		t.Fatalf("the policy was asked for a carrier without context")
	}
	if events := trail.Events(); len(events) != 0 {
		t.Fatalf("trail = %+v, want nothing for a carrier without context", events)
	}
	for _, statement := range recorder.statements() {
		if strings.Contains(statement, "probes") {
			t.Fatalf("the projection sent %q, want no I/O for a carrier without context", statement)
		}
	}
}
