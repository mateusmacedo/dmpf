package grpc

import "github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"

type APIEnv struct {
	GRPCAddr     string
	GRPCInsecure bool
	GRPCCertFile string
	GRPCKeyFile  string

	// GRPCClientCAFile and GRPCTrustedClients authenticate the caller (IDN-03):
	// the metadata it propagates is only read from a workload they verified.
	GRPCClientCAFile   string
	GRPCTrustedClients []string
}

func ReadAPIEnv(lookup func(string) string, defaultAddr string) (APIEnv, error) {
	insecure, err := envconfig.ParseBool("GRPC_INSECURE", lookup("GRPC_INSECURE"))
	if err != nil {
		return APIEnv{}, err
	}
	return APIEnv{
		GRPCAddr:           envconfig.OrDefault(lookup("GRPC_ADDR"), defaultAddr),
		GRPCInsecure:       insecure,
		GRPCCertFile:       lookup("GRPC_TLS_CERT_FILE"),
		GRPCKeyFile:        lookup("GRPC_TLS_KEY_FILE"),
		GRPCClientCAFile:   lookup("GRPC_CLIENT_CA_FILE"),
		GRPCTrustedClients: envconfig.SplitList(lookup("GRPC_TRUSTED_CLIENTS")),
	}, nil
}

// Missing names what the transport policy lacks, in order: TLS with a certificate
// pair that authenticates its caller, or the development-only opt-out (GRP-15).
func (e APIEnv) Missing() []string {
	switch {
	case e.GRPCInsecure:
		return nil
	case e.GRPCCertFile == "" && e.GRPCKeyFile == "":
		return []string{"GRPC_INSECURE or GRPC_TLS_CERT_FILE and GRPC_TLS_KEY_FILE"}
	}
	var missing []string
	for _, v := range []struct {
		variable string
		absent   bool
	}{
		{"GRPC_TLS_CERT_FILE", e.GRPCCertFile == ""},
		{"GRPC_TLS_KEY_FILE", e.GRPCKeyFile == ""},
		{"GRPC_CLIENT_CA_FILE", e.GRPCClientCAFile == ""},
		{"GRPC_TRUSTED_CLIENTS", len(e.GRPCTrustedClients) == 0},
	} {
		if v.absent {
			missing = append(missing, v.variable)
		}
	}
	return missing
}
