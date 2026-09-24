package domainkit_test

import (
	"strconv"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// The positive vectors of ORA-39, one per branch of each UPR, driven by a
// projection fixture in the ORA-30 format contracts/ keeps for every context:
// the kit is proved against the file a fixture author writes, not Go literals.

func TestTheCounterMatchesTheProjectionFixture(t *testing.T) {
	f := tb.LoadProjection(t, "libs/backend/go/testkit/domainkit/testdata/counter.golden")
	if f.Identity.Aggregate != "counter" || len(f.Cases) != 5 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	var decided domainkit.Verdict
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got, twice := runCounter(t, c)
			equal := domainkit.Equal(got, c.Expected.Projection())
			tb.Require(t, equal)
			tb.Require(t, twice)
			collect(&decided, equal, twice)
		})
	}
	evidence.RecordVerdict(t, "domain", "counter", decided)
}

func collect(into *domainkit.Verdict, verdicts ...domainkit.Verdict) {
	for _, v := range verdicts {
		into.Diagnostics = append(into.Diagnostics, v.Diagnostics...)
	}
}

func runCounter(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	at := instant(t, c.Command["at"])
	switch c.Command["upr"] {
	case "bump":
		by, _ := strconv.Atoi(c.Command["by"])
		s := bumpSubject(bump{By: by, At: at})
		return domainkit.Run(counterFromState(c.StateBefore), s), domainkit.ReadTwice(counterFromState(c.StateBefore), s)
	case "reset":
		s := resetSubject(reset{At: at})
		return domainkit.Run(counterFromState(c.StateBefore), s), domainkit.ReadTwice(counterFromState(c.StateBefore), s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func instant(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatalf("at %q: %v", s, err)
	}
	return n
}
