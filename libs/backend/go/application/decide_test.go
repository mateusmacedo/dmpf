package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type counterState struct{ Count int }

type counter struct {
	fresh bool
	count int
}

func (c *counter) Snapshot() counterState { return counterState{Count: c.count} }

func freshCounter(string) *counter { return &counter{fresh: true} }

func counterFrom(s counterState) *counter { return &counter{count: s.Count} }

type savedState struct {
	id       string
	state    counterState
	expected ports.Version
}

type counterRepo struct {
	steps   *[]string
	state   counterState
	stored  ports.Version
	loadErr error
	saveErr error
	saves   []savedState
}

func (r *counterRepo) Load(context.Context, string) (counterState, ports.Version, error) {
	*r.steps = append(*r.steps, "load")
	if r.loadErr != nil {
		return counterState{}, 0, r.loadErr
	}
	return r.state, r.stored, nil
}

func (r *counterRepo) Save(_ context.Context, id string, state counterState, expected ports.Version) error {
	*r.steps = append(*r.steps, "save")
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saves = append(r.saves, savedState{id: id, state: state, expected: expected})
	return nil
}

type stepOutbox struct {
	steps *[]string
	inner *recordingOutbox
}

func (o stepOutbox) Enqueue(ctx context.Context, entry ports.OutboxEntry) error {
	*o.steps = append(*o.steps, "enqueue")
	return o.inner.Enqueue(ctx, entry)
}

type decideCase struct {
	steps   []string
	repo    *counterRepo
	outbox  *recordingOutbox
	verdict func(*counter) (kernel.Accepted[int], *kernel.Rejection)
}

var counterIdentity = usecase.Identity{OccurredAt: now, MessageIDs: []ports.MessageID{"m-1", "m-2"}}

func newDecideCase() *decideCase {
	c := &decideCase{outbox: &recordingOutbox{}}
	c.repo = &counterRepo{steps: &c.steps, state: counterState{Count: 4}, stored: 3}
	c.verdict = func(agg *counter) (kernel.Accepted[int], *kernel.Rejection) {
		agg.count++
		return kernel.Accept(agg.count, namedEvent("counted")), nil
	}
	return c
}

func (c *decideCase) decide(load usecase.Loader[string, counterState, *counter]) (usecase.Outcome[int], error) {
	return usecase.Decide(context.Background(), c.repo, stepOutbox{&c.steps, c.outbox}, origin, "c-1", counterIdentity, load,
		func(agg *counter) (kernel.Accepted[int], *kernel.Rejection) {
			c.steps = append(c.steps, "decide")
			return c.verdict(agg)
		})
}

func TestLoadersMapTheReaderAnswer(t *testing.T) {
	boom := errors.New("boom")
	crossTenant := ports.CrossTenantAccess{Object: "c-1", ContextTenant: "acme", DataTenant: "other"}
	orNew := usecase.OrNew(freshCounter, counterFrom)
	existing := usecase.Existing[string](counterFrom)
	absent := usecase.Absent[counterState](freshCounter)
	for _, tc := range []struct {
		name      string
		load      usecase.Loader[string, counterState, *counter]
		loadErr   error
		wantFresh bool
		wantCount int
		wantVer   ports.Version
		wantIs    error
		wantMsg   string
	}{
		{name: "OrNew, absent", load: orNew, loadErr: ports.ErrNotFound, wantFresh: true},
		{name: "OrNew, found", load: orNew, wantCount: 4, wantVer: 3},
		{name: "OrNew, failure", load: orNew, loadErr: boom, wantIs: boom, wantMsg: "boom"},
		{name: "OrNew, other tenant", load: orNew, loadErr: crossTenant, wantFresh: true},
		{name: "Existing, found", load: existing, wantCount: 4, wantVer: 3},
		{name: "Existing, absent", load: existing, loadErr: ports.ErrNotFound, wantIs: ports.ErrNotFound, wantMsg: "ports: aggregate not found"},
		{name: "Existing, failure", load: existing, loadErr: boom, wantIs: boom, wantMsg: "boom"},
		{name: "Absent, found", load: absent, wantIs: ports.ErrAlreadyExists, wantMsg: "ports: aggregate already exists"},
		{name: "Absent, absent", load: absent, loadErr: ports.ErrNotFound, wantFresh: true},
		{name: "Absent, failure", load: absent, loadErr: boom, wantIs: boom, wantMsg: "boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var steps []string
			repo := &counterRepo{steps: &steps, state: counterState{Count: 4}, stored: 3, loadErr: tc.loadErr}

			agg, version, err := tc.load(context.Background(), repo, "c-1")

			if tc.wantIs != nil {
				if !errors.Is(err, tc.wantIs) || err.Error() != tc.wantMsg {
					t.Fatalf("err = %v, want %q matching %v", err, tc.wantMsg, tc.wantIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if agg.fresh != tc.wantFresh || agg.count != tc.wantCount || version != tc.wantVer {
				t.Fatalf("loaded (fresh=%v, count=%d, version=%d), want (%v, %d, %d)",
					agg.fresh, agg.count, version, tc.wantFresh, tc.wantCount, tc.wantVer)
			}
		})
	}
}

func TestDecideSavesThenEnqueuesTheAcceptedDecision(t *testing.T) {
	c := newDecideCase()

	outcome, err := c.decide(usecase.Existing[string](counterFrom))

	if err != nil || outcome.Response() != 5 {
		t.Fatalf("Decide() = (%d, %v), want (5, nil)", outcome.Response(), err)
	}
	if want := []string{"load", "decide", "save", "enqueue"}; !slices.Equal(c.steps, want) {
		t.Fatalf("steps = %q, want %q", c.steps, want)
	}
	if want := []savedState{{id: "c-1", state: counterState{Count: 5}, expected: 3}}; !slices.Equal(c.repo.saves, want) {
		t.Fatalf("saves = %+v, want %+v", c.repo.saves, want)
	}
	entry := c.outbox.entries[0]
	if len(c.outbox.entries) != 1 || entry.AggregateVersion != 4 || entry.AggregateID != origin.AggregateID ||
		entry.Intent.Destination != origin.Destination || entry.Event != namedEvent("counted") {
		t.Fatalf("entries = %+v, want one counted event at version 4 from %+v", c.outbox.entries, origin)
	}
}

func TestDecideSavesWithoutEnqueueingADecisionWithoutEvents(t *testing.T) {
	c := newDecideCase()
	c.verdict = func(agg *counter) (kernel.Accepted[int], *kernel.Rejection) { return kernel.Accept(agg.count), nil }

	_, err := c.decide(usecase.Existing[string](counterFrom))

	if err != nil || len(c.repo.saves) != 1 || len(c.outbox.entries) != 0 {
		t.Fatalf("Decide() = %v with %d saves and %d entries, want nil, 1 and 0", err, len(c.repo.saves), len(c.outbox.entries))
	}
}

func TestDecideRejectsWithoutWriting(t *testing.T) {
	c := newDecideCase()
	c.verdict = func(*counter) (kernel.Accepted[int], *kernel.Rejection) {
		return kernel.Refuse[int]("kernel/counter/full", "counter is full")
	}

	outcome, err := c.decide(usecase.Existing[string](counterFrom))

	if rejection, rejected := outcome.Rejection(); err != nil || !rejected || rejection.Code() != "kernel/counter/full" {
		t.Fatalf("Decide() = (%+v, %v), want the rejection and nil", outcome, err)
	}
	if want := []string{"load", "decide"}; !slices.Equal(c.steps, want) {
		t.Fatalf("steps = %q, want %q", c.steps, want)
	}
}

func TestDecideReturnsEachFailureOfItsSteps(t *testing.T) {
	boom := errors.New("boom")
	for _, tc := range []struct {
		name    string
		arrange func(*decideCase)
		load    usecase.Loader[string, counterState, *counter]
		steps   []string
		wantIs  error
		wantMsg string
	}{
		{
			name:    "load fails",
			arrange: func(c *decideCase) { c.repo.loadErr = boom },
			load:    usecase.Existing[string](counterFrom),
			steps:   []string{"load"}, wantIs: boom, wantMsg: "boom",
		},
		{
			name:  "already exists",
			load:  usecase.Absent[counterState](freshCounter),
			steps: []string{"load"}, wantIs: ports.ErrAlreadyExists, wantMsg: "ports: aggregate already exists",
		},
		{
			name:    "save fails",
			arrange: func(c *decideCase) { c.repo.saveErr = boom },
			load:    usecase.Existing[string](counterFrom),
			steps:   []string{"load", "decide", "save"}, wantIs: boom, wantMsg: "boom",
		},
		{
			name:    "save conflicts",
			arrange: func(c *decideCase) { c.repo.saveErr = ports.ErrVersionConflict },
			load:    usecase.Existing[string](counterFrom),
			steps:   []string{"load", "decide", "save"}, wantIs: ports.ErrVersionConflict, wantMsg: "ports: version conflict",
		},
		{
			name:    "enqueue fails",
			arrange: func(c *decideCase) { c.outbox.err = boom },
			load:    usecase.Existing[string](counterFrom),
			steps:   []string{"load", "decide", "save", "enqueue"}, wantIs: boom, wantMsg: "enqueue: boom",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newDecideCase()
			if tc.arrange != nil {
				tc.arrange(c)
			}

			_, err := c.decide(tc.load)

			if !errors.Is(err, tc.wantIs) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q matching %v", err, tc.wantMsg, tc.wantIs)
			}
			if !slices.Equal(c.steps, tc.steps) {
				t.Fatalf("steps = %q, want %q", c.steps, tc.steps)
			}
		})
	}
}
