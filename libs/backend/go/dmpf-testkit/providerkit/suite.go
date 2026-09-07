package providerkit

import "fmt"

// Diagnostic names the clause a realization failed, the rule it belongs to and
// what was observed. Rule is the FND-04 identifier (UOW-01, INB-06, OBX-11 …).
type Diagnostic struct {
	Clause string
	Rule   string
	Detail string
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("%s [%s]: %s", d.Clause, d.Rule, d.Detail)
}

// Verdict is decidable: no diagnostic is a pass. Skipped lists the clauses the
// candidate could not exercise (a realization that cannot inject a commit
// failure), out loud rather than quietly absent from the contract.
type Verdict struct {
	Diagnostics []Diagnostic
	Skipped     []string
}

func (v Verdict) OK() bool { return len(v.Diagnostics) == 0 }

func (v Verdict) Failures() []string {
	out := make([]string, 0, len(v.Diagnostics))
	for _, d := range v.Diagnostics {
		out = append(out, d.String())
	}
	return out
}

func (v *Verdict) fail(clause, rule, format string, args ...any) {
	v.Diagnostics = append(v.Diagnostics, Diagnostic{Clause: clause, Rule: rule, Detail: fmt.Sprintf(format, args...)})
}

func (v *Verdict) skip(clause string) { v.Skipped = append(v.Skipped, clause) }
