package rpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

func ClientTLS(caFile, serverName string) (*tls.Config, error) {
	authority, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("rpc: read certificate authority: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(authority) {
		return nil, fmt.Errorf("rpc: %s holds no PEM certificate", caFile)
	}
	return &tls.Config{RootCAs: pool, ServerName: serverName, MinVersion: tls.VersionTLS12}, nil
}
