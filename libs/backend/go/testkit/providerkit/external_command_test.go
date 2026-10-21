package providerkit_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const (
	clauseTableValid       = "the declared table maps every status to a valid class"
	clauseDeclaredClass    = "each declared status lands on the declared class"
	clauseUndeclaredStatus = "an undeclared status lands on unknown_ambiguous"
	clauseRefusesTable     = "the adapter refuses a table mapping a status to no class"
)

var errInvalidTable = errors.New("external_command_test: invalid table")

func partnerTable() map[int]ports.ExternalClass {
	return map[int]ports.ExternalClass{
		201: ports.ClassAccepted,
		400: ports.ClassFinalValidation,
		403: ports.ClassFinalAuthorization,
		422: ports.ClassFinalBusiness,
		503: ports.ClassRetryableUpstream,
	}
}

func classifyBy(table map[int]ports.ExternalClass, unknown ports.ExternalClass) func(int) ports.ExternalClass {
	return func(status int) ports.ExternalClass {
		if class, declared := table[status]; declared {
			return class
		}
		return unknown
	}
}

func validateClasses(table map[int]ports.ExternalClass) error {
	for _, class := range table {
		if class < ports.ClassAccepted || class > ports.ClassSchemaMismatch {
			return errInvalidTable
		}
	}
	return nil
}

func conformingAdapter() providerkit.ExternalCommandSubject {
	table := partnerTable()
	return providerkit.ExternalCommandSubject{
		Table:    table,
		Classify: classifyBy(table, ports.ClassUnknownAmbiguous),
		Validate: validateClasses,
	}
}

func TestAConformingAdapterPassesEveryExternalCommandClause(t *testing.T) {
	v := providerkit.ExternalCommand(conformingAdapter)
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped = %v, want none", v.Skipped)
	}
}

func TestAnAdapterWithoutValidateSkipsTheRefusalClause(t *testing.T) {
	v := providerkit.ExternalCommand(func() providerkit.ExternalCommandSubject {
		s := conformingAdapter()
		s.Validate = nil
		return s
	})
	tb.Require(t, v)
	if want := []string{clauseRefusesTable}; !slices.Equal(v.Skipped, want) {
		t.Fatalf("skipped = %v, want %v", v.Skipped, want)
	}
}

func TestExternalCommandRejectsTheNegativeVectors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*providerkit.ExternalCommandSubject)
		clauses []string
	}{
		{
			name:    "an undeclared status taken as sent",
			mutate:  func(s *providerkit.ExternalCommandSubject) { s.Classify = classifyBy(s.Table, ports.ClassUnknownSent) },
			clauses: []string{clauseUndeclaredStatus},
		},
		{
			name: "a declared status classified against the table",
			mutate: func(s *providerkit.ExternalCommandSubject) {
				declared := s.Classify
				s.Classify = func(status int) ports.ExternalClass {
					if status == 503 {
						return ports.ClassRetryableTransport
					}
					return declared(status)
				}
			},
			clauses: []string{clauseDeclaredClass},
		},
		{
			name: "a table mapping a status to no class",
			mutate: func(s *providerkit.ExternalCommandSubject) {
				s.Table[500] = 0
				s.Validate = nil
			},
			clauses: []string{clauseTableValid},
		},
		{
			name: "an empty table",
			mutate: func(s *providerkit.ExternalCommandSubject) {
				s.Table = map[int]ports.ExternalClass{}
				s.Classify = classifyBy(s.Table, ports.ClassUnknownAmbiguous)
			},
			clauses: []string{clauseTableValid},
		},
		{
			name: "an adapter that accepts any table",
			mutate: func(s *providerkit.ExternalCommandSubject) {
				s.Validate = func(map[int]ports.ExternalClass) error { return nil }
			},
			clauses: []string{clauseRefusesTable},
		},
		{
			name: "an adapter that refuses its own table",
			mutate: func(s *providerkit.ExternalCommandSubject) {
				s.Validate = func(map[int]ports.ExternalClass) error { return errInvalidTable }
			},
			clauses: []string{clauseRefusesTable},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := providerkit.ExternalCommand(func() providerkit.ExternalCommandSubject {
				s := conformingAdapter()
				tt.mutate(&s)
				return s
			})
			if got := failedClauses(v); !slices.Equal(got, tt.clauses) {
				t.Fatalf("failed clauses = %v, want %v (%v)", got, tt.clauses, v.Failures())
			}
		})
	}
}

func TestAnAdapterWithoutClassifyIsRefusedUpFront(t *testing.T) {
	v := providerkit.ExternalCommand(func() providerkit.ExternalCommandSubject {
		return providerkit.ExternalCommandSubject{Table: partnerTable()}
	})
	if got := failedClauses(v); !slices.Equal(got, []string{"setup"}) {
		t.Fatalf("failed clauses = %v, want setup", got)
	}
}
