package rpc_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	ordersv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/service/v1"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

func findOrderOver(t *testing.T, target string, opts rpc.Options) error {
	t.Helper()
	conn, err := rpc.Dial(target, rpc.OrdersConfig(opts))
	if err != nil {
		return err
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = rpc.NewOrders(conn).FindOrder(ctx, &ordersv1.FindOrderRequest{OrderId: "o-1"})
	return err
}

func TestAClientWithoutTransportPolicyIsRefused(t *testing.T) {
	_, err := rpc.Dial("127.0.0.1:1", rpc.OrdersConfig(rpc.Options{Clock: obsclock.System()}))

	if !errors.Is(err, kernelgrpc.ErrTLSRequired) {
		t.Fatalf("Dial() = %v, want ErrTLSRequired (GRP-15)", err)
	}
}

func TestTheDevelopmentOptOutConnects(t *testing.T) {
	target := (&fakeContexts{}).serveTCP(t)

	if err := findOrderOver(t, target, rpc.Options{Insecure: true, Clock: obsclock.System()}); err != nil {
		t.Fatalf("FindOrder() over the opt-out = %v, want nil", err)
	}
}

// The edge presents its own certificate, because the contexts demand a
// verified workload before trusting anything it propagates (IDN-03).
func TestATrustedCertificateConnectsPresentingTheEdgesOwn(t *testing.T) {
	p := tb.NewPKI(t)
	serverCert, serverKey := p.Server(t, "orders-api")
	pair, err := tls.LoadX509KeyPair(serverCert, serverKey)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := os.ReadFile(p.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	clients := x509.NewCertPool()
	clients.AppendCertsFromPEM(authority)
	target := (&fakeContexts{}).serveTCP(t, grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{pair}, ClientCAs: clients, ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS12,
	})))

	clientCert, clientKey := p.Client(t, "bff")
	clientTLS, err := rpc.ClientTLS(p.CAFile, "localhost", clientCert, clientKey)
	if err != nil {
		t.Fatalf("ClientTLS() = %v", err)
	}
	if err := findOrderOver(t, target, rpc.Options{TLS: clientTLS, Clock: obsclock.System()}); err != nil {
		t.Fatalf("FindOrder() over mutual TLS = %v, want nil", err)
	}
}

func TestAnUnreadableAuthorityIsRefused(t *testing.T) {
	if _, err := rpc.ClientTLS(filepath.Join(t.TempDir(), "absent.pem"), "", "", ""); err == nil {
		t.Fatal("ClientTLS() with an absent file = nil, want an error")
	}
	empty := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(empty, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	if _, err := rpc.ClientTLS(empty, "", "", ""); err == nil {
		t.Fatal("ClientTLS() with no PEM certificate = nil, want an error")
	}
}
