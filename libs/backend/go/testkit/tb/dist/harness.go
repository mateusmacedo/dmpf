package dist

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const (
	EnvRole    = "TESTKIT_ROLE"
	EnvBrokers = "KAFKA_BROKERS"
	RoleTest   = "^TestDistkitRole$"
	Relays     = 2

	childTimeoutWithoutDeadline = 10 * time.Minute
)

type Role string

const RoleRelay Role = "relay"

type RoleFunc func(t *testing.T, ctx context.Context)

type Spec struct {
	Name  string
	Pool  pg.Options
	Roles map[Role]RoleFunc
}

// RunRole is the body of a TestDistkitRole: it reads the role before anything
// else and skips without one, so the test is inert unless the harness
// re-executed the binary as a child.
func RunRole(t *testing.T, s Spec) {
	t.Helper()
	role := Role(os.Getenv(EnvRole))
	if role == "" {
		t.Skip(EnvRole + " unset: this test only runs as a re-executed child of dist.Harness")
	}
	run, ok := s.Roles[role]
	if !ok {
		t.Fatalf("dist: %s has no role %q", s.Name, role)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	run(t, ctx)
}

// Relay runs the drain the way the binary does, from its own configuration, so
// the harness judges the production path. A cancelled context is the parent
// asking the child to finish, not a failure of the drain.
func Relay(run func(ctx context.Context, getenv func(string) string) error) RoleFunc {
	return func(t *testing.T, ctx context.Context) {
		t.Helper()
		if err := run(ctx, os.Getenv); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("dist: the drain returned %v", err)
		}
	}
}

type Harness struct {
	Spec    Spec
	Brokers []string
	Pool    *pgxpool.Pool
	Topic   string
	Group   string
	DLQ     string
}

func New(t testing.TB, s Spec) *Harness {
	t.Helper()
	seeds := strings.Split(tb.Env(t, EnvBrokers), ",")
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	h := &Harness{
		Spec:    s,
		Brokers: seeds,
		Pool:    pg.OpenPool(t, s.Pool),
		Topic:   "distkit-" + s.Name + "-" + suffix,
		Group:   "distkit-" + s.Name + "-group-" + suffix,
		DLQ:     "distkit-" + s.Name + "-" + suffix + "-dlq",
	}
	CreateTopics(t, seeds, h.Group, h.Topic, h.DLQ)
	return h
}

func CreateTopics(t testing.TB, seeds []string, group string, topics ...string) {
	t.Helper()
	cl, err := kgo.NewClient(kgo.SeedBrokers(seeds...))
	if err != nil {
		t.Fatalf("dist: kgo.NewClient: %v", err)
	}
	t.Cleanup(cl.Close)
	admin := kadm.NewClient(cl)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	created, err := admin.CreateTopics(ctx, 1, 1, nil, topics...)
	if err == nil {
		err = created.Error()
	}
	if err != nil {
		t.Fatalf("dist: CreateTopics: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = admin.DeleteTopics(ctx, topics...)
		_, _ = admin.DeleteGroups(ctx, group)
	})
}

// Start re-executes os.Executable() with explicit arguments, never the parent's
// os.Args, and hands the child the variables the binary of the context reads,
// so the child configures itself exactly as production does.
func (h *Harness) Start(t testing.TB, role Role, env ...string) *Process {
	t.Helper()
	return h.start(t, string(role), role, env...)
}

func (h *Harness) start(t testing.TB, name string, role Role, env ...string) *Process {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("dist: os.Executable: %v", err)
	}
	prefix := "KAFKA_" + strings.ToUpper(h.Spec.Name)
	cmd := exec.Command(exe, "-test.run="+RoleTest, "-test.v", "-test.count=1", "-test.timeout="+childTimeout(t).String())
	cmd.Env = append(os.Environ(),
		EnvRole+"="+string(role),
		prefix+"_TOPIC="+h.Topic,
		prefix+"_DLQ="+h.DLQ,
		"KAFKA_GROUP="+h.Group,
		"KAFKA_INSECURE=true",
		"OTEL_SERVICE_NAME="+h.Spec.Name+"-distkit",
		"OTEL_RESOURCE_ATTRIBUTES=service.version=distkit,service.instance.id="+h.Spec.Name+"-distkit-"+string(role),
		"OTEL_TRACES_EXPORTER=none",
		"OTEL_METRICS_EXPORTER=none",
		"OTEL_LOGS_EXPORTER=none",
		EnvBrokers+"="+strings.Join(h.Brokers, ","),
		pg.PostgresDSN+"="+pg.DSN(t, h.Spec.Pool.Project),
	)
	cmd.Env = append(cmd.Env, env...)
	return Supervise(t, name, cmd)
}

// A parent killed by its own -test.timeout never runs the cleanup that stops
// the child, so the child carries the same deadline and does not outlive it
// holding the Kafka group and the Postgres connections.
func childTimeout(t testing.TB) time.Duration {
	if d, ok := t.(interface{ Deadline() (time.Time, bool) }); ok {
		if deadline, set := d.Deadline(); set {
			return max(time.Until(deadline), time.Second)
		}
	}
	return childTimeoutWithoutDeadline
}

func (h *Harness) Settled(t testing.TB, want int, timeout time.Duration) map[string]string {
	t.Helper()
	return pg.Settled(t, h.Pool, want, timeout)
}

// Collect reads the topic up to the end offsets it has once every writer
// stopped, so a duplicate published after the outbox settled is still read.
func (h *Harness) Collect(t testing.TB, timeout time.Duration) []Published {
	t.Helper()
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(h.Brokers...),
		kgo.ConsumeTopics(h.Topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		t.Fatalf("dist.Collect: kgo.NewClient: %v", err)
	}
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ends, err := kadm.NewClient(cl).ListEndOffsets(ctx, h.Topic)
	if err == nil {
		err = ends.Error()
	}
	if err != nil {
		t.Fatalf("dist.Collect: ListEndOffsets: %v", err)
	}
	unread := map[int32]int64{}
	ends.Each(func(end kadm.ListedOffset) {
		if end.Offset > 0 {
			unread[end.Partition] = end.Offset
		}
	})

	var out []Published
	for len(unread) > 0 {
		fetches := cl.PollFetches(ctx)
		if err := ctx.Err(); err != nil {
			t.Fatalf("dist.Collect: %d envelopes read, partitions %v not read to their end within %v", len(out), unread, timeout)
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			t.Fatalf("dist.Collect: %v", errs)
		}
		fetches.EachRecord(func(r *kgo.Record) {
			env, err := envelope.Unmarshal(r.Value)
			if err != nil {
				t.Fatalf("dist.Collect: the topic carries something that is not an envelope: %v", err)
			}
			out = append(out, Published{MessageID: env.ID, PayloadHash: payloadhash.Sum(env.Payload)})
			if end, ok := unread[r.Partition]; ok && r.Offset+1 >= end {
				delete(unread, r.Partition)
			}
		})
	}
	return out
}

func WaitFor(t testing.TB, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("dist: timed out waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// RelayVector is the vector of a producing context (DMPF-P001 to P003). The
// outbox stays locked until both drains block on the same claim, so they
// compete for the same records instead of the first one draining them alone.
func RelayVector(t testing.TB, h *Harness, want int) Verdict {
	t.Helper()
	if want <= 0 {
		t.Fatalf("dist: RelayVector(want = %d): the vector needs pending records to compete for", want)
	}
	var pending int
	if err := h.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM outbox WHERE status = 'pending'").Scan(&pending); err != nil {
		t.Fatalf("dist: counting the outbox: %v", err)
	}
	if pending != want {
		t.Fatalf("dist: outbox holds %d pending records before the drains start, want %d: the vector needs something to compete for", pending, want)
	}
	relays := make([]*Process, Relays)
	t.Cleanup(func() {
		if t.Failed() {
			h.dumpOutbox(t)
			for _, r := range relays {
				if r != nil {
					t.Logf("%s wrote: %s", r.Name(), r.Output())
				}
			}
		}
	})
	release := h.lockOutbox(t)
	names := make([]string, len(relays))
	for i := range relays {
		names[i] = fmt.Sprintf("%s-%s-%d", h.Spec.Name, RoleRelay, i+1)
		relays[i] = h.start(t, fmt.Sprintf("%s-%d", RoleRelay, i+1), RoleRelay, "PGAPPNAME="+names[i])
	}
	WaitFor(t, "every relay to block on the claim of the locked outbox", 60*time.Second, func() bool {
		return h.blockedClaims(t, names) == len(names)
	})
	release()

	settled := h.Settled(t, want, 30*time.Second)
	for _, r := range relays {
		r.Stop(t, 30*time.Second)
	}
	return DecideRelay(settled, h.Collect(t, 60*time.Second))
}

// EXCLUSIVE conflicts with the row locks of the claim and not with the plain
// reads a relay runs before it, so each drain boots and then waits on the claim.
func (h *Harness) lockOutbox(t testing.TB) (release func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("dist: begin the outbox lock: %v", err)
	}
	if _, err := tx.Exec(ctx, "LOCK TABLE outbox IN EXCLUSIVE MODE"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("dist: lock the outbox: %v", err)
	}
	var once sync.Once
	release = func() { once.Do(func() { _ = tx.Rollback(ctx) }) }
	t.Cleanup(release)
	return release
}

func (h *Harness) blockedClaims(t testing.TB, names []string) int {
	t.Helper()
	var blocked int
	if err := h.Pool.QueryRow(context.Background(), `
		SELECT count(DISTINCT application_name) FROM pg_stat_activity
		 WHERE datname = current_database() AND application_name = ANY($1)
		   AND wait_event_type = 'Lock' AND query LIKE '%FOR UPDATE SKIP LOCKED%'`, names).Scan(&blocked); err != nil {
		t.Fatalf("dist: reading the claims blocked on the outbox: %v", err)
	}
	return blocked
}

func (h *Harness) dumpOutbox(t testing.TB) {
	rows, err := h.Pool.Query(context.Background(),
		"SELECT message_id, status, attempt_count, coalesce(last_error, '') FROM outbox ORDER BY id")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id, status, lastErr string
		var attempts int
		_ = rows.Scan(&id, &status, &attempts, &lastErr)
		t.Logf("outbox %s: status=%s attempts=%d last_error=%q", id, status, attempts, lastErr)
	}
}
