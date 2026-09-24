//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

type probe struct {
	Items int `json:"items"`
}

var probeTable = postgres.Table[string, probe]{
	Name:     "dmpf_example_orders",
	IDColumn: "order_id",
	Columns:  []string{"snapshot"},
	Encode: func(p probe) ([]any, error) {
		raw, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}
		return []any{raw}, nil
	},
	Decode: func(scan func(dest ...any) error) (probe, error) {
		var raw []byte
		if err := scan(&raw); err != nil {
			return probe{}, err
		}
		var p probe
		if err := json.Unmarshal(raw, &p); err != nil {
			return probe{}, err
		}
		return p, nil
	},
}

// scopedTo is what the edge would have deposited for a caller of that tenant.
func scopedTo(t *testing.T, tenant string) context.Context {
	t.Helper()

	subject, scope := ports.SubjectID("s-1"), ports.TenantID(tenant)
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID:     "r-1",
		CorrelationID: "c-1",
		TraceContext:  "t-1",
		Subject:       &subject,
		Tenant:        &scope,
		Permissions:   []ports.Permission{},
		Deadline:      ports.Instant(1_755_432_000_000_000_000),
		Locale:        "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v, want nil", err)
	}
	return ports.WithExecutionContext(context.Background(), execution)
}

// tenantless is a context whose caller authenticated but resolved no tenant,
// which IDN-15 answers with a refusal rather than an unscoped query.
func tenantless(t *testing.T) context.Context {
	t.Helper()

	subject := ports.SubjectID("s-1")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID:     "r-1",
		CorrelationID: "c-1",
		TraceContext:  "t-1",
		Subject:       &subject,
		Permissions:   []ports.Permission{},
		Deadline:      ports.Instant(1_755_432_000_000_000_000),
		Locale:        "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v, want nil", err)
	}
	return ports.WithExecutionContext(context.Background(), execution)
}

func saveProbe(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string, state probe, expected ports.Version) error {
	t.Helper()

	uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) ports.Repository[string, probe] {
		return probeTable.Repository(tx)
	})
	return uow.Within(ctx, func(ctx context.Context, repo ports.Repository[string, probe]) error {
		return repo.Save(ctx, id, state, expected)
	})
}

func TestTableRoundTripsWithinTheTenant(t *testing.T) {
	pool := openPool(t)
	ctx := scopedTo(t, "acme")

	if err := saveProbe(t, ctx, pool, "P-1", probe{Items: 3}, 0); err != nil {
		t.Fatalf("Save() = %v, want nil", err)
	}

	state, version, err := probeTable.Reader(postgres.NewReadPool(pool)).Load(ctx, "P-1")
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if state.Items != 3 || version != 1 {
		t.Fatalf("Load() = %+v, v%d; want {Items:3}, v1", state, version)
	}
}

// This is the central acceptance vector of the spec: the read that omits the
// scope does not exist, because the scope is not the caller's to omit.
func TestTableNeverReturnsAnotherTenantsRow(t *testing.T) {
	pool := openPool(t)

	if err := saveProbe(t, scopedTo(t, "acme"), pool, "P-1", probe{Items: 3}, 0); err != nil {
		t.Fatalf("Save() = %v, want nil", err)
	}

	_, _, err := probeTable.Reader(postgres.NewReadPool(pool)).Load(scopedTo(t, "globex"), "P-1")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() from another tenant = %v, want ErrNotFound (IDN-12, IDN-13)", err)
	}
}

func TestTableKeepsTheSameIdentifierApartPerTenant(t *testing.T) {
	pool := openPool(t)

	if err := saveProbe(t, scopedTo(t, "acme"), pool, "P-1", probe{Items: 3}, 0); err != nil {
		t.Fatalf("Save() for acme = %v, want nil", err)
	}
	if err := saveProbe(t, scopedTo(t, "globex"), pool, "P-1", probe{Items: 7}, 0); err != nil {
		t.Fatalf("Save() for globex = %v, want nil: the composite key admits both", err)
	}

	acme, _, err := probeTable.Reader(postgres.NewReadPool(pool)).Load(scopedTo(t, "acme"), "P-1")
	if err != nil || acme.Items != 3 {
		t.Fatalf("acme Load() = %+v, %v; want {Items:3}, nil", acme, err)
	}
	globex, _, err := probeTable.Reader(postgres.NewReadPool(pool)).Load(scopedTo(t, "globex"), "P-1")
	if err != nil || globex.Items != 7 {
		t.Fatalf("globex Load() = %+v, %v; want {Items:7}, nil", globex, err)
	}
}

func TestTableRefusesAWriteOverAnotherTenantsRow(t *testing.T) {
	pool := openPool(t)

	if err := saveProbe(t, scopedTo(t, "acme"), pool, "P-1", probe{Items: 3}, 0); err != nil {
		t.Fatalf("Save() = %v, want nil", err)
	}

	// Version 1 is what acme's row holds; globex must not reach it even so.
	err := saveProbe(t, scopedTo(t, "globex"), pool, "P-1", probe{Items: 99}, 1)
	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Save() over another tenant = %v, want ErrVersionConflict", err)
	}

	acme, version, err := probeTable.Reader(postgres.NewReadPool(pool)).Load(scopedTo(t, "acme"), "P-1")
	if err != nil || acme.Items != 3 || version != 1 {
		t.Fatalf("acme row = %+v, v%d, %v; want {Items:3}, v1, nil", acme, version, err)
	}
}

func TestTableRefusesWhenTheCarrierResolvedNoTenant(t *testing.T) {
	pool := openPool(t)

	_, _, err := probeTable.Reader(postgres.NewReadPool(pool)).Load(tenantless(t), "P-1")
	if !errors.Is(err, postgres.ErrTenantUnresolved) {
		t.Fatalf("Load() without a tenant = %v, want ErrTenantUnresolved (IDN-15)", err)
	}

	if err := saveProbe(t, tenantless(t), pool, "P-1", probe{Items: 1}, 0); !errors.Is(err, postgres.ErrTenantUnresolved) {
		t.Fatalf("Save() without a tenant = %v, want ErrTenantUnresolved (IDN-15)", err)
	}
}

func TestTableRefusesWhenTheCarrierHoldsNoContext(t *testing.T) {
	pool := openPool(t)

	_, _, err := probeTable.Reader(postgres.NewReadPool(pool)).Load(context.Background(), "P-1")
	if !errors.Is(err, ports.ErrContextAbsent) {
		t.Fatalf("Load() off a bare context = %v, want ErrContextAbsent", err)
	}
}

func TestTableHonoursTheStoredVersion(t *testing.T) {
	pool := openPool(t)
	ctx := scopedTo(t, "acme")

	if err := saveProbe(t, ctx, pool, "P-1", probe{Items: 1}, 0); err != nil {
		t.Fatalf("Save() = %v, want nil", err)
	}
	if err := saveProbe(t, ctx, pool, "P-1", probe{Items: 2}, 1); err != nil {
		t.Fatalf("Save() at v1 = %v, want nil", err)
	}
	if err := saveProbe(t, ctx, pool, "P-1", probe{Items: 3}, 1); !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Save() at a stale version = %v, want ErrVersionConflict", err)
	}
}

func TestTableRefusesAMalformedDeclaration(t *testing.T) {
	tests := []struct {
		name  string
		table postgres.Table[string, probe]
	}{
		{
			name:  "name that is not an identifier",
			table: postgres.Table[string, probe]{Name: "orders; DROP TABLE x", IDColumn: "order_id", Columns: []string{"snapshot"}, Encode: probeTable.Encode, Decode: probeTable.Decode},
		},
		{
			name:  "column the table owns",
			table: postgres.Table[string, probe]{Name: "dmpf_example_orders", IDColumn: "order_id", Columns: []string{"tenant_id"}, Encode: probeTable.Encode, Decode: probeTable.Decode},
		},
		{
			name:  "no state column",
			table: postgres.Table[string, probe]{Name: "dmpf_example_orders", IDColumn: "order_id", Encode: probeTable.Encode, Decode: probeTable.Decode},
		},
		{
			name:  "no codec",
			table: postgres.Table[string, probe]{Name: "dmpf_example_orders", IDColumn: "order_id", Columns: []string{"snapshot"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("a malformed Table compiled; the declaration has to fail before any statement reaches the server")
				}
			}()
			tt.table.Reader(postgres.ReadPool{})
		})
	}
}

// IDN-12 on the relation: a filter value held only by another tenant reports
// the access, and one held by nobody stays an empty answer (IDN-13 lets the
// caller make both look alike; the internal record tells them apart).
func TestRelationReportsAValueHeldOnlyByAnotherTenant(t *testing.T) {
	pool := openPool(t)
	if err := saveProbe(t, scopedTo(t, "acme"), pool, "P-9", probe{Items: 1}, 0); err != nil {
		t.Fatalf("Save() = %v", err)
	}
	relation := probeTable.Relation("order_id")
	read := postgres.NewReadPool(pool)

	if rows, err := relation.Query(scopedTo(t, "acme"), read, "P-9"); err != nil || len(rows) != 1 {
		t.Fatalf("Query() in the owning tenant = %v, %v; want the row", rows, err)
	}
	_, err := relation.Query(scopedTo(t, "globex"), read, "P-9")
	var access ports.CrossTenantAccess
	if !errors.As(err, &access) || access.ContextTenant != "globex" || access.DataTenant != "acme" {
		t.Fatalf("Query() from another tenant = %v, want CrossTenantAccess globex over acme", err)
	}
	if rows, err := relation.Query(scopedTo(t, "globex"), read, "P-nobody"); err != nil || len(rows) != 0 {
		t.Fatalf("Query() of a value nobody holds = %v, %v; want an empty answer", rows, err)
	}
}
