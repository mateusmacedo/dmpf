package dmpfgrpc_test

import (
	"crypto/tls"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	dmpfgrpc "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-grpc"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/deadline"
)

var start = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

const checkMethod = "/grpc.health.v1.Health/Check"

func policy(idempotent bool) dmpfgrpc.MethodPolicy {
	return dmpfgrpc.MethodPolicy{
		Budget: deadline.Budget{
			Dependency:        "orders",
			Method:            checkMethod,
			Limit:             time.Second,
			Slack:             50 * time.Millisecond,
			EstimatedDuration: 100 * time.Millisecond,
		},
		Idempotent:     idempotent,
		RetryableCodes: []codes.Code{codes.Unavailable},
	}
}

func validConfig() dmpfgrpc.Config {
	return dmpfgrpc.Config{
		TLS:     &tls.Config{MinVersion: tls.VersionTLS13},
		Sheet:   resilience.Defaults("orders"),
		Methods: map[string]dmpfgrpc.MethodPolicy{checkMethod: policy(true)},
		Clock:   clock.NewFake(start),
	}
}

func TestConfigValidate(t *testing.T) {
	t.Run("complete configuration", func(t *testing.T) {
		if err := validConfig().Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})

	t.Run("without TLS and without the opt-out", func(t *testing.T) {
		cfg := validConfig()
		cfg.TLS = nil
		if err := cfg.Validate(); !errors.Is(err, dmpfgrpc.ErrTLSRequired) {
			t.Fatalf("Validate() = %v, want ErrTLSRequired", err)
		}
	})

	t.Run("TLS that skips verification or admits less than 1.2", func(t *testing.T) {
		for name, weak := range map[string]*tls.Config{"skip verify": {InsecureSkipVerify: true}, "tls 1.0": {MinVersion: tls.VersionTLS10}} {
			cfg := validConfig()
			cfg.TLS = weak
			if err := cfg.Validate(); !errors.Is(err, dmpfgrpc.ErrTLSTooWeak) {
				t.Fatalf("%s: Validate() = %v, want ErrTLSTooWeak (GRP-15)", name, err)
			}
			if err := (dmpfgrpc.ServerConfig{TLS: weak}).Validate(); !errors.Is(err, dmpfgrpc.ErrTLSTooWeak) {
				t.Fatalf("%s: ServerConfig.Validate() = %v, want ErrTLSTooWeak (GRP-15)", name, err)
			}
		}
	})

	t.Run("without TLS under the explicit opt-out", func(t *testing.T) {
		cfg := validConfig()
		cfg.TLS = nil
		cfg.InsecureForDevelopmentOnly = true
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})

	t.Run("without clock", func(t *testing.T) {
		cfg := validConfig()
		cfg.Clock = nil
		if err := cfg.Validate(); !errors.Is(err, dmpfgrpc.ErrIncompleteConfig) {
			t.Fatalf("Validate() = %v, want ErrIncompleteConfig", err)
		}
	})

	t.Run("without methods", func(t *testing.T) {
		cfg := validConfig()
		cfg.Methods = nil
		if err := cfg.Validate(); !errors.Is(err, dmpfgrpc.ErrIncompleteConfig) {
			t.Fatalf("Validate() = %v, want ErrIncompleteConfig", err)
		}
	})

	t.Run("sheet with a blank field", func(t *testing.T) {
		cfg := validConfig()
		cfg.Sheet.Breaker = resilience.Field[resilience.BreakerPolicy]{}
		if err := cfg.Validate(); !errors.Is(err, resilience.ErrBlankField) {
			t.Fatalf("Validate() = %v, want ErrBlankField (RES-21)", err)
		}
	})

	t.Run("method with an invalid budget", func(t *testing.T) {
		cfg := validConfig()
		p := policy(true)
		p.Budget.Slack = 0
		cfg.Methods[checkMethod] = p
		if err := cfg.Validate(); err == nil {
			t.Fatal("Validate() = nil, want an error for the budget without slack (GRP-17)")
		}
	})
}

func TestConfigPolicy(t *testing.T) {
	cfg := validConfig()

	if _, err := cfg.Policy(checkMethod); err != nil {
		t.Fatalf("Policy(declared) = %v, want nil", err)
	}
	if _, err := cfg.Policy("/orders.v1.Orders/Place"); !errors.Is(err, dmpfgrpc.ErrMethodNotDeclared) {
		t.Fatalf("Policy(undeclared) = %v, want ErrMethodNotDeclared", err)
	}
}
