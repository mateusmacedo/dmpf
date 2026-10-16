//go:build integration

package serve_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/serve"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

const probeSchema = "CREATE TABLE IF NOT EXISTS probes (probe_id text PRIMARY KEY)"

var policies = kernelapp.Policies{
	IdempotencyWait:      time.Second,
	IdempotencyRetention: time.Hour,
	OutboxRetention:      time.Hour,
	InboxRetention:       time.Hour,
	PurgeInterval:        time.Hour,
	PurgeBatch:           100,
}

func probeAPI() serve.API {
	return serve.API{Desc: probeDesc(), Register: registerProbe, Schema: probeSchema, CommandConsumer: "serve.commands"}
}

func apiRoleOver(t *testing.T, migrate bool) serve.APIRole {
	t.Helper()
	return serve.APIRole{
		DSN:       pg.DSN(t, "serve"),
		Server:    kernelgrpc.APIEnv{GRPCAddr: obstest.UnusedLoopbackAddr(t), GRPCInsecure: true},
		Migrate:   migrate,
		Admission: admission.Limit{PerSecond: 10, Burst: 10, Concurrency: 10},
		Policies:  policies,
	}
}

func startAPI(t *testing.T, role serve.APIRole) (stop func() error) {
	t.Helper()
	return startAPIWith(t, role, probeAPI())
}

func startAPIWith(t *testing.T, role serve.APIRole, api serve.API) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stopped := make(chan error, 1)
	go func() { stopped <- serve.ServeAPI(ctx, runtime(t), role, api) }()
	return func() error {
		cancel()
		select {
		case err := <-stopped:
			return err
		case <-time.After(15 * time.Second):
			t.Fatal("ServeAPI() did not return within 15s of the cancellation")
			return nil
		}
	}
}

func servingStatus(t *testing.T, addr string) healthpb.HealthCheckResponse_ServingStatus {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient() = %v", err)
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		return healthpb.HealthCheckResponse_UNKNOWN
	}
	return resp.GetStatus()
}

func requireTables(t *testing.T, role serve.APIRole, tables ...string) {
	t.Helper()
	pool := pg.OpenPool(t, pg.Options{Project: "serve"})
	for _, table := range tables {
		var present bool
		if err := pool.QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", table).Scan(&present); err != nil || !present {
			t.Errorf("table %s present = %v (%v), want it created by the ready of the api role", table, present, err)
		}
	}
}

func TestServeAPIMigratesServesAndStopsCleanly(t *testing.T) {
	role := apiRoleOver(t, true)
	stop := startAPI(t, role)

	dist.WaitFor(t, "the api role to serve", 30*time.Second, func() bool {
		return servingStatus(t, role.Server.GRPCAddr) == healthpb.HealthCheckResponse_SERVING
	})
	requireTables(t, role, "outbox", "inbox", "probes")

	if err := stop(); err != nil {
		t.Fatalf("ServeAPI() = %v, want nil after the cancellation", err)
	}
}

func TestServeAPIWithoutMigrateWaitsForTheKernelTables(t *testing.T) {
	role := apiRoleOver(t, false)
	stop := startAPI(t, role)

	time.Sleep(2 * time.Second)
	if got := servingStatus(t, role.Server.GRPCAddr); got == healthpb.HealthCheckResponse_SERVING {
		t.Fatal("the api role served before the outbox and the inbox existed")
	}
	pg.OpenPool(t, pg.Options{Project: "serve", Capabilities: []postgres.Capability{postgres.Outbox, postgres.Inbox}})
	dist.WaitFor(t, "the api role to serve once the tables exist", 30*time.Second, func() bool {
		return servingStatus(t, role.Server.GRPCAddr) == healthpb.HealthCheckResponse_SERVING
	})

	if err := stop(); err != nil {
		t.Fatalf("ServeAPI() = %v, want nil after the cancellation", err)
	}
}

func TestRunRelayDrainsTheOwnOutboxUntilStopped(t *testing.T) {
	pg.OpenPool(t, pg.Options{Project: "serve", Capabilities: []postgres.Capability{postgres.Outbox}})
	brokers := strings.Split(tb.Env(t, dist.EnvBrokers), ",")
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	topic, dlq, group := "serve-relay-"+suffix, "serve-relay-"+suffix+"-dlq", "serve-relay-group-"+suffix
	dist.CreateTopics(t, brokers, group, topic, dlq)
	role := serve.RelayRole{
		DSN:           pg.DSN(t, "serve"),
		Brokers:       brokers,
		KafkaInsecure: true,
		Relay: relay.Config{
			Source:         "urn:dmpf:serve",
			Interval:       100 * time.Millisecond,
			BatchSize:      10,
			Lease:          30 * time.Second,
			Concurrency:    1,
			MaxAttempts:    3,
			BackoffBase:    10 * time.Millisecond,
			BackoffCeiling: time.Second,
			System:         semconv.MessagingSystemKafka.Value.AsString(),
		},
		Policies: policies,
	}
	ch := channel.NewKafka("serve.events", topic, dlq, group, "com.company.serve.probed", "serve.v1")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stopped := make(chan error, 1)
	go func() {
		stopped <- serve.RunRelay(ctx, runtime(t), role, serve.Relay{Component: "serve", Channel: ch, Scope: "serve"})
	}()
	dist.WaitFor(t, "the relay to start draining", 30*time.Second, func() bool {
		return len(obstest.ScopesOf(logs, "relay draining")) > 0
	})
	cancel()

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("RunRelay() = %v, want nil after the cancellation", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("RunRelay() did not return within 15s of the cancellation")
	}
}

func grantedProbeAPI() serve.API {
	api := probeAPI()
	api.Grants = []postgres.Grant{postgres.Mutable("probes")}
	return api
}

func privilegesOf(t *testing.T, admin *pgxpool.Pool, table string) []string {
	t.Helper()
	rows, err := admin.Query(context.Background(), `
SELECT p FROM unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE']) AS p
WHERE has_table_privilege($1, $2, p) ORDER BY p`, pg.RuntimeRole, table)
	if err != nil {
		t.Fatalf("has_table_privilege(%s) = %v", table, err)
	}
	privileges, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect privileges on %s: %v", table, err)
	}
	return privileges
}

func TestServeAPIWithAMigratorGrantsTheRuntimeRoleWhatTheAppDeclares(t *testing.T) {
	role := apiRoleOver(t, true)
	role.DSN = pg.RuntimeDSN(t, "serve")
	role.MigrateDSN = pg.DSN(t, "serve")
	stop := startAPIWith(t, role, grantedProbeAPI())

	dist.WaitFor(t, "the api role to serve", 30*time.Second, func() bool {
		return servingStatus(t, role.Server.GRPCAddr) == healthpb.HealthCheckResponse_SERVING
	})
	admin := pg.OpenPool(t, pg.Options{Project: "serve"})
	want := map[string][]string{
		"outbox":     {"DELETE", "INSERT", "SELECT", "UPDATE"},
		"inbox":      {"DELETE", "INSERT", "SELECT", "UPDATE"},
		"quarantine": {"INSERT", "SELECT"},
		"probes":     {"INSERT", "SELECT", "UPDATE"},
	}
	for table, privileges := range want {
		if got := privilegesOf(t, admin, table); !slices.Equal(got, privileges) {
			t.Errorf("privileges of the runtime role on %s = %v, want %v", table, got, privileges)
		}
	}

	if err := stop(); err != nil {
		t.Fatalf("ServeAPI() = %v, want nil after the cancellation", err)
	}
}

func TestServeAPIMigratesAsAnOwnerThatIsNotASuperuser(t *testing.T) {
	role := apiRoleOver(t, true)
	role.DSN = pg.RuntimeDSN(t, "serve")
	role.MigrateDSN = pg.MigratorDSN(t, "serve")
	stop := startAPIWith(t, role, grantedProbeAPI())

	dist.WaitFor(t, "the api role to serve", 30*time.Second, func() bool {
		return servingStatus(t, role.Server.GRPCAddr) == healthpb.HealthCheckResponse_SERVING
	})
	admin := pg.OpenPool(t, pg.Options{Project: "serve"})
	rows, err := admin.Query(context.Background(), "SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tableowner <> $1", pg.MigratorRole)
	if err != nil {
		t.Fatalf("pg_tables = %v", err)
	}
	foreign, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(foreign) > 0 {
		t.Errorf("tables not owned by %s = %v (%v), want none", pg.MigratorRole, foreign, err)
	}
	if got := privilegesOf(t, admin, "probes"); !slices.Equal(got, []string{"INSERT", "SELECT", "UPDATE"}) {
		t.Errorf("privileges of the runtime role on probes = %v", got)
	}

	if err := stop(); err != nil {
		t.Fatalf("ServeAPI() = %v, want nil after the cancellation", err)
	}
}

func TestServeAPIRefusesAMigratorOfAnotherDatabase(t *testing.T) {
	role := apiRoleOver(t, true)
	role.DSN = pg.RuntimeDSN(t, "serve")
	role.MigrateDSN = pg.MigratorDSN(t, "serve_elsewhere")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := serve.ServeAPI(ctx, runtime(t), role, grantedProbeAPI())

	if !errors.Is(err, serve.ErrMigratorElsewhere) {
		t.Fatalf("ServeAPI() with the migrator on another database = %v, want ErrMigratorElsewhere", err)
	}
	elsewhere := pg.OpenPool(t, pg.Options{Project: "serve_elsewhere"})
	var migrated bool
	if err := elsewhere.QueryRow(context.Background(), "SELECT to_regclass('public.outbox') IS NOT NULL").Scan(&migrated); err != nil || migrated {
		t.Fatalf("outbox in the database of the migrator = %v (%v), want nothing migrated there", migrated, err)
	}
}

func TestServeAPIRefusesARuntimeRoleThatStillOwnsATableBeforeMigrating(t *testing.T) {
	role := apiRoleOver(t, true)
	role.DSN = pg.RuntimeDSN(t, "serve")
	admin := pg.OpenPool(t, pg.Options{Project: "serve", Capabilities: []postgres.Capability{postgres.Outbox, postgres.Inbox}})
	if _, err := admin.Exec(context.Background(), "ALTER TABLE outbox OWNER TO "+pg.RuntimeRole); err != nil {
		t.Fatalf("ALTER TABLE outbox OWNER = %v", err)
	}
	role.MigrateDSN = pg.MigratorDSN(t, "serve")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := serve.ServeAPI(ctx, runtime(t), role, grantedProbeAPI())

	if !errors.Is(err, serve.ErrMigrator) || !strings.Contains(err.Error(), "owns outbox") || !strings.Contains(err.Error(), "REASSIGN OWNED BY") {
		t.Fatalf("ServeAPI() over a database whose outbox the runtime role owns = %v, want ErrMigrator with the REASSIGN OWNED that fixes it", err)
	}
	if strings.Contains(err.Error(), "must be owner") {
		t.Fatalf("ServeAPI() = %v reached the DDL before refusing", err)
	}
}

func TestServeAPIWithoutMigrateWaitsForTheGrantsOfTheRuntimeRole(t *testing.T) {
	role := apiRoleOver(t, false)
	role.DSN = pg.RuntimeDSN(t, "serve")
	options := pg.Options{Project: "serve", Capabilities: []postgres.Capability{postgres.Outbox, postgres.Inbox}, Schemas: []string{probeSchema}, Grants: grantedProbeAPI().Grants}
	admin := pg.OpenPool(t, options)
	stop := startAPIWith(t, role, grantedProbeAPI())

	time.Sleep(2 * time.Second)
	if got := servingStatus(t, role.Server.GRPCAddr); got == healthpb.HealthCheckResponse_SERVING {
		t.Fatal("the api role served before the runtime role held its grants")
	}
	pg.GrantRuntime(t, admin, options)
	dist.WaitFor(t, "the api role to serve once the grants exist", 30*time.Second, func() bool {
		return servingStatus(t, role.Server.GRPCAddr) == healthpb.HealthCheckResponse_SERVING
	})

	if err := stop(); err != nil {
		t.Fatalf("ServeAPI() = %v, want nil after the cancellation", err)
	}
}

func TestServeAPIRefusesAMigratorThatIsTheRuntimeRole(t *testing.T) {
	role := apiRoleOver(t, true)
	role.MigrateDSN = role.DSN
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := serve.ServeAPI(ctx, runtime(t), role, grantedProbeAPI())

	if !errors.Is(err, serve.ErrMigratorIsRuntime) {
		t.Fatalf("ServeAPI() = %v, want ErrMigratorIsRuntime", err)
	}
}

func TestServeAPIKeepsTheMigratorCredentialsOutOfItsError(t *testing.T) {
	server, err := url.Parse(pg.DSN(t, "serve"))
	if err != nil {
		t.Fatalf("parse DSN = %v", err)
	}
	server.User = url.UserPassword("sentinel_migrator", "sentinel-password")
	cases := map[string]string{
		"a refused login":         server.String(),
		"a malformed connection":  "postgres://sentinel_migrator:sentinel-password@[::1/serve",
		"an unreachable database": "postgres://sentinel_migrator:sentinel-password@127.0.0.1:1/serve?connect_timeout=1",
	}
	for name, dsn := range cases {
		t.Run(name, func(t *testing.T) {
			role := apiRoleOver(t, true)
			role.DSN = pg.RuntimeDSN(t, "serve")
			role.MigrateDSN = dsn
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := serve.ServeAPI(ctx, runtime(t), role, grantedProbeAPI())

			if !errors.Is(err, serve.ErrMigrator) {
				t.Fatalf("ServeAPI() = %v, want ErrMigrator", err)
			}
			for _, secret := range []string{dsn, "sentinel_migrator", "sentinel-password"} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("ServeAPI() = %q carries %q", err, secret)
				}
			}
		})
	}
}

func TestRunRelayWaitsForTheGrantsOfTheOutbox(t *testing.T) {
	pg.OpenPool(t, pg.Options{Project: "serve", Capabilities: []postgres.Capability{postgres.Outbox}})
	role := serve.RelayRole{DSN: pg.RuntimeDSN(t, "serve"), Brokers: []string{"127.0.0.1:1"}, KafkaInsecure: true, Policies: policies}
	ch := channel.NewKafka("serve.events", "serve-relay", "serve-relay-dlq", "serve-relay-group", "com.company.serve.probed", "serve.v1")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := serve.RunRelay(ctx, runtime(t), role, serve.Relay{Component: "serve", Channel: ch, Scope: "serve"})

	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "grant") {
		t.Fatalf("RunRelay() without the grants of the outbox = %v, want it still waiting for a grant at the deadline", err)
	}
}
