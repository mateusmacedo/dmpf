package grpc_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
)

func TestHealthServicesCoverTheOverallStatus(t *testing.T) {
	services := kernel.HealthServices("company.orders.v1.OrdersService")

	if len(services) != 2 || services[0] != "" {
		t.Fatalf("HealthServices() = %q, want the empty name first: a probe without a service asks for the overall status", services)
	}
	if services[1] != "company.orders.v1.OrdersService" {
		t.Fatalf("HealthServices()[1] = %q, want the service it was given", services[1])
	}
}

func TestServerTLSAnswersNilWhenNoPairIsDeclared(t *testing.T) {
	config, err := kernel.ServerTLS("", "")

	if err != nil || config != nil {
		t.Fatalf("ServerTLS(\"\", \"\") = (%v, %v), want (nil, nil)", config, err)
	}
}

func TestServerTLSRefusesAPairItCannotLoad(t *testing.T) {
	if _, err := kernel.ServerTLS("/nonexistent/cert.pem", "/nonexistent/key.pem"); err == nil {
		t.Fatal("ServerTLS() accepted a pair it cannot load; the process must refuse to start")
	}
}

func TestAPIServerConfigOptsOutOfSecurityOnlyWithoutAPair(t *testing.T) {
	config, err := kernel.APIServerConfig(kernel.APIServer{Insecure: true, Services: kernel.HealthServices("svc")})

	if err != nil {
		t.Fatalf("APIServerConfig() = %v, want nil", err)
	}
	if !config.InsecureForDevelopmentOnly {
		t.Fatal("the opt-out was declared and no pair was given, want the insecure server")
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestAPIServerConfigRefusesASilentPlaintextFallback(t *testing.T) {
	config, err := kernel.APIServerConfig(kernel.APIServer{Services: kernel.HealthServices("svc")})

	if err != nil {
		t.Fatalf("APIServerConfig() = %v, want nil", err)
	}
	if err := config.Validate(); err == nil {
		t.Fatal("a server without credentials and without the opt-out was accepted (GRP-15)")
	}
}
