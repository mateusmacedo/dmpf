//go:build integration

package dmpfpostgres_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
)

// claimEligibility mirrors the predicate Claim runs, so the applicability proof
// below cannot drift from the statement it is meant to cover.
const claimEligibility = `
SELECT id FROM dmpf_outbox
WHERE available_at <= $1
  AND ( status = 'pending'
     OR ( status = 'publishing'
          AND (locked_until IS NULL OR locked_until <= $1) ) )
  AND jsonb_typeof(metadata -> 'correlationid') = 'string' AND metadata ->> 'correlationid' <> ''
  AND jsonb_typeof(metadata -> 'causationid') = 'string' AND metadata ->> 'causationid' <> ''
  AND jsonb_typeof(metadata -> 'traceparent') = 'string' AND metadata ->> 'traceparent' <> ''
ORDER BY available_at, id
LIMIT $2
FOR UPDATE SKIP LOCKED`

// row is the outbox row these tests write straight through SQL: the relay reads
// rows the writer already froze (OBX-14), so building them by hand is closer to
// what Claim actually sees than replaying Enqueue.
type row struct {
	messageID   string
	occurredAt  int64
	availableAt int64
	status      string
	lockedBy    *string
	lockedUntil *int64
	attempts    int
	metadata    string
	version     int64
}

func defaultRow(messageID string) row {
	return row{
		messageID:   messageID,
		occurredAt:  1_000,
		availableAt: 1_000,
		status:      "pending",
		metadata:    fullMetadata,
		version:     7,
	}
}

const fullMetadata = `{"correlationid":"corr-1","causationid":"caus-1","traceparent":"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"}`

const insertRow = `
INSERT INTO dmpf_outbox (
	message_id, message_type, schema_version,
	aggregate_type, aggregate_id, aggregate_version,
	partition_key, destination,
	payload, payload_hash, metadata,
	occurred_at, available_at, attempt_count, status, locked_by, locked_until
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING id`

func insert(t *testing.T, ctx context.Context, pool *pgxpool.Pool, r row) int64 {
	t.Helper()

	var id int64
	err := pool.QueryRow(ctx, insertRow,
		r.messageID, "com.company.orders.order-placed.v1", "type.googleapis.com/company.orders.event.v1.OrderPlaced",
		"order", "o-"+r.messageID, r.version,
		"pk-1", "orders.integration",
		[]byte{0x0a, 0x03, 0x6f, 0x2d, 0x31}, "sha-256:stub", r.metadata,
		r.occurredAt, r.availableAt, r.attempts, r.status, r.lockedBy, r.lockedUntil,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert(%s) = %v, want nil", r.messageID, err)
	}
	return id
}

func TestClaimIndexIsApplicableToTheEligibilityPredicate(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	for i := range 8 {
		insert(t, ctx, pool, defaultRow(fmt.Sprintf("m-%d", i)))
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v, want nil", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// enable_seqscan = off proves the index *can* serve the predicate; on a
	// table this small the planner would pick a sequential scan either way, and
	// that choice says nothing about applicability.
	if _, err := tx.Exec(ctx, "SET LOCAL enable_seqscan = off"); err != nil {
		t.Fatalf("SET LOCAL enable_seqscan = %v, want nil", err)
	}

	plan := explain(t, ctx, tx, claimEligibility, int64(2_000), 10)
	if !strings.Contains(plan, "dmpf_outbox_claim_idx") {
		t.Fatalf("plan does not use dmpf_outbox_claim_idx:\n%s", plan)
	}
}

func explain(t *testing.T, ctx context.Context, tx pgx.Tx, query string, args ...any) string {
	t.Helper()

	rows, err := tx.Query(ctx, "EXPLAIN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN = %v, want nil", err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan plan line = %v, want nil", err)
		}
		plan.WriteString(line)
		plan.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows = %v, want nil", err)
	}
	return plan.String()
}

const (
	testLease = 5 * time.Minute
	claimA    = "claim-a"
	claimB    = "claim-b"
)

// systemClock is the real clock: the lease tests below need instants that
// actually move while a connection is held, which a frozen clock cannot show.
type systemClock struct{}

func (systemClock) Now() dmpfports.Instant { return dmpfports.Instant(time.Now().UnixNano()) }

// fixedClock pins the instant so eligibility cases state their own deadlines
// instead of racing the wall clock.
type fixedClock dmpfports.Instant

func (c fixedClock) Now() dmpfports.Instant { return dmpfports.Instant(c) }

func ptr[T any](v T) *T { return &v }

func TestClaimEligibility(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	now := dmpfports.Instant(10_000)

	cases := []struct {
		name string
		row  func(r row) row
		want bool
	}{
		{"pending due", func(r row) row { return r }, true},
		{"pending not due yet", func(r row) row { r.availableAt = int64(now) + 1; return r }, false},
		{"pending due exactly now", func(r row) row { r.availableAt = int64(now); return r }, true},
		{"publishing with an expired lease", func(r row) row {
			r.status, r.lockedBy, r.lockedUntil = "publishing", ptr(claimA), ptr(int64(now)-1)
			return r
		}, true},
		{"publishing with a live lease", func(r row) row {
			r.status, r.lockedBy, r.lockedUntil = "publishing", ptr(claimA), ptr(int64(now)+1)
			return r
		}, false},
		// OBX-18 releases the lease by writing NULL; without this case the
		// predicate would strand the row in publishing forever.
		{"publishing with no lease at all", func(r row) row {
			r.status, r.lockedBy = "publishing", ptr(claimA)
			return r
		}, true},
		{"published", func(r row) row { r.status = "published"; return r }, false},
		{"failed", func(r row) row { r.status = "failed"; return r }, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			truncate(t, ctx, pool)
			id := insert(t, ctx, pool, c.row(defaultRow("m-1")))

			store := dmpfpostgres.NewOutboxStore(pool, fixedClock(now))
			claimed, err := store.Claim(ctx, claimB, 10, testLease)
			if err != nil {
				t.Fatalf("Claim() = %v, want nil", err)
			}

			got := len(claimed) == 1 && claimed[0].ID == id
			if got != c.want {
				t.Fatalf("claimed = %v (%d rows), want %v", got, len(claimed), c.want)
			}
		})
	}
}

func TestClaimWritesTheFourFieldsInOneCommit(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	now := dmpfports.Instant(10_000)

	id := insert(t, ctx, pool, defaultRow("m-1"))

	store := dmpfpostgres.NewOutboxStore(pool, fixedClock(now))
	claimed, err := store.Claim(ctx, claimA, 10, testLease)
	if err != nil {
		t.Fatalf("Claim() = %v, want nil", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("Claim() returned %d rows, want 1", len(claimed))
	}

	var (
		status      string
		lockedBy    *string
		lockedUntil *int64
		attempts    int
	)
	err = pool.QueryRow(ctx, `SELECT status, locked_by, locked_until, attempt_count FROM dmpf_outbox WHERE id = $1`, id).
		Scan(&status, &lockedBy, &lockedUntil, &attempts)
	if err != nil {
		t.Fatalf("read back = %v, want nil", err)
	}

	if status != "publishing" {
		t.Errorf("status = %q, want publishing", status)
	}
	if lockedBy == nil || *lockedBy != claimA {
		t.Errorf("locked_by = %v, want %q", lockedBy, claimA)
	}
	if lockedUntil == nil || *lockedUntil != int64(now)+int64(testLease) {
		t.Errorf("locked_until = %v, want %d", lockedUntil, int64(now)+int64(testLease))
	}
	if attempts != 1 {
		t.Errorf("attempt_count = %d, want 1", attempts)
	}
	if claimed[0].LockedBy != claimA {
		t.Errorf("Claimed.LockedBy = %q, want %q", claimed[0].LockedBy, claimA)
	}
}

func TestClaimNeverLeavesPublishingWithoutALease(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	for i := range 5 {
		insert(t, ctx, pool, defaultRow(fmt.Sprintf("m-%d", i)))
	}

	store := dmpfpostgres.NewOutboxStore(pool, fixedClock(10_000))
	if _, err := store.Claim(ctx, claimA, 5, testLease); err != nil {
		t.Fatalf("Claim() = %v, want nil", err)
	}

	var orphans int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM dmpf_outbox WHERE status = 'publishing' AND locked_until IS NULL`).Scan(&orphans)
	if err != nil {
		t.Fatalf("count orphans = %v, want nil", err)
	}
	if orphans != 0 {
		t.Fatalf("%d rows are publishing without a lease, want 0", orphans)
	}
}

func TestReclaimingTheSameRecordProducesADifferentClaimIdentity(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	now := dmpfports.Instant(10_000)

	insert(t, ctx, pool, defaultRow("m-1"))
	store := dmpfpostgres.NewOutboxStore(pool, fixedClock(now))

	first, err := store.Claim(ctx, claimA, 10, testLease)
	if err != nil || len(first) != 1 {
		t.Fatalf("first Claim() = %v, %d rows; want nil, 1", err, len(first))
	}

	// The lease is what stands between the two claims; expiring it is how the
	// same worker gets a second acquisition of the same record (OBX-08).
	expired := dmpfpostgres.NewOutboxStore(pool, fixedClock(now+dmpfports.Instant(testLease)+1))
	second, err := expired.Claim(ctx, claimB, 10, testLease)
	if err != nil || len(second) != 1 {
		t.Fatalf("second Claim() = %v, %d rows; want nil, 1", err, len(second))
	}

	if second[0].LockedBy == first[0].LockedBy {
		t.Fatalf("both claims carry %q, want different identities", first[0].LockedBy)
	}
}

func TestClaimReadsTheLeaseInstantAfterAcquiringTheConnection(t *testing.T) {
	pool := openPoolWithOneConn(t)
	ctx := context.Background()

	insert(t, ctx, pool, defaultRow("m-1"))

	const (
		lease = 300 * time.Millisecond
		hold  = 600 * time.Millisecond
	)

	held, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() = %v, want nil", err)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(hold)
		held.Release()
		close(released)
	}()

	// Claim blocks on the single connection for `hold`; if it had read the
	// clock before waiting, the lease it writes would already be expired.
	store := dmpfpostgres.NewOutboxStore(pool, systemClock{})
	claimed, err := store.Claim(ctx, claimA, 10, lease)
	<-released
	if err != nil {
		t.Fatalf("Claim() = %v, want nil", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("Claim() returned %d rows, want 1", len(claimed))
	}

	var lockedUntil int64
	if err := pool.QueryRow(ctx, `SELECT locked_until FROM dmpf_outbox WHERE id = $1`, claimed[0].ID).Scan(&lockedUntil); err != nil {
		t.Fatalf("read back = %v, want nil", err)
	}
	if lockedUntil <= time.Now().UnixNano() {
		t.Fatalf("lease already expired on commit: locked_until = %d, now = %d", lockedUntil, time.Now().UnixNano())
	}
}

func TestClaimReturnsThePostIncrementAttemptCount(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	now := dmpfports.Instant(10_000)

	cases := []struct {
		name    string
		already int
		want    int
	}{
		{"first acquisition", 0, 1},
		{"second acquisition", 1, 2},
		{"nth acquisition", 41, 42},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			truncate(t, ctx, pool)
			r := defaultRow("m-1")
			r.attempts = c.already
			insert(t, ctx, pool, r)

			store := dmpfpostgres.NewOutboxStore(pool, fixedClock(now))
			claimed, err := store.Claim(ctx, claimA, 10, testLease)
			if err != nil || len(claimed) != 1 {
				t.Fatalf("Claim() = %v, %d rows; want nil, 1", err, len(claimed))
			}
			if claimed[0].AttemptCount != c.want {
				t.Fatalf("AttemptCount = %d, want %d (post-increment)", claimed[0].AttemptCount, c.want)
			}
		})
	}
}

// A record without the three context attributes cannot be turned into a valid
// envelope, so the claim leaves it alone instead of burning it down to failed:
// when KRN-09 starts writing metadata, the backlog drains on its own (D5).
func TestClaimSkipsRecordsWithoutTheContextAttributes(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	// The last four are the ones a presence check would let through: the claim
	// would acquire them, the envelope assembly would refuse them, and the
	// record would end up failed — the outcome D5 exists to prevent.
	cases := []struct {
		name     string
		metadata string
	}{
		{"empty metadata", `{}`},
		{"only correlationid", `{"correlationid":"corr-1"}`},
		{"missing traceparent", `{"correlationid":"corr-1","causationid":"caus-1"}`},
		{"correlationid present but empty", `{"correlationid":"","causationid":"caus-1","traceparent":"tp-1"}`},
		{"causationid carried as a number", `{"correlationid":"corr-1","causationid":42,"traceparent":"tp-1"}`},
		{"traceparent carried as null", `{"correlationid":"corr-1","causationid":"caus-1","traceparent":null}`},
		{"traceparent carried as an object", `{"correlationid":"corr-1","causationid":"caus-1","traceparent":{"a":1}}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			truncate(t, ctx, pool)
			r := defaultRow("m-1")
			r.metadata = c.metadata
			id := insert(t, ctx, pool, r)

			store := dmpfpostgres.NewOutboxStore(pool, fixedClock(10_000))
			claimed, err := store.Claim(ctx, claimA, 10, testLease)
			if err != nil {
				t.Fatalf("Claim() = %v, want nil", err)
			}
			if len(claimed) != 0 {
				t.Fatalf("Claim() returned %d rows, want 0", len(claimed))
			}

			var (
				status   string
				attempts int
			)
			if err := pool.QueryRow(ctx, `SELECT status, attempt_count FROM dmpf_outbox WHERE id = $1`, id).Scan(&status, &attempts); err != nil {
				t.Fatalf("read back = %v, want nil", err)
			}
			if status != "pending" || attempts != 0 {
				t.Fatalf("record was touched: status = %q, attempt_count = %d; want pending, 0", status, attempts)
			}
		})
	}
}

func openPoolWithOneConn(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool := openPool(t)
	config := pool.Config().Copy()
	config.MaxConns = 1

	single, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() = %v, want nil", err)
	}
	t.Cleanup(single.Close)
	return single
}

func claimOne(t *testing.T, ctx context.Context, pool *pgxpool.Pool, now dmpfports.Instant, claimID string) dmpfpostgres.Claimed {
	t.Helper()

	store := dmpfpostgres.NewOutboxStore(pool, fixedClock(now))
	claimed, err := store.Claim(ctx, claimID, 10, testLease)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("Claim(%s) = %v, %d rows; want nil, 1", claimID, err, len(claimed))
	}
	return claimed[0]
}

func TestTransitionsRequireTheCurrentClaim(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	now := dmpfports.Instant(10_000)
	store := dmpfpostgres.NewOutboxStore(pool, fixedClock(now))

	cases := []struct {
		name       string
		apply      func(id int64, claimID string) (int64, error)
		wantStatus string
	}{
		{"MarkPublished", func(id int64, claimID string) (int64, error) {
			return store.MarkPublished(ctx, id, claimID)
		}, "published"},
		{"Reschedule", func(id int64, claimID string) (int64, error) {
			return store.Reschedule(ctx, id, claimID, now+500, "transient")
		}, "publishing"},
		{"Fail", func(id int64, claimID string) (int64, error) {
			return store.Fail(ctx, id, claimID, "exhausted")
		}, "failed"},
	}

	for _, c := range cases {
		t.Run(c.name+" with the current claim", func(t *testing.T) {
			truncate(t, ctx, pool)
			insert(t, ctx, pool, defaultRow("m-1"))
			claimed := claimOne(t, ctx, pool, now, claimA)

			affected, err := c.apply(claimed.ID, claimed.LockedBy)
			if err != nil {
				t.Fatalf("%s = %v, want nil", c.name, err)
			}
			if affected != 1 {
				t.Fatalf("%s affected %d rows, want 1", c.name, affected)
			}
			if got := statusOf(t, ctx, pool, claimed.ID); got != c.wantStatus {
				t.Fatalf("status = %q, want %q", got, c.wantStatus)
			}
		})

		t.Run(c.name+" with a replaced claim", func(t *testing.T) {
			truncate(t, ctx, pool)
			insert(t, ctx, pool, defaultRow("m-1"))
			claimed := claimOne(t, ctx, pool, now, claimA)
			before := snapshot(t, ctx, pool, claimed.ID)

			affected, err := c.apply(claimed.ID, claimB)
			if err != nil {
				t.Fatalf("%s = %v, want nil", c.name, err)
			}
			if affected != 0 {
				t.Fatalf("%s affected %d rows, want 0", c.name, affected)
			}
			if after := snapshot(t, ctx, pool, claimed.ID); after != before {
				t.Fatalf("record changed under a replaced claim:\nbefore=%+v\n after=%+v", before, after)
			}
		})
	}
}

// OBX-18: after a transient outcome the lease is gone and available_at alone
// decides. A backoff shorter than the lease has to win, or the record would sit
// out the remainder of a lease nobody holds.
func TestBackoffShorterThanTheLeaseGovernsTheNextClaim(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	now := dmpfports.Instant(10_000)
	const backoff = dmpfports.Instant(1_000)

	insert(t, ctx, pool, defaultRow("m-1"))
	claimed := claimOne(t, ctx, pool, now, claimA)

	store := dmpfpostgres.NewOutboxStore(pool, fixedClock(now))
	if _, err := store.Reschedule(ctx, claimed.ID, claimed.LockedBy, now+backoff, "transient"); err != nil {
		t.Fatalf("Reschedule() = %v, want nil", err)
	}

	tooEarly := dmpfpostgres.NewOutboxStore(pool, fixedClock(now+backoff-1))
	if got, err := tooEarly.Claim(ctx, claimB, 10, testLease); err != nil || len(got) != 0 {
		t.Fatalf("Claim() before the backoff = %v, %d rows; want nil, 0", err, len(got))
	}

	// The original lease still has testLease - backoff to run; the record comes
	// back anyway, because Reschedule released it in the same commit.
	onTime := dmpfpostgres.NewOutboxStore(pool, fixedClock(now+backoff))
	if got, err := onTime.Claim(ctx, claimB, 10, testLease); err != nil || len(got) != 1 {
		t.Fatalf("Claim() at the backoff = %v, %d rows; want nil, 1", err, len(got))
	}
}

// OBX-04: the record never goes back to pending. A record in publishing returns
// to the pool by deadline comparison, so no statement in this package may write
// that status over it.
func TestNoStatementWritesPendingOverAClaimedRecord(t *testing.T) {
	forbidden := regexp.MustCompile(`(?is)\bset\b[^;` + "`" + `]*?status\s*=\s*'pending'`)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir() = %v, want nil", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile(%s) = %v, want nil", name, err)
		}
		if match := forbidden.Find(source); match != nil {
			t.Errorf("%s writes pending over an existing status: %q", name, match)
		}
	}
}

func statusOf(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id int64) string {
	t.Helper()

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM dmpf_outbox WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("read status = %v, want nil", err)
	}
	return status
}

// recordState is comparable so a rejected transition can be asserted as "the
// record did not move", which is OBX-11, rather than field by field.
type recordState struct {
	status      string
	lockedBy    string
	lockedUntil int64
	availableAt int64
	attempts    int
	publishedAt int64
	lastError   string
}

func snapshot(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id int64) recordState {
	t.Helper()

	var (
		s                        recordState
		lockedBy, lastError      *string
		lockedUntil, publishedAt *int64
	)
	err := pool.QueryRow(ctx, `
		SELECT status, locked_by, locked_until, available_at, attempt_count, published_at, last_error
		  FROM dmpf_outbox WHERE id = $1`, id).
		Scan(&s.status, &lockedBy, &lockedUntil, &s.availableAt, &s.attempts, &publishedAt, &lastError)
	if err != nil {
		t.Fatalf("snapshot = %v, want nil", err)
	}
	if lockedBy != nil {
		s.lockedBy = *lockedBy
	}
	if lastError != nil {
		s.lastError = *lastError
	}
	if lockedUntil != nil {
		s.lockedUntil = *lockedUntil
	}
	if publishedAt != nil {
		s.publishedAt = *publishedAt
	}
	return s
}

// The schema requires available_at >= occurred_at. An event whose occurred_at
// is ahead of this relay's clock would make a naive backoff write fail with
// 23514 and strand the record in publishing until the lease runs out.
func TestRescheduleNeverWritesAvailableAtBeforeOccurredAt(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	now := dmpfports.Instant(10_000)

	r := defaultRow("m-1")
	r.occurredAt, r.availableAt = int64(now)+50_000, int64(now)+50_000
	insert(t, ctx, pool, r)

	// The record is due only in the future, so the claim has to reach past it.
	ahead := dmpfpostgres.NewOutboxStore(pool, fixedClock(now+60_000))
	claimed, err := ahead.Claim(ctx, claimA, 10, testLease)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("Claim() = %v, %d rows; want nil, 1", err, len(claimed))
	}

	behind := dmpfpostgres.NewOutboxStore(pool, fixedClock(now))
	affected, err := behind.Reschedule(ctx, claimed[0].ID, claimed[0].LockedBy, now, "transient")
	if err != nil {
		t.Fatalf("Reschedule() with a backoff before occurred_at = %v, want nil", err)
	}
	if affected != 1 {
		t.Fatalf("Reschedule() affected %d rows, want 1", affected)
	}

	got := snapshot(t, ctx, pool, claimed[0].ID)
	if got.availableAt != r.occurredAt {
		t.Fatalf("available_at = %d, want %d (clamped to occurred_at)", got.availableAt, r.occurredAt)
	}
}

func TestMarkPublishedLeavesNoLiveLeaseBehind(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	now := dmpfports.Instant(10_000)

	insert(t, ctx, pool, defaultRow("m-1"))
	claimed := claimOne(t, ctx, pool, now, claimA)

	store := dmpfpostgres.NewOutboxStore(pool, fixedClock(now))
	if affected, err := store.MarkPublished(ctx, claimed.ID, claimed.LockedBy); err != nil || affected != 1 {
		t.Fatalf("MarkPublished() = %v, %d rows; want nil, 1", err, affected)
	}

	var lockedUntil *int64
	if err := pool.QueryRow(ctx, `SELECT locked_until FROM dmpf_outbox WHERE id = $1`, claimed.ID).Scan(&lockedUntil); err != nil {
		t.Fatalf("read back = %v, want nil", err)
	}
	if lockedUntil != nil {
		t.Fatalf("locked_until = %d on a published record, want nothing", *lockedUntil)
	}
}
