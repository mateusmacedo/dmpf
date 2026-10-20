package providerkit_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const (
	clauseRejects     = "refuses a second row holding a declared value"
	clauseUpdate      = "refuses an update onto a value another row holds"
	clausePerTenant   = "writes a taken value under another tenant"
	clauseTxUsable    = "keeps the transaction usable after the violation"
	clausePriorSave   = "leaves the rejection out after a prior save"
	clausePriorOutbox = "leaves the rejection out after a prior enqueue"
	clauseConcurrent  = "lets exactly one of two concurrent creations through"
	clauseEmpty       = "collides on an empty value as on any other"
	clauseComposite   = "tells composite keys apart column by column"
	clauseOwnCode     = "carries the rejection of the constraint it broke"
	clauseFirstWins   = "carries the rejection of the first declared constraint it broke"
	clauseSameID      = "refuses a creation over the same identity as a version conflict"
)

type member struct {
	Email, Org, Team, Handle string
	Marker                   int
}

const (
	memberEmailKey  = "members_tenant_id_email_key"
	memberHandleKey = "members_tenant_id_org_handle_key"
	memberTrioKey   = "members_tenant_id_org_team_handle_key"
)

var (
	memberEmailTaken  = domain.Reject("dmpf-kit/member/email-taken", "the email is taken")
	memberHandleTaken = domain.Reject("dmpf-kit/member/handle-taken", "the handle is taken in the org")
)

func emailOf(m member) ([]string, bool)  { return []string{m.Email}, true }
func handleOf(m member) ([]string, bool) { return []string{m.Org, m.Handle}, true }

var declaredMembers = memory.Table[string, member]{
	Name: "members",
	Unique: []memory.Unique[member]{
		{Name: memberEmailKey, Key: emailOf, Rejection: memberEmailTaken},
		{Name: memberHandleKey, Key: handleOf, Rejection: memberHandleTaken},
	},
}

type memberResources struct {
	Members ports.Repository[string, member]
	Outbox  ports.Outbox
}

func memoryUniqueOver(table memory.Table[string, member]) func() providerkit.UniqueSubject[string, member] {
	return func() providerkit.UniqueSubject[string, member] {
		store := memory.New()
		return providerkit.UniqueSubject[string, member]{
			Within: func(ctx context.Context, fn func(ctx context.Context, w providerkit.UniqueWrites[string, member]) error) error {
				uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) memberResources {
					return memberResources{Members: table.Repository(tx), Outbox: tx.Outbox()}
				})
				return uow.Within(ctx, func(ctx context.Context, res memberResources) error {
					return fn(ctx, providerkit.UniqueWrites[string, member]{
						Repo: res.Members,
						Enqueue: func(ctx context.Context, n int) error {
							return res.Outbox.Enqueue(ctx, memberEntry(n))
						},
					})
				})
			},
			Reader: table.Reader(store),
			NewID:  func(n int) string { return "m-" + strconv.Itoa(n) },
			NewState: func(marker int, keys [][]string) member {
				return member{Email: keys[0][0], Org: keys[1][0], Handle: keys[1][1], Marker: marker}
			},
			Marker: func(m member) int { return m.Marker },
			Table:  "members",
			Constraints: []providerkit.UniqueConstraint{
				{Name: memberEmailKey, Columns: []string{"tenant_id", "email"}, Code: memberEmailTaken.Code()},
				{Name: memberHandleKey, Columns: []string{"tenant_id", "org", "handle"}, Code: memberHandleTaken.Code()},
			},
		}
	}
}

func trioMembersKeyedBy(key func(member) ([]string, bool)) func() providerkit.UniqueSubject[string, member] {
	table := memory.Table[string, member]{Name: "members", Unique: []memory.Unique[member]{
		{Name: memberEmailKey, Key: emailOf, Rejection: memberEmailTaken},
		{Name: memberTrioKey, Key: key, Rejection: memberHandleTaken},
	}}
	return func() providerkit.UniqueSubject[string, member] {
		s := memoryUniqueOver(table)()
		s.NewState = func(marker int, keys [][]string) member {
			return member{Email: keys[0][0], Org: keys[1][0], Team: keys[1][1], Handle: keys[1][2], Marker: marker}
		}
		s.Constraints = []providerkit.UniqueConstraint{
			s.Constraints[0],
			{Name: memberTrioKey, Columns: []string{"tenant_id", "org", "team", "handle"}, Code: memberHandleTaken.Code()},
		}
		return s
	}
}

func trioOf(m member) ([]string, bool) { return []string{m.Org, m.Team, m.Handle}, true }

func memberEntry(n int) ports.OutboxEntry {
	id := "m-" + strconv.Itoa(n)
	return ports.OutboxEntry{
		MessageID: ports.MessageID("evt-" + id), OccurredAt: 1,
		Intent:        ports.PublishIntent{Destination: "members.events", PartitionKey: id},
		AggregateType: "members.Member", AggregateID: id, AggregateVersion: 1,
	}
}

func TestTheUniqueKitPassesOverTheDeclaredMemoryTable(t *testing.T) {
	v := providerkit.Unique(memoryUniqueOver(declaredMembers))
	tb.Require(t, v)
	if want := []string{clauseConcurrent}; !slices.Equal(v.Skipped, want) {
		t.Fatalf("skipped = %v, want %v: memory serializes Within", v.Skipped, want)
	}
}

func TestTheUniqueKitPassesOverAThreeColumnCompositeKey(t *testing.T) {
	v := providerkit.Unique(trioMembersKeyedBy(trioOf))
	tb.Require(t, v)
	if want := []string{clauseConcurrent}; !slices.Equal(v.Skipped, want) {
		t.Fatalf("skipped = %v, want %v: memory serializes Within", v.Skipped, want)
	}
}

func TestTheUniqueKitNamesWhatTheCandidateCannotExercise(t *testing.T) {
	emailOnly := memory.Table[string, member]{Name: "members", Unique: declaredMembers.Unique[:1]}
	v := providerkit.Unique(func() providerkit.UniqueSubject[string, member] {
		s := memoryUniqueOver(emailOnly)()
		within := s.Within
		s.Within = func(ctx context.Context, fn func(ctx context.Context, w providerkit.UniqueWrites[string, member]) error) error {
			return within(ctx, func(ctx context.Context, w providerkit.UniqueWrites[string, member]) error {
				w.Enqueue = nil
				return fn(ctx, w)
			})
		}
		s.Constraints = s.Constraints[:1]
		s.NewState = func(marker int, keys [][]string) member { return member{Email: keys[0][0], Marker: marker} }
		return s
	})

	tb.Require(t, v)
	want := []string{clausePriorOutbox, clauseConcurrent, clauseComposite, clauseOwnCode, clauseFirstWins}
	if got := slices.Sorted(slices.Values(v.Skipped)); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Fatalf("skipped = %v, want %v", v.Skipped, want)
	}
}

func TestASubjectWithoutConstraintsIsReproved(t *testing.T) {
	v := providerkit.Unique(func() providerkit.UniqueSubject[string, member] {
		s := memoryUniqueOver(declaredMembers)()
		s.Constraints = nil
		return s
	})
	if v.OK() {
		t.Fatal("a subject that declares no constraint passed")
	}
}

func TestASubjectWithAMalformedConstraintIsReproved(t *testing.T) {
	for _, columns := range [][]string{nil, {"tenant_id"}, {"email"}, {"email", "tenant_id"}, {"tenant_id", ""}} {
		v := providerkit.Unique(func() providerkit.UniqueSubject[string, member] {
			s := memoryUniqueOver(declaredMembers)()
			s.Constraints = slices.Clone(s.Constraints)
			s.Constraints[0].Columns = columns
			return s
		})
		if v.OK() || !reprovedOn(v, "declares the constraints it is checked against") {
			t.Errorf("Columns %q: %v, want the subject reproved for its declaration", columns, v.Failures())
		}
	}
}

func TestEverySabotageIsReprovedOnTheClauseItBreaks(t *testing.T) {
	for _, tc := range []struct {
		name    string
		subject func() providerkit.UniqueSubject[string, member]
		want    []string
	}{
		{
			name:    "uniqueness ignored",
			subject: memoryUniqueOver(memory.Table[string, member]{Name: "members"}),
			want:    []string{clauseRejects, clauseUpdate, clauseTxUsable, clausePriorSave, clausePriorOutbox, clauseEmpty, clauseOwnCode, clauseFirstWins},
		},
		{
			name: "uniqueness across tenants",
			subject: sabotaged(func(inner ports.Repository[string, member]) ports.Repository[string, member] {
				return inner
			}, asFirstTenant),
			want: []string{clausePerTenant},
		},
		{
			name: "converted after a prior write",
			subject: sabotaged(func(inner ports.Repository[string, member]) ports.Repository[string, member] {
				return convertingRepository{inner: inner}
			}, nil),
			want: []string{clausePriorSave, clausePriorOutbox},
		},
		{
			name: "transaction poisoned by the violation",
			subject: sabotaged(func(inner ports.Repository[string, member]) ports.Repository[string, member] {
				return &poisonedRepository{inner: inner}
			}, nil),
			want: []string{clauseTxUsable},
		},
		{
			name: "same identity reported as a violation",
			subject: sabotaged(func(inner ports.Repository[string, member]) ports.Repository[string, member] {
				return identityAsViolation{inner: inner}
			}, nil),
			want: []string{clauseSameID},
		},
		{
			name: "composite key concatenated",
			subject: memoryUniqueOver(memory.Table[string, member]{Name: "members", Unique: []memory.Unique[member]{
				{Name: memberEmailKey, Key: emailOf, Rejection: memberEmailTaken},
				{Name: memberHandleKey, Key: func(m member) ([]string, bool) { return []string{m.Org + m.Handle}, true }, Rejection: memberHandleTaken},
			}}),
			want: []string{clauseComposite},
		},
		{
			name: "composite key read by its first column",
			subject: memoryUniqueOver(memory.Table[string, member]{Name: "members", Unique: []memory.Unique[member]{
				{Name: memberEmailKey, Key: emailOf, Rejection: memberEmailTaken},
				{Name: memberHandleKey, Key: func(m member) ([]string, bool) { return []string{m.Org}, true }, Rejection: memberHandleTaken},
			}}),
			want: []string{clauseComposite},
		},
		{
			name: "composite key read by its last column",
			subject: memoryUniqueOver(memory.Table[string, member]{Name: "members", Unique: []memory.Unique[member]{
				{Name: memberEmailKey, Key: emailOf, Rejection: memberEmailTaken},
				{Name: memberHandleKey, Key: func(m member) ([]string, bool) { return []string{m.Handle}, true }, Rejection: memberHandleTaken},
			}}),
			want: []string{clauseComposite},
		},
		{
			name:    "composite key read without its middle column",
			subject: trioMembersKeyedBy(func(m member) ([]string, bool) { return []string{m.Org, m.Handle}, true }),
			want:    []string{clauseComposite},
		},
		{
			name: "constraints declared out of DDL order",
			subject: memoryUniqueOver(memory.Table[string, member]{Name: "members", Unique: []memory.Unique[member]{
				{Name: memberHandleKey, Key: handleOf, Rejection: memberHandleTaken},
				{Name: memberEmailKey, Key: emailOf, Rejection: memberEmailTaken},
			}}),
			want: []string{clauseFirstWins},
		},
		{
			name: "empty value read as NULL",
			subject: memoryUniqueOver(memory.Table[string, member]{Name: "members", Unique: []memory.Unique[member]{
				{Name: memberEmailKey, Key: func(m member) ([]string, bool) { return []string{m.Email}, m.Email != "" }, Rejection: memberEmailTaken},
				{Name: memberHandleKey, Key: handleOf, Rejection: memberHandleTaken},
			}}),
			want: []string{clauseEmpty},
		},
		{
			name: "one rejection for both constraints",
			subject: memoryUniqueOver(memory.Table[string, member]{Name: "members", Unique: []memory.Unique[member]{
				{Name: memberEmailKey, Key: emailOf, Rejection: memberEmailTaken},
				{Name: memberHandleKey, Key: handleOf, Rejection: memberEmailTaken},
			}}),
			want: []string{clauseOwnCode},
		},
		{
			name:    "concurrent creations both let through",
			subject: unguardedConcurrentMembers,
			want:    []string{clauseConcurrent},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := providerkit.Unique(tc.subject)
			for _, clause := range tc.want {
				if !reprovedOn(v, clause) {
					t.Errorf("clause %q not reproved: %v", clause, v.Failures())
				}
			}
		})
	}
}

func TestAConcurrentCreationOverASerializingRealizationFailsInsteadOfHanging(t *testing.T) {
	defer providerkit.ShortenConcurrentBarrier(100 * time.Millisecond)()
	v := providerkit.Unique(func() providerkit.UniqueSubject[string, member] {
		s := memoryUniqueOver(declaredMembers)()
		s.Concurrent = true
		return s
	})

	var named bool
	for _, d := range v.Diagnostics {
		if d.Clause == clauseConcurrent && strings.Contains(d.Detail, "Within serializes callers") {
			named = true
		}
	}
	if !named {
		t.Fatalf("diagnostics = %v, want the concurrent clause to name the serialization", v.Failures())
	}
}

func TestAViolationThatNamesTheValueIsReprovedOnERR07(t *testing.T) {
	v := providerkit.Unique(sabotaged(func(inner ports.Repository[string, member]) ports.Repository[string, member] {
		return leakingRepository{inner: inner}
	}, nil))
	var named bool
	for _, d := range v.Diagnostics {
		if d.Rule == "ERR-07" {
			named = true
		}
	}
	if !named {
		t.Fatalf("ERR-07 not named: %v", v.Failures())
	}
}

func reprovedOn(v providerkit.Verdict, clause string) bool {
	for _, d := range v.Diagnostics {
		if d.Clause == clause {
			return true
		}
	}
	return false
}

func sabotaged(wrap func(inner ports.Repository[string, member]) ports.Repository[string, member], rescope func(context.Context) context.Context) func() providerkit.UniqueSubject[string, member] {
	return func() providerkit.UniqueSubject[string, member] {
		s := memoryUniqueOver(declaredMembers)()
		within := s.Within
		s.Within = func(ctx context.Context, fn func(ctx context.Context, w providerkit.UniqueWrites[string, member]) error) error {
			if rescope != nil {
				ctx = rescope(ctx)
			}
			return within(ctx, func(ctx context.Context, w providerkit.UniqueWrites[string, member]) error {
				w.Repo = wrap(w.Repo)
				return fn(ctx, w)
			})
		}
		reader := s.Reader
		if rescope != nil {
			s.Reader = rescopedReader{inner: reader, rescope: rescope}
		}
		return s
	}
}

func asFirstTenant(ctx context.Context) context.Context {
	subject := ports.SubjectID("dmpf-kit-subject")
	tenant := ports.TenantID("dmpf-kit-a")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "dmpf-kit-request", CorrelationID: "dmpf-kit-correlation", TraceContext: "dmpf-kit-trace",
		Subject: &subject, Tenant: &tenant, Permissions: []ports.Permission{}, Deadline: 1, Locale: "en",
	})
	if err != nil {
		panic(err)
	}
	return ports.WithExecutionContext(ctx, execution)
}

type rescopedReader struct {
	inner   ports.Reader[string, member]
	rescope func(context.Context) context.Context
}

func (r rescopedReader) Load(ctx context.Context, id string) (member, ports.Version, error) {
	return r.inner.Load(r.rescope(ctx), id)
}

type convertingRepository struct {
	inner ports.Repository[string, member]
}

func (r convertingRepository) Load(ctx context.Context, id string) (member, ports.Version, error) {
	return r.inner.Load(ctx, id)
}

func (r convertingRepository) Save(ctx context.Context, id string, state member, expected ports.Version) error {
	err := r.inner.Save(ctx, id, state, expected)
	if violation, ok := errors.AsType[ports.UniqueViolation](err); ok && violation.Rejection == nil {
		violation.Rejection = memberEmailTaken
		return violation
	}
	return err
}

type poisonedRepository struct {
	inner    ports.Repository[string, member]
	poisoned bool
}

var errAborted = errors.New("current transaction is aborted")

func (r *poisonedRepository) Load(ctx context.Context, id string) (member, ports.Version, error) {
	return r.inner.Load(ctx, id)
}

func (r *poisonedRepository) Save(ctx context.Context, id string, state member, expected ports.Version) error {
	if r.poisoned {
		return errAborted
	}
	err := r.inner.Save(ctx, id, state, expected)
	r.poisoned = errors.Is(err, ports.ErrUniqueViolation)
	return err
}

type identityAsViolation struct {
	inner ports.Repository[string, member]
}

func (r identityAsViolation) Load(ctx context.Context, id string) (member, ports.Version, error) {
	return r.inner.Load(ctx, id)
}

func (r identityAsViolation) Save(ctx context.Context, id string, state member, expected ports.Version) error {
	err := r.inner.Save(ctx, id, state, expected)
	if expected == 0 && errors.Is(err, ports.ErrVersionConflict) {
		return ports.UniqueViolation{Table: "members", Constraint: memberEmailKey, Rejection: memberEmailTaken}
	}
	return err
}

type leakingRepository struct {
	inner ports.Repository[string, member]
}

func (r leakingRepository) Load(ctx context.Context, id string) (member, ports.Version, error) {
	return r.inner.Load(ctx, id)
}

func (r leakingRepository) Save(ctx context.Context, id string, state member, expected ports.Version) error {
	err := r.inner.Save(ctx, id, state, expected)
	if errors.Is(err, ports.ErrUniqueViolation) {
		return errors.Join(err, errors.New("duplicate "+state.Email))
	}
	return err
}

// unguardedConcurrentMembers runs callbacks at once over one map with no
// unique check, the realization whose missing index the concurrent clause
// exists to catch.
func unguardedConcurrentMembers() providerkit.UniqueSubject[string, member] {
	s := memoryUniqueOver(declaredMembers)()
	rows := &unguardedRows{rows: map[string]member{}}
	s.Within = func(ctx context.Context, fn func(ctx context.Context, w providerkit.UniqueWrites[string, member]) error) error {
		return fn(ctx, providerkit.UniqueWrites[string, member]{Repo: rows})
	}
	s.Reader = rows
	s.Concurrent = true
	return s
}

type unguardedRows struct {
	mu   sync.Mutex
	rows map[string]member
}

func (r *unguardedRows) Load(_ context.Context, id string) (member, ports.Version, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.rows[id]
	if !ok {
		return member{}, 0, ports.ErrNotFound
	}
	return m, 1, nil
}

func (r *unguardedRows) Save(_ context.Context, id string, state member, _ ports.Version) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[id] = state
	return nil
}
