package memory

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type PlatformOption func(*platformOptions)

type platformOptions struct{ pageMax int }

func WithPageMax(n int) PlatformOption {
	if n <= 0 {
		panic(fmt.Sprintf("memory: page ceiling %d is not positive", n))
	}
	return func(c *platformOptions) { c.pageMax = n }
}

// List pages by the %#v of the ID: distinct IDs that render alike (pointers to
// equal values) tie and may be skipped, so IDs must render distinctly.
func (t Table[ID, S]) PlatformReader(s *Store, trail ports.AuditTrail, opts ...PlatformOption) ports.PlatformReader[ID, S] {
	t.requireTrail(trail)
	options := platformOptions{pageMax: ports.PlatformPageMax}
	for _, opt := range opts {
		opt(&options)
	}
	return platformReader[ID, S]{store: s, table: t, trail: trail, pageMax: options.pageMax}
}

// A nil policy is not a construction error: every call is denied (IDN-17).
func (t Table[ID, S]) Projector(tx *Tx, policy ports.ProjectionPolicy, trail ports.AuditTrail) ports.Projector[ID, S] {
	t.requireTrail(trail)
	t.validate()
	return txProjector[ID, S]{tx: tx, table: t, policy: policy, trail: trail}
}

func (t Table[ID, S]) requireTrail(trail ports.AuditTrail) {
	if trail == nil {
		panic(fmt.Sprintf("memory: table %q crosses tenants without an audit trail", t.Name))
	}
}

type platformReader[ID comparable, S any] struct {
	store   *Store
	table   Table[ID, S]
	trail   ports.AuditTrail
	pageMax int
}

func (r platformReader[ID, S]) LoadIn(ctx context.Context, tenant ports.TenantID, id ID) (ports.PlatformRow[ID, S], error) {
	var none ports.PlatformRow[ID, S]
	if err := r.declared(ctx, tenant); err != nil {
		return none, err
	}
	if tenant == "" {
		return none, ports.ErrNotFound
	}
	if err := r.trail.Record(ctx, trailEvent(ctx, ports.AuditPlatformRead, r.table.Name, tenant, ports.OutcomeAccepted)); err != nil {
		return none, err
	}

	r.store.dataMu.Lock()
	defer r.store.dataMu.Unlock()
	rows, ok := r.store.tables[r.table.Name]
	if !ok {
		return none, ports.ErrNotFound
	}
	rec, ok := rows.rows[rowKey{tenant: tenant, id: id}]
	if !ok {
		return none, ports.ErrNotFound
	}
	return ports.PlatformRow[ID, S]{Tenant: tenant, ID: id, State: r.table.copy(rec.snapshot.(S)), Version: rec.version}, nil
}

func (r platformReader[ID, S]) List(ctx context.Context, after *ports.PlatformKey[ID], limit int) ([]ports.PlatformRow[ID, S], error) {
	if err := r.declared(ctx, ""); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > r.pageMax {
		return nil, fmt.Errorf("memory: %s: limit %d: %w", r.table.Name, limit, ports.ErrPlatformLimitInvalid)
	}

	page := r.page(after, limit)
	for i, row := range page {
		if i > 0 && page[i-1].Tenant == row.Tenant {
			continue
		}
		if err := r.trail.Record(ctx, trailEvent(ctx, ports.AuditPlatformRead, r.table.Name, row.Tenant, ports.OutcomeAccepted)); err != nil {
			return nil, err
		}
	}
	return page, nil
}

func (r platformReader[ID, S]) page(after *ports.PlatformKey[ID], limit int) []ports.PlatformRow[ID, S] {
	r.store.dataMu.Lock()
	defer r.store.dataMu.Unlock()
	rows, ok := r.store.tables[r.table.Name]
	if !ok {
		return []ports.PlatformRow[ID, S]{}
	}

	keys := make([]rowKey, 0, len(rows.rows))
	for candidate := range rows.rows {
		key := candidate.(rowKey)
		if after == nil || comparePlatform(key, rowKey{tenant: after.Tenant, id: after.ID}) > 0 {
			keys = append(keys, key)
		}
	}
	slices.SortFunc(keys, comparePlatform)

	page := make([]ports.PlatformRow[ID, S], 0, min(limit, len(keys)))
	for _, key := range keys[:min(limit, len(keys))] {
		rec := rows.rows[key]
		page = append(page, ports.PlatformRow[ID, S]{Tenant: key.tenant, ID: key.id.(ID), State: r.table.copy(rec.snapshot.(S)), Version: rec.version})
	}
	return page
}

func comparePlatform(a, b rowKey) int {
	return cmp.Or(cmp.Compare(a.tenant, b.tenant), cmp.Compare(fmt.Sprintf("%#v", a.id), fmt.Sprintf("%#v", b.id)))
}

// declared refuses before any read, in the order the Postgres realization does,
// so the two answer an undeclared or contextless call alike.
func (r platformReader[ID, S]) declared(ctx context.Context, asked ports.TenantID) error {
	if reach, _ := ports.PlatformReachFrom(ctx); reach != ports.ReachAllTenants {
		_ = r.trail.Record(ctx, trailEvent(ctx, ports.AuditPlatformRead, r.table.Name, asked, ports.OutcomeDenied))
		return fmt.Errorf("memory: %s: %w", r.table.Name, ports.ErrPlatformReachUndeclared)
	}
	if _, err := ports.RequireExecutionContext(ctx); err != nil {
		return err
	}
	return nil
}

type txProjector[ID comparable, S any] struct {
	tx     *Tx
	table  Table[ID, S]
	policy ports.ProjectionPolicy
	trail  ports.AuditTrail
}

func (p txProjector[ID, S]) Load(ctx context.Context, destination ports.TenantID, id ID) (S, ports.Version, error) {
	var zero S
	if err := p.authorize(ctx, destination); err != nil {
		return zero, 0, err
	}
	rows, ok := p.tx.tables[p.table.Name]
	if !ok {
		return zero, 0, ports.ErrNotFound
	}
	rec, ok := rows.rows[rowKey{tenant: destination, id: id}]
	if !ok {
		return zero, 0, ports.ErrNotFound
	}
	return p.table.copy(rec.snapshot.(S)), rec.version, nil
}

func (p txProjector[ID, S]) Save(ctx context.Context, destination ports.TenantID, id ID, state S, expected ports.Version) error {
	if err := p.authorize(ctx, destination); err != nil {
		return err
	}
	return p.table.saveAs(p.tx, rowKey{tenant: destination, id: id}, state, expected)
}

func (p txProjector[ID, S]) authorize(ctx context.Context, destination ports.TenantID) error {
	if _, err := ports.RequireExecutionContext(ctx); err != nil {
		return err
	}
	if destination == "" {
		_ = p.trail.Record(ctx, trailEvent(ctx, ports.AuditProjectionWrite, p.table.Name, "", ports.OutcomeDenied))
		return fmt.Errorf("memory: %s: %w", p.table.Name, ports.ErrProjectionDestinationAbsent)
	}
	if err := p.permitted(ctx, destination); err != nil {
		_ = p.trail.Record(ctx, trailEvent(ctx, ports.AuditProjectionWrite, p.table.Name, destination, ports.OutcomeDenied))
		return err
	}
	return p.trail.Record(ctx, trailEvent(ctx, ports.AuditProjectionWrite, p.table.Name, destination, ports.OutcomeAccepted))
}

func (p txProjector[ID, S]) permitted(ctx context.Context, destination ports.TenantID) error {
	origin := contextTenant(ctx)
	if origin != nil && *origin == destination {
		return fmt.Errorf("memory: %s into the origin's own tenant %q: %w", p.table.Name, destination, ports.ErrProjectionDenied)
	}
	if p.policy == nil {
		return fmt.Errorf("memory: %s into %q without a policy: %w", p.table.Name, destination, ports.ErrProjectionDenied)
	}
	if err := p.policy(ctx, origin, destination); err != nil {
		return fmt.Errorf("memory: %s into %q: %w: %w", p.table.Name, destination, ports.ErrProjectionDenied, err)
	}
	return nil
}

// Absence reaches the policy and the trail as absence, never as a stand-in
// value (CTX-26, IDN-20).
func contextTenant(ctx context.Context) *ports.TenantID {
	execution, ok := ports.ExecutionContextFrom(ctx)
	if !ok {
		return nil
	}
	tenant, ok := execution.Tenant()
	if !ok {
		return nil
	}
	return &tenant
}

func trailEvent(ctx context.Context, action, object string, data ports.TenantID, outcome ports.OutcomeCategory) ports.AuditRecord {
	var tenant string
	if origin := contextTenant(ctx); origin != nil {
		tenant = string(*origin)
	}
	return ports.AuditRecord{
		Subject:    ports.SubjectOf(ctx),
		Object:     object,
		Action:     action,
		Outcome:    string(outcome),
		Tenant:     tenant,
		DataTenant: string(data),
	}
}
