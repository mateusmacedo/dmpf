package grpc

import (
	"crypto/tls"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
)

// HealthServices is what a server declares to the health protocol: the empty
// name, which is the overall status a probe without a service asks for, plus
// the service itself.
func HealthServices(name string) []string { return []string{"", name} }

// ServerTLS loads the pair the process declares, or answers nil when it
// declares none — the caller then has to opt out explicitly, because NewServer
// refuses a server without transport security and without the opt-out (GRP-15).
func ServerTLS(certFile, keyFile string) (*tls.Config, error) {
	if certFile == "" {
		return nil, nil
	}
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("grpc tls: %w", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}, nil
}

// APIServer declares what a process needs to serve its API: where its
// credentials are, whether it opts out of them, and the interceptors and
// services that belong to the context.
type APIServer struct {
	CertFile     string
	KeyFile      string
	Insecure     bool
	Services     []string
	Interceptors []grpc.UnaryServerInterceptor
	Logger       *slog.Logger
}

// APIServerConfig turns that declaration into the server configuration. The
// opt-out only applies when no pair was declared, so a process with a
// certificate never falls back to plaintext by a stray variable.
func APIServerConfig(api APIServer) (ServerConfig, error) {
	tlsConfig, err := ServerTLS(api.CertFile, api.KeyFile)
	if err != nil {
		return ServerConfig{}, err
	}
	return ServerConfig{
		TLS:                        tlsConfig,
		InsecureForDevelopmentOnly: tlsConfig == nil && api.Insecure,
		Services:                   api.Services,
		UnaryInterceptors:          api.Interceptors,
		Logger:                     api.Logger,
	}, nil
}
