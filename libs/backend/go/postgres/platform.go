package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type PlatformOption func(*platformOptions)

type platformOptions struct{ pageMax int }

func WithPageMax(n int) PlatformOption {
	if n <= 0 {
		panic(fmt.Sprintf("postgres: WithPageMax(%d) needs a positive cap", n))
	}
	return func(o *platformOptions) { o.pageMax = n }
}

// Every tenant reached is recorded before the rows leave (DAT-25).
func (t Table[ID, S]) PlatformReader(pool ReadPool, trail ports.AuditTrail, opts ...PlatformOption) ports.PlatformReader[ID, S] {
	if trail == nil {
		panic(fmt.Sprintf("postgres: Table %q PlatformReader needs an audit trail", t.Name))
	}
	options := platformOptions{pageMax: ports.PlatformPageMax}
	for _, opt := range opts {
		opt(&options)
	}
	return platformReader[ID, S]{
		table:      t,
		statements: t.compile(),
		platform:   t.compilePlatform(),
		pool:       pool.pool,
		trail:      trail,
		pageMax:    options.pageMax,
	}
}

// Within tx, so the projection commits with the inbox and the outbox (UOW-01); a
// nil policy refuses every call, because the default is closed (IDN-17).
func (t Table[ID, S]) Projector(tx *Tx, policy ports.ProjectionPolicy, trail ports.AuditTrail) ports.Projector[ID, S] {
	if trail == nil {
		panic(fmt.Sprintf("postgres: Table %q Projector needs an audit trail", t.Name))
	}
	return projector[ID, S]{
		repository: tableRepository[ID, S]{table: t, statements: t.compile(), tx: tx},
		policy:     policy,
		trail:      trail,
	}
}

type platformStatements struct {
	first string
	after string
}

// compilePlatform stays apart from compile so a Table that never reads across
// tenants declares no statement without the tenant predicate.
func (t Table[ID, S]) compilePlatform() platformStatements {
	t.validate()

	head := fmt.Sprintf("SELECT tenant_id, %s, version, %s FROM %s", t.IDColumn, strings.Join(t.Columns, ", "), t.Name)
	order := fmt.Sprintf("ORDER BY tenant_id, %s", t.IDColumn)
	return platformStatements{
		first: declare(fmt.Sprintf("%s %s LIMIT $1", head, order), "SELECT", t.Name),
		after: declare(fmt.Sprintf("%s WHERE (tenant_id, %s) > ($1, $2) %s LIMIT $3", head, t.IDColumn, order), "SELECT", t.Name),
	}
}

type platformReader[ID ~string, S any] struct {
	table      Table[ID, S]
	statements statements
	platform   platformStatements
	pool       *pgxpool.Pool
	trail      ports.AuditTrail
	pageMax    int
}

func (r platformReader[ID, S]) LoadIn(ctx context.Context, tenant ports.TenantID, id ID) (ports.PlatformRow[ID, S], error) {
	var zero ports.PlatformRow[ID, S]
	if err := r.admit(ctx, tenant); err != nil {
		return zero, err
	}
	if tenant == "" {
		return zero, ports.ErrNotFound
	}
	if err := r.trail.Record(ctx, crossTenantEvent(ctx, ports.AuditPlatformRead, r.table.Name, tenant, string(ports.OutcomeAccepted))); err != nil {
		return zero, err
	}

	state, version, found, err := r.table.loadAs(ctx, r.pool, r.statements, tenant, id)
	switch {
	case err != nil:
		return zero, err
	case !found:
		return zero, ports.ErrNotFound
	}
	return ports.PlatformRow[ID, S]{Tenant: tenant, ID: id, State: state, Version: version}, nil
}

func (r platformReader[ID, S]) List(ctx context.Context, after *ports.PlatformKey[ID], limit int) ([]ports.PlatformRow[ID, S], error) {
	if err := r.admit(ctx, ""); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > r.pageMax {
		return nil, fmt.Errorf("postgres: list %s with limit %d, cap %d: %w", r.table.Name, limit, r.pageMax, ports.ErrPlatformLimitInvalid)
	}

	page, err := r.query(ctx, after, limit)
	if err != nil {
		return nil, err
	}
	for i, row := range page {
		if i > 0 && page[i-1].Tenant == row.Tenant {
			continue
		}
		if err := r.trail.Record(ctx, crossTenantEvent(ctx, ports.AuditPlatformRead, r.table.Name, row.Tenant, string(ports.OutcomeAccepted))); err != nil {
			return nil, err
		}
	}
	return page, nil
}

func (r platformReader[ID, S]) admit(ctx context.Context, asked ports.TenantID) error {
	if reach, _ := ports.PlatformReachFrom(ctx); reach != ports.ReachAllTenants {
		_ = r.trail.Record(ctx, crossTenantEvent(ctx, ports.AuditPlatformRead, r.table.Name, asked, string(ports.OutcomeDenied)))
		return fmt.Errorf("postgres: read %s across tenants: %w", r.table.Name, ports.ErrPlatformReachUndeclared)
	}
	_, err := ports.RequireExecutionContext(ctx)
	return err
}

func (r platformReader[ID, S]) query(ctx context.Context, after *ports.PlatformKey[ID], limit int) ([]ports.PlatformRow[ID, S], error) {
	statement, args := r.platform.first, []any{limit}
	if after != nil {
		statement, args = r.platform.after, []any{string(after.Tenant), string(after.ID), limit}
	}

	rows, err := r.pool.Query(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: list %s: %w", r.table.Name, err)
	}
	defer rows.Close()

	page := []ports.PlatformRow[ID, S]{}
	for rows.Next() {
		var (
			tenant  string
			id      ID
			version int64
		)
		state, err := r.table.Decode(func(dest ...any) error {
			return rows.Scan(append([]any{&tenant, &id, &version}, dest...)...)
		})
		if err != nil {
			return nil, fmt.Errorf("postgres: scan %s: %w", r.table.Name, err)
		}
		page = append(page, ports.PlatformRow[ID, S]{
			Tenant: ports.TenantID(tenant), ID: id, State: r.table.identify(state, id), Version: ports.Version(version),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: rows of %s: %w", r.table.Name, err)
	}
	return page, nil
}

type projector[ID ~string, S any] struct {
	repository tableRepository[ID, S]
	policy     ports.ProjectionPolicy
	trail      ports.AuditTrail
}

// Load answers absence with ports.ErrNotFound alone: the owner probe would
// record the destination as the reader, and the reader is the carrier (IDN-12).
func (p projector[ID, S]) Load(ctx context.Context, destination ports.TenantID, id ID) (S, ports.Version, error) {
	var zero S
	if err := p.authorize(ctx, destination); err != nil {
		return zero, 0, err
	}
	state, version, found, err := p.repository.table.loadAs(ctx, p.repository.tx.conn, p.repository.statements, destination, id)
	if err == nil && !found {
		return zero, 0, ports.ErrNotFound
	}
	return state, version, err
}

func (p projector[ID, S]) Save(ctx context.Context, destination ports.TenantID, id ID, state S, expected ports.Version) error {
	if err := p.authorize(ctx, destination); err != nil {
		return err
	}
	return p.repository.saveAs(ctx, destination, id, state, expected)
}

func (p projector[ID, S]) authorize(ctx context.Context, destination ports.TenantID) error {
	if _, err := ports.RequireExecutionContext(ctx); err != nil {
		return err
	}
	name := p.repository.table.Name
	if destination == "" {
		_ = p.trail.Record(ctx, crossTenantEvent(ctx, ports.AuditProjectionWrite, name, "", string(ports.OutcomeDenied)))
		return fmt.Errorf("postgres: project into %s: %w", name, ports.ErrProjectionDestinationAbsent)
	}

	origin := originOf(ctx)
	refusal := ports.ErrProjectionDenied
	switch {
	case origin != nil && *origin == destination:
		refusal = fmt.Errorf("%w: the destination is the origin's own tenant", ports.ErrProjectionDenied)
	case p.policy != nil:
		refusal = nil
		if cause := p.policy(ctx, origin, destination); cause != nil {
			refusal = fmt.Errorf("%w: %w", ports.ErrProjectionDenied, cause)
		}
	}
	if refusal != nil {
		_ = p.trail.Record(ctx, crossTenantEvent(ctx, ports.AuditProjectionWrite, name, destination, string(ports.OutcomeDenied)))
		return fmt.Errorf("postgres: project into %s of %s: %w", name, destination, refusal)
	}
	return p.trail.Record(ctx, crossTenantEvent(ctx, ports.AuditProjectionWrite, name, destination, string(ports.OutcomeAccepted)))
}

// originOf never invents a tenant for a carrier that resolved none (IDN-20).
func originOf(ctx context.Context) *ports.TenantID {
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

func crossTenantEvent(ctx context.Context, action, object string, reached ports.TenantID, outcome string) ports.AuditRecord {
	event := ports.AuditRecord{
		Subject:    ports.SubjectOf(ctx),
		Object:     object,
		Action:     action,
		Outcome:    outcome,
		DataTenant: string(reached),
	}
	if origin := originOf(ctx); origin != nil {
		event.Tenant = string(*origin)
	}
	return event
}
