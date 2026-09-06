// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (GRP-08..10) ou FND-08 (RES-21, RES-22, RES-31) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfgrpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/compose"
)

// ComposeUnaryInterceptor wraps every unary call in the composition of RES-22,
// one resilience.Call per declared method, and never retries a method that did
// not declare itself idempotent (GRP-08, GRP-09); no budget, one attempt (RES-31).
func ComposeUnaryInterceptor(cfg Config) (grpc.UnaryClientInterceptor, error) {
	calls, err := composeCalls(cfg)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		call, declared := calls[method]
		if !declared {
			return fmt.Errorf("%w: %q", ErrMethodNotDeclared, method)
		}
		policy := cfg.Methods[method]
		op := policy.Budget.Operation()
		op.Idempotent = policy.Idempotent

		return call(ctx, op, func(ctx context.Context) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		})
	}, nil
}

// composeCalls builds one composition per method. Breaker and bulkhead are
// one per dependency and shared, because their state is the dependency's;
// the retry decorator is per method, because the transient codes are.
func composeCalls(cfg Config) (map[string]resilience.Call, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cc := composition(cfg)
	shared := compose.Shared(cc)

	calls := make(map[string]resilience.Call, len(cfg.Methods))
	for method, policy := range cfg.Methods {
		slots := shared
		retryDecorator, err := compose.Retry(cc, StatusClassifier(policy.RetryableCodes))
		if err != nil {
			return nil, fmt.Errorf("dmpfgrpc: %s: %w", method, err)
		}
		slots.Retry = retryDecorator

		call, err := resilience.Compose(cfg.Sheet, slots)
		if err != nil {
			return nil, fmt.Errorf("dmpfgrpc: %s: %w", method, err)
		}
		calls[method] = call
	}
	return calls, nil
}
