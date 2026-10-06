package tb

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
)

func RequireRejected[R comparable](t testing.TB, acc domain.Accepted[R], rej *domain.Rejection, code domain.Code) {
	t.Helper()
	if rej == nil {
		t.Fatalf("expected Rejected, got Accepted")
		return
	}
	if rej.Code() != code {
		t.Fatalf("Code() = %q, want %q", rej.Code(), code)
		return
	}
	var zero R
	if acc.Response() != zero {
		t.Fatalf("Rejected must carry the zero response, got %v", acc.Response())
		return
	}
	if n := len(acc.Events()); n != 0 {
		t.Fatalf("Rejected must carry no events, got %d", n)
	}
}

func RequireAccepted(t testing.TB, rej *domain.Rejection) {
	t.Helper()
	if rej != nil {
		t.Fatalf("expected Accepted, got Rejected %v", rej)
	}
}

func RequireSameEvents(t testing.TB, a, b []domain.DomainEvent) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("len(a)=%d, len(b)=%d", len(a), len(b))
		return
	}
	for i := range a {
		if a[i].EventName() != b[i].EventName() {
			t.Fatalf("event[%d] name: %q vs %q", i, a[i].EventName(), b[i].EventName())
			return
		}
		if a[i] != b[i] {
			t.Fatalf("event[%d] value: %+v vs %+v", i, a[i], b[i])
			return
		}
	}
}

// RunProjection runs each case of the fixture as a subtest under its name and
// returns the merged verdict.
func RunProjection(t *testing.T, f ProjectionFixture, run func(*testing.T, ProjectionCase) (domainkit.Projection, domainkit.Verdict)) domainkit.Verdict {
	t.Helper()
	var decided domainkit.Verdict
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got, twice := run(t, c)
			equal := domainkit.Equal(got, c.Expected.Projection())
			Require(t, equal)
			Require(t, twice)
			decided = decided.Merge(equal, twice)
		})
	}
	return decided
}
