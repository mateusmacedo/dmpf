package golden

import (
	"cmp"
	"encoding/json"
	"slices"
)

// Report is the evidence of a fixture's round trip, one Outcome per fixture ×
// case × direction × oracle, serialized in a stable order (FIX-13) so two runs
// over the same fixture produce the same bytes.
type Report struct {
	Outcomes []Outcome `json:"outcomes"`
}

func (r *Report) Add(outcomes ...Outcome) { r.Outcomes = append(r.Outcomes, outcomes...) }

func (r Report) Failed() []Outcome {
	var out []Outcome
	for _, o := range r.sorted() {
		if !o.OK {
			out = append(out, o)
		}
	}
	return out
}

func (r Report) OK() bool { return len(r.Failed()) == 0 }

func (r Report) MarshalJSON() ([]byte, error) {
	type alias struct {
		Outcomes []Outcome `json:"outcomes"`
	}
	return json.Marshal(alias{Outcomes: r.sorted()})
}

func (r Report) sorted() []Outcome {
	out := slices.Clone(r.Outcomes)
	slices.SortStableFunc(out, func(a, b Outcome) int {
		return cmp.Or(
			cmp.Compare(a.Fixture, b.Fixture),
			cmp.Compare(a.Case, b.Case),
			cmp.Compare(a.Direction, b.Direction),
			cmp.Compare(a.Oracle, b.Oracle),
		)
	})
	return out
}
