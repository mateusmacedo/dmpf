package providerkit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type ProjectionSubject[ID comparable, S any] struct {
	Within   func(ctx context.Context, policy ports.ProjectionPolicy, trail ports.AuditTrail, fn func(ctx context.Context, p ports.Projector[ID, S]) error) error
	Reader   ports.Reader[ID, S]
	NewID    func(n int) ID
	NewState func(marker int) S
	Marker   func(S) int
	Table    string
}

var (
	errKitPolicyDenied = errors.New("providerkit: the destination policy denied the projection")
	errKitWorkFailed   = errors.New("providerkit: the unit of work failed after projecting")
)

// Projection exercises the write into an explicit destination tenant; newSubject
// must return a subject over a fresh resource on every call. A clause commits
// after the refusal it provokes, so a write let through before refusing shows.
func Projection[ID comparable, S any](newSubject func() ProjectionSubject[ID, S]) Verdict {
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
	bare, err := tenantless()
	if err != nil {
		v.fail("mounts the execution context the edge would have mounted", "CTX-01", "NewExecutionContext() = %v", err)
		return v
	}

	{
		const clause = "writes into the destination, out of the origin's sight"
		s := newSubject()
		var asked policyCalls
		trail := &recordingTrail{}
		var saved error
		if err := s.project(acme, asked.allow, trail, func(ctx context.Context, p ports.Projector[ID, S]) {
			saved = p.Save(ctx, kitTenantB, s.NewID(1), s.NewState(31), 0)
		}); err != nil {
			v.fail(clause, "UOW-01", "Within() = %v, want nil", err)
		} else if saved != nil {
			v.fail(clause, "IDN-11", "Save() into the destination = %v, want nil", saved)
		} else {
			s.expectStored(&v, clause, "IDN-11", globex, s.NewID(1), 31, 1, "the destination")
			if _, _, err := s.Reader.Load(acme, s.NewID(1)); !errors.Is(err, ports.ErrNotFound) {
				v.fail(clause, "IDN-11", "Reader.Load() for the origin = %v, want ErrNotFound: the row belongs to the destination", err)
			}
			asked.expect(&v, clause, ptr(kitTenantA), kitTenantB)
			expectTrail(&v, clause, "LOG-14", "Save()", trail, s.writeEvent(string(kitTenantA), kitTenantB, ports.OutcomeAccepted))
		}
	}

	{
		const clause = "denies without an accepting policy and writes nothing"
		s := newSubject()
		for row, policy := range []struct {
			name   string
			policy ports.ProjectionPolicy
			cause  error
		}{
			{"an absent policy", nil, nil},
			{"a denying policy", denyAll, errKitPolicyDenied},
		} {
			id := s.NewID(row + 1)
			trail := &recordingTrail{}
			var saved, loaded error
			if err := s.project(acme, policy.policy, trail, func(ctx context.Context, p ports.Projector[ID, S]) {
				saved = p.Save(ctx, kitTenantB, id, s.NewState(1), 0)
				_, _, loaded = p.Load(ctx, kitTenantB, id)
			}); err != nil {
				v.fail(clause, "UOW-01", "Within() under %s = %v, want nil", policy.name, err)
				continue
			}
			for _, call := range []struct {
				by  string
				err error
			}{{"Save()", saved}, {"Load()", loaded}} {
				if !errors.Is(call.err, ports.ErrProjectionDenied) || (policy.cause != nil && !errors.Is(call.err, policy.cause)) {
					v.fail(clause, "IDN-17", "%s under %s = %v, want ErrProjectionDenied carrying the policy's reason", call.by, policy.name, call.err)
				}
			}
			s.expectAbsent(&v, clause, "IDN-17", globex, id, "under "+policy.name)
			denied := s.writeEvent(string(kitTenantA), kitTenantB, ports.OutcomeDenied)
			expectTrail(&v, clause, "LOG-14", "the denials under "+policy.name, trail, denied, denied)
		}

		var saved error
		if err := s.project(acme, denyAll, refusingTrail{}, func(ctx context.Context, p ports.Projector[ID, S]) {
			saved = p.Save(ctx, kitTenantB, s.NewID(3), s.NewState(1), 0)
		}); err != nil {
			v.fail(clause, "UOW-01", "Within() = %v, want nil", err)
		} else if !errors.Is(saved, ports.ErrProjectionDenied) {
			v.fail(clause, "IDN-17", "Save() denied over a refusing trail = %v, want ErrProjectionDenied: the failed record leaves the denial as it is", saved)
		}
	}

	{
		const clause = "refuses an empty destination before the policy"
		s := newSubject()
		var asked policyCalls
		trail := &recordingTrail{}
		var saved, loaded error
		if err := s.project(acme, asked.allow, trail, func(ctx context.Context, p ports.Projector[ID, S]) {
			saved = p.Save(ctx, "", s.NewID(1), s.NewState(1), 0)
			_, _, loaded = p.Load(ctx, "", s.NewID(1))
		}); err != nil {
			v.fail(clause, "UOW-01", "Within() = %v, want nil", err)
		} else {
			for _, call := range []struct {
				by  string
				err error
			}{{"Save()", saved}, {"Load()", loaded}} {
				if !errors.Is(call.err, ports.ErrProjectionDestinationAbsent) {
					v.fail(clause, "IDN-20", "%s into an empty destination = %v, want ErrProjectionDestinationAbsent", call.by, call.err)
				}
			}
			if len(asked) != 0 {
				v.fail(clause, "IDN-20", "the policy was asked %+v, want nothing: there is no destination to decide on", asked)
			}
			denied := s.writeEvent(string(kitTenantA), "", ports.OutcomeDenied)
			expectTrail(&v, clause, "LOG-14", "the refusals", trail, denied, denied)
		}
	}

	{
		const clause = "refuses to project into the origin's own tenant"
		s := newSubject()
		var asked policyCalls
		trail := &recordingTrail{}
		var saved, loaded error
		if err := s.project(acme, asked.allow, trail, func(ctx context.Context, p ports.Projector[ID, S]) {
			saved = p.Save(ctx, kitTenantA, s.NewID(1), s.NewState(1), 0)
			_, _, loaded = p.Load(ctx, kitTenantA, s.NewID(1))
		}); err != nil {
			v.fail(clause, "UOW-01", "Within() = %v, want nil", err)
		} else {
			for _, call := range []struct {
				by  string
				err error
			}{{"Save()", saved}, {"Load()", loaded}} {
				if !errors.Is(call.err, ports.ErrProjectionDenied) {
					v.fail(clause, "IDN-17", "%s into the origin's own tenant = %v, want ErrProjectionDenied: the projection is no second write path into it", call.by, call.err)
				}
			}
			if len(asked) != 0 {
				v.fail(clause, "IDN-17", "the policy was asked %+v, want nothing: no policy opens the origin's own tenant", asked)
			}
			s.expectAbsent(&v, clause, "IDN-17", acme, s.NewID(1), "into the origin's own tenant")
			denied := s.writeEvent(string(kitTenantA), kitTenantA, ports.OutcomeDenied)
			expectTrail(&v, clause, "LOG-14", "the refusals", trail, denied, denied)
		}
	}

	{
		const clause = "refuses when the carrier holds no context"
		s := newSubject()
		var asked policyCalls
		trail := &recordingTrail{}
		var saved, loaded error
		if err := s.project(context.Background(), asked.allow, trail, func(ctx context.Context, p ports.Projector[ID, S]) {
			saved = p.Save(ctx, kitTenantB, s.NewID(1), s.NewState(1), 0)
			_, _, loaded = p.Load(ctx, kitTenantB, s.NewID(1))
		}); err != nil {
			v.fail(clause, "UOW-01", "Within() off a bare context = %v, want nil", err)
		} else {
			for _, call := range []struct {
				by  string
				err error
			}{{"Save()", saved}, {"Load()", loaded}} {
				if !errors.Is(call.err, ports.ErrContextAbsent) {
					v.fail(clause, "IDN-15", "%s off a bare context = %v, want ErrContextAbsent", call.by, call.err)
				}
			}
			if len(asked) != 0 {
				v.fail(clause, "IDN-15", "the policy was asked %+v, want nothing: there is no carrier to decide on", asked)
			}
			expectTrail(&v, clause, "LOG-14", "the refusals", trail)
			s.expectAbsent(&v, clause, "IDN-15", globex, s.NewID(1), "off a bare context")
		}
	}

	{
		const clause = "hands an absent origin to the policy as nil"
		s := newSubject()
		var asked policyCalls
		trail := &recordingTrail{}
		var saved error
		if err := s.project(bare, asked.allow, trail, func(ctx context.Context, p ports.Projector[ID, S]) {
			saved = p.Save(ctx, kitTenantB, s.NewID(1), s.NewState(5), 0)
		}); err != nil {
			v.fail(clause, "UOW-01", "Within() = %v, want nil", err)
		} else if saved != nil {
			v.fail(clause, "CTX-26", "Save() from a carrier without a tenant = %v, want nil", saved)
		} else {
			asked.expect(&v, clause, nil, kitTenantB)
			expectTrail(&v, clause, "CTX-26", "Save()", trail, s.writeEvent("", kitTenantB, ports.OutcomeAccepted))
			s.expectStored(&v, clause, "IDN-11", globex, s.NewID(1), 5, 1, "the destination")
		}
	}

	{
		const clause = "writes and reads nothing when the trail refuses the record"
		s := newSubject()
		if s.create(&v, clause, acme, kitTenantB, 1, 61) {
			var saved, loaded error
			var version ports.Version
			if err := s.project(acme, allowAll, refusingTrail{}, func(ctx context.Context, p ports.Projector[ID, S]) {
				saved = p.Save(ctx, kitTenantB, s.NewID(2), s.NewState(62), 0)
				_, version, loaded = p.Load(ctx, kitTenantB, s.NewID(1))
			}); err != nil {
				v.fail(clause, "UOW-01", "Within() = %v, want nil", err)
			} else {
				if !errors.Is(saved, ports.ErrAuditUnavailable) {
					v.fail(clause, "DAT-25", "Save() over a refusing trail = %v, want ErrAuditUnavailable", saved)
				}
				if !errors.Is(loaded, ports.ErrAuditUnavailable) || version != 0 {
					v.fail(clause, "DAT-25", "Load() over a refusing trail = v%d, %v; want nothing and ErrAuditUnavailable", version, loaded)
				}
				s.expectAbsent(&v, clause, "DAT-25", globex, s.NewID(2), "after a refused record")
			}
		}
	}

	{
		const clause = "loads the destination under the policy"
		s := newSubject()
		if s.create(&v, clause, acme, kitTenantB, 1, 41) {
			trail := &recordingTrail{}
			var state S
			var version ports.Version
			var loaded error
			if err := s.project(acme, allowAll, trail, func(ctx context.Context, p ports.Projector[ID, S]) {
				state, version, loaded = p.Load(ctx, kitTenantB, s.NewID(1))
			}); err != nil {
				v.fail(clause, "UOW-01", "Within() = %v, want nil", err)
			} else if loaded != nil {
				v.fail(clause, "IDN-11", "Load() of the destination = %v, want nil", loaded)
			} else {
				if got := s.Marker(state); got != 41 || version != 1 {
					v.fail(clause, "IDN-11", "Load() of the destination = marker %d, v%d; want marker 41, v1", got, version)
				}
				expectTrail(&v, clause, "LOG-14", "Load()", trail, s.writeEvent(string(kitTenantA), kitTenantB, ports.OutcomeAccepted))
			}
		}
	}

	{
		const clause = "misses in the destination without probing the owner"
		s := newSubject()
		if s.create(&v, clause, bare, kitTenantA, 1, 51) {
			trail := &recordingTrail{}
			var loaded error
			if err := s.project(acme, allowAll, trail, func(ctx context.Context, p ports.Projector[ID, S]) {
				_, _, loaded = p.Load(ctx, kitTenantB, s.NewID(1))
			}); err != nil {
				v.fail(clause, "UOW-01", "Within() = %v, want nil", err)
			} else {
				if !errors.Is(loaded, ports.ErrNotFound) {
					v.fail(clause, "IDN-13", "Load() of an identifier another tenant holds = %v, want ErrNotFound", loaded)
				} else if _, probed := crossTenant(loaded); probed {
					v.fail(clause, "IDN-12", "Load() = %v; the projection runs no owner probe, which would reveal the owner", loaded)
				}
				expectTrail(&v, clause, "LOG-14", "the miss", trail, s.writeEvent(string(kitTenantA), kitTenantB, ports.OutcomeAccepted))
			}
		}
	}

	{
		const clause = "honours the stored version"
		s := newSubject()
		id := s.NewID(1)
		save := func(marker int, expected ports.Version) error {
			var saved error
			if err := s.project(acme, allowAll, &recordingTrail{}, func(ctx context.Context, p ports.Projector[ID, S]) {
				saved = p.Save(ctx, kitTenantB, id, s.NewState(marker), expected)
			}); err != nil {
				return err
			}
			return saved
		}
		created, updated := save(1, 0), save(2, 1)
		switch {
		case created != nil:
			v.fail(clause, "UOW-09", "Save() creating the aggregate = %v, want nil", created)
		case updated != nil:
			v.fail(clause, "UOW-09", "Save() at the stored version = %v, want nil", updated)
		default:
			for _, stale := range []struct {
				what     string
				expected ports.Version
			}{{"a stale version", 1}, {"a version ahead", 99}, {"a creation over the aggregate", 0}} {
				if err := save(9, stale.expected); !errors.Is(err, ports.ErrVersionConflict) {
					v.fail(clause, "UOW-09", "Save() at %s = %v, want ErrVersionConflict", stale.what, err)
				}
			}
			s.expectStored(&v, clause, "UOW-09", globex, id, 2, 2, "the destination")
		}
	}

	{
		const clause = "rolls the destination back with the unit of work"
		s := newSubject()
		var saved error
		err := s.Within(acme, allowAll, &recordingTrail{}, func(ctx context.Context, p ports.Projector[ID, S]) error {
			saved = p.Save(ctx, kitTenantB, s.NewID(1), s.NewState(71), 0)
			return errKitWorkFailed
		})
		switch {
		case saved != nil:
			v.fail(clause, "IDN-11", "Save() into the destination = %v, want nil", saved)
		case !errors.Is(err, errKitWorkFailed):
			v.fail(clause, "UOW-01", "Within() = %v, want the callback's error", err)
		default:
			s.expectAbsent(&v, clause, "UOW-01", globex, s.NewID(1), "after the unit of work failed")
		}
	}

	{
		const clause = "refuses to be built without a trail"
		s := newSubject()
		var ran bool
		recovered := panicOf(func() {
			_ = s.Within(acme, allowAll, nil, func(context.Context, ports.Projector[ID, S]) error {
				ran = true
				return nil
			})
		})
		switch {
		case recovered == nil:
			v.fail(clause, "DAT-25", "a projector built without a trail did not panic: an access across tenants without its trail is a composition error")
		case ran:
			v.fail(clause, "DAT-25", "the callback ran over a projector built without a trail")
		case !strings.Contains(fmt.Sprint(recovered), s.Table):
			v.fail(clause, "DAT-25", "the projector panicked with %q, want the table %q named", recovered, s.Table)
		}
	}

	return v
}

func (s ProjectionSubject[ID, S]) project(ctx context.Context, policy ports.ProjectionPolicy, trail ports.AuditTrail, fn func(ctx context.Context, p ports.Projector[ID, S])) error {
	return s.Within(ctx, policy, trail, func(ctx context.Context, p ports.Projector[ID, S]) error {
		fn(ctx, p)
		return nil
	})
}

func (s ProjectionSubject[ID, S]) create(v *Verdict, clause string, ctx context.Context, destination ports.TenantID, row, marker int) bool {
	var saved error
	err := s.project(ctx, allowAll, &recordingTrail{}, func(ctx context.Context, p ports.Projector[ID, S]) {
		saved = p.Save(ctx, destination, s.NewID(row), s.NewState(marker), 0)
	})
	if err = errors.Join(err, saved); err != nil {
		v.fail(clause, "IDN-11", "Save() into %q = %v, want nil", destination, err)
		return false
	}
	return true
}

func (s ProjectionSubject[ID, S]) expectStored(v *Verdict, clause, rule string, ctx context.Context, id ID, wantMarker int, wantVersion ports.Version, whose string) {
	state, version, err := s.Reader.Load(ctx, id)
	switch {
	case err != nil:
		v.fail(clause, rule, "Reader.Load() for %s = %v, want nil", whose, err)
	case s.Marker(state) != wantMarker || version != wantVersion:
		v.fail(clause, rule, "%s reads marker %d v%d, want marker %d v%d", whose, s.Marker(state), version, wantMarker, wantVersion)
	}
}

func (s ProjectionSubject[ID, S]) expectAbsent(v *Verdict, clause, rule string, ctx context.Context, id ID, when string) {
	if _, _, err := s.Reader.Load(ctx, id); !errors.Is(err, ports.ErrNotFound) {
		v.fail(clause, rule, "Reader.Load() of the destination %s = %v, want ErrNotFound: nothing may be written", when, err)
	}
}

func (s ProjectionSubject[ID, S]) writeEvent(tenant string, data ports.TenantID, outcome ports.OutcomeCategory) ports.AuditRecord {
	return ports.AuditRecord{Subject: kitSubject, Object: s.Table, Action: ports.AuditProjectionWrite, Outcome: string(outcome), Tenant: tenant, DataTenant: string(data)}
}

type policyCall struct {
	origin      *ports.TenantID
	destination ports.TenantID
}

type policyCalls []policyCall

func (c *policyCalls) allow(_ context.Context, origin *ports.TenantID, destination ports.TenantID) error {
	*c = append(*c, policyCall{origin: origin, destination: destination})
	return nil
}

func (c policyCalls) expect(v *Verdict, clause string, origin *ports.TenantID, destination ports.TenantID) {
	if len(c) != 1 {
		v.fail(clause, "IDN-17", "the policy was asked %d times, want once", len(c))
		return
	}
	got := c[0]
	switch {
	case (got.origin == nil) != (origin == nil) || (origin != nil && *got.origin != *origin):
		v.fail(clause, "CTX-26", "the policy got origin %s, want %s — absence stays absence, never a stand-in tenant", describe(got.origin), describe(origin))
	case got.destination != destination:
		v.fail(clause, "IDN-17", "the policy got destination %q, want %q", got.destination, destination)
	}
}

func describe(tenant *ports.TenantID) string {
	if tenant == nil {
		return "nil"
	}
	return fmt.Sprintf("%q", *tenant)
}

func ptr(tenant ports.TenantID) *ports.TenantID { return &tenant }

func allowAll(context.Context, *ports.TenantID, ports.TenantID) error { return nil }

func denyAll(context.Context, *ports.TenantID, ports.TenantID) error { return errKitPolicyDenied }
