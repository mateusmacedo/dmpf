package grpc_test

import (
	"errors"
	"slices"
	"testing"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
)

func envOf(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

func TestReadAPIEnvReadsTheGRPCVariables(t *testing.T) {
	env, err := kernelgrpc.ReadAPIEnv(envOf(
		"GRPC_ADDR", ":7070", "GRPC_INSECURE", "false",
		"GRPC_TLS_CERT_FILE", "/tls/cert.pem", "GRPC_TLS_KEY_FILE", "/tls/key.pem",
		"GRPC_CLIENT_CA_FILE", "/tls/clients.pem", "GRPC_TRUSTED_CLIENTS", "spiffe://dmpf/bff, spiffe://dmpf/ops",
	), ":9090")

	if err != nil {
		t.Fatalf("ReadAPIEnv() = %v, want nil", err)
	}
	want := kernelgrpc.APIEnv{
		GRPCAddr: ":7070", GRPCCertFile: "/tls/cert.pem", GRPCKeyFile: "/tls/key.pem",
		GRPCClientCAFile: "/tls/clients.pem", GRPCTrustedClients: []string{"spiffe://dmpf/bff", "spiffe://dmpf/ops"},
	}
	if env.GRPCAddr != want.GRPCAddr || env.GRPCInsecure || env.GRPCCertFile != want.GRPCCertFile ||
		env.GRPCKeyFile != want.GRPCKeyFile || env.GRPCClientCAFile != want.GRPCClientCAFile ||
		!slices.Equal(env.GRPCTrustedClients, want.GRPCTrustedClients) {
		t.Fatalf("ReadAPIEnv() = %+v, want %+v", env, want)
	}
}

func TestReadAPIEnvFallsBackToTheDefaultAddress(t *testing.T) {
	env, err := kernelgrpc.ReadAPIEnv(envOf("GRPC_INSECURE", "true"), ":9090")

	if err != nil || env.GRPCAddr != ":9090" || !env.GRPCInsecure {
		t.Fatalf("ReadAPIEnv() = (%+v, %v), want :9090 and insecure", env, err)
	}
}

func TestReadAPIEnvRefusesAnInvalidFlag(t *testing.T) {
	_, err := kernelgrpc.ReadAPIEnv(envOf("GRPC_INSECURE", "maybe"), ":9090")

	if !errors.Is(err, envconfig.ErrInvalidVariable) {
		t.Fatalf("ReadAPIEnv() = %v, want ErrInvalidVariable", err)
	}
}

func TestAPIEnvMissingNamesWhatTheTransportPolicyLacks(t *testing.T) {
	complete := kernelgrpc.APIEnv{
		GRPCCertFile: "/tls/cert.pem", GRPCKeyFile: "/tls/key.pem",
		GRPCClientCAFile: "/tls/clients.pem", GRPCTrustedClients: []string{"spiffe://dmpf/bff"},
	}
	for name, tc := range map[string]struct {
		env  func(kernelgrpc.APIEnv) kernelgrpc.APIEnv
		want []string
	}{
		"insecure opt-out": {func(kernelgrpc.APIEnv) kernelgrpc.APIEnv { return kernelgrpc.APIEnv{GRPCInsecure: true} }, nil},
		"complete pair":    {func(e kernelgrpc.APIEnv) kernelgrpc.APIEnv { return e }, nil},
		"no pair": {
			func(kernelgrpc.APIEnv) kernelgrpc.APIEnv { return kernelgrpc.APIEnv{} },
			[]string{"GRPC_INSECURE or GRPC_TLS_CERT_FILE and GRPC_TLS_KEY_FILE"},
		},
		"half a pair": {
			func(e kernelgrpc.APIEnv) kernelgrpc.APIEnv { e.GRPCKeyFile = ""; return e },
			[]string{"GRPC_TLS_KEY_FILE"},
		},
		"no client CA": {
			func(e kernelgrpc.APIEnv) kernelgrpc.APIEnv { e.GRPCClientCAFile = ""; return e },
			[]string{"GRPC_CLIENT_CA_FILE"},
		},
		"no trusted client": {
			func(e kernelgrpc.APIEnv) kernelgrpc.APIEnv { e.GRPCTrustedClients = nil; return e },
			[]string{"GRPC_TRUSTED_CLIENTS"},
		},
		"only the certificate": {
			func(kernelgrpc.APIEnv) kernelgrpc.APIEnv { return kernelgrpc.APIEnv{GRPCCertFile: "/tls/cert.pem"} },
			[]string{"GRPC_TLS_KEY_FILE", "GRPC_CLIENT_CA_FILE", "GRPC_TRUSTED_CLIENTS"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.env(complete).Missing(); !slices.Equal(got, tc.want) {
				t.Fatalf("Missing() = %q, want %q", got, tc.want)
			}
		})
	}
}
