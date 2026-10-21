package providerkit

import (
	"maps"
	"slices"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ExternalCommandSubject is an adapter of ports.ExternalCommand reduced to its
// partner table: Classify yields the class of a reply carrying status, Validate
// is the adapter's own check of a table; nil skips the refusal clause.
type ExternalCommandSubject struct {
	Table    map[int]ports.ExternalClass
	Classify func(status int) ports.ExternalClass
	Validate func(table map[int]ports.ExternalClass) error
}

const kitStatusSpan = 1000

// ExternalCommand certifies the partner table of an adapter (KIT-04): each
// declared status lands on the declared class, any other on ClassUnknownAmbiguous,
// and a table mapping a status to no valid class is refused.
func ExternalCommand(newSubject func() ExternalCommandSubject) Verdict {
	var v Verdict
	if newSubject().Classify == nil {
		v.fail("setup", "KIT-04", "the subject declares no Classify; the adapter's classification is what is certified")
		return v
	}

	{
		const clause = "the declared table maps every status to a valid class"
		s := newSubject()
		if len(s.Table) == 0 {
			v.fail(clause, "KIT-04", "the subject declares no status; the partner table is the contract")
		}
		for _, status := range slices.Sorted(maps.Keys(s.Table)) {
			if class := s.Table[status]; !validExternalClass(class) {
				v.fail(clause, "KIT-04", "status %d maps to class %d, which is no class", status, class)
			}
		}
	}

	{
		const clause = "each declared status lands on the declared class"
		s := newSubject()
		for _, status := range slices.Sorted(maps.Keys(s.Table)) {
			if got, want := s.Classify(status), s.Table[status]; got != want {
				v.fail(clause, "KIT-04", "Classify(%d) = %v, want %v", status, got, want)
			}
		}
	}

	{
		const clause = "an undeclared status lands on unknown_ambiguous"
		s := newSubject()
		for status := range kitStatusSpan {
			if _, declared := s.Table[status]; declared {
				continue
			}
			if got := s.Classify(status); got != ports.ClassUnknownAmbiguous {
				v.fail(clause, "KIT-04", "Classify(%d) = %v, want %v — a status the partner never declared is no evidence", status, got, ports.ClassUnknownAmbiguous)
			}
		}
	}

	{
		const clause = "the adapter refuses a table mapping a status to no class"
		s := newSubject()
		if s.Validate == nil {
			v.skip(clause)
		} else {
			if err := s.Validate(maps.Clone(s.Table)); err != nil {
				v.fail(clause, "KIT-04", "Validate(declared table) = %v, want nil", err)
			}
			for _, invalid := range []ports.ExternalClass{0, ports.ClassSchemaMismatch + 1} {
				table := maps.Clone(s.Table)
				if table == nil {
					table = map[int]ports.ExternalClass{}
				}
				table[kitInvalidStatus(table)] = invalid
				if err := s.Validate(table); err == nil {
					v.fail(clause, "KIT-04", "Validate accepted a table mapping a status to class %d", invalid)
				}
			}
		}
	}

	return v
}

func validExternalClass(c ports.ExternalClass) bool {
	return c >= ports.ClassAccepted && c <= ports.ClassSchemaMismatch
}

func kitInvalidStatus(table map[int]ports.ExternalClass) int {
	for status := 200; status < 600; status++ {
		if _, declared := table[status]; !declared {
			return status
		}
	}
	return 599
}
