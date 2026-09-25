//go:build integration && distributed

package distkit

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/appkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

// The variables the parent hands the re-executed child. Role selects which
// side the child plays; the others name the channel, so the catalogue is
// identical on both ends (ASY-02).
const (
	EnvRole    = "DMPF_TESTKIT_ROLE"
	EnvTopic   = "DMPF_TESTKIT_TOPIC"
	EnvGroup   = "DMPF_TESTKIT_GROUP"
	EnvDLQ     = "DMPF_TESTKIT_DLQ"
	EnvBrokers = "DMPF_KAFKA_BROKERS"
)

// Role is what a child process does. A producing context has one: drain the
// outbox. The parent runs two of them at once, which is the vector.
type Role string

const RoleRelay Role = "relay"

// Relays is how many drains compete for the same outbox.
const Relays = 2

// Harness is KIT-06 for a producing context: two operating-system processes
// over a real broker (PIR-14), draining the outbox both share.
type Harness struct {
	Brokers []string
	Pool    *pgxpool.Pool
	Topic   string
	Group   string
	DLQ     string
}

// New resolves the broker and the database from the environment (skipping
// outside CI when unset), creates a topic unique to this run and returns the
// harness. The topics are deleted when the test ends.
func New(t testing.TB) Harness {
	t.Helper()
	seeds := strings.Split(tb.Env(t, EnvBrokers), ",")
	pool := pg.OpenPool(t, appkit.PoolOptions)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	h := Harness{
		Brokers: seeds,
		Pool:    pool,
		Topic:   "dmpf-distkit-orders-" + suffix,
		Group:   "dmpf-distkit-orders-group-" + suffix,
		DLQ:     "dmpf-distkit-orders-" + suffix + "-dlq",
	}
	createTopics(t, seeds, h.Topic, h.DLQ)
	return h
}

func createTopics(t testing.TB, seeds []string, topics ...string) {
	t.Helper()
	cl, err := kgo.NewClient(kgo.SeedBrokers(seeds...))
	if err != nil {
		t.Fatalf("distkit: kgo.NewClient: %v", err)
	}
	t.Cleanup(cl.Close)
	admin := kadm.NewClient(cl)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := admin.CreateTopics(ctx, 1, 1, nil, topics...); err != nil {
		t.Fatalf("distkit: CreateTopics: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = admin.DeleteTopics(ctx, topics...)
	})
}

// Process is a re-executed child and what it wrote.
type Process struct {
	Role     Role
	cmd      *exec.Cmd
	output   bytes.Buffer
	err      error
	finished chan struct{}
}

// Start re-executes os.Executable() with explicit arguments — never the
// parent's os.Args — and an environment rebuilt from the parent's plus the
// role variables. The child cannot start a harness of its own: RunRole reads
// the role before anything else and the parent never sets it for itself.
func (h Harness) Start(t testing.TB, role Role) *Process {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("distkit: os.Executable: %v", err)
	}
	cmd := exec.Command(exe, "-test.run=^TestDistkitRole$", "-test.v", "-test.count=1")
	cmd.Env = append(os.Environ(),
		EnvRole+"="+string(role),
		// The child reads its configuration the way the binary does, so these
		// are the variables of the app, not names of the harness.
		"DMPF_KAFKA_ORDERS_TOPIC="+h.Topic,
		"DMPF_KAFKA_ORDERS_DLQ="+h.DLQ,
		"DMPF_KAFKA_GROUP="+h.Group,
		"DMPF_KAFKA_INSECURE=true",
		"DMPF_SERVICE=orders-distkit",
		EnvBrokers+"="+strings.Join(h.Brokers, ","),
		pg.PostgresDSN+"="+pg.DSN(t, appkit.PoolOptions.Project),
	)
	p := &Process{Role: role, cmd: cmd, finished: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = &p.output, &p.output
	if err := cmd.Start(); err != nil {
		t.Fatalf("distkit: start %s: %v", role, err)
	}
	go func() {
		p.err = cmd.Wait()
		close(p.finished)
	}()
	t.Cleanup(func() {
		select {
		case <-p.finished:
		default:
			_ = cmd.Process.Kill()
			<-p.finished
		}
	})
	return p
}

// Output is what the child wrote, which is where a drain that refused to
// start says why.
func (p *Process) Output() string { return p.output.String() }

// Stop asks the child to finish and waits, so the drain closes its publisher
// instead of being killed mid-publication.
func (p *Process) Stop(t testing.TB, timeout time.Duration) {
	t.Helper()
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-p.finished:
	case <-time.After(timeout):
		_ = p.cmd.Process.Kill()
		<-p.finished
		t.Fatalf("distkit: %s did not exit within %v\n%s", p.Role, timeout, p.output.String())
	}
}

// Settled waits for the drain to publish every record and answers what the
// outbox stored for each: the identifier and the digest the relay checked on
// assembly.
func (h Harness) Settled(t testing.TB, want int, timeout time.Duration) map[string]string {
	t.Helper()
	const query = `SELECT message_id, payload_hash FROM outbox WHERE status = 'published'`
	deadline := time.Now().Add(timeout)
	for {
		settled := map[string]string{}
		rows, err := h.Pool.Query(context.Background(), query)
		if err != nil {
			t.Fatalf("distkit.Settled: %v", err)
		}
		for rows.Next() {
			var id, hash string
			if err := rows.Scan(&id, &hash); err != nil {
				rows.Close()
				t.Fatalf("distkit.Settled: scan: %v", err)
			}
			settled[id] = hash
		}
		rows.Close()
		if len(settled) >= want {
			return settled
		}
		if time.Now().After(deadline) {
			t.Fatalf("distkit.Settled: %d of %d records settled within %v", len(settled), want, timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Collect reads the topic from the beginning and reduces every envelope to
// what the verdict decides on, recomputing the hash over the payload it
// carries.
func (h Harness) Collect(t testing.TB, want int, timeout time.Duration) []Published {
	t.Helper()
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(h.Brokers...),
		kgo.ConsumeTopics(h.Topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.ConsumerGroup(h.Group+"-collector"),
	)
	if err != nil {
		t.Fatalf("distkit.Collect: kgo.NewClient: %v", err)
	}
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var out []Published
	for len(out) < want {
		fetches := cl.PollFetches(ctx)
		if err := ctx.Err(); err != nil {
			t.Fatalf("distkit.Collect: %d of %d envelopes within %v", len(out), want, timeout)
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			t.Fatalf("distkit.Collect: %v", errs)
		}
		fetches.EachRecord(func(r *kgo.Record) {
			env, err := envelope.Unmarshal(r.Value)
			if err != nil {
				t.Fatalf("distkit.Collect: the topic carries something that is not an envelope: %v", err)
			}
			out = append(out, Published{
				MessageID:   env.ID,
				PayloadHash: payloadhash.Sum(env.Payload),
			})
		})
	}
	return out
}
