package tb

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// PKI is a throwaway certificate authority for a test, writing PEM files the
// way a process reads them from a mounted secret. No key outlives the test.
type PKI struct {
	dir    string
	ca     *x509.Certificate
	caKey  *ecdsa.PrivateKey
	serial atomic.Int64
	CAFile string
}

// NewPKI creates the authority under t.TempDir.
func NewPKI(t testing.TB) *PKI {
	t.Helper()
	key := newKey(t)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "dmpf-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("tb.NewPKI: %v", err)
	}
	ca, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("tb.NewPKI: %v", err)
	}
	p := &PKI{dir: t.TempDir(), ca: ca, caKey: key}
	p.serial.Store(1)
	p.CAFile = p.write(t, "ca.crt", "CERTIFICATE", der)
	return p
}

// Server issues a server pair valid for localhost, the loopback address and
// the given host names.
func (p *PKI) Server(t testing.TB, name string, hosts ...string) (certFile, keyFile string) {
	t.Helper()
	template := p.leaf(name)
	template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	template.DNSNames = append([]string{"localhost", name}, hosts...)
	template.IPAddresses = []net.IP{net.IPv4(127, 0, 0, 1)}
	return p.sign(t, name, template)
}

// Client issues a client pair whose identity is the URI SAN spiffe://dmpf/<name>,
// the form a server's trusted-workload allowlist names.
func (p *PKI) Client(t testing.TB, name string) (certFile, keyFile string) {
	t.Helper()
	template := p.leaf(name)
	template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	template.URIs = []*url.URL{{Scheme: "spiffe", Host: "dmpf", Path: "/" + name}}
	return p.sign(t, name+"-client", template)
}

// Identity is the URI SAN Client puts in the certificate for name.
func Identity(name string) string { return "spiffe://dmpf/" + name }

func (p *PKI) leaf(name string) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(p.serial.Add(1)),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
}

func (p *PKI) sign(t testing.TB, file string, template *x509.Certificate) (string, string) {
	t.Helper()
	key := newKey(t)
	der, err := x509.CreateCertificate(rand.Reader, template, p.ca, &key.PublicKey, p.caKey)
	if err != nil {
		t.Fatalf("tb.PKI: sign %s: %v", file, err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("tb.PKI: key %s: %v", file, err)
	}
	return p.write(t, file+".crt", "CERTIFICATE", der), p.write(t, file+".key", "EC PRIVATE KEY", keyDER)
}

func (p *PKI) write(t testing.TB, name, kind string, der []byte) string {
	t.Helper()
	path := filepath.Join(p.dir, name)
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0o600); err != nil {
		t.Fatalf("tb.PKI: write %s: %v", name, err)
	}
	return path
}

func newKey(t testing.TB) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("tb.PKI: key: %v", err)
	}
	return key
}
