//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func TestVerifyUniquesAcceptsTheFixtureAsDeclared(t *testing.T) {
	pool := openPool(t)

	if err := postgres.VerifyUniques(context.Background(), pool, uniqueTable.Uniques()); err != nil {
		t.Fatalf("VerifyUniques() of unique_probes = %v, want nil", err)
	}
}

const partitioned = "partitioned or inherited table"

func TestVerifyUniquesRefusesADivergentSchema(t *testing.T) {
	tests := []struct {
		name   string
		table  string
		ddl    string
		reason string
	}{
		{"an absent table", "verify_ghost", "", "absent"},
		{"an absent constraint", "verify_bare", "CREATE TABLE verify_bare (tenant_id text, name text)", "absent"},
		{"columns out of order", "verify_order",
			"CREATE TABLE verify_order (tenant_id text, name text, CONSTRAINT verify_order_tenant_id_name_key UNIQUE (name, tenant_id))", "columns"},
		{"tenant_id missing", "verify_lead",
			"CREATE TABLE verify_lead (tenant_id text, name text, code text, CONSTRAINT verify_lead_tenant_id_name_key UNIQUE (name, code))", "columns"},
		{"DEFERRABLE", "verify_deferred",
			"CREATE TABLE verify_deferred (tenant_id text, name text, CONSTRAINT verify_deferred_tenant_id_name_key UNIQUE (tenant_id, name) DEFERRABLE)", "deferrable"},
		{"NULLS NOT DISTINCT", "verify_nulls",
			"CREATE TABLE verify_nulls (tenant_id text, name text, CONSTRAINT verify_nulls_tenant_id_name_key UNIQUE NULLS NOT DISTINCT (tenant_id, name))", "nulls not distinct"},
		{"a CHECK under the name", "verify_check",
			"CREATE TABLE verify_check (tenant_id text, name text, CONSTRAINT verify_check_tenant_id_name_key CHECK (name <> ''))", "not a unique constraint"},
		{"a CREATE UNIQUE INDEX", "verify_index",
			"CREATE TABLE verify_index (tenant_id text, name text); CREATE UNIQUE INDEX verify_index_tenant_id_name_key ON verify_index (tenant_id, name)", "absent"},
		{"the name held by another table", "verify_home",
			"CREATE TABLE verify_home (tenant_id text, name text); CREATE TABLE verify_other (tenant_id text, name text, CONSTRAINT verify_home_tenant_id_name_key UNIQUE (tenant_id, name))", "absent"},
		{"a partitioned table", "verify_parted",
			"CREATE TABLE verify_parted (tenant_id text, name text, CONSTRAINT verify_parted_tenant_id_name_key UNIQUE (tenant_id, name)) PARTITION BY LIST (tenant_id); " +
				"CREATE TABLE verify_parted_acme PARTITION OF verify_parted FOR VALUES IN ('acme')", partitioned},
		{"a partitioned table without partitions", "verify_empty",
			"CREATE TABLE verify_empty (tenant_id text, name text, CONSTRAINT verify_empty_tenant_id_name_key UNIQUE (tenant_id, name)) PARTITION BY LIST (tenant_id)", partitioned},
		{"a parent by inheritance", "verify_parent",
			"CREATE TABLE verify_parent (tenant_id text, name text, CONSTRAINT verify_parent_tenant_id_name_key UNIQUE (tenant_id, name)); " +
				"CREATE TABLE verify_heir () INHERITS (verify_parent)", partitioned},
		{"a child by inheritance", "verify_child",
			"CREATE TABLE verify_base (tenant_id text, name text); " +
				"CREATE TABLE verify_child (CONSTRAINT verify_child_tenant_id_name_key UNIQUE (tenant_id, name)) INHERITS (verify_base)", partitioned},
		{"a partition", "verify_slice",
			"CREATE TABLE verify_whole (tenant_id text, name text) PARTITION BY LIST (tenant_id); " +
				"CREATE TABLE verify_slice PARTITION OF verify_whole (CONSTRAINT verify_slice_tenant_id_name_key UNIQUE (tenant_id, name)) FOR VALUES IN ('acme')", partitioned},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := openPool(t)
			if tt.ddl != "" {
				mustExec(t, openAdminPool(t), tt.ddl)
			}
			set := postgres.UniqueSet{Table: tt.table, Constraints: []postgres.Unique{{
				Name: tt.table + "_tenant_id_name_key", Columns: []string{"tenant_id", "name"}, Rejection: nameTaken,
			}}}

			err := postgres.VerifyUniques(context.Background(), pool, uniqueTable.Uniques(), set)

			if !errors.Is(err, postgres.ErrUniqueDivergent) || !strings.Contains(err.Error(), set.Constraints[0].Name) || !strings.Contains(err.Error(), tt.reason) {
				t.Fatalf("VerifyUniques() = %v, want ErrUniqueDivergent naming %s as %s", err, set.Constraints[0].Name, tt.reason)
			}
		})
	}
}

func TestVerifyUniquesRefusesConstraintsDeclaredOutOfIndexOrder(t *testing.T) {
	pool := openPool(t)
	mustExec(t, openAdminPool(t), `CREATE TABLE verify_sequence (tenant_id text, name text, code text,
  CONSTRAINT verify_sequence_tenant_id_code_key UNIQUE (tenant_id, code),
  CONSTRAINT verify_sequence_tenant_id_name_key UNIQUE (tenant_id, name))`)
	name := postgres.Unique{Name: "verify_sequence_tenant_id_name_key", Columns: []string{"tenant_id", "name"}, Rejection: nameTaken}
	code := postgres.Unique{Name: "verify_sequence_tenant_id_code_key", Columns: []string{"tenant_id", "code"}, Rejection: codeTaken}

	if err := postgres.VerifyUniques(context.Background(), pool, postgres.UniqueSet{Table: "verify_sequence", Constraints: []postgres.Unique{code, name}}); err != nil {
		t.Fatalf("VerifyUniques() in DDL order = %v, want nil", err)
	}
	err := postgres.VerifyUniques(context.Background(), pool, uniqueTable.Uniques(), postgres.UniqueSet{Table: "verify_sequence", Constraints: []postgres.Unique{name, code}})
	if !errors.Is(err, postgres.ErrUniqueDivergent) || !strings.Contains(err.Error(), code.Name) || !strings.Contains(err.Error(), "order") {
		t.Fatalf("VerifyUniques() out of DDL order = %v, want ErrUniqueDivergent naming %s out of order", err, code.Name)
	}
}

func TestWaitForUniquesShowsWhyAPartitionedTableNeverConverges(t *testing.T) {
	pool := openPool(t)
	mustExec(t, openAdminPool(t), "CREATE TABLE late_parted (tenant_id text, name text, CONSTRAINT late_parted_tenant_id_name_key UNIQUE (tenant_id, name)) PARTITION BY LIST (tenant_id)")
	set := postgres.UniqueSet{Table: "late_parted", Constraints: []postgres.Unique{{
		Name: "late_parted_tenant_id_name_key", Columns: []string{"tenant_id", "name"}, Rejection: nameTaken,
	}}}
	if err := postgres.VerifyUniques(context.Background(), pool, set); !errors.Is(err, postgres.ErrUniqueDivergent) || !strings.Contains(err.Error(), partitioned) {
		t.Fatalf("VerifyUniques() = %v, want the divergence %q", err, partitioned)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	logs := &recordingProcessor{}

	err := postgres.WaitForUniques(ctx, pool, 20*time.Millisecond, logs.provider(), set)

	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, postgres.ErrUniqueDivergent) || !strings.Contains(err.Error(), partitioned) {
		t.Fatalf("WaitForUniques() = %v, want the deadline with the divergence %q", err, partitioned)
	}
	records := logs.named("waiting for unique constraint")
	if len(records) != 1 || records[0]["dmpf.db.constraint.divergence"] != partitioned {
		t.Fatalf("log = %v, want one record with the divergence %q", records, partitioned)
	}
}
