//go:build integration

package app_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const (
	e2eTimeout  = 90 * time.Second
	pollEvery   = 100 * time.Millisecond
	adminWindow = 30 * time.Second
	stopGrace   = 20 * time.Second

	modulePrefix = "github.com/mateusmacedo/dmpf/apps/backend/"
)

// topology is the six processes of ADR-044 over two databases and four topics
// unique to this run; nothing in it imports a context package, so the BFF test
// proves the topology without crossing a bounded context.
type topology struct {
	brokers []string
	admin   *kadm.Client

	ordersDSN, reservationsDSN, bookingsDSN string

	ordersTopic, ordersDLQ             string
	reservationsTopic, reservationsDLQ string
	bookingsTopic, bookingsDLQ         string
	group                              string

	bffAddr string
}

type binaries struct{ bff, orders, reservations, bookings string }

func buildBinaries(t *testing.T) binaries {
	t.Helper()
	root := workspaceRoot(t)
	dir := t.TempDir()
	out := binaries{
		bff:          filepath.Join(dir, "bff"),
		orders:       filepath.Join(dir, "orders"),
		reservations: filepath.Join(dir, "reservations"),
		bookings:     filepath.Join(dir, "bookings"),
	}
	for path, pkg := range map[string]string{
		out.bff:          modulePrefix + "bff/cmd",
		out.orders:       modulePrefix + "orders/cmd",
		out.reservations: modulePrefix + "reservations/cmd",
		out.bookings:     modulePrefix + "bookings/cmd",
	} {
		cmd := exec.Command("go", "build", "-race", "-o", path, pkg)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go build -race %s: %v\n%s", pkg, err, output)
		}
	}
	return out
}

func workspaceRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOWORK").Output()
	if err != nil {
		t.Fatalf("go env GOWORK: %v", err)
	}
	work := strings.TrimSpace(string(out))
	if work == "" || work == "off" {
		t.Fatal("the e2e builds the three binaries through go.work; GOWORK is unset")
	}
	return filepath.Dir(work)
}

func newTopology(t *testing.T) *topology {
	t.Helper()
	brokers := strings.Split(tb.Env(t, "KAFKA_BROKERS"), ",")
	admin := openAdmin(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	top := &topology{
		brokers:           brokers,
		ordersTopic:       "dmpf-e2e-orders-" + suffix,
		ordersDLQ:         "dmpf-e2e-orders-" + suffix + "-dlq",
		reservationsTopic: "dmpf-e2e-reservations-" + suffix,
		reservationsDLQ:   "dmpf-e2e-reservations-" + suffix + "-dlq",
		group:             "dmpf-e2e-reservations-group-" + suffix,
		bookingsTopic:     "dmpf-e2e-bookings-" + suffix,
		bookingsDLQ:       "dmpf-e2e-bookings-" + suffix + "-dlq",
	}
	top.ordersDSN = createDatabase(t, admin, "e2e_orders_"+suffix)
	top.reservationsDSN = createDatabase(t, admin, "e2e_reservations_"+suffix)
	top.bookingsDSN = createDatabase(t, admin, "e2e_bookings_"+suffix)

	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatalf("kgo.NewClient() = %v", err)
	}
	t.Cleanup(cl.Close)
	top.admin = kadm.NewClient(cl)
	topics := []string{top.ordersTopic, top.ordersDLQ, top.reservationsTopic, top.reservationsDLQ, top.bookingsTopic, top.bookingsDLQ}
	ctx, cancel := context.WithTimeout(context.Background(), adminWindow)
	defer cancel()
	if _, err := top.admin.CreateTopics(ctx, 1, 1, nil, topics...); err != nil {
		t.Fatalf("CreateTopics() = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), adminWindow)
		defer cancel()
		_, _ = top.admin.DeleteTopics(ctx, topics...)
	})
	return top
}

// openAdmin is not pg.OpenPool on purpose: that one migrates and truncates a
// project's test database, and this run only creates databases.
func openAdmin(t *testing.T) *pgx.Conn {
	t.Helper()
	cfg, err := pgx.ParseConfig(tb.Env(t, pg.PostgresDSN))
	if err != nil {
		t.Fatalf("parse %s: %v", pg.PostgresDSN, err)
	}
	if host := cfg.Host; !loopback(host) {
		t.Fatalf("%s points at %q; the e2e creates and drops databases and only accepts a loopback host", pg.PostgresDSN, host)
	}
	conn, err := pgx.ConnectConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pgx.ConnectConfig() = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

func loopback(host string) bool {
	if host == "localhost" || strings.HasPrefix(host, "/") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func createDatabase(t *testing.T, admin *pgx.Conn, name string) string {
	t.Helper()
	ctx := context.Background()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("CREATE DATABASE %s: %v", name, err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("DROP DATABASE %s: %v", name, err)
		}
	})
	return dsnFor(t, name)
}

func dsnFor(t *testing.T, database string) string {
	t.Helper()
	u, err := url.Parse(tb.Env(t, pg.PostgresDSN))
	if err != nil {
		t.Fatalf("PG_DSN: %v", err)
	}
	u.Path = "/" + database
	return u.String()
}

type process struct {
	name     string
	cmd      *exec.Cmd
	mu       sync.Mutex
	out      bytes.Buffer
	finished chan struct{}
	err      error
}

func (p *process) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.out.Write(b)
}

func (p *process) logs() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.out.String()
}

func start(t *testing.T, name, binary string, env map[string]string, args ...string) *process {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	p := &process{name: name, cmd: cmd, finished: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = p, p
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", name, err)
	}
	go func() {
		p.err = cmd.Wait()
		close(p.finished)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-p.finished:
		case <-time.After(stopGrace):
			_ = cmd.Process.Kill()
			<-p.finished
		}
		if t.Failed() {
			t.Logf("%s logs:\n%s", name, p.logs())
		}
	})
	return p
}

func (p *process) waitLog(t *testing.T, message string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(e2eTimeout)
	for time.Now().Before(deadline) {
		scanner := bufio.NewScanner(strings.NewReader(p.logs()))
		for scanner.Scan() {
			var record map[string]any
			if json.Unmarshal(scanner.Bytes(), &record) == nil && record["msg"] == message {
				return record
			}
		}
		select {
		case <-p.finished:
			t.Fatalf("%s exited (%v) before logging %q\n%s", p.name, p.err, message, p.logs())
		case <-time.After(pollEvery):
		}
	}
	t.Fatalf("%s did not log %q within %v\n%s", p.name, message, e2eTimeout, p.logs())
	return nil
}

func (top *topology) boot(t *testing.T, bin binaries) {
	t.Helper()
	common := map[string]string{"KAFKA_BROKERS": strings.Join(top.brokers, ","), "KAFKA_INSECURE": "true"}
	with := func(extra map[string]string) map[string]string {
		env := map[string]string{}
		for k, v := range common {
			env[k] = v
		}
		for k, v := range extra {
			env[k] = v
		}
		return env
	}
	ordersDSN, reservationsDSN, bookingsDSN := top.ordersDSN, top.reservationsDSN, top.bookingsDSN

	// IDN-03 end to end: the contexts only serve the workloads they trust, over
	// certificates the test mints, so the hop the e2e crosses is the real one.
	pki := tb.NewPKI(t)
	bffCert, bffKey := pki.Client(t, "bff")
	mutual := func(name string) map[string]string {
		cert, key := pki.Server(t, name)
		return map[string]string{
			"GRPC_TLS_CERT_FILE": cert, "GRPC_TLS_KEY_FILE": key,
			"GRPC_CLIENT_CA_FILE": pki.CAFile, "GRPC_TRUSTED_CLIENTS": tb.Identity("bff"),
		}
	}
	probe := mutualProbe(t, pki.CAFile, bffCert, bffKey)

	ordersAPI := start(t, "orders api", bin.orders, with(merge(mutual("orders-api"), map[string]string{
		"PG_DSN": ordersDSN, "MIGRATE": "true", "GRPC_ADDR": "127.0.0.1:0",
		"INSTANCE_ID": "e2e-orders-api", "ITEM_LIMIT": "3",
	})), "--role", "api")
	reservationsAPI := start(t, "reservations api", bin.reservations, with(merge(mutual("reservations-api"), map[string]string{
		"PG_DSN": reservationsDSN, "MIGRATE": "true", "GRPC_ADDR": "127.0.0.1:0",
		"INSTANCE_ID": "e2e-reservations-api",
	})), "--role", "api")
	bookingsAPI := start(t, "bookings api", bin.bookings, with(merge(mutual("bookings-api"), map[string]string{
		"PG_DSN": bookingsDSN, "MIGRATE": "true", "GRPC_ADDR": "127.0.0.1:0",
		"INSTANCE_ID": "e2e-bookings-api",
	})), "--role", "api")
	ordersAddr := ordersAPI.waitLog(t, "grpc listening")["addr"].(string)
	reservationsAddr := reservationsAPI.waitLog(t, "grpc listening")["addr"].(string)
	bookingsAddr := bookingsAPI.waitLog(t, "grpc listening")["addr"].(string)
	waitServing(t, ordersAddr, rpc.OrdersServiceName, probe)
	waitServing(t, reservationsAddr, rpc.ReservationsServiceName, probe)
	waitServing(t, bookingsAddr, rpc.BookingsServiceName, probe)

	start(t, "orders relay", bin.orders, with(map[string]string{
		"PG_DSN": ordersDSN, "KAFKA_ORDERS_TOPIC": top.ordersTopic, "KAFKA_ORDERS_DLQ": top.ordersDLQ,
		"KAFKA_GROUP": top.group, "INSTANCE_ID": "e2e-orders-relay",
	}), "--role", "relay").waitLog(t, "relay draining")
	start(t, "reservations relay", bin.reservations, with(map[string]string{
		"PG_DSN": reservationsDSN, "KAFKA_RESERVATIONS_TOPIC": top.reservationsTopic, "KAFKA_RESERVATIONS_DLQ": top.reservationsDLQ,
		"KAFKA_GROUP": top.group, "INSTANCE_ID": "e2e-reservations-relay",
	}), "--role", "relay").waitLog(t, "relay draining")
	start(t, "bookings relay", bin.bookings, with(map[string]string{
		"PG_DSN": bookingsDSN, "KAFKA_BOOKINGS_TOPIC": top.bookingsTopic, "KAFKA_BOOKINGS_DLQ": top.bookingsDLQ,
		"KAFKA_GROUP": top.group, "INSTANCE_ID": "e2e-bookings-relay",
	}), "--role", "relay").waitLog(t, "relay draining")
	start(t, "reservations consumer", bin.reservations, with(map[string]string{
		"PG_DSN": reservationsDSN, "KAFKA_ORDERS_TOPIC": top.ordersTopic, "KAFKA_ORDERS_DLQ": top.ordersDLQ,
		"KAFKA_GROUP": top.group, "INSTANCE_ID": "e2e-reservations-consumer",
	}), "--role", "consumer").waitLog(t, "consumer joining")

	bff := start(t, "bff", bin.bff, map[string]string{
		"HTTP_ADDR": "127.0.0.1:0", "INSTANCE_ID": "e2e-bff",
		"GRPC_CA_FILE": pki.CAFile, "GRPC_SERVER_NAME": "localhost",
		"GRPC_CLIENT_CERT_FILE": bffCert, "GRPC_CLIENT_KEY_FILE": bffKey,
		"ORDERS_GRPC_TARGET":       "dns:///" + ordersAddr,
		"RESERVATIONS_GRPC_TARGET": "dns:///" + reservationsAddr,
		"BOOKINGS_GRPC_TARGET":     "dns:///" + bookingsAddr,
		"AUTH_DEV_MOCK":            "true",
	})
	top.bffAddr = bff.waitLog(t, "http listening")["addr"].(string)
}

// waitServing is the readiness the contexts declare: they log "grpc listening"
// before Ping and Migrate and turn SERVING only after both.
func waitServing(t *testing.T, addr, service string, creds credentials.TransportCredentials) {
	t.Helper()
	conn, err := grpc.NewClient("dns:///"+addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		t.Fatalf("grpc.NewClient(%s) = %v", addr, err)
	}
	defer conn.Close()
	client := healthpb.NewHealthClient(conn)
	var (
		last    healthpb.HealthCheckResponse_ServingStatus
		lastErr error
	)
	deadline := time.Now().Add(e2eTimeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		res, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
		cancel()
		if err == nil && res.GetStatus() == healthpb.HealthCheckResponse_SERVING {
			return
		}
		last, lastErr = res.GetStatus(), err
		time.Sleep(pollEvery)
	}
	t.Fatalf("%s at %s did not turn SERVING within %v: last status %v, last error %v", service, addr, e2eTimeout, last, lastErr)
}

// mutualProbe presents the edge's own certificate: the contexts answer nothing,
// health included, to a workload they do not trust.
func mutualProbe(t *testing.T, caFile, certFile, keyFile string) credentials.TransportCredentials {
	t.Helper()
	config, err := rpc.ClientTLS(caFile, "localhost", certFile, keyFile)
	if err != nil {
		t.Fatalf("ClientTLS() = %v", err)
	}
	return credentials.NewTLS(config)
}

func merge(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

func (top *topology) waitUntil(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(e2eTimeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(pollEvery)
	}
	t.Fatalf("timed out after %v waiting until %s", e2eTimeout, what)
}
