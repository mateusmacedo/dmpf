package providerkit

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// UniqueConstraint is one constraint the realization declares: Columns lists
// tenant_id and then the attributes, as postgres.Unique does, and Code is the
// code of its declared rejection.
type UniqueConstraint struct {
	Name    string
	Columns []string
	Code    domain.Code
}

// UniqueWrites is what one transaction offers the suite. Enqueue writes the nth
// outbox entry in that transaction; nil skips the clause that needs it.
type UniqueWrites[ID comparable, S any] struct {
	Repo    ports.Repository[ID, S]
	Enqueue func(ctx context.Context, n int) error
}

// UniqueSubject is RepositorySubject over a table that declares unique
// constraints. NewState gets, per constraint, one value per column, and a
// marker no other state of the run carries, so undeclared columns can derive from it.
type UniqueSubject[ID comparable, S any] struct {
	Within      func(ctx context.Context, fn func(ctx context.Context, w UniqueWrites[ID, S]) error) error
	Reader      ports.Reader[ID, S]
	NewID       func(n int) ID
	NewState    func(marker int, keys [][]string) S
	Marker      func(S) int
	Table       string
	Constraints []UniqueConstraint
	Concurrent  bool
}

// Unique exercises the declared unique constraints of ADR-066 over any
// realization; newSubject must return a subject over a fresh resource on every
// call. The violations a clause provokes are swallowed and committed, as Decide does.
func Unique[ID comparable, S any](newSubject func() UniqueSubject[ID, S]) Verdict {
	var v Verdict

	acme, err := scopedTo(kitTenantA)
	if err != nil {
		v.fail("mounts the execution context the edge would have mounted", "CTX-01", "NewExecutionContext() = %v", err)
		return v
	}
	globex, err := scopedTo(kitTenantB)
	if err != nil {
		v.fail("mounts the execution context the edge would have mounted", "CTX-01", "NewExecutionContext() = %v", err)
		return v
	}
	if err := newSubject().declared(); err != nil {
		v.fail("declares the constraints it is checked against", "UOW-06", "%v", err)
		return v
	}

	row := 0
	next := func() int { row++; return row }

	{
		const clause = "refuses a second row holding a declared value"
		s := newSubject()
		taken, second := next(), next()
		if s.create(&v, clause, acme, taken, s.keys(taken)) {
			keys := s.taking(second, 0, taken)
			var saved error
			if err := s.Within(acme, func(ctx context.Context, w UniqueWrites[ID, S]) error {
				saved = w.Repo.Save(ctx, s.NewID(second), s.NewState(second, keys), 0)
				return nil
			}); err != nil {
				v.fail(clause, "UOW-06", "Within() after swallowing the violation = %v, want nil", err)
			}
			s.expectViolation(&v, clause, "UOW-06", saved, s.Constraints[0], keys[0], true)
			s.expectAbsent(&v, clause, "DEC-10", acme, s.NewID(second))
			s.expectStored(&v, clause, "DEC-10", acme, s.NewID(taken), taken, 1)
		}
	}

	{
		const clause = "refuses an update onto a value another row holds"
		s := newSubject()
		taken, moved, marker := next(), next(), next()
		if s.create(&v, clause, acme, taken, s.keys(taken)) && s.create(&v, clause, acme, moved, s.keys(moved)) {
			keys := s.taking(marker, 0, taken)
			var saved error
			if err := s.Within(acme, func(ctx context.Context, w UniqueWrites[ID, S]) error {
				saved = w.Repo.Save(ctx, s.NewID(moved), s.NewState(marker, keys), 1)
				return nil
			}); err != nil {
				v.fail(clause, "UOW-06", "Within() after swallowing the violation = %v, want nil", err)
			}
			s.expectViolation(&v, clause, "UOW-06", saved, s.Constraints[0], keys[0], true)
			s.expectStored(&v, clause, "DEC-10", acme, s.NewID(moved), moved, 1)
		}
	}

	{
		const clause = "writes a taken value under another tenant"
		s := newSubject()
		taken, other := next(), next()
		if s.create(&v, clause, acme, taken, s.keys(taken)) {
			if err := s.save(globex, other, s.keys(taken)); err != nil {
				v.fail(clause, "IDN-11", "Save() of a value another tenant holds = %v, want nil — the constraint starts at tenant_id", err)
			} else {
				s.expectStored(&v, clause, "IDN-11", globex, s.NewID(other), other, 1)
			}
		}
	}

	{
		const clause = "keeps the transaction usable after the violation"
		s := newSubject()
		taken, refused, after := next(), next(), next()
		if s.create(&v, clause, acme, taken, s.keys(taken)) {
			keys := s.taking(refused, 0, taken)
			var saved, then error
			if err := s.Within(acme, func(ctx context.Context, w UniqueWrites[ID, S]) error {
				saved = w.Repo.Save(ctx, s.NewID(refused), s.NewState(refused, keys), 0)
				then = w.Repo.Save(ctx, s.NewID(after), s.NewState(after, s.keys(after)), 0)
				return nil
			}); err != nil {
				v.fail(clause, "UOW-06", "Within() after swallowing the violation = %v, want nil", err)
			}
			s.expectViolation(&v, clause, "UOW-06", saved, s.Constraints[0], keys[0], true)
			if then != nil {
				v.fail(clause, "UOW-06", "Save() after the violation = %v, want nil — the transaction must outlive it", then)
			} else {
				s.expectStored(&v, clause, "UOW-06", acme, s.NewID(after), after, 1)
			}
			s.expectAbsent(&v, clause, "DEC-10", acme, s.NewID(refused))
		}
	}

	{
		const clause = "leaves the rejection out after a prior save"
		s := newSubject()
		taken, prior, refused := next(), next(), next()
		if s.create(&v, clause, acme, taken, s.keys(taken)) {
			s.afterPriorWrite(&v, clause, acme, taken, prior, refused, func(ctx context.Context, w UniqueWrites[ID, S]) error {
				return w.Repo.Save(ctx, s.NewID(prior), s.NewState(prior, s.keys(prior)), 0)
			})
			s.expectAbsent(&v, clause, "DEC-10", acme, s.NewID(prior))
		}
	}

	{
		const clause = "leaves the rejection out after a prior enqueue"
		s := newSubject()
		taken, prior, refused := next(), next(), next()
		if s.create(&v, clause, acme, taken, s.keys(taken)) {
			s.afterPriorWrite(&v, clause, acme, taken, prior, refused, func(ctx context.Context, w UniqueWrites[ID, S]) error {
				if w.Enqueue == nil {
					return errNoEnqueue
				}
				return w.Enqueue(ctx, prior)
			})
		}
	}

	{
		const clause = "lets exactly one of two concurrent creations through"
		s := newSubject()
		first, second := next(), next()
		if !s.Concurrent {
			v.skip(clause)
		} else {
			concurrentCreations(&v, clause, s, acme, [2]int{first, second})
		}
	}

	{
		const clause = "collides on an empty value as on any other"
		s := newSubject()
		taken, second := next(), next()
		empty := func(row int) [][]string {
			keys := s.keys(row)
			keys[0] = make([]string, len(keys[0]))
			return keys
		}
		if s.create(&v, clause, acme, taken, empty(taken)) {
			keys := empty(second)
			var saved error
			if err := s.Within(acme, func(ctx context.Context, w UniqueWrites[ID, S]) error {
				saved = w.Repo.Save(ctx, s.NewID(second), s.NewState(second, keys), 0)
				return nil
			}); err != nil {
				v.fail(clause, "UOW-06", "Within() after swallowing the violation = %v, want nil", err)
			}
			s.expectViolation(&v, clause, "UOW-06", saved, s.Constraints[0], keys[0], true)
		}
	}

	{
		const clause = "tells composite keys apart column by column"
		s := newSubject()
		first := next()
		composite := s.composite()
		if composite < 0 {
			v.skip(clause)
		} else {
			taken := append([]string{"dmpf-kit-ab", "c"}, s.keys(0)[composite][2:]...)
			with := func(row int, tuple []string) [][]string {
				keys := s.keys(row)
				keys[composite] = tuple
				return keys
			}
			concatenated := slices.Clone(taken)
			concatenated[0], concatenated[1] = "dmpf-kit-a", "bc"
			besides := [][]string{concatenated}
			for at := range taken {
				beside := slices.Clone(taken)
				beside[at] = fmt.Sprintf("dmpf-kit-other-%d", at)
				besides = append(besides, beside)
			}
			if s.create(&v, clause, acme, first, with(first, taken)) {
				for _, beside := range besides {
					row := next()
					if err := s.save(acme, row, with(row, beside)); err != nil {
						v.fail(clause, "UOW-06", "Save() of %q beside %q on %s = %v, want nil — each column is a value of its own",
							beside, taken, s.Constraints[composite].Name, err)
					}
				}
			}
		}
	}

	{
		const clause = "carries the rejection of the constraint it broke"
		s := newSubject()
		taken := next()
		if len(s.Constraints) < 2 {
			v.skip(clause)
		} else if code, ok := repeatedCode(s.Constraints); ok {
			v.fail(clause, "UOW-06", "two constraints declare %q; the clause cannot tell their rejections apart", code)
		} else if s.create(&v, clause, acme, taken, s.keys(taken)) {
			for i, c := range s.Constraints {
				refused := next()
				keys := s.taking(refused, i, taken)
				var saved error
				if err := s.Within(acme, func(ctx context.Context, w UniqueWrites[ID, S]) error {
					saved = w.Repo.Save(ctx, s.NewID(refused), s.NewState(refused, keys), 0)
					return nil
				}); err != nil {
					v.fail(clause, "UOW-06", "Within() after swallowing the violation of %s = %v, want nil", c.Name, err)
				}
				s.expectViolation(&v, clause, "UOW-06", saved, c, keys[i], true)
			}
		}
	}

	{
		const clause = "carries the rejection of the first declared constraint it broke"
		s := newSubject()
		taken, refused := next(), next()
		if len(s.Constraints) < 2 {
			v.skip(clause)
		} else if s.create(&v, clause, acme, taken, s.keys(taken)) {
			keys := s.keys(taken)
			var saved error
			if err := s.Within(acme, func(ctx context.Context, w UniqueWrites[ID, S]) error {
				saved = w.Repo.Save(ctx, s.NewID(refused), s.NewState(refused, keys), 0)
				return nil
			}); err != nil {
				v.fail(clause, "UOW-06", "Within() after swallowing the violation = %v, want nil", err)
			}
			s.expectViolation(&v, clause, "UOW-06", saved, s.Constraints[0], slices.Concat(keys...), true)
			s.expectAbsent(&v, clause, "DEC-10", acme, s.NewID(refused))
		}
	}

	{
		const clause = "refuses a creation over the same identity as a version conflict"
		s := newSubject()
		taken, marker := next(), next()
		if s.create(&v, clause, acme, taken, s.keys(taken)) {
			err := s.Within(acme, func(ctx context.Context, w UniqueWrites[ID, S]) error {
				return w.Repo.Save(ctx, s.NewID(taken), s.NewState(marker, s.keys(taken)), 0)
			})
			if !errors.Is(err, ports.ErrVersionConflict) || errors.Is(err, ports.ErrUniqueViolation) {
				v.fail(clause, "UOW-09", "Save(expected=0) over the same identity and values = %v, want ErrVersionConflict and no UniqueViolation", err)
			}
			s.expectStored(&v, clause, "UOW-09", acme, s.NewID(taken), taken, 1)
		}
	}

	return v
}

var errNoEnqueue = errors.New("providerkit: the subject offers no Enqueue")

// afterPriorWrite returns the violation from the callback, so the transaction
// rolls back with the prior write: converting it would keep that write (DEC-10).
func (s UniqueSubject[ID, S]) afterPriorWrite(v *Verdict, clause string, ctx context.Context, taken, prior, refused int, write func(context.Context, UniqueWrites[ID, S]) error) {
	keys := s.taking(refused, 0, taken)
	var wrote, saved error
	err := s.Within(ctx, func(ctx context.Context, w UniqueWrites[ID, S]) error {
		if wrote = write(ctx, w); wrote != nil {
			return nil
		}
		saved = w.Repo.Save(ctx, s.NewID(refused), s.NewState(refused, keys), 0)
		return saved
	})
	switch {
	case errors.Is(wrote, errNoEnqueue):
		v.skip(clause)
		return
	case wrote != nil:
		v.fail(clause, "DEC-10", "the prior write = %v, want nil", wrote)
		return
	case !errors.Is(err, ports.ErrUniqueViolation):
		v.fail(clause, "DEC-10", "Within() = %v, want the UniqueViolation the callback returned", err)
	}
	s.expectViolation(v, clause, "DEC-10", saved, s.Constraints[0], keys[0], false)
	s.expectAbsent(v, clause, "DEC-10", ctx, s.NewID(refused))
}

// concurrentCreations releases both writers into Save together, so the second
// waits on the first one's index entry instead of reading a committed row.
func concurrentCreations[ID comparable, S any](v *Verdict, clause string, s UniqueSubject[ID, S], ctx context.Context, rows [2]int) {
	keys := [2][][]string{s.keys(rows[0]), s.taking(rows[1], 0, rows[0])}
	results := raceTwo(func(writer int, meet func() error) error {
		return s.Within(ctx, func(ctx context.Context, w UniqueWrites[ID, S]) error {
			if err := meet(); err != nil {
				return err
			}
			return w.Repo.Save(ctx, s.NewID(rows[writer]), s.NewState(rows[writer], keys[writer]), 0)
		})
	})

	var created int
	var refused, unexpected []error
	var serialized bool
	for _, err := range results {
		switch {
		case err == nil:
			created++
		case errors.Is(err, errSerialized):
			serialized = true
		case errors.Is(err, ports.ErrUniqueViolation):
			refused = append(refused, err)
		default:
			unexpected = append(unexpected, err)
		}
	}
	switch {
	case len(unexpected) > 0:
		v.fail(clause, "KRN-06", "Within() = %v, want nil or a UniqueViolation", unexpected)
	case serialized:
		v.fail(clause, "KRN-06", "Within serializes callers, so Concurrent must be false")
	case created != 1 || len(refused) != 1:
		v.fail(clause, "KRN-06", "%d created and %d refused, want 1 and 1 — two rows share the value", created, len(refused))
	default:
		s.expectViolation(v, clause, "KRN-06", refused[0], s.Constraints[0], keys[0][0], true)
	}
}

func (s UniqueSubject[ID, S]) declared() error {
	if len(s.Constraints) == 0 {
		return errors.New("the subject declares no unique constraint")
	}
	for _, c := range s.Constraints {
		if c.Name == "" || len(c.Columns) < 2 || c.Columns[0] != "tenant_id" || slices.Contains(c.Columns, "") {
			return fmt.Errorf("constraint %+v needs a name and its columns as postgres.Unique lists them: tenant_id, then at least one more", c)
		}
	}
	return nil
}

func (c UniqueConstraint) attributes() int { return len(c.Columns) - 1 }

func (s UniqueSubject[ID, S]) keys(row int) [][]string {
	keys := make([][]string, len(s.Constraints))
	for i, c := range s.Constraints {
		keys[i] = make([]string, c.attributes())
		for j := range keys[i] {
			keys[i][j] = fmt.Sprintf("dmpf-kit-%d-%d-%d", row, i, j)
		}
	}
	return keys
}

func (s UniqueSubject[ID, S]) taking(row, constraint, from int) [][]string {
	keys := s.keys(row)
	keys[constraint] = s.keys(from)[constraint]
	return keys
}

func (s UniqueSubject[ID, S]) composite() int {
	for i, c := range s.Constraints {
		if c.attributes() >= 2 {
			return i
		}
	}
	return -1
}

func repeatedCode(constraints []UniqueConstraint) (domain.Code, bool) {
	seen := map[domain.Code]bool{}
	for _, c := range constraints {
		if seen[c.Code] {
			return c.Code, true
		}
		seen[c.Code] = true
	}
	return "", false
}

func (s UniqueSubject[ID, S]) save(ctx context.Context, row int, keys [][]string) error {
	return s.Within(ctx, func(ctx context.Context, w UniqueWrites[ID, S]) error {
		return w.Repo.Save(ctx, s.NewID(row), s.NewState(row, keys), 0)
	})
}

func (s UniqueSubject[ID, S]) create(v *Verdict, clause string, ctx context.Context, row int, keys [][]string) bool {
	if err := s.save(ctx, row, keys); err != nil {
		v.fail(clause, "UOW-01", "Save() creating the row the clause collides with = %v, want nil", err)
		return false
	}
	return true
}

// expectViolation also reads the error text for the values: the record of a
// refusal names the constraint, never what collided (ERR-07).
func (s UniqueSubject[ID, S]) expectViolation(v *Verdict, clause, rule string, err error, c UniqueConstraint, values []string, declared bool) {
	violation, ok := errors.AsType[ports.UniqueViolation](err)
	if !ok || !errors.Is(err, ports.ErrUniqueViolation) {
		v.fail(clause, rule, "Save() = %v, want a UniqueViolation on %s", err, c.Name)
		return
	}
	if violation.Table != s.Table || violation.Constraint != c.Name {
		v.fail(clause, rule, "UniqueViolation names %s/%s, want %s/%s", violation.Table, violation.Constraint, s.Table, c.Name)
	}
	switch {
	case !declared && violation.Rejection != nil:
		v.fail(clause, rule, "UniqueViolation carries %q after a prior write, want no rejection", violation.Rejection.Code())
	case declared && violation.Rejection == nil:
		v.fail(clause, rule, "UniqueViolation carries no rejection, want %q", c.Code)
	case declared && violation.Rejection.Code() != c.Code:
		v.fail(clause, rule, "UniqueViolation carries %q, want %q", violation.Rejection.Code(), c.Code)
	}
	for _, value := range values {
		if value != "" && strings.Contains(err.Error(), value) {
			v.fail(clause, "ERR-07", "the error names the value %q it collided on", value)
		}
	}
}

func (s UniqueSubject[ID, S]) expectAbsent(v *Verdict, clause, rule string, ctx context.Context, id ID) {
	if _, _, err := s.Reader.Load(ctx, id); !errors.Is(err, ports.ErrNotFound) {
		v.fail(clause, rule, "Load() of the refused row = %v, want ErrNotFound", err)
	}
}

func (s UniqueSubject[ID, S]) expectStored(v *Verdict, clause, rule string, ctx context.Context, id ID, wantMarker int, wantVersion ports.Version) {
	state, version, err := s.Reader.Load(ctx, id)
	switch {
	case err != nil:
		v.fail(clause, rule, "Load() = %v, want nil", err)
	case s.Marker(state) != wantMarker || version != wantVersion:
		v.fail(clause, rule, "Load() = marker %d, v%d; want marker %d, v%d", s.Marker(state), version, wantMarker, wantVersion)
	}
}
