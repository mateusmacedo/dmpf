package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var (
	// ErrClientCARequired is a TLS server that would not authenticate its
	// callers: without a verified workload no boundary is trusted (IDN-03).
	ErrClientCARequired = errors.New("grpc: a TLS server must verify client certificates against a declared authority (IDN-03)")

	// ErrTrustedClientsRequired is a TLS server that declares no workload
	// allowed to call it, which would admit any certificate the authority signed.
	ErrTrustedClientsRequired = errors.New("grpc: a TLS server must declare the workloads it trusts (IDN-03)")
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
	CertFile       string
	KeyFile        string
	Insecure       bool
	ClientCAFile   string
	TrustedClients []string
	Services       []string
	Interceptors   []grpc.UnaryServerInterceptor
	Logger         *slog.Logger
}

// APIServerConfig turns that declaration into the server configuration. The
// opt-out only applies when no pair was declared, so a process with a
// certificate never falls back to plaintext by a stray variable.
func APIServerConfig(api APIServer) (ServerConfig, error) {
	tlsConfig, err := ServerTLS(api.CertFile, api.KeyFile)
	if err != nil {
		return ServerConfig{}, err
	}
	interceptors := api.Interceptors
	var streams []grpc.StreamServerInterceptor
	if tlsConfig != nil {
		if err := requireClientAuth(tlsConfig, api.ClientCAFile, api.TrustedClients); err != nil {
			return ServerConfig{}, err
		}
		interceptors = append([]grpc.UnaryServerInterceptor{TrustedPeers(api.TrustedClients)}, interceptors...)
		streams = []grpc.StreamServerInterceptor{TrustedStreamPeers(api.TrustedClients)}
	}
	return ServerConfig{
		TLS:                        tlsConfig,
		InsecureForDevelopmentOnly: tlsConfig == nil && api.Insecure,
		Services:                   api.Services,
		UnaryInterceptors:          interceptors,
		StreamInterceptors:         streams,
		Logger:                     api.Logger,
	}, nil
}

func requireClientAuth(config *tls.Config, caFile string, trusted []string) error {
	switch {
	case caFile == "":
		return ErrClientCARequired
	case len(trusted) == 0:
		return ErrTrustedClientsRequired
	}
	authority, err := os.ReadFile(caFile)
	if err != nil {
		return fmt.Errorf("grpc client ca: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(authority) {
		return fmt.Errorf("grpc client ca: %s holds no PEM certificate", caFile)
	}
	config.ClientCAs = pool
	config.ClientAuth = tls.RequireAndVerifyClientCert
	return nil
}

// TrustedPeers admits a call only from a workload whose verified certificate
// names one of the trusted identities by URI or DNS SAN. It runs first, so
// nothing downstream reads metadata from an unverified caller.
func TrustedPeers(trusted []string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !trustedPeer(ctx, trusted) {
			return nil, status.Error(codes.PermissionDenied, "the calling workload is not trusted")
		}
		return handler(ctx, req)
	}
}

// TrustedStreamPeers is TrustedPeers for streaming calls.
func TrustedStreamPeers(trusted []string) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !trustedPeer(stream.Context(), trusted) {
			return status.Error(codes.PermissionDenied, "the calling workload is not trusted")
		}
		return handler(srv, stream)
	}
}

func trustedPeer(ctx context.Context, trusted []string) bool {
	caller, ok := peer.FromContext(ctx)
	if !ok {
		return false
	}
	info, ok := caller.AuthInfo.(credentials.TLSInfo)
	if !ok || len(info.State.VerifiedChains) == 0 || len(info.State.VerifiedChains[0]) == 0 {
		return false
	}
	leaf := info.State.VerifiedChains[0][0]
	identities := slices.Clone(leaf.DNSNames)
	for _, uri := range leaf.URIs {
		identities = append(identities, uri.String())
	}
	for _, identity := range identities {
		if identity != "" && slices.Contains(trusted, identity) {
			return true
		}
	}
	return false
}
