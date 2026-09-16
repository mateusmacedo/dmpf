package rpc_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	ordersv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/service/v1"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/rpc"
)

func selfSigned(t *testing.T) (tls.Certificate, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() = %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey() = %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	cert, err := tls.X509KeyPair(certPEM, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatalf("X509KeyPair() = %v", err)
	}
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, certPEM, 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	return cert, caFile
}

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

	if !errors.Is(err, provider.ErrTLSRequired) {
		t.Fatalf("Dial() = %v, want ErrTLSRequired (GRP-15)", err)
	}
}

func TestTheDevelopmentOptOutConnects(t *testing.T) {
	target := (&fakeContexts{}).serveTCP(t)

	if err := findOrderOver(t, target, rpc.Options{Insecure: true, Clock: obsclock.System()}); err != nil {
		t.Fatalf("FindOrder() over the opt-out = %v, want nil", err)
	}
}

func TestATrustedCertificateConnects(t *testing.T) {
	cert, caFile := selfSigned(t)
	target := (&fakeContexts{}).serveTCP(t, grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})))

	clientTLS, err := rpc.ClientTLS(caFile, "localhost")
	if err != nil {
		t.Fatalf("ClientTLS() = %v", err)
	}
	if err := findOrderOver(t, target, rpc.Options{TLS: clientTLS, Clock: obsclock.System()}); err != nil {
		t.Fatalf("FindOrder() over TLS = %v, want nil", err)
	}
}

func TestAnUnreadableAuthorityIsRefused(t *testing.T) {
	if _, err := rpc.ClientTLS(filepath.Join(t.TempDir(), "absent.pem"), ""); err == nil {
		t.Fatal("ClientTLS() with an absent file = nil, want an error")
	}
	empty := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(empty, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	if _, err := rpc.ClientTLS(empty, ""); err == nil {
		t.Fatal("ClientTLS() with no PEM certificate = nil, want an error")
	}
}
