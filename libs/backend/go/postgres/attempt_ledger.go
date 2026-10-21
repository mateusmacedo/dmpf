package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	selectDoneAttempt = `
SELECT attempt, started_at, class
  FROM external_attempt
 WHERE tenant_id = $1 AND outbound_key = $2 AND message_id = $3
   AND state = 'finished' AND class <> ALL ($4::smallint[])
 ORDER BY attempt DESC LIMIT 1`

	selectLastAttempt = `
SELECT attempt, message_id, state, started_at
  FROM external_attempt
 WHERE tenant_id = $1 AND outbound_key = $2
 ORDER BY attempt DESC LIMIT 1`

	insertAttempt = `
INSERT INTO external_attempt (tenant_id, outbound_key, attempt, message_id, state, started_at)
VALUES ($1, $2, $3, $4, 'begun', $5)
ON CONFLICT (tenant_id, outbound_key, attempt) DO NOTHING`

	finishBegunAttempt = `
UPDATE external_attempt
   SET state = 'finished', finished_at = $4, class = $5, finding = $6, terminal_finding = $7,
       observation_ids = CASE WHEN $8::text = '' THEN observation_ids ELSE array_append(observation_ids, $8::text) END,
       observation_id = $9, provider = $10, external_ref = $11, source_at = $12
 WHERE tenant_id = $1 AND outbound_key = $2 AND attempt = $3 AND state = 'begun'`

	lockAttemptClass = `
SELECT class, cardinality(observation_ids), jsonb_array_length(conflicts) FROM external_attempt
 WHERE tenant_id = $1 AND outbound_key = $2 AND attempt = $3
   FOR UPDATE`

	appendAttemptConflict = `
UPDATE external_attempt
   SET conflicts = conflicts || jsonb_build_array(jsonb_build_object('class', $4::smallint, 'finding', $5::smallint, 'at', $6::bigint))
 WHERE tenant_id = $1 AND outbound_key = $2 AND attempt = $3`

	lockLastAttempt = `
SELECT attempt, state, class, terminal_finding, observation_ids, queries, jsonb_array_length(conflicts)
  FROM external_attempt
 WHERE tenant_id = $1 AND outbound_key = $2
 ORDER BY attempt DESC LIMIT 1
   FOR UPDATE`

	recordObservationID = `
UPDATE external_attempt SET observation_ids = array_append(observation_ids, $4::text)
 WHERE tenant_id = $1 AND outbound_key = $2 AND attempt = $3`

	recordObservation = `
UPDATE external_attempt
   SET finding = $4, terminal_finding = $5, queries = queries + 1,
       observation_ids = CASE WHEN $10::boolean THEN observation_ids ELSE array_append(observation_ids, $6::text) END,
       observation_id = $6::text, provider = $7, external_ref = $8, source_at = $9
 WHERE tenant_id = $1 AND outbound_key = $2 AND attempt = $3
RETURNING queries`

	selectStaleAttempts = `
SELECT tenant_id, outbound_key, attempt, message_id, started_at
  FROM external_attempt
 WHERE state = 'begun' AND started_at <= $1
   AND ($2::boolean OR (started_at, tenant_id COLLATE "C", outbound_key COLLATE "C", attempt)
                     > ($3::bigint, $4::text COLLATE "C", $5::text COLLATE "C", $6::integer))
 ORDER BY started_at, tenant_id COLLATE "C", outbound_key COLLATE "C", attempt
 LIMIT $7`
)

var (
	_ = declare(selectDoneAttempt, "SELECT", "external_attempt")
	_ = declare(selectLastAttempt, "SELECT", "external_attempt")
	_ = declare(insertAttempt, "INSERT", "external_attempt")
	_ = declare(finishBegunAttempt, "UPDATE", "external_attempt")
	_ = declare(lockAttemptClass, "SELECT", "external_attempt")
	_ = declare(appendAttemptConflict, "UPDATE", "external_attempt")
	_ = declare(lockLastAttempt, "SELECT", "external_attempt")
	_ = declare(recordObservationID, "UPDATE", "external_attempt")
	_ = declare(recordObservation, "UPDATE", "external_attempt")
	_ = declare(selectStaleAttempts, "SELECT", "external_attempt")
)

// beginRounds bounds the re-reads after a lost insert race: each loss means a
// concurrent Begin committed the next attempt, which the re-read then finds begun.
const beginRounds = 8

const stateBegun = "begun"

// AttemptLedger is the ledger of this open transaction. Begin serializes on the
// key through the primary key: a concurrent insert of the same attempt waits on
// it and then does nothing, and the re-read sees the winner (INB-06).
func (t *Tx) AttemptLedger() ports.AttemptLedger {
	return txAttemptLedger{tx: t}
}

type txAttemptLedger struct{ tx *Tx }

var _ ports.AttemptLedger = txAttemptLedger{}

type lastAttempt struct {
	n         int
	messageID string
	state     string
	startedAt int64
}

func retryableClasses() []int16 {
	var retryable []int16
	for c := ports.ClassAccepted; c <= ports.ClassSchemaMismatch; c++ {
		if c.Retryable() {
			retryable = append(retryable, int16(c))
		}
	}
	return retryable
}

func (l txAttemptLedger) Begin(ctx context.Context, key ports.OutboundKey, message ports.MessageID, now ports.Instant) (ports.AttemptStart, error) {
	switch {
	case key == "":
		return ports.AttemptStart{}, fmt.Errorf("postgres: begin: %w: empty key", ports.ErrInvalidAttempt)
	case message == "":
		return ports.AttemptStart{}, fmt.Errorf("postgres: begin: %w: empty message ID", ports.ErrInvalidAttempt)
	}
	tenant, err := tenantOf(ctx)
	if err != nil {
		return ports.AttemptStart{}, err
	}
	for range beginRounds {
		start, settled, err := l.beginRound(ctx, tenant, key, message, now)
		if err != nil || settled {
			return start, err
		}
	}
	return ports.AttemptStart{}, fmt.Errorf("postgres: begin: lost %d insert races in a row", beginRounds)
}

func (l txAttemptLedger) beginRound(ctx context.Context, tenant ports.TenantID, key ports.OutboundKey, message ports.MessageID, now ports.Instant) (ports.AttemptStart, bool, error) {
	if done, ok, err := l.doneAttempt(ctx, tenant, key, message); err != nil || ok {
		return done, ok, err
	}
	var last lastAttempt
	err := l.tx.conn.QueryRow(ctx, selectLastAttempt, string(tenant), string(key)).
		Scan(&last.n, &last.messageID, &last.state, &last.startedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return ports.AttemptStart{}, false, err
	case last.state == stateBegun:
		attempt := ports.Attempt{Tenant: tenant, Key: key, N: last.n, MessageID: ports.MessageID(last.messageID), StartedAt: ports.Instant(last.startedAt)}
		return ports.PendingAttempt(attempt), true, nil
	}

	next := last.n + 1
	tag, err := l.tx.conn.Exec(ctx, insertAttempt, string(tenant), string(key), next, string(message), int64(now))
	if err != nil || tag.RowsAffected() == 0 {
		return ports.AttemptStart{}, false, err
	}
	return ports.StartedAttempt(ports.Attempt{Tenant: tenant, Key: key, N: next, MessageID: message, StartedAt: now}), true, nil
}

func (l txAttemptLedger) doneAttempt(ctx context.Context, tenant ports.TenantID, key ports.OutboundKey, message ports.MessageID) (ports.AttemptStart, bool, error) {
	var (
		n         int
		startedAt int64
		class     *int16
	)
	err := l.tx.conn.QueryRow(ctx, selectDoneAttempt, string(tenant), string(key), string(message), retryableClasses()).
		Scan(&n, &startedAt, &class)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ports.AttemptStart{}, false, nil
	case err != nil:
		return ports.AttemptStart{}, false, err
	}
	attempt := ports.Attempt{Tenant: tenant, Key: key, N: n, MessageID: message, StartedAt: ports.Instant(startedAt)}
	return ports.DoneAttempt(attempt, classOf(class)), true, nil
}

func (l txAttemptLedger) Finish(ctx context.Context, o ports.AttemptOutcome) (ports.Finished, error) {
	if err := validOutcome(o); err != nil {
		return ports.Finished{}, err
	}
	tenant, err := tenantOf(ctx)
	if err != nil {
		return ports.Finished{}, err
	}
	finding := o.Observation.Finding
	args := []any{string(tenant), string(o.Key), o.N, int64(o.FinishedAt), int16(o.Class), nullableFinding(finding), terminalOf(finding), o.Observation.ID}
	tag, err := l.tx.conn.Exec(ctx, finishBegunAttempt, append(args, observationColumns(o.Observation)...)...)
	if err != nil {
		return ports.Finished{}, err
	}
	if tag.RowsAffected() == 1 {
		return ports.Finished{Result: ports.FinishRecorded, Recorded: o.Class}, nil
	}

	var (
		stored          *int16
		seen, conflicts int
	)
	err = l.tx.conn.QueryRow(ctx, lockAttemptClass, string(tenant), string(o.Key), o.N).Scan(&stored, &seen, &conflicts)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ports.Finished{}, fmt.Errorf("postgres: finish attempt %d: %w", o.N, ports.ErrAttemptUnknown)
	case err != nil:
		return ports.Finished{}, err
	}
	recorded := classOf(stored)
	if recorded == o.Class || o.Presumed {
		return ports.Finished{Result: ports.FinishUnchanged, Recorded: recorded}, nil
	}
	if saturated(seen, conflicts) {
		return ports.Finished{Result: ports.FinishSaturated, Recorded: recorded}, nil
	}
	if _, err := l.tx.conn.Exec(ctx, appendAttemptConflict, string(tenant), string(o.Key), o.N, int16(o.Class), nil, int64(o.FinishedAt)); err != nil {
		return ports.Finished{}, err
	}
	return ports.Finished{Result: ports.FinishConflict, Recorded: recorded}, nil
}

type lockedAttempt struct {
	n         int
	state     string
	class     *int16
	terminal  *int16
	seen      []string
	queries   int
	conflicts int
}

func saturated(seen, conflicts int) bool {
	return seen >= ports.MaxAttemptEntries || conflicts >= ports.MaxAttemptEntries
}

func (l txAttemptLedger) Observe(ctx context.Context, o ports.Observation, now ports.Instant) (ports.Observed, error) {
	if err := validObservation(o); err != nil {
		return ports.Observed{}, err
	}
	tenant, err := tenantOf(ctx)
	if err != nil {
		return ports.Observed{}, err
	}
	var last lockedAttempt
	err = l.tx.conn.QueryRow(ctx, lockLastAttempt, string(tenant), string(o.Key)).
		Scan(&last.n, &last.state, &last.class, &last.terminal, &last.seen, &last.queries, &last.conflicts)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ports.Observed{}, fmt.Errorf("postgres: observe: %w", ports.ErrAttemptUnknown)
	case err != nil:
		return ports.Observed{}, err
	case last.state == stateBegun:
		return ports.Observed{}, fmt.Errorf("postgres: observe attempt %d: %w", last.n, ports.ErrAttemptInFlight)
	}

	observed := ports.Observed{N: last.n, Class: classOf(last.class), QueriesDone: last.queries}
	if slices.Contains(last.seen, o.ID) {
		observed.Result = ports.ObserveDuplicate
		return observed, nil
	}
	full := saturated(len(last.seen), last.conflicts)
	switch {
	case full && last.terminal != nil && ports.Finding(*last.terminal) == o.Finding:
		observed.Result = ports.ObserveDuplicate
		return observed, nil
	case full && (last.terminal != nil || terminalOf(o.Finding) == nil):
		observed.Result = ports.ObserveSaturated
		return observed, nil
	case last.terminal != nil:
		return l.observeAfterTerminal(ctx, tenant, o, now, last, observed)
	}
	err = l.tx.conn.QueryRow(ctx, recordObservation, string(tenant), string(o.Key), last.n,
		int16(o.Finding), terminalOf(o.Finding), o.ID, o.Provider, o.ExternalRef, int64(o.SourceAt), full).Scan(&observed.QueriesDone)
	if err != nil {
		return ports.Observed{}, err
	}
	observed.Result = ports.ObserveRecorded
	return observed, nil
}

func (l txAttemptLedger) observeAfterTerminal(ctx context.Context, tenant ports.TenantID, o ports.Observation, now ports.Instant, last lockedAttempt, observed ports.Observed) (ports.Observed, error) {
	if _, err := l.tx.conn.Exec(ctx, recordObservationID, string(tenant), string(o.Key), last.n, o.ID); err != nil {
		return ports.Observed{}, err
	}
	if terminalOf(o.Finding) == nil || ports.Finding(*last.terminal) == o.Finding {
		observed.Result = ports.ObserveDuplicate
		return observed, nil
	}
	if _, err := l.tx.conn.Exec(ctx, appendAttemptConflict, string(tenant), string(o.Key), last.n, nil, int16(o.Finding), int64(now)); err != nil {
		return ports.Observed{}, err
	}
	observed.Result = ports.ObserveConflict
	return observed, nil
}

// Stale reads no tenant from ctx: the ledger is a platform table with the tenant
// in its key (ADR-050, ADR-070). COLLATE "C" orders as the memory double does.
func (l txAttemptLedger) Stale(ctx context.Context, cutoff ports.Instant, after ports.AttemptCursor, limit int) ([]ports.Attempt, error) {
	if limit < 1 {
		return nil, fmt.Errorf("postgres: stale limit %d: %w", limit, ports.ErrInvalidAttempt)
	}
	rows, err := l.tx.conn.Query(ctx, selectStaleAttempts, int64(cutoff), after == ports.AttemptCursor{},
		int64(after.StartedAt), string(after.Tenant), string(after.Key), after.N, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stale := []ports.Attempt{}
	for rows.Next() {
		var (
			tenant, key, message string
			n                    int
			startedAt            int64
		)
		if err := rows.Scan(&tenant, &key, &n, &message, &startedAt); err != nil {
			return nil, err
		}
		stale = append(stale, ports.Attempt{
			Tenant: ports.TenantID(tenant), Key: ports.OutboundKey(key), N: n,
			MessageID: ports.MessageID(message), StartedAt: ports.Instant(startedAt),
		})
	}
	return stale, rows.Err()
}

// observationColumns keeps the row free of an observation Submit did not carry,
// the same test the memory double applies.
func observationColumns(o ports.Observation) []any {
	if o.ID == "" && o.Finding == 0 {
		return []any{nil, nil, nil, nil}
	}
	return []any{o.ID, o.Provider, o.ExternalRef, int64(o.SourceAt)}
}

func classOf(stored *int16) ports.ExternalClass {
	if stored == nil {
		return 0
	}
	return ports.ExternalClass(*stored)
}

func nullableFinding(f ports.Finding) *int16 {
	if f == 0 {
		return nil
	}
	v := int16(f)
	return &v
}

func terminalOf(f ports.Finding) *int16 {
	if f != ports.FindingApplied && f != ports.FindingRejected {
		return nil
	}
	return nullableFinding(f)
}

func validOutcome(o ports.AttemptOutcome) error {
	switch {
	case o.Key == "":
		return fmt.Errorf("postgres: finish: %w: empty key", ports.ErrInvalidAttempt)
	case o.N < 1:
		return fmt.Errorf("postgres: finish: %w: attempt %d", ports.ErrInvalidAttempt, o.N)
	case o.Class < ports.ClassAccepted || o.Class > ports.ClassSchemaMismatch:
		return fmt.Errorf("postgres: finish: %w: class %d", ports.ErrInvalidAttempt, o.Class)
	case o.Observation.Key != "" && o.Observation.Key != o.Key:
		return fmt.Errorf("postgres: finish: %w: observation of another key", ports.ErrInvalidAttempt)
	case o.Observation.Finding > ports.FindingInconclusive:
		return fmt.Errorf("postgres: finish: %w: finding %d", ports.ErrInvalidAttempt, o.Observation.Finding)
	}
	return nil
}

func validObservation(o ports.Observation) error {
	switch {
	case o.Key == "":
		return fmt.Errorf("postgres: observe: %w: empty key", ports.ErrInvalidAttempt)
	case o.ID == "":
		return fmt.Errorf("postgres: observe: %w: empty observation ID", ports.ErrInvalidAttempt)
	case o.Finding < ports.FindingApplied || o.Finding > ports.FindingInconclusive:
		return fmt.Errorf("postgres: observe: %w: finding %d", ports.ErrInvalidAttempt, o.Finding)
	}
	return nil
}

// AttemptStore runs each ledger call in a transaction of its own, the one Begin
// needs before the partner is called outside any transaction (OBX-07, OBX-16).
type AttemptStore struct{ pool *pgxpool.Pool }

func NewAttemptStore(pool *pgxpool.Pool) AttemptStore { return AttemptStore{pool: pool} }

var _ ports.AttemptLedger = AttemptStore{}

func (s AttemptStore) within(ctx context.Context, fn func(context.Context, ports.AttemptLedger) error) error {
	if s.pool == nil {
		return ErrIncompleteAttemptStore
	}
	return NewUnitOfWork(s.pool, (*Tx).AttemptLedger).Within(ctx, fn)
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
