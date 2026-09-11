package dmpfgrpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/resilience"
	dmpfgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/deadline"
)

// routeConfig is the client configuration each hop uses to call the next one,
// with the system clock because the route crosses real bufconn transports.
func routeConfig(limit, slack time.Duration) dmpfgrpc.Config {
	return dmpfgrpc.Config{
		InsecureForDevelopmentOnly: true,
		Sheet:                      resilience.Defaults("next-hop"),
		Clock:                      clock.System(),
		Methods: map[string]dmpfgrpc.MethodPolicy{
			checkMethod: {
				Budget: deadline.Budget{
					Dependency:        "next-hop",
					Method:            checkMethod,
					Limit:             limit,
					Slack:             slack,
					EstimatedDuration: limit / 4,
				},
				Idempotent: true,
			},
		},
	}
}

// chain builds A → B → C: C is a plain health server, B forwards to C through
// the deadline interceptor, and the returned client is A's view of B. Every
// hop reports the deadline it received on the channel.
func chain(t *testing.T, cfg dmpfgrpc.Config, blockC <-chan struct{}) (healthpb.HealthClient, chan deadlineSeen) {
	t.Helper()
	seen := make(chan deadlineSeen, 8)

	hopC := &hop{name: "C", deadlines: seen, block: blockC}
	dialC := serve(t, hopC)
	connC := connect(t, dialC, grpc.WithChainUnaryInterceptor(dmpfgrpc.DeadlineUnaryInterceptor(cfg)))

	hopB := &hop{name: "B", deadlines: seen, next: healthpb.NewHealthClient(connC)}
	dialB := serve(t, hopB)
	connB := connect(t, dialB, grpc.WithChainUnaryInterceptor(dmpfgrpc.DeadlineUnaryInterceptor(cfg)))

	return healthpb.NewHealthClient(connB), seen
}

func collect(t *testing.T, seen chan deadlineSeen, n int) map[string]deadlineSeen {
	t.Helper()
	out := map[string]deadlineSeen{}
	for range n {
		select {
		case s := <-seen:
			out[s.hop] = s
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d of %d hops reported a deadline", len(out), n)
		}
	}
	return out
}

func TestTwoHopsNeverRestartTheDeadline(t *testing.T) {
	client, seen := chain(t, routeConfig(1500*time.Millisecond, 100*time.Millisecond), nil)

	ctxA, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	deadlineA, _ := ctxA.Deadline()

	if _, err := client.Check(ctxA, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}
	hops := collect(t, seen, 2)

	b, c := hops["B"], hops["C"]
	if !b.ok || !c.ok {
		t.Fatalf("a hop received no deadline: B=%v C=%v (GRP-04)", b.ok, c.ok)
	}
	if c.deadline >= b.deadline || b.deadline >= deadlineA.UnixNano() {
		t.Fatalf("deadlines are not strictly decreasing along the route: A=%d B=%d C=%d (GRP-05, GRP-17)",
			deadlineA.UnixNano(), b.deadline, c.deadline)
	}
	// Each hop keeps its slack, within the transit tolerance: the wire carries
	// the remaining duration (GRP-06) and the receiver rebuilds the instant on
	// arrival, so the gap it observes is the slack minus the transit time.
	const transit = 20 * time.Millisecond
	gapAB, gapBC := time.Duration(deadlineA.UnixNano()-b.deadline), time.Duration(b.deadline-c.deadline)
	if gapAB < 100*time.Millisecond-transit || gapBC < 100*time.Millisecond-transit {
		t.Fatalf("a hop kept less than its slack: A−B=%v B−C=%v", gapAB, gapBC)
	}
}

func TestALargerLimitNeverExtendsTheRoute(t *testing.T) {
	client, seen := chain(t, routeConfig(10*time.Second, 100*time.Millisecond), nil)

	ctxA, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	deadlineA, _ := ctxA.Deadline()

	if _, err := client.Check(ctxA, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}
	hops := collect(t, seen, 2)
	for _, name := range []string{"B", "C"} {
		if hops[name].deadline >= deadlineA.UnixNano() {
			t.Fatalf("hop %s deadline %d is not below A's %d: a 10s limit extended the route (GRP-16)", name, hops[name].deadline, deadlineA.UnixNano())
		}
	}
}

func TestCancellationInAIsObservedInC(t *testing.T) {
	blockC := make(chan struct{})
	client, seen := chain(t, routeConfig(5*time.Second, 100*time.Millisecond), blockC)

	ctxA, cancelA := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelA()

	done := make(chan error, 1)
	go func() {
		_, err := client.Check(ctxA, &healthpb.HealthCheckRequest{})
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	cancelA()

	select {
	case err := <-done:
		if status.Code(err) != codes.Canceled && !errors.Is(err, context.Canceled) {
			t.Fatalf("Check() = %v, want a cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("A did not return after cancelling")
	}

	hops := collect(t, seen, 2)
	if !errors.Is(hops["C"].err, context.Canceled) && status.Code(hops["C"].err) != codes.Canceled {
		t.Fatalf("C did not observe the cancellation: %v (GRP-07)", hops["C"].err)
	}
	close(blockC)
}
