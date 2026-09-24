// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (GRP-08..13, GRP-15) que o símbolo realiza, dentro do limite de 3 linhas.

package grpc

import (
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/health" // registers the client-side health check the service config enables
)

// ServiceConfig is the client service config: round-robin balancing (GRP-12)
// and the health check of the named service (GRP-13). pick_first would inhibit
// the health check, which is why round_robin is not a default left to grpc.
func ServiceConfig(healthServiceName string) string {
	config := map[string]any{
		"loadBalancingConfig": []map[string]any{{"round_robin": map[string]any{}}},
		"healthCheckConfig":   map[string]any{"serviceName": healthServiceName},
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		panic(fmt.Sprintf("grpc: service config is not encodable: %v", err))
	}
	return string(encoded)
}

// Dial opens the client of one dependency: credentials (GRP-15), the service
// config above, grpc's own retry disabled because it cannot see idempotency
// (GRP-08), and the deadline and composition interceptors. Extras are appended.
func Dial(target string, cfg Config, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	compose, err := ComposeUnaryInterceptor(cfg)
	if err != nil {
		return nil, err
	}

	options := []grpc.DialOption{
		grpc.WithTransportCredentials(transportCredentials(cfg)),
		grpc.WithDefaultServiceConfig(ServiceConfig(cfg.HealthServiceName)),
		grpc.WithDisableRetry(),
		grpc.WithChainUnaryInterceptor(DeadlineUnaryInterceptor(cfg), compose),
		grpc.WithChainStreamInterceptor(DeadlineStreamInterceptor(cfg)),
	}
	options = append(options, extra...)

	conn, err := grpc.NewClient(target, options...)
	if err != nil {
		return nil, fmt.Errorf("grpc: %s: %w", cfg.Sheet.Dependency, err)
	}
	return conn, nil
}
