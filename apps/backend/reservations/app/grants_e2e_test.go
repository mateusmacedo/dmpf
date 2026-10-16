//go:build integration

package app_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/appkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

func runtimePrivileges(t *testing.T, admin *pgxpool.Pool, table string) []string {
	t.Helper()
	rows, err := admin.Query(context.Background(), `
SELECT p FROM unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE']) AS p
WHERE COALESCE(has_table_privilege($1, to_regclass($2), p), false) ORDER BY p`, pg.RuntimeRole, table)
	if err != nil {
		t.Fatalf("has_table_privilege(%s) = %v", table, err)
	}
	privileges, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect privileges on %s: %v", table, err)
	}
	return privileges
}

func runRole(t *testing.T, cfg app.Config, runtime *otelboot.Runtime) (stopped <-chan error, stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- app.RunWith(ctx, cfg, runtime) }()
	return done, func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(15 * time.Second):
			t.Fatalf("RunWith(%s) did not return within 15s of the cancellation", cfg.Role)
			return nil
		}
	}
}

func TestTheAPIWithAMigratorGrantsTheRuntimeRoleTheReservationsItWrites(t *testing.T) {
	migrator := pg.MigratorDSN(t, appkit.PoolOptions.Project)
	admin := pg.OpenPool(t, pg.Options{Project: appkit.PoolOptions.Project})
	cfg := app.Defaults(app.RoleAPI)
	cfg.DSN = pg.RuntimeDSN(t, appkit.PoolOptions.Project)
	cfg.MigrateDSN = migrator
	cfg.Migrate = true
	cfg.API.GRPCAddr, cfg.API.GRPCInsecure = obstest.UnusedLoopbackAddr(t), true
	stopped, stop := runRole(t, cfg, obstest.MeteredRuntime(t, "reservations", string(app.RoleAPI), sdkmetric.NewManualReader()))

	want := []string{"INSERT", "SELECT", "UPDATE"}
	deadline := time.Now().Add(30 * time.Second)
	for got := runtimePrivileges(t, admin, "reservations"); !slices.Equal(got, want); got = runtimePrivileges(t, admin, "reservations") {
		select {
		case err := <-stopped:
			t.Fatalf("RunWith(api) = %v while the runtime role held %v on reservations, want %v", err, got, want)
		case <-time.After(100 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatalf("privileges of the runtime role on reservations = %v after 30s, want %v", got, want)
		}
	}

	if err := stop(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("RunWith(api) = %v, want nil or the cancellation", err)
	}
}

func loggedRuntime(t *testing.T, logs *obstest.MemoryLogs) (*sdklog.LoggerProvider, *otelboot.Runtime) {
	t.Helper()
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "reservations", ServiceVersion: "dev", ServiceInstanceID: "reservations-consumer-1", Role: "consumer"},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	provider := otelboot.NewLoggerProvider(config, logs)
	config.LoggerProvider = provider
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("otelboot.Start() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	return provider, runtime
}

func TestTheConsumerWaitsForTheGrantsOfTheRuntimeRoleBeforeJoining(t *testing.T) {
	brokers := strings.Split(tb.Env(t, "KAFKA_BROKERS"), ",")
	admin := appkit.OpenPool(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	cfg := app.Defaults(app.RoleConsumer)
	cfg.DSN = pg.RuntimeDSN(t, appkit.PoolOptions.Project)
	cfg.Brokers, cfg.KafkaInsecure = brokers, true
	cfg.OrdersTopic = "grants-orders-" + suffix
	cfg.OrdersDLQ = cfg.OrdersTopic + "-dlq"
	cfg.Group = "grants-reservations-group-" + suffix
	dist.CreateTopics(t, brokers, cfg.Group, cfg.OrdersTopic, cfg.OrdersDLQ)
	logs := &obstest.MemoryLogs{}
	provider, runtime := loggedRuntime(t, logs)
	stopped, stop := runRole(t, cfg, runtime)

	awaitRecord(t, provider, logs, "waiting for table privilege", stopped, 15*time.Second)
	if _, joined := obstest.RecordAttributes(logs, "consumer joining"); joined {
		t.Fatal("the consumer joined before the runtime role held its grants")
	}
	pg.GrantRuntime(t, admin, appkit.PoolOptions)
	awaitRecord(t, provider, logs, "consumer joining", stopped, 15*time.Second)

	if err := stop(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("RunWith(consumer) = %v, want nil or the cancellation", err)
	}
}
