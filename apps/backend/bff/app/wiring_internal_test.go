package app

import (
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
)

func TestEveryBackendIsDialedUnderItsName(t *testing.T) {
	cfg := Config{OrdersTarget: "passthrough:///orders", ReservationsTarget: "passthrough:///reservations", BookingsTarget: "passthrough:///bookings"}

	conns, err := dialBackends(rpc.Dial, rpc.Options{Insecure: true, Clock: obsclock.System()}, backendsOf(cfg))
	if err != nil {
		t.Fatalf("dialBackends() = %v, want nil", err)
	}
	t.Cleanup(func() { closeBackends(backendsOf(cfg), conns) })

	for _, name := range []string{ordersBackend, reservationsBackend, bookingsBackend} {
		if conns[name] == nil {
			t.Fatalf("conns[%q] = nil, want the dialed connection", name)
		}
	}
}

func TestAFailedDialClosesTheBackendsAlreadyOpen(t *testing.T) {
	refused := errors.New("dial refused")
	var opened []*grpc.ClientConn
	dial := func(target string, cfg kernelgrpc.Config, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
		if len(opened) == 1 {
			return nil, refused
		}
		conn, err := rpc.Dial(target, cfg, extra...)
		if err == nil {
			opened = append(opened, conn)
		}
		return conn, err
	}
	cfg := Config{OrdersTarget: "passthrough:///orders", ReservationsTarget: "passthrough:///reservations", BookingsTarget: "passthrough:///bookings"}

	conns, err := dialBackends(dial, rpc.Options{Insecure: true, Clock: obsclock.System()}, backendsOf(cfg))

	if !errors.Is(err, refused) || conns != nil {
		t.Fatalf("dialBackends() = (%v, %v), want (nil, the dial error)", conns, err)
	}
	if len(opened) != 1 || opened[0].GetState() != connectivity.Shutdown {
		t.Fatalf("opened = %v, want the first backend closed after the second dial failed", opened)
	}
}
