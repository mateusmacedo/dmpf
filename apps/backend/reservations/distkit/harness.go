//go:build integration && distributed

package distkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/appkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

// The variables the parent hands the re-executed child. Role selects which
// side of the harness the child plays; the others name the channel the two
// processes share so the catalogue is identical on both ends (ASY-02).
const (
	EnvRole    = dist.EnvRole
	EnvTopic   = "TESTKIT_TOPIC"
	EnvGroup   = "TESTKIT_GROUP"
	EnvDLQ     = "TESTKIT_DLQ"
	EnvPlan    = "TESTKIT_PLAN"
	EnvBrokers = dist.EnvBrokers
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
	pool := pg.OpenPool(t, appkit.PoolOptions)
	pg.GrantRuntime(t, pool, appkit.PoolOptions)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	h := Harness{
		Brokers: seeds,
		Pool:    pool,
		Topic:   "distkit-" + suffix,
		Group:   "distkit-group-" + suffix,
		DLQ:     "distkit-" + suffix + "-dlq",
		Plan:    Default,
	}
	dist.CreateTopics(t, seeds, h.Group, h.Topic, h.DLQ)
	return h
}

// Start re-executes os.Executable() with explicit arguments — never the
// parent's os.Args — and an environment rebuilt from the parent's plus the
// role variables and the plan, so the child publishes exactly what the parent
// judges. The child cannot start a harness of its own: RunRole reads the role
// before anything else and the parent never sets it for itself.
func (h Harness) Start(t testing.TB, role Role) *dist.Process {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("distkit: os.Executable: %v", err)
	}
	plan, err := json.Marshal(h.Plan)
	if err != nil {
		t.Fatalf("distkit: encode plan: %v", err)
	}
	cmd := exec.Command(exe, "-test.run=^TestDistkitRole$", "-test.v", "-test.count=1")
	cmd.Env = append(os.Environ(),
		EnvRole+"="+string(role),
		EnvTopic+"="+h.Topic,
		EnvGroup+"="+h.Group,
		EnvDLQ+"="+h.DLQ,
		EnvPlan+"="+string(plan),
		"OTEL_TRACES_EXPORTER=none",
		"OTEL_METRICS_EXPORTER=none",
		"OTEL_LOGS_EXPORTER=none",
		EnvBrokers+"="+strings.Join(h.Brokers, ","),
		pg.PostgresDSN+"="+pg.RuntimeDSN(t, appkit.PoolOptions.Project),
	)
	return dist.Supervise(t, string(role), cmd)
}

// Effects reads the effect edge: the four tables and the reservation of the
// plan's order as it stands.
func (h Harness) Effects(t testing.TB) Effects {
	t.Helper()
	var e Effects
	const counts = `SELECT
		(SELECT count(*) FROM inbox),
		(SELECT count(*) FROM reservations),
		(SELECT count(*) FROM outbox),
		(SELECT count(*) FROM quarantine)`
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	if err := h.Pool.QueryRow(ctx, counts).Scan(&e.Inbox, &e.Reservations, &e.Outbox, &e.Quarantine); err != nil {
		t.Fatalf("distkit: effects: %v", err)
	}
	row := h.Pool.QueryRow(ctx,
		`SELECT version, (snapshot->>'items')::int FROM reservations WHERE order_id = $1`, h.Plan.Order)
	if err := row.Scan(&e.Version, &e.Items); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("distkit: reservation of %s: %v", h.Plan.Order, err)
	}
	return e
}

// queryTimeout bounds each read of the effect edge, so a Postgres that stops
// answering fails the test by name instead of hanging in WaitFor's loop.
const queryTimeout = 10 * time.Second

// RelaySpec drives the relay of this context through the producing vector of
// tb/dist, which proves DMPF-P001 to P003 on the outbox the consumer writes to.
var RelaySpec = dist.Spec{
	Name: "reservations",
	Pool: appkit.PoolOptions,
	Roles: map[dist.Role]dist.RoleFunc{dist.RoleRelay: dist.Relay(func(ctx context.Context, getenv func(string) string) error {
		cfg, err := app.FromEnv(app.RoleRelay, getenv)
		if err != nil {
			return err
		}
		return app.Run(ctx, cfg)
	})},
}
