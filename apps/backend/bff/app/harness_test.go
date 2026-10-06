//go:build integration

package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const (
	e2eTimeout  = 90 * time.Second
	pollEvery   = 100 * time.Millisecond
	adminWindow = 30 * time.Second
	stopGrace   = 20 * time.Second

	portAttempts = 3

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

	processes []*process
	pick      func(*testing.T) string
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
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	top := &topology{
		brokers:           brokers,
		ordersTopic:       "e2e-orders-" + suffix,
		ordersDLQ:         "e2e-orders-" + suffix + "-dlq",
		reservationsTopic: "e2e-reservations-" + suffix,
		reservationsDLQ:   "e2e-reservations-" + suffix + "-dlq",
		group:             "e2e-reservations-group-" + suffix,
		bookingsTopic:     "e2e-bookings-" + suffix,
		bookingsDLQ:       "e2e-bookings-" + suffix + "-dlq",
	}
	top.ordersDSN = pg.DSN(t, "e2e_orders")
	top.reservationsDSN = pg.DSN(t, "e2e_reservations")
	top.bookingsDSN = pg.DSN(t, "e2e_bookings")

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
		_, _ = top.admin.DeleteGroups(ctx, top.group)
	})
	return top
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

func (top *topology) start(t *testing.T, name, binary string, env map[string]string, args ...string) *process {
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
	top.processes = append(top.processes, p)
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

func (top *topology) exited() *process {
	for _, p := range top.processes {
		select {
		case <-p.finished:
			return p
		default:
		}
	}
	return nil
}

func (top *topology) await(what string, ready func() error) error {
	var last error
	deadline := time.Now().Add(e2eTimeout)
	for time.Now().Before(deadline) {
		if last = ready(); last == nil {
			return nil
		}
		if p := top.exited(); p != nil {
			return fmt.Errorf("%s exited (%v) before %s\n%s", p.name, p.err, what, p.logs())
		}
		time.Sleep(pollEvery)
	}
	return fmt.Errorf("%s did not happen within %v: %v", what, e2eTimeout, last)
}

func (top *topology) startOnFreePort(t *testing.T, name, binary, addrKey string, env map[string]string, ready func(addr string) error, args ...string) string {
	t.Helper()
	return top.startOnFreePorts(t, name, binary, []string{addrKey}, env, func(addrs map[string]string) error {
		return ready(addrs[addrKey])
	}, args...)[addrKey]
}

func (top *topology) startOnFreePorts(t *testing.T, name, binary string, addrKeys []string, env map[string]string, ready func(addrs map[string]string) error, args ...string) map[string]string {
	t.Helper()
	for attempt := 1; ; attempt++ {
		addrs := map[string]string{}
		for _, key := range addrKeys {
			addrs[key] = top.pickAddr(t)
		}
		p := top.start(t, name, binary, merge(env, addrs), args...)
		err := ready(addrs)
		if err == nil {
			return addrs
		}
		if attempt == portAttempts || !p.lostItsPort() {
			t.Fatal(err)
		}
		top.processes = slices.DeleteFunc(top.processes, func(q *process) bool { return q == p })
	}
}

func (p *process) lostItsPort() bool {
	select {
	case <-p.finished:
		return strings.Contains(p.logs(), syscall.EADDRINUSE.Error())
	default:
		return false
	}
}

func (top *topology) pickAddr(t *testing.T) string {
	t.Helper()
	if top.pick != nil {
		return top.pick(t)
	}
	return freeAddr(t)
}

func freeAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().String()
}

func (top *topology) boot(t *testing.T, bin binaries) {
	t.Helper()
	quiet := quietTelemetry(t)
	common := merge(quiet, map[string]string{"KAFKA_BROKERS": strings.Join(top.brokers, ","), "KAFKA_INSECURE": "true"})
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
	serving := func(service string) func(addr string) error {
		return func(addr string) error { return top.waitServing(addr, service, probe) }
	}

	ordersAddr := top.startOnFreePort(t, "orders api", bin.orders, "GRPC_ADDR", with(merge(mutual("orders-api"), map[string]string{
		"PG_DSN": ordersDSN, "MIGRATE": "true",
		"OTEL_RESOURCE_ATTRIBUTES": "service.version=e2e,service.instance.id=e2e-orders-api", "ITEM_LIMIT": "3",
	})), serving(rpc.OrdersServiceName), "--role", "api")
	reservationsAddr := top.startOnFreePort(t, "reservations api", bin.reservations, "GRPC_ADDR", with(merge(mutual("reservations-api"), map[string]string{
		"PG_DSN": reservationsDSN, "MIGRATE": "true",
		"OTEL_RESOURCE_ATTRIBUTES": "service.version=e2e,service.instance.id=e2e-reservations-api",
	})), serving(rpc.ReservationsServiceName), "--role", "api")
	bookingsAddr := top.startOnFreePort(t, "bookings api", bin.bookings, "GRPC_ADDR", with(merge(mutual("bookings-api"), map[string]string{
		"PG_DSN": bookingsDSN, "MIGRATE": "true",
		"OTEL_RESOURCE_ATTRIBUTES": "service.version=e2e,service.instance.id=e2e-bookings-api",
	})), serving(rpc.BookingsServiceName), "--role", "api")

	top.start(t, "orders relay", bin.orders, with(map[string]string{
		"PG_DSN": ordersDSN, "KAFKA_ORDERS_TOPIC": top.ordersTopic, "KAFKA_ORDERS_DLQ": top.ordersDLQ,
		"KAFKA_GROUP": top.group, "OTEL_RESOURCE_ATTRIBUTES": "service.version=e2e,service.instance.id=e2e-orders-relay",
	}), "--role", "relay")
	top.start(t, "reservations relay", bin.reservations, with(map[string]string{
		"PG_DSN": reservationsDSN, "KAFKA_RESERVATIONS_TOPIC": top.reservationsTopic, "KAFKA_RESERVATIONS_DLQ": top.reservationsDLQ,
		"KAFKA_GROUP": top.group, "OTEL_RESOURCE_ATTRIBUTES": "service.version=e2e,service.instance.id=e2e-reservations-relay",
	}), "--role", "relay")
	top.start(t, "bookings relay", bin.bookings, with(map[string]string{
		"PG_DSN": bookingsDSN, "KAFKA_BOOKINGS_TOPIC": top.bookingsTopic, "KAFKA_BOOKINGS_DLQ": top.bookingsDLQ,
		"KAFKA_GROUP": top.group, "OTEL_RESOURCE_ATTRIBUTES": "service.version=e2e,service.instance.id=e2e-bookings-relay",
	}), "--role", "relay")
	top.start(t, "reservations consumer", bin.reservations, with(map[string]string{
		"PG_DSN": reservationsDSN, "KAFKA_ORDERS_TOPIC": top.ordersTopic, "KAFKA_ORDERS_DLQ": top.ordersDLQ,
		"KAFKA_GROUP": top.group, "OTEL_RESOURCE_ATTRIBUTES": "service.version=e2e,service.instance.id=e2e-reservations-consumer",
	}), "--role", "consumer")

	bffAddrs := top.startOnFreePorts(t, "bff", bin.bff, []string{"HTTP_ADDR", "ADMIN_ADDR"}, merge(quiet, map[string]string{
		"OTEL_RESOURCE_ATTRIBUTES": "service.version=e2e,service.instance.id=e2e-bff",
		"GRPC_CA_FILE":             pki.CAFile, "GRPC_SERVER_NAME": "localhost",
		"GRPC_CLIENT_CERT_FILE": bffCert, "GRPC_CLIENT_KEY_FILE": bffKey,
		"ORDERS_GRPC_TARGET":       "dns:///" + ordersAddr,
		"RESERVATIONS_GRPC_TARGET": "dns:///" + reservationsAddr,
		"BOOKINGS_GRPC_TARGET":     "dns:///" + bookingsAddr,
		"AUTH_DEV_MOCK":            "true",
	}), func(addrs map[string]string) error {
		return top.await("bff ready", func() error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			return app.Probe(ctx, app.Config{AdminAddr: addrs["ADMIN_ADDR"]})
		})
	})
	top.bffAddr = bffAddrs["HTTP_ADDR"]
}

func quietTelemetry(t *testing.T) map[string]string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	var accepted atomic.Int64
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		if got := accepted.Load(); got != 0 {
			t.Errorf("the topology opened %d connections to the OTLP endpoint, want none with every exporter declared none", got)
		}
	})
	return map[string]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + listener.Addr().String(),
		"OTEL_TRACES_EXPORTER":        "none",
		"OTEL_METRICS_EXPORTER":       "none",
		"OTEL_LOGS_EXPORTER":          "none",
	}
}

// waitServing is the readiness the contexts declare: they log "grpc listening"
// before Ping and Migrate and turn SERVING only after both.
func (top *topology) waitServing(addr, service string, creds credentials.TransportCredentials) error {
	conn, err := grpc.NewClient("dns:///"+addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return fmt.Errorf("grpc.NewClient(%s) = %w", addr, err)
	}
	defer conn.Close()
	client := healthpb.NewHealthClient(conn)
	return top.await(service+" at "+addr+" turning SERVING", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		res, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
		if err == nil && res.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			err = fmt.Errorf("status %v", res.GetStatus())
		}
		return err
	})
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
	err := top.await(what, func() error {
		if condition() {
			return nil
		}
		return errNotYet
	})
	if err != nil {
		t.Fatal(err)
	}
}

var errNotYet = errors.New("the condition does not hold yet")

func TestAProcessThatDiesAtStartFailsTheBootAtOnce(t *testing.T) {
	top := &topology{}
	top.start(t, "dying relay", "/bin/sh", nil, "-c", "echo refused >&2; exit 3")
	began := time.Now()

	err := top.await("the dying relay draining", func() error { return errNotYet })

	if err == nil || !strings.Contains(err.Error(), "dying relay exited") || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("await() = %v, want the death of the process named with its output", err)
	}
	if elapsed := time.Since(began); elapsed > 10*time.Second {
		t.Fatalf("await() took %v, want the death noticed long before the %v timeout", elapsed, e2eTimeout)
	}
}

const (
	holderAddrKey  = "HARNESS_HOLDER_ADDR"
	holderGreeting = "holding"
)

func TestAProcessThatLosesItsPortStartsAgainOnAnother(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	t.Cleanup(func() { _ = taken.Close() })
	picked := false
	top := &topology{pick: func(t *testing.T) string {
		if picked {
			return freeAddr(t)
		}
		picked = true
		return taken.Addr().String()
	}}

	addr := top.startOnFreePort(t, "holder", os.Args[0], holderAddrKey, nil, func(addr string) error {
		return top.await("holder greeting at "+addr, func() error { return greeting(addr) })
	}, "-test.run=^TestHoldingTheAddressItWasGiven$")

	if addr == taken.Addr().String() {
		t.Fatalf("startOnFreePort() = %s, want a port other than the one taken before the bind", addr)
	}
	if p := top.exited(); p != nil {
		t.Fatalf("%s still counts as exited after starting again on %s", p.name, addr)
	}
}

func TestHoldingTheAddressItWasGiven(t *testing.T) {
	addr := os.Getenv(holderAddrKey)
	if addr == "" {
		t.Skip("runs only as the process TestAProcessThatLosesItsPortStartsAgainOnAnother starts")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_, _ = io.WriteString(conn, holderGreeting)
			_ = conn.Close()
		}
	}()
	terminated := make(chan os.Signal, 1)
	signal.Notify(terminated, syscall.SIGTERM)
	<-terminated
	_ = listener.Close()
}

func greeting(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		return err
	}
	got, err := io.ReadAll(conn)
	if err != nil {
		return err
	}
	if string(got) != holderGreeting {
		return fmt.Errorf("greeting = %q, want %q", got, holderGreeting)
	}
	return nil
}
