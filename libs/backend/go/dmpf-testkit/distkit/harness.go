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

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb/pg"
)

// The variables the parent hands the re-executed child. Role selects which
// side of the harness the child plays; the others name the channel the two
// processes share so the catalogue is identical on both ends (ASY-02).
const (
	EnvRole    = "DMPF_TESTKIT_ROLE"
	EnvTopic   = "DMPF_TESTKIT_TOPIC"
	EnvGroup   = "DMPF_TESTKIT_GROUP"
	EnvDLQ     = "DMPF_TESTKIT_DLQ"
	EnvBrokers = "DMPF_KAFKA_BROKERS"
)

// Role is what a child process does: publish the plan, consume through the
// DMPF adapter, or consume naively — applying the effect on every delivery.
type Role string

const (
	RoleProducer      Role = "producer"
	RoleConsumer      Role = "consumer"
	RoleNaiveConsumer Role = "consumer-naive"
)

// Harness is KIT-06: two operating-system processes over a real broker
// (PIR-14), with the redelivery the plan injects and the effect edge in the
// Postgres both share.
type Harness struct {
	Brokers []string
	Pool    *pgxpool.Pool
	Topic   string
	Group   string
	DLQ     string
	Plan    Plan
}

// New resolves the broker and the database from the environment (skipping
// outside CI when unset), creates a topic unique to this run and returns the
// harness. The topics are deleted when the test ends.
func New(t testing.TB) Harness {
	t.Helper()
	seeds := strings.Split(tb.Env(t, EnvBrokers), ",")
	pool := pg.OpenPool(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	h := Harness{
		Brokers: seeds,
		Pool:    pool,
		Topic:   "dmpf-distkit-" + suffix,
		Group:   "dmpf-distkit-group-" + suffix,
		DLQ:     "dmpf-distkit-" + suffix + "-dlq",
		Plan:    Default,
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
		cl.Close()
	})
}

// Process is one re-executed child: the test binary itself, running only
// TestDistkitRole under the role the parent chose.
type Process struct {
	Role   Role
	cmd    *exec.Cmd
	output bytes.Buffer
	done   chan error
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
		EnvTopic+"="+h.Topic,
		EnvGroup+"="+h.Group,
		EnvDLQ+"="+h.DLQ,
	)
	p := &Process{Role: role, cmd: cmd, done: make(chan error, 1)}
	cmd.Stdout, cmd.Stderr = &p.output, &p.output
	if err := cmd.Start(); err != nil {
		t.Fatalf("distkit: start %s: %v", role, err)
	}
	go func() { p.done <- cmd.Wait() }()
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			<-p.done
		}
	})
	return p
}

// Wait blocks until the child exits and fails the test if it did not exit
// cleanly within the timeout, printing what it wrote.
func (p *Process) Wait(t testing.TB, timeout time.Duration) {
	t.Helper()
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatalf("distkit: %s exited with %v\n%s", p.Role, err, p.output.String())
		}
	case <-time.After(timeout):
		_ = p.cmd.Process.Kill()
		t.Fatalf("distkit: %s did not exit within %v\n%s", p.Role, timeout, p.output.String())
	}
}

// Stop asks the child to finish (SIGTERM, which the consumer roles honour
// through signal.NotifyContext) and waits for it.
func (p *Process) Stop(t testing.TB, timeout time.Duration) {
	t.Helper()
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	p.Wait(t, timeout)
}

// Output is what the child wrote so far.
func (p *Process) Output() string { return p.output.String() }

// Effects reads the effect edge: the four tables and the reservation of the
// plan's order as it stands.
func (h Harness) Effects(t testing.TB) Effects {
	t.Helper()
	var e Effects
	const counts = `SELECT
		(SELECT count(*) FROM dmpf_inbox),
		(SELECT count(*) FROM dmpf_example_reservations),
		(SELECT count(*) FROM dmpf_outbox),
		(SELECT count(*) FROM dmpf_quarantine)`
	ctx := context.Background()
	if err := h.Pool.QueryRow(ctx, counts).Scan(&e.Inbox, &e.Reservations, &e.Outbox, &e.Quarantine); err != nil {
		t.Fatalf("distkit: effects: %v", err)
	}
	row := h.Pool.QueryRow(ctx,
		`SELECT version, (snapshot->>'Items')::int FROM dmpf_example_reservations WHERE order_id = $1`, h.Plan.Order)
	if err := row.Scan(&e.Version, &e.Items); err != nil && !strings.Contains(err.Error(), "no rows") {
		t.Fatalf("distkit: reservation of %s: %v", h.Plan.Order, err)
	}
	return e
}

// WaitFor polls until cond holds or the timeout passes.
func WaitFor(t testing.TB, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("distkit: timed out waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
