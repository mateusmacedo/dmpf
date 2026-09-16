package bff_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bff"
)

func lookup(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

var targets = []string{"DMPF_ORDERS_GRPC_TARGET", "orders:9090", "DMPF_RESERVATIONS_GRPC_TARGET", "reservations:9090"}

func TestFromEnvAppliesTheDefaults(t *testing.T) {
	cfg, err := bff.FromEnv(lookup(append(targets, "DMPF_GRPC_INSECURE", "true")...))

	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.Service != "bff" || !cfg.GRPCInsecure {
		t.Fatalf("cfg = %+v, want :8080, bff and the opt-out", cfg)
	}
	if cfg.RouteBudget.Validate() != nil || cfg.RouteBudget.Limit <= 0 {
		t.Fatalf("RouteBudget = %+v, want a valid budget", cfg.RouteBudget)
	}
}

func TestFromEnvNamesEachMissingTarget(t *testing.T) {
	for _, variable := range []string{"DMPF_ORDERS_GRPC_TARGET", "DMPF_RESERVATIONS_GRPC_TARGET"} {
		t.Run(variable, func(t *testing.T) {
			env := []string{"DMPF_GRPC_INSECURE", "true"}
			for i := 0; i < len(targets); i += 2 {
				if targets[i] != variable {
					env = append(env, targets[i], targets[i+1])
				}
			}
			_, err := bff.FromEnv(lookup(env...))
			if !errors.Is(err, bff.ErrMissingVariable) || !strings.Contains(err.Error(), variable) {
				t.Fatalf("FromEnv() = %v, want %s named", err, variable)
			}
		})
	}
}

func TestFromEnvRequiresATransportPolicy(t *testing.T) {
	_, err := bff.FromEnv(lookup(targets...))

	if !errors.Is(err, bff.ErrMissingVariable) || !strings.Contains(err.Error(), "DMPF_GRPC_INSECURE") || !strings.Contains(err.Error(), "DMPF_GRPC_CA_FILE") {
		t.Fatalf("FromEnv() = %v, want both transport variables named (GRP-15)", err)
	}
}

func TestFromEnvAcceptsATrustAuthority(t *testing.T) {
	cfg, err := bff.FromEnv(lookup(append(targets, "DMPF_GRPC_CA_FILE", "/etc/ca.pem", "DMPF_GRPC_SERVER_NAME", "orders.internal", "DMPF_CORS_ORIGINS", "http://a, http://b")...))

	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	if cfg.GRPCInsecure || cfg.CAFile != "/etc/ca.pem" || cfg.ServerName != "orders.internal" || len(cfg.CORSOrigins) != 2 {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestFromEnvRefusesAnInvalidBoolean(t *testing.T) {
	_, err := bff.FromEnv(lookup(append(targets, "DMPF_GRPC_INSECURE", "maybe")...))

	if !errors.Is(err, bff.ErrInvalidVariable) {
		t.Fatalf("FromEnv() = %v, want ErrInvalidVariable", err)
	}
}
