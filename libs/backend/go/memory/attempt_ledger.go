package memory

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type attemptKey struct {
	tenant ports.TenantID
	key    ports.OutboundKey
}

type attemptConflict struct {
	class   ports.ExternalClass
	finding ports.Finding
	at      ports.Instant
}

type attemptRow struct {
	messageID   ports.MessageID
	startedAt   ports.Instant
	finished    bool
	finishedAt  ports.Instant
	class       ports.ExternalClass
	observation ports.Observation
	finding     ports.Finding
	observedAt  ports.Instant
	seen        []string
	queries     int
	conflicts   []attemptConflict
}

func cloneAttempts(src map[attemptKey][]attemptRow) map[attemptKey][]attemptRow {
	out := make(map[attemptKey][]attemptRow, len(src))
	for key, rows := range src {
		copied := make([]attemptRow, len(rows))
		for i, row := range rows {
			row.seen = slices.Clone(row.seen)
			row.conflicts = slices.Clone(row.conflicts)
			copied[i] = row
		}
		out[key] = copied
	}
	return out
}

// AttemptLedger is the ledger of the open transaction; Store.txMu serializes
// the whole callback, so Begin needs no lock of its own to serialize on a key.
func (t *Tx) AttemptLedger() ports.AttemptLedger {
	return txAttemptLedger{tx: t}
}

type txAttemptLedger struct{ tx *Tx }

var _ ports.AttemptLedger = txAttemptLedger{}

func (l txAttemptLedger) scope(ctx context.Context, key ports.OutboundKey) (attemptKey, error) {
	scoped, err := scopedKey(ctx, key)
	if err != nil {
		return attemptKey{}, err
	}
	return attemptKey{tenant: scoped.tenant, key: key}, nil
}

func (l txAttemptLedger) Begin(ctx context.Context, key ports.OutboundKey, message ports.MessageID, now ports.Instant) (ports.AttemptStart, error) {
	switch {
	case key == "":
		return ports.AttemptStart{}, fmt.Errorf("memory: begin: %w: empty key", ports.ErrInvalidAttempt)
	case message == "":
		return ports.AttemptStart{}, fmt.Errorf("memory: begin: %w: empty message ID", ports.ErrInvalidAttempt)
	}
	scope, err := l.scope(ctx, key)
	if err != nil {
		return ports.AttemptStart{}, err
	}
	rows := l.tx.attempts[scope]
	for i := len(rows) - 1; i >= 0; i-- {
		if done := rows[i]; done.finished && done.messageID == message && !done.class.Retryable() {
			return ports.DoneAttempt(attemptOf(scope, i+1, done), done.class), nil
		}
	}
	if n := len(rows); n > 0 && !rows[n-1].finished {
		return ports.PendingAttempt(attemptOf(scope, n, rows[n-1])), nil
	}
	row := attemptRow{messageID: message, startedAt: now}
	l.tx.attempts[scope] = append(rows, row)
	return ports.StartedAttempt(attemptOf(scope, len(rows)+1, row)), nil
}

func (l txAttemptLedger) Finish(ctx context.Context, o ports.AttemptOutcome) (ports.Finished, error) {
	if err := validOutcome(o); err != nil {
		return ports.Finished{}, err
	}
	scope, err := l.scope(ctx, o.Key)
	if err != nil {
		return ports.Finished{}, err
	}
	rows := l.tx.attempts[scope]
	if o.N > len(rows) {
		return ports.Finished{}, fmt.Errorf("memory: finish attempt %d: %w", o.N, ports.ErrAttemptUnknown)
	}
	row := &rows[o.N-1]
	switch {
	case !row.finished:
		row.finished, row.finishedAt, row.class = true, o.FinishedAt, o.Class
		if o.Observation.ID != "" || o.Observation.Finding != 0 {
			row.observation, row.finding, row.observedAt = o.Observation, o.Observation.Finding, o.FinishedAt
		}
		if o.Observation.ID != "" {
			row.seen = append(row.seen, o.Observation.ID)
		}
		return ports.Finished{Result: ports.FinishRecorded, Recorded: o.Class}, nil
	case row.class == o.Class || o.Presumed:
		return ports.Finished{Result: ports.FinishUnchanged, Recorded: row.class}, nil
	default:
		row.conflicts = append(row.conflicts, attemptConflict{class: o.Class, at: o.FinishedAt})
		return ports.Finished{Result: ports.FinishConflict, Recorded: row.class}, nil
	}
}

func (l txAttemptLedger) Observe(ctx context.Context, o ports.Observation, now ports.Instant) (ports.Observed, error) {
	if err := validObservation(o); err != nil {
		return ports.Observed{}, err
	}
	scope, err := l.scope(ctx, o.Key)
	if err != nil {
		return ports.Observed{}, err
	}
	rows := l.tx.attempts[scope]
	if len(rows) == 0 {
		return ports.Observed{}, fmt.Errorf("memory: observe: %w", ports.ErrAttemptUnknown)
	}
	last := &rows[len(rows)-1]
	if !last.finished {
		return ports.Observed{}, fmt.Errorf("memory: observe attempt %d: %w", len(rows), ports.ErrAttemptInFlight)
	}

	observed := ports.Observed{N: len(rows), Class: last.class, QueriesDone: last.queries}
	if slices.Contains(last.seen, o.ID) {
		observed.Result = ports.ObserveDuplicate
		return observed, nil
	}
	last.seen = append(last.seen, o.ID)
	if terminal(last.finding) {
		if !terminal(o.Finding) || o.Finding == last.finding {
			observed.Result = ports.ObserveDuplicate
			return observed, nil
		}
		last.conflicts = append(last.conflicts, attemptConflict{finding: o.Finding, at: now})
		observed.Result = ports.ObserveConflict
		return observed, nil
	}
	last.observation, last.finding, last.observedAt = o, o.Finding, now
	last.queries++
	observed.Result, observed.QueriesDone = ports.ObserveRecorded, last.queries
	return observed, nil
}

// Stale reads no tenant from ctx: the ledger is a platform table with the tenant
// in its key (ADR-050, ADR-070), and the tenant goes back on each Attempt.
func (l txAttemptLedger) Stale(_ context.Context, cutoff ports.Instant, after ports.AttemptCursor, limit int) ([]ports.Attempt, error) {
	if limit < 1 {
		return nil, fmt.Errorf("memory: stale limit %d: %w", limit, ports.ErrInvalidAttempt)
	}
	stale := []ports.Attempt{}
	for scope, rows := range l.tx.attempts {
		for i, row := range rows {
			attempt := attemptOf(scope, i+1, row)
			if !row.finished && row.startedAt <= cutoff && (after == ports.AttemptCursor{} || compareCursor(attempt.Cursor(), after) > 0) {
				stale = append(stale, attempt)
			}
		}
	}
	slices.SortFunc(stale, func(a, b ports.Attempt) int { return compareCursor(a.Cursor(), b.Cursor()) })
	return stale[:min(limit, len(stale))], nil
}

func compareCursor(a, b ports.AttemptCursor) int {
	return cmp.Or(
		cmp.Compare(a.StartedAt, b.StartedAt),
		cmp.Compare(a.Tenant, b.Tenant),
		cmp.Compare(a.Key, b.Key),
		cmp.Compare(a.N, b.N),
	)
}

func attemptOf(scope attemptKey, n int, row attemptRow) ports.Attempt {
	return ports.Attempt{Tenant: scope.tenant, Key: scope.key, N: n, MessageID: row.messageID, StartedAt: row.startedAt}
}

func terminal(f ports.Finding) bool {
	return f == ports.FindingApplied || f == ports.FindingRejected
}

func validClass(c ports.ExternalClass) bool {
	return c >= ports.ClassAccepted && c <= ports.ClassSchemaMismatch
}

func validFinding(f ports.Finding) bool {
	return f >= ports.FindingApplied && f <= ports.FindingInconclusive
}

func validOutcome(o ports.AttemptOutcome) error {
	switch {
	case o.Key == "":
		return fmt.Errorf("memory: finish: %w: empty key", ports.ErrInvalidAttempt)
	case o.N < 1:
		return fmt.Errorf("memory: finish: %w: attempt %d", ports.ErrInvalidAttempt, o.N)
	case !validClass(o.Class):
		return fmt.Errorf("memory: finish: %w: class %d", ports.ErrInvalidAttempt, o.Class)
	case o.Observation.Key != "" && o.Observation.Key != o.Key:
		return fmt.Errorf("memory: finish: %w: observation of another key", ports.ErrInvalidAttempt)
	case o.Observation.Finding != 0 && !validFinding(o.Observation.Finding):
		return fmt.Errorf("memory: finish: %w: finding %d", ports.ErrInvalidAttempt, o.Observation.Finding)
	}
	return nil
}

func validObservation(o ports.Observation) error {
	switch {
	case o.Key == "":
		return fmt.Errorf("memory: observe: %w: empty key", ports.ErrInvalidAttempt)
	case o.ID == "":
		return fmt.Errorf("memory: observe: %w: empty observation ID", ports.ErrInvalidAttempt)
	case !validFinding(o.Finding):
		return fmt.Errorf("memory: observe: %w: finding %d", ports.ErrInvalidAttempt, o.Finding)
	}
	return nil
}

// AttemptStore runs each ledger call in a transaction of its own, the one Begin
// needs before the partner is called outside any transaction (OBX-07, OBX-16).
type AttemptStore struct{ store *Store }

func NewAttemptStore(store *Store) AttemptStore { return AttemptStore{store: store} }

var _ ports.AttemptLedger = AttemptStore{}

func (s AttemptStore) within(ctx context.Context, fn func(context.Context, ports.AttemptLedger) error) error {
	if s.store == nil {
		return ErrIncompleteAttemptStore
	}
	return NewUnitOfWork(s.store, (*Tx).AttemptLedger).Within(ctx, fn)
}

func (s AttemptStore) Begin(ctx context.Context, key ports.OutboundKey, message ports.MessageID, now ports.Instant) (ports.AttemptStart, error) {
	var start ports.AttemptStart
	err := s.within(ctx, func(ctx context.Context, l ports.AttemptLedger) (err error) {
		start, err = l.Begin(ctx, key, message, now)
		return err
	})
	return start, err
}

func (s AttemptStore) Finish(ctx context.Context, o ports.AttemptOutcome) (ports.Finished, error) {
	var finished ports.Finished
	err := s.within(ctx, func(ctx context.Context, l ports.AttemptLedger) (err error) {
		finished, err = l.Finish(ctx, o)
		return err
	})
	return finished, err
}

func (s AttemptStore) Observe(ctx context.Context, o ports.Observation, now ports.Instant) (ports.Observed, error) {
	var observed ports.Observed
	err := s.within(ctx, func(ctx context.Context, l ports.AttemptLedger) (err error) {
		observed, err = l.Observe(ctx, o, now)
		return err
	})
	return observed, err
}

func (s AttemptStore) Stale(ctx context.Context, cutoff ports.Instant, after ports.AttemptCursor, limit int) ([]ports.Attempt, error) {
	var stale []ports.Attempt
	err := s.within(ctx, func(ctx context.Context, l ports.AttemptLedger) (err error) {
		stale, err = l.Stale(ctx, cutoff, after, limit)
		return err
	})
	return stale, err
}
