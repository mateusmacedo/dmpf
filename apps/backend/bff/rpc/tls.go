package rpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

func ClientTLS(caFile, serverName, certFile, keyFile string) (*tls.Config, error) {
	authority, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("rpc: read certificate authority: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(authority) {
		return nil, fmt.Errorf("rpc: %s holds no PEM certificate", caFile)
	}
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("rpc: client certificate: %w", err)
	}
	return &tls.Config{RootCAs: pool, ServerName: serverName, Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}, nil
}
