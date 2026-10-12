//go:build integration

package serve_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

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
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stopped := make(chan error, 1)
	go func() { stopped <- serve.ServeAPI(ctx, runtime(t), role, probeAPI()) }()
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
