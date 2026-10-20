package memory_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

const (
	tenantA = ports.TenantID("acme")
	tenantB = ports.TenantID("globex")
)

type recordingTrail struct{ records []ports.AuditRecord }

func (t *recordingTrail) Record(_ context.Context, r ports.AuditRecord) error {
	t.records = append(t.records, r)
	return nil
}

func (t *recordingTrail) Events() []ports.AuditRecord { return t.records }

type refusingTrail struct{}

func (refusingTrail) Record(context.Context, ports.AuditRecord) error {
	return fmt.Errorf("memory_test: trail down: %w", ports.ErrAuditUnavailable)
}

func inTenant(t *testing.T, tenant ports.TenantID) context.Context {
	t.Helper()
	return execctx.WithExecution(t, context.Background(), execctx.WithTenant(tenant))
}

func withReach(ctx context.Context) context.Context {
	return ports.WithPlatformReach(ctx, ports.ReachAllTenants)
}

func tenantlessCtx(t *testing.T) context.Context {
	t.Helper()
	subject := ports.SubjectID("s-test")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-test", CorrelationID: "c-test", TraceContext: "t-test",
		Subject: &subject, Permissions: []ports.Permission{},
		Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	return ports.WithExecutionContext(context.Background(), execution)
}

func seedIn(t *testing.T, store *memory.Store, tenant ports.TenantID, at counterID, total int) {
	t.Helper()
	uow := memory.NewUnitOfWork(store, bind)
	if err := uow.Within(inTenant(t, tenant), func(ctx context.Context, res resources) error {
		return res.Counters.Save(ctx, at, counter{ID: at, Total: total, Steps: []int{total}}, 0)
	}); err != nil {
		t.Fatalf("seed %s/%s: %v", tenant, at, err)
	}
}

func keysOf(rows []ports.PlatformRow[counterID, counter]) []ports.PlatformKey[counterID] {
	keys := make([]ports.PlatformKey[counterID], 0, len(rows))
	for _, r := range rows {
		keys = append(keys, ports.PlatformKey[counterID]{Tenant: r.Tenant, ID: r.ID})
	}
	return keys
}

func TestPlatformListOrdersByTenantThenByThePrintedIdentity(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantB, "c-1", 21)
	seedIn(t, store, tenantA, "c-2", 12)
	seedIn(t, store, tenantA, "c-1", 11)
	reader := counters.PlatformReader(store, &recordingTrail{})

	rows, err := reader.List(withReach(inTenant(t, tenantA)), nil, 10)
	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	want := []ports.PlatformKey[counterID]{{Tenant: tenantA, ID: "c-1"}, {Tenant: tenantA, ID: "c-2"}, {Tenant: tenantB, ID: "c-1"}}
	if got := keysOf(rows); !slices.Equal(got, want) {
		t.Fatalf("List() keys = %v, want %v", got, want)
	}
	if rows[2].State.Total != 21 || rows[2].Version != 1 {
		t.Fatalf("List()[2] = %+v, want total 21 at v1", rows[2])
	}
}

func TestPlatformListResumesStrictlyAfterTheCursor(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantA, "c-1", 1)
	seedIn(t, store, tenantA, "c-2", 2)
	seedIn(t, store, tenantB, "c-1", 3)
	reader := counters.PlatformReader(store, &recordingTrail{})
	ctx := withReach(inTenant(t, tenantA))

	rows, err := reader.List(ctx, &ports.PlatformKey[counterID]{Tenant: tenantA, ID: "c-1"}, 1)
	if err != nil || len(rows) != 1 || rows[0].Tenant != tenantA || rows[0].ID != "c-2" {
		t.Fatalf("List(after acme/c-1, 1) = %v, %v; want acme/c-2 alone", keysOf(rows), err)
	}
	rows, err = reader.List(ctx, &ports.PlatformKey[counterID]{Tenant: tenantB, ID: "c-1"}, 1)
	if err != nil || len(rows) != 0 {
		t.Fatalf("List(after the last key) = %v, %v; want an empty page", keysOf(rows), err)
	}
}

type ticket struct{ n int }

func (ticket) String() string { return "ticket" }

func TestPlatformListPagesEveryRowWhenTheIdentifiersPrintAlike(t *testing.T) {
	tickets := memory.Table[ticket, int]{Name: "tickets"}
	store := memory.New()
	uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) ports.Repository[ticket, int] { return tickets.Repository(tx) })
	if err := uow.Within(inTenant(t, tenantA), func(ctx context.Context, repo ports.Repository[ticket, int]) error {
		for n := range 3 {
			if err := repo.Save(ctx, ticket{n}, n, 0); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed tickets: %v", err)
	}
	reader := tickets.PlatformReader(store, &recordingTrail{})
	ctx := withReach(inTenant(t, tenantA))

	var seen []ticket
	var after *ports.PlatformKey[ticket]
	for range 4 {
		rows, err := reader.List(ctx, after, 1)
		if err != nil {
			t.Fatalf("List(after %v, 1) = %v", after, err)
		}
		if len(rows) == 0 {
			break
		}
		seen = append(seen, rows[0].ID)
		after = &ports.PlatformKey[ticket]{Tenant: rows[0].Tenant, ID: rows[0].ID}
	}
	slices.SortFunc(seen, func(a, b ticket) int { return cmp.Compare(a.n, b.n) })
	if want := []ticket{{0}, {1}, {2}}; !slices.Equal(seen, want) {
		t.Fatalf("paged identifiers = %#v, want each of %#v once", seen, want)
	}
}

func TestPlatformListSharesNoSliceWithTheStore(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantA, "c-1", 1)
	reader := counters.PlatformReader(store, &recordingTrail{})
	ctx := withReach(inTenant(t, tenantA))

	rows, err := reader.List(ctx, nil, 1)
	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	rows[0].State.Steps[0] = 99
	again, err := reader.List(ctx, nil, 1)
	if err != nil || again[0].State.Steps[0] != 1 {
		t.Fatalf("List() after mutating a returned row = %+v, %v; want the stored step 1", again, err)
	}
}

func TestPlatformListEmitsOneAcceptedEventPerTenantBeforeReturning(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantA, "c-1", 1)
	seedIn(t, store, tenantA, "c-2", 2)
	seedIn(t, store, tenantB, "c-1", 3)

	for _, tc := range []struct {
		name       string
		ctx        context.Context
		wantTenant string
	}{
		{"with the carrier's tenant", withReach(inTenant(t, tenantA)), string(tenantA)},
		{"without a tenant on the carrier", withReach(tenantlessCtx(t)), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trail := &recordingTrail{}
			if _, err := counters.PlatformReader(store, trail).List(tc.ctx, nil, 10); err != nil {
				t.Fatalf("List() = %v", err)
			}
			want := []ports.AuditRecord{
				{Subject: "s-test", Object: "counters", Action: ports.AuditPlatformRead, Outcome: "accepted", Tenant: tc.wantTenant, DataTenant: string(tenantA)},
				{Subject: "s-test", Object: "counters", Action: ports.AuditPlatformRead, Outcome: "accepted", Tenant: tc.wantTenant, DataTenant: string(tenantB)},
			}
			if got := trail.Events(); !slices.Equal(got, want) {
				t.Fatalf("events = %+v, want %+v", got, want)
			}
		})
	}
}

func TestAnEmptyPlatformPageEmitsNothing(t *testing.T) {
	trail := &recordingTrail{}
	rows, err := counters.PlatformReader(memory.New(), trail).List(withReach(inTenant(t, tenantA)), nil, 10)
	if err != nil || len(rows) != 0 {
		t.Fatalf("List() = %v, %v; want an empty page", rows, err)
	}
	if got := trail.Events(); len(got) != 0 {
		t.Fatalf("events = %+v, want none: an empty page reaches no tenant", got)
	}
}

func TestPlatformListFailsClosedWhenTheSinkRefuses(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantA, "c-1", 1)

	rows, err := counters.PlatformReader(store, refusingTrail{}).List(withReach(inTenant(t, tenantA)), nil, 10)
	if !errors.Is(err, ports.ErrAuditUnavailable) || rows != nil {
		t.Fatalf("List() = %v, %v; want nil rows and ErrAuditUnavailable", rows, err)
	}
}

func TestPlatformAccessWithoutTheDeclaredReachIsRefusedAndRecorded(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantA, "c-1", 1)

	for _, tc := range []struct {
		name        string
		ctx         context.Context
		wantTenant  string
		wantSubject string
	}{
		{"scoped carrier", inTenant(t, tenantA), string(tenantA), "s-test"},
		{"reach declared undeclared", ports.WithPlatformReach(inTenant(t, tenantA), ports.ReachUndeclared), string(tenantA), "s-test"},
		{"carrier without a tenant", tenantlessCtx(t), "", "s-test"},
		{"bare carrier", context.Background(), "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trail := &recordingTrail{}
			reader := counters.PlatformReader(store, trail)

			rows, err := reader.List(tc.ctx, nil, 10)
			if !errors.Is(err, ports.ErrPlatformReachUndeclared) || rows != nil {
				t.Fatalf("List() = %v, %v; want ErrPlatformReachUndeclared and no rows", rows, err)
			}
			if _, err := reader.LoadIn(tc.ctx, tenantA, "c-1"); !errors.Is(err, ports.ErrPlatformReachUndeclared) {
				t.Fatalf("LoadIn() = %v, want ErrPlatformReachUndeclared", err)
			}
			listDenied := ports.AuditRecord{Subject: tc.wantSubject, Object: "counters", Action: ports.AuditPlatformRead, Outcome: "denied", Tenant: tc.wantTenant}
			loadDenied := listDenied
			loadDenied.DataTenant = string(tenantA)
			if got := trail.Events(); !slices.Equal(got, []ports.AuditRecord{listDenied, loadDenied}) {
				t.Fatalf("events = %+v, want %+v then %+v", got, listDenied, loadDenied)
			}
		})
	}
}

func TestARefusingSinkLeavesTheReachRefusalAsItIs(t *testing.T) {
	reader := counters.PlatformReader(memory.New(), refusingTrail{})
	if _, err := reader.List(inTenant(t, tenantA), nil, 10); !errors.Is(err, ports.ErrPlatformReachUndeclared) {
		t.Fatalf("List() = %v, want ErrPlatformReachUndeclared", err)
	}
}

func TestPlatformAccessWithoutAnExecutionContextIsRefused(t *testing.T) {
	trail := &recordingTrail{}
	reader := counters.PlatformReader(memory.New(), trail)
	ctx := withReach(context.Background())

	if _, err := reader.List(ctx, nil, 10); !errors.Is(err, ports.ErrContextAbsent) {
		t.Fatalf("List() = %v, want ErrContextAbsent", err)
	}
	if _, err := reader.LoadIn(ctx, tenantA, "c-1"); !errors.Is(err, ports.ErrContextAbsent) {
		t.Fatalf("LoadIn() = %v, want ErrContextAbsent", err)
	}
	if got := trail.Events(); len(got) != 0 {
		t.Fatalf("events = %+v, want none", got)
	}
}

func TestPlatformListRefusesALimitOutOfRange(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantA, "c-1", 1)
	ctx := withReach(inTenant(t, tenantA))

	for _, tc := range []struct {
		name   string
		reader ports.PlatformReader[counterID, counter]
		limit  int
		ok     bool
	}{
		{"zero", counters.PlatformReader(store, &recordingTrail{}), 0, false},
		{"negative", counters.PlatformReader(store, &recordingTrail{}), -1, false},
		{"above the default ceiling", counters.PlatformReader(store, &recordingTrail{}), ports.PlatformPageMax + 1, false},
		{"at the default ceiling", counters.PlatformReader(store, &recordingTrail{}), ports.PlatformPageMax, true},
		{"above a declared ceiling", counters.PlatformReader(store, &recordingTrail{}, memory.WithPageMax(2)), 3, false},
		{"at a declared ceiling", counters.PlatformReader(store, &recordingTrail{}, memory.WithPageMax(2)), 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := tc.reader.List(ctx, nil, tc.limit)
			switch {
			case tc.ok && (err != nil || len(rows) != 1):
				t.Fatalf("List(limit %d) = %v, %v; want the one row", tc.limit, rows, err)
			case !tc.ok && (!errors.Is(err, ports.ErrPlatformLimitInvalid) || rows != nil):
				t.Fatalf("List(limit %d) = %v, %v; want ErrPlatformLimitInvalid", tc.limit, rows, err)
			}
		})
	}
}

func TestAPageCeilingBelowOnePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("WithPageMax(0) did not panic")
		}
	}()
	memory.WithPageMax(0)
}

func TestPlatformLoadInReadsTheRowOfTheAskedTenant(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantB, "c-1", 21)
	trail := &recordingTrail{}

	row, err := counters.PlatformReader(store, trail).LoadIn(withReach(inTenant(t, tenantA)), tenantB, "c-1")
	if err != nil || row.Tenant != tenantB || row.ID != "c-1" || row.State.Total != 21 || row.Version != 1 {
		t.Fatalf("LoadIn() = %+v, %v; want globex/c-1 total 21 at v1", row, err)
	}
	want := ports.AuditRecord{Subject: "s-test", Object: "counters", Action: ports.AuditPlatformRead, Outcome: "accepted", Tenant: string(tenantA), DataTenant: string(tenantB)}
	if got := trail.Events(); !slices.Equal(got, []ports.AuditRecord{want}) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}
}

func TestPlatformLoadInMissesWithoutProbingTheOwner(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantA, "c-1", 1)
	trail := &recordingTrail{}

	_, err := counters.PlatformReader(store, trail).LoadIn(withReach(inTenant(t, tenantA)), tenantB, "c-1")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LoadIn() = %v, want ErrNotFound", err)
	}
	if _, probed := errors.AsType[ports.CrossTenantAccess](err); probed {
		t.Fatalf("LoadIn() = %v; a declared read across tenants is no mistaken access (IDN-12)", err)
	}
	if got := trail.Events(); len(got) != 1 || got[0].DataTenant != string(tenantB) || got[0].Outcome != "accepted" {
		t.Fatalf("events = %+v, want the accepted read of globex", got)
	}
}

func TestPlatformLoadInOfAnEmptyTenantFindsNothingAndRecordsNothing(t *testing.T) {
	trail := &recordingTrail{}
	_, err := counters.PlatformReader(memory.New(), trail).LoadIn(withReach(inTenant(t, tenantA)), "", "c-1")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LoadIn(empty tenant) = %v, want ErrNotFound", err)
	}
	if got := trail.Events(); len(got) != 0 {
		t.Fatalf("events = %+v, want none", got)
	}
}

func TestPlatformLoadInFailsClosedWhenTheSinkRefuses(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantB, "c-1", 21)

	row, err := counters.PlatformReader(store, refusingTrail{}).LoadIn(withReach(inTenant(t, tenantA)), tenantB, "c-1")
	if !errors.Is(err, ports.ErrAuditUnavailable) || row.Version != 0 || row.State.Total != 0 {
		t.Fatalf("LoadIn() = %+v, %v; want nothing and ErrAuditUnavailable", row, err)
	}
}

func TestTheScopedReaderIgnoresTheDeclaredReach(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantB, "c-1", 21)

	_, _, err := counters.Reader(store).Load(withReach(inTenant(t, tenantA)), "c-1")
	if _, ok := errors.AsType[ports.CrossTenantAccess](err); !ok || !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Reader.Load() under ReachAllTenants = %v, want the CrossTenantAccess of a scoped read", err)
	}
}

func TestAPlatformReaderWithoutATrailPanicsNamingTheTable(t *testing.T) {
	defer func() {
		got, _ := recover().(string)
		if !strings.Contains(got, "counters") {
			t.Fatalf("panic = %q, want it to name the table", got)
		}
	}()
	counters.PlatformReader(memory.New(), nil)
}

type policyCall struct {
	origin      *ports.TenantID
	destination ports.TenantID
}

func allowing(calls *[]policyCall) ports.ProjectionPolicy {
	return func(_ context.Context, origin *ports.TenantID, destination ports.TenantID) error {
		*calls = append(*calls, policyCall{origin: origin, destination: destination})
		return nil
	}
}

var errNotFunder = errors.New("memory_test: not the funder")

func denying(context.Context, *ports.TenantID, ports.TenantID) error { return errNotFunder }

func project(t *testing.T, store *memory.Store, ctx context.Context, policy ports.ProjectionPolicy, trail ports.AuditTrail, fn func(ctx context.Context, p ports.Projector[counterID, counter]) error) {
	t.Helper()
	uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) ports.Projector[counterID, counter] {
		return counters.Projector(tx, policy, trail)
	})
	if err := uow.Within(ctx, fn); err != nil {
		t.Fatalf("Within() = %v", err)
	}
}

func TestAProjectionWritesIntoTheDestinationOutOfTheOriginsSight(t *testing.T) {
	store := memory.New()
	trail := &recordingTrail{}
	var calls []policyCall
	var saved error

	project(t, store, inTenant(t, tenantA), allowing(&calls), trail, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
		saved = p.Save(ctx, tenantB, "c-1", counter{ID: "c-1", Total: 7}, 0)
		return nil
	})

	if saved != nil {
		t.Fatalf("Save() = %v", saved)
	}
	if got, v, err := counters.Reader(store).Load(inTenant(t, tenantB), "c-1"); err != nil || got.Total != 7 || v != 1 {
		t.Fatalf("destination Load() = %+v v%d, %v; want total 7 at v1", got, v, err)
	}
	if _, _, err := counters.Reader(store).Load(inTenant(t, tenantA), "c-1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("origin Load() = %v, want ErrNotFound", err)
	}
	if len(calls) != 1 || calls[0].origin == nil || *calls[0].origin != tenantA || calls[0].destination != tenantB {
		t.Fatalf("policy calls = %+v, want origin acme and destination globex", calls)
	}
	want := ports.AuditRecord{Subject: "s-test", Object: "counters", Action: ports.AuditProjectionWrite, Outcome: "accepted", Tenant: string(tenantA), DataTenant: string(tenantB)}
	if got := trail.Events(); !slices.Equal(got, []ports.AuditRecord{want}) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}
}

func TestAProjectionWithoutAnAcceptingPolicyIsDeniedAndWritesNothing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy ports.ProjectionPolicy
		cause  error
	}{
		{"absent policy", nil, nil},
		{"denying policy", denying, errNotFunder},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := memory.New()
			trail := &recordingTrail{}
			var saved, loaded error

			project(t, store, inTenant(t, tenantA), tc.policy, trail, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
				saved = p.Save(ctx, tenantB, "c-1", counter{ID: "c-1", Total: 7}, 0)
				_, _, loaded = p.Load(ctx, tenantB, "c-1")
				return nil
			})

			for _, err := range []error{saved, loaded} {
				if !errors.Is(err, ports.ErrProjectionDenied) || (tc.cause != nil && !errors.Is(err, tc.cause)) {
					t.Fatalf("err = %v, want ErrProjectionDenied wrapping %v", err, tc.cause)
				}
			}
			if _, _, err := counters.Reader(store).Load(inTenant(t, tenantB), "c-1"); !errors.Is(err, ports.ErrNotFound) {
				t.Fatalf("destination Load() = %v, want ErrNotFound: nothing written", err)
			}
			denied := ports.AuditRecord{Subject: "s-test", Object: "counters", Action: ports.AuditProjectionWrite, Outcome: "denied", Tenant: string(tenantA), DataTenant: string(tenantB)}
			if got := trail.Events(); !slices.Equal(got, []ports.AuditRecord{denied, denied}) {
				t.Fatalf("events = %+v, want two %+v", got, denied)
			}
		})
	}
}

func TestARefusingSinkLeavesTheProjectionDenialAsItIs(t *testing.T) {
	var saved error
	project(t, memory.New(), inTenant(t, tenantA), denying, refusingTrail{}, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
		saved = p.Save(ctx, tenantB, "c-1", counter{}, 0)
		return nil
	})
	if !errors.Is(saved, ports.ErrProjectionDenied) {
		t.Fatalf("Save() = %v, want ErrProjectionDenied", saved)
	}
}

func TestAProjectionWithoutADestinationIsRefusedBeforeThePolicy(t *testing.T) {
	trail := &recordingTrail{}
	var calls []policyCall
	var saved, loaded error

	project(t, memory.New(), inTenant(t, tenantA), allowing(&calls), trail, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
		saved = p.Save(ctx, "", "c-1", counter{}, 0)
		_, _, loaded = p.Load(ctx, "", "c-1")
		return nil
	})

	for _, err := range []error{saved, loaded} {
		if !errors.Is(err, ports.ErrProjectionDestinationAbsent) {
			t.Fatalf("err = %v, want ErrProjectionDestinationAbsent", err)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("policy calls = %+v, want none", calls)
	}
	denied := ports.AuditRecord{Subject: "s-test", Object: "counters", Action: ports.AuditProjectionWrite, Outcome: "denied", Tenant: string(tenantA)}
	if got := trail.Events(); !slices.Equal(got, []ports.AuditRecord{denied, denied}) {
		t.Fatalf("events = %+v, want two %+v", got, denied)
	}
}

func TestAProjectionIntoTheOriginsOwnTenantIsDeniedBeforeThePolicy(t *testing.T) {
	store := memory.New()
	trail := &recordingTrail{}
	var calls []policyCall
	var saved, loaded error

	project(t, store, inTenant(t, tenantA), allowing(&calls), trail, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
		saved = p.Save(ctx, tenantA, "c-1", counter{ID: "c-1", Total: 7}, 0)
		_, _, loaded = p.Load(ctx, tenantA, "c-1")
		return nil
	})

	for _, err := range []error{saved, loaded} {
		if !errors.Is(err, ports.ErrProjectionDenied) {
			t.Fatalf("err = %v, want ErrProjectionDenied", err)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("policy calls = %+v, want none", calls)
	}
	if _, _, err := counters.Reader(store).Load(inTenant(t, tenantA), "c-1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("origin Load() = %v, want ErrNotFound: nothing written", err)
	}
	denied := ports.AuditRecord{Subject: "s-test", Object: "counters", Action: ports.AuditProjectionWrite, Outcome: "denied", Tenant: string(tenantA), DataTenant: string(tenantA)}
	if got := trail.Events(); !slices.Equal(got, []ports.AuditRecord{denied, denied}) {
		t.Fatalf("events = %+v, want two %+v", got, denied)
	}
}

func TestAnAbsentOriginReachesThePolicyAsNil(t *testing.T) {
	store := memory.New()
	trail := &recordingTrail{}
	var calls []policyCall
	var saved error

	project(t, store, tenantlessCtx(t), allowing(&calls), trail, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
		saved = p.Save(ctx, tenantB, "c-1", counter{ID: "c-1", Total: 7}, 0)
		return nil
	})

	if saved != nil {
		t.Fatalf("Save() = %v", saved)
	}
	if len(calls) != 1 || calls[0].origin != nil {
		t.Fatalf("policy calls = %+v, want one with a nil origin (CTX-26)", calls)
	}
	if got := trail.Events(); len(got) != 1 || got[0].Tenant != "" || got[0].DataTenant != string(tenantB) {
		t.Fatalf("events = %+v, want one with no tenant and globex as data tenant", got)
	}
}

func TestAProjectionFailsClosedWhenTheSinkRefuses(t *testing.T) {
	store := memory.New()
	var none []policyCall
	project(t, store, inTenant(t, tenantA), allowing(&none), &recordingTrail{}, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
		return p.Save(ctx, tenantB, "c-1", counter{ID: "c-1", Total: 7}, 0)
	})
	var saved, loaded error
	var got counter

	project(t, store, inTenant(t, tenantA), allowing(&none), refusingTrail{}, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
		saved = p.Save(ctx, tenantB, "c-2", counter{ID: "c-2", Total: 9}, 0)
		got, _, loaded = p.Load(ctx, tenantB, "c-1")
		return nil
	})

	if !errors.Is(saved, ports.ErrAuditUnavailable) || !errors.Is(loaded, ports.ErrAuditUnavailable) || got.Total != 0 {
		t.Fatalf("Save() = %v, Load() = %+v, %v; want ErrAuditUnavailable and nothing read", saved, got, loaded)
	}
	if _, _, err := counters.Reader(store).Load(inTenant(t, tenantB), "c-2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("destination Load() = %v, want ErrNotFound: nothing written", err)
	}
}

func TestAProjectionLoadReadsTheDestinationWithoutProbingTheOwner(t *testing.T) {
	store := memory.New()
	seedIn(t, store, tenantA, "c-9", 1)
	var calls []policyCall
	project(t, store, inTenant(t, tenantA), allowing(&calls), &recordingTrail{}, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
		return p.Save(ctx, tenantB, "c-1", counter{ID: "c-1", Total: 7}, 0)
	})
	trail := &recordingTrail{}
	var got counter
	var version ports.Version
	var loaded, missed error

	project(t, store, inTenant(t, tenantA), allowing(&calls), trail, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
		got, version, loaded = p.Load(ctx, tenantB, "c-1")
		_, _, missed = p.Load(ctx, tenantB, "c-9")
		return nil
	})

	if loaded != nil || got.Total != 7 || version != 1 {
		t.Fatalf("Load() = %+v v%d, %v; want total 7 at v1", got, version, loaded)
	}
	if !errors.Is(missed, ports.ErrNotFound) {
		t.Fatalf("Load() of an identifier held by another tenant = %v, want ErrNotFound", missed)
	}
	if _, probed := errors.AsType[ports.CrossTenantAccess](missed); probed {
		t.Fatalf("Load() = %v; the projection runs no owner probe", missed)
	}
	if events := trail.Events(); len(events) != 2 || events[0].Outcome != "accepted" || events[1].DataTenant != string(tenantB) {
		t.Fatalf("events = %+v, want two accepted reads of globex", events)
	}
}

func TestAProjectionHonoursTheStoredVersion(t *testing.T) {
	store := memory.New()
	var calls []policyCall
	var errs [5]error

	project(t, store, inTenant(t, tenantA), allowing(&calls), &recordingTrail{}, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
		errs[0] = p.Save(ctx, tenantB, "c-1", counter{ID: "c-1", Total: 1}, 0)
		errs[1] = p.Save(ctx, tenantB, "c-1", counter{ID: "c-1", Total: 2}, 1)
		errs[2] = p.Save(ctx, tenantB, "c-1", counter{ID: "c-1", Total: 3}, 1)
		errs[3] = p.Save(ctx, tenantB, "c-1", counter{ID: "c-1", Total: 4}, 0)
		errs[4] = p.Save(ctx, tenantB, "c-1", counter{ID: "c-1", Total: 5}, 99)
		return nil
	})

	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("Save() at the stored versions = %v, %v; want nil", errs[0], errs[1])
	}
	for _, err := range errs[2:] {
		if !errors.Is(err, ports.ErrVersionConflict) {
			t.Fatalf("Save() off the stored version = %v, want ErrVersionConflict", err)
		}
	}
	if got, v, err := counters.Reader(store).Load(inTenant(t, tenantB), "c-1"); err != nil || got.Total != 2 || v != 2 {
		t.Fatalf("destination Load() = %+v v%d, %v; want total 2 at v2", got, v, err)
	}
}

func TestAProjectionHonoursTheDestinationsDeclaredUniqueness(t *testing.T) {
	store := memory.New()
	seedAccount(t, store, inTenant(t, tenantB), "a-1", newAccount(1))
	var calls []policyCall
	var saved, elsewhere error

	uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) ports.Projector[string, account] {
		return accounts.Projector(tx, allowing(&calls), &recordingTrail{})
	})
	if err := uow.Within(tenantlessCtx(t), func(ctx context.Context, p ports.Projector[string, account]) error {
		saved = p.Save(ctx, tenantB, "a-2", newAccount(1), 0)
		elsewhere = p.Save(ctx, tenantA, "a-3", newAccount(1), 0)
		return nil
	}); err != nil {
		t.Fatalf("Within() = %v", err)
	}

	requireViolation(t, saved, emailKey, emailTaken)
	if elsewhere != nil {
		t.Fatalf("Save() of the same value into another tenant = %v, want nil", elsewhere)
	}
}

func TestAProjectorWithoutATrailPanicsNamingTheTable(t *testing.T) {
	defer func() {
		got, _ := recover().(string)
		if !strings.Contains(got, "counters") {
			t.Fatalf("panic = %q, want it to name the table", got)
		}
	}()
	counters.Projector(nil, denying, nil)
}

func TestAProjectorOverAnInvalidDeclarationPanics(t *testing.T) {
	defer func() {
		got, _ := recover().(string)
		if !strings.Contains(got, "without a key") {
			t.Fatalf("panic = %q, want the declaration named", got)
		}
	}()
	table := memory.Table[string, account]{Name: "accounts", Unique: []memory.Unique[account]{{Name: emailKey, Rejection: emailTaken}}}
	table.Projector(nil, denying, &recordingTrail{})
}

func TestAProjectionWithoutAnExecutionContextIsRefusedUnrecorded(t *testing.T) {
	store := memory.New()
	trail := &recordingTrail{}
	var calls []policyCall
	var saved, loaded error

	project(t, store, context.Background(), allowing(&calls), trail, func(ctx context.Context, p ports.Projector[counterID, counter]) error {
		saved = p.Save(ctx, tenantB, "c-1", counter{ID: "c-1", Total: 7}, 0)
		_, _, loaded = p.Load(ctx, tenantB, "c-1")
		return nil
	})

	for _, err := range []error{saved, loaded} {
		if !errors.Is(err, ports.ErrContextAbsent) {
			t.Fatalf("err = %v, want ErrContextAbsent", err)
		}
	}
	if len(calls) != 0 || len(trail.Events()) != 0 {
		t.Fatalf("policy calls = %+v, events = %+v; want neither", calls, trail.Events())
	}
	if _, _, err := counters.Reader(store).Load(inTenant(t, tenantB), "c-1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("destination Load() = %v, want ErrNotFound: nothing written", err)
	}
}
