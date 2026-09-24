package grpc_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
)

func mutualServer(t *testing.T, p *pki, trusted ...string) string {
	t.Helper()
	certFile, keyFile := p.issue(t, "orders-api", false)
	config, err := kernel.APIServerConfig(kernel.APIServer{
		CertFile: certFile, KeyFile: keyFile, ClientCAFile: p.CAFile, TrustedClients: trusted,
		Services: kernel.HealthServices("svc"),
	})
	if err != nil {
		t.Fatalf("APIServerConfig() = %v", err)
	}
	server, health, err := kernel.NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return listener.Addr().String()
}

func check(t *testing.T, p *pki, addr string, clientName string) error {
	t.Helper()
	authority, err := os.ReadFile(p.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(authority)
	config := &tls.Config{RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12}
	if clientName != "" {
		certFile, keyFile := p.issue(t, clientName, true)
		pair, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			t.Fatal(err)
		}
		config.Certificates = []tls.Certificate{pair}
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	return err
}

// IDN-03: a trusted boundary is one whose workload identity is verified. The
// server demands the client's certificate and admits only the workloads it
// declares, so a caller that merely reaches the port asserts nothing.
func TestAMutualServerServesOnlyTheWorkloadsItTrusts(t *testing.T) {
	p := newPKI(t)
	addr := mutualServer(t, p, "spiffe://dmpf/bff")

	if err := check(t, p, addr, "bff"); err != nil {
		t.Fatalf("the trusted workload = %v, want served", err)
	}
	if err := check(t, p, addr, "intruder"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("a workload outside the allowlist = %v, want PermissionDenied", err)
	}
	if err := check(t, p, addr, ""); err == nil {
		t.Fatal("a client without a certificate was served; the handshake must refuse it")
	}
}

func TestAPIServerConfigRefusesTLSWithoutClientAuthentication(t *testing.T) {
	p := newPKI(t)
	certFile, keyFile := p.issue(t, "orders-api", false)

	if _, err := kernel.APIServerConfig(kernel.APIServer{CertFile: certFile, KeyFile: keyFile, TrustedClients: []string{"spiffe://dmpf/bff"}}); !errors.Is(err, kernel.ErrClientCARequired) {
		t.Fatalf("APIServerConfig() without a client CA = %v, want ErrClientCARequired", err)
	}
	if _, err := kernel.APIServerConfig(kernel.APIServer{CertFile: certFile, KeyFile: keyFile, ClientCAFile: p.CAFile}); !errors.Is(err, kernel.ErrTrustedClientsRequired) {
		t.Fatalf("APIServerConfig() without trusted clients = %v, want ErrTrustedClientsRequired", err)
	}
}

func watch(t *testing.T, p *pki, addr string, clientName string) error {
	t.Helper()
	authority, err := os.ReadFile(p.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(authority)
	certFile, keyFile := p.issue(t, clientName, true)
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	config := &tls.Config{RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := healthpb.NewHealthClient(conn).Watch(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		return err
	}
	_, err = stream.Recv()
	return err
}

func TestAMutualServerRefusesAnUntrustedWorkloadOnAStream(t *testing.T) {
	p := newPKI(t)
	addr := mutualServer(t, p, "spiffe://dmpf/bff")

	if err := watch(t, p, addr, "bff"); err != nil {
		t.Fatalf("the trusted workload on a stream = %v, want served", err)
	}
	if err := watch(t, p, addr, "intruder"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("a workload outside the allowlist on a stream = %v, want PermissionDenied", err)
	}
}

// The common name carries no naming semantics a CA is bound to check, so a
// certificate is trusted by its SANs alone.
func TestAMutualServerIgnoresTheCommonName(t *testing.T) {
	p := newPKI(t)
	addr := mutualServer(t, p, "bff")

	if err := check(t, p, addr, "bff"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("a workload trusted only by its common name = %v, want PermissionDenied", err)
	}
}

func TestServerConfigRefusesTLSThatDoesNotVerifyTheClient(t *testing.T) {
	p := newPKI(t)
	certFile, keyFile := p.issue(t, "orders-api", false)
	serverTLS, err := kernel.ServerTLS(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}

	if err := (kernel.ServerConfig{TLS: serverTLS}).Validate(); !errors.Is(err, kernel.ErrClientCARequired) {
		t.Fatalf("Validate() with server-only TLS = %v, want ErrClientCARequired", err)
	}
}
