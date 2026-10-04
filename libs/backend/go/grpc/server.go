// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (GRP-13, GRP-15) ou FND-08 (RES-16) que o símbolo realiza, dentro do limite de 3 linhas.

package grpc

import (
	"crypto/tls"
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/filters"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// ServerConfig is the server side: transport security (GRP-15), the services
// whose health is reported individually (GRP-13) and the interceptors — the
// admission of RES-16 first among them — applied to every unary call.
type ServerConfig struct {
	TLS                        *tls.Config
	InsecureForDevelopmentOnly bool
	Services                   []string
	UnaryInterceptors          []grpc.UnaryServerInterceptor
	StreamInterceptors         []grpc.StreamServerInterceptor
	LoggerProvider             log.LoggerProvider
	TracerProvider             trace.TracerProvider
	MeterProvider              metric.MeterProvider
	Propagator                 propagation.TextMapPropagator
}

// Validate refuses a server without transport security and without the
// explicit development-only opt-out (GRP-15), and a TLS server that does not
// verify its callers' certificates (IDN-03).
func (c ServerConfig) Validate() error {
	if err := validateTLS(c.TLS, c.InsecureForDevelopmentOnly); err != nil {
		return err
	}
	if c.TLS != nil && c.TLS.ClientAuth != tls.RequireAndVerifyClientCert {
		return ErrClientCARequired
	}
	return nil
}

// NewServer builds the server with its credentials, interceptor chains and the
// health service, every declared service starting as NOT_SERVING until the
// composition root says otherwise. Extra options are appended.
func NewServer(cfg ServerConfig, extra ...grpc.ServerOption) (*grpc.Server, *health.Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	logger := loggerOf(cfg.LoggerProvider)
	traffic := filters.None(filters.HealthCheck(), filters.ServicePrefix("grpc.reflection."))
	options := []grpc.ServerOption{
		grpc.Creds(serverCredentials(cfg)),
		grpc.StatsHandler(otelgrpc.NewServerHandler(
			otelgrpc.WithTracerProvider(cfg.TracerProvider),
			otelgrpc.WithMeterProvider(cfg.MeterProvider),
			otelgrpc.WithPropagators(cfg.Propagator),
			otelgrpc.WithFilter(traffic),
		)),
		grpc.StatsHandler(accessLog{logger: logger, filter: traffic}),
		grpc.ChainUnaryInterceptor(append([]grpc.UnaryServerInterceptor{recoverUnary(logger)}, cfg.UnaryInterceptors...)...),
		grpc.ChainStreamInterceptor(append([]grpc.StreamServerInterceptor{recoverStream(logger)}, cfg.StreamInterceptors...)...),
	}
	options = append(options, extra...)

	server := grpc.NewServer(options...)
	healthServer := health.NewServer()
	for _, service := range cfg.Services {
		healthServer.SetServingStatus(service, healthpb.HealthCheckResponse_NOT_SERVING)
	}
	healthpb.RegisterHealthServer(server, healthServer)
	return server, healthServer, nil
}

var insecureServerWarning = new(sync.Once)

func serverCredentials(c ServerConfig) credentials.TransportCredentials {
	if c.TLS != nil {
		return credentials.NewTLS(c.TLS)
	}
	insecureServerWarning.Do(func() {
		loggerOf(c.LoggerProvider).Warn("grpc: server without TLS by explicit development-only opt-out (GRP-15)")
	})
	return insecure.NewCredentials()
}
