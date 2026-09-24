package tb_test

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

func TestTheIssuedPairsChainToTheAuthority(t *testing.T) {
	p := tb.NewPKI(t)
	authority, err := os.ReadFile(p.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(authority)

	for name, pair := range map[string][2]string{"server": pairOf(p.Server(t, "orders-api")), "client": pairOf(p.Client(t, "bff"))} {
		loaded, err := tls.LoadX509KeyPair(pair[0], pair[1])
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		leaf, err := x509.ParseCertificate(loaded.Certificate[0])
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
			t.Fatalf("%s does not chain to the authority: %v", name, err)
		}
		if name == "client" && (len(leaf.URIs) != 1 || leaf.URIs[0].String() != tb.Identity("bff")) {
			t.Fatalf("client identity = %v, want %s", leaf.URIs, tb.Identity("bff"))
		}
	}
}

func pairOf(cert, key string) [2]string { return [2]string{cert, key} }
