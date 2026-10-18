package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

var duplicateName = domain.Reject("probes/duplicate-name", "the name is taken")

func declaring(name string, uniques ...postgres.Unique) postgres.Table[string, string] {
	return postgres.Table[string, string]{
		Name: name, IDColumn: "probe_id", Columns: []string{"name", "code"},
		Encode: func(string) ([]any, error) { return nil, nil },
		Decode: func(func(...any) error) (string, error) { return "", nil },
		Unique: uniques,
	}
}

func onName(name string) postgres.Unique {
	return postgres.Unique{Name: name, Columns: []string{"tenant_id", "name"}, Rejection: duplicateName}
}

func TestATableCompilesItsUniqueDeclaration(t *testing.T) {
	for name, table := range map[string]postgres.Table[string, string]{
		"lower-case table":           declaring("probes", onName("probes_tenant_id_name_key")),
		"table folded by the server": declaring("Probes", onName("probes_tenant_id_name_key")),
		"two constraints": declaring("probes", onName("probes_tenant_id_name_key"), postgres.Unique{
			Name: "probes_tenant_id_name_code_key", Columns: []string{"tenant_id", "name", "code"},
			Rejection: domain.Reject("probes/duplicate-pair", "the pair is taken"),
		}),
	} {
		t.Run(name, func(t *testing.T) {
			table.Reader(postgres.ReadPool{})
		})
	}
}

func TestTableRefusesAMalformedUniqueDeclaration(t *testing.T) {
	longColumn := strings.Repeat("c", 50)
	tests := map[string]postgres.Unique{
		"upper-case name":          {Name: "probes_tenant_id_Name_key", Columns: []string{"tenant_id", "Name"}, Rejection: duplicateName},
		"upper-case column":        {Name: "probes_tenant_id_name_key", Columns: []string{"tenant_id", "NAME"}, Rejection: duplicateName},
		"column not an identifier": {Name: "probes_tenant_id_name;_key", Columns: []string{"tenant_id", "name;"}, Rejection: duplicateName},
		"name off the pattern":     {Name: "probes_name_unique", Columns: []string{"tenant_id", "name"}, Rejection: duplicateName},
		"name over 63 bytes":       {Name: "probes_tenant_id_" + longColumn + "_key", Columns: []string{"tenant_id", longColumn}, Rejection: duplicateName},
		"a single column":          {Name: "probes_tenant_id_key", Columns: []string{"tenant_id"}, Rejection: duplicateName},
		"tenant not leading":       {Name: "probes_name_tenant_id_key", Columns: []string{"name", "tenant_id"}, Rejection: duplicateName},
		"no rejection":             {Name: "probes_tenant_id_name_key", Columns: []string{"tenant_id", "name"}},
		"invalid code":             {Name: "probes_tenant_id_name_key", Columns: []string{"tenant_id", "name"}, Rejection: domain.Reject("Duplicate Name", "taken")},
	}

	for name, unique := range tests {
		t.Run(name, func(t *testing.T) {
			requireRefused(t, declaring("probes", unique))
		})
	}
	t.Run("repeated name", func(t *testing.T) {
		requireRefused(t, declaring("probes", onName("probes_tenant_id_name_key"), onName("probes_tenant_id_name_key")))
	})
}

func requireRefused(t *testing.T, table postgres.Table[string, string]) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("a malformed Unique compiled; a constraint name the server would not report back has to fail before any statement")
		}
	}()
	table.Reader(postgres.ReadPool{})
}

func TestUniquesCarriesTheTableAndADetachedCopyOfTheDeclaration(t *testing.T) {
	table := declaring("Probes", onName("probes_tenant_id_name_key"))

	set := table.Uniques()
	if set.Table != "Probes" || len(set.Constraints) != 1 || set.Constraints[0].Name != "probes_tenant_id_name_key" {
		t.Fatalf("Uniques() = %+v, want the table name and its one constraint", set)
	}
	set.Constraints[0].Name = "changed"
	if table.Unique[0].Name != "probes_tenant_id_name_key" {
		t.Fatal("Uniques() shares its slice with the Table; a caller could rewrite the declaration")
	}
}

func TestUniquesRefusesAMalformedDeclaration(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Uniques() of a malformed declaration returned; startup would verify a set the Save cannot translate")
		}
	}()
	declaring("probes", onName("probes_name_key")).Uniques()
}

func TestVerifyUniquesWithNothingDeclaredMakesNoRoundTrip(t *testing.T) {
	if err := postgres.VerifyUniques(context.Background(), nil); err != nil {
		t.Fatalf("VerifyUniques() with no set = %v, want nil", err)
	}
	if err := postgres.VerifyUniques(context.Background(), nil, declaring("probes").Uniques()); err != nil {
		t.Fatalf("VerifyUniques() of a table without Unique = %v, want nil without touching the pool", err)
	}
}

func TestVerifyUniquesRefusesAHandBuiltSetThatWouldNotCompile(t *testing.T) {
	set := postgres.UniqueSet{Table: "probes", Constraints: []postgres.Unique{onName("probes_name_key")}}
	if err := postgres.VerifyUniques(context.Background(), nil, set); err == nil {
		t.Fatal("VerifyUniques() of a malformed set = nil, want the declaration refused before the catalog is read")
	}
}
