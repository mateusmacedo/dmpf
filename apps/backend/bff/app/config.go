package app

import (
	"errors"
	"fmt"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
)

var (
	ErrMissingVariable = errors.New("bff: required variable is not set")
	ErrInvalidVariable = envconfig.ErrInvalidVariable

	// ErrInsecureNotDeclared refuses a clear-text dial that no one asked for:
	// without a CA file, DMPF_GRPC_INSECURE has to say so explicitly.
	ErrInsecureNotDeclared = errors.New("bff: no CA file and DMPF_GRPC_INSECURE is not set")
)

const (
	envHTTPAddr                 = "DMPF_HTTP_ADDR"
	envOrdersTarget             = "DMPF_ORDERS_GRPC_TARGET"
	envReservationsTarget       = "DMPF_RESERVATIONS_GRPC_TARGET"
	envGRPCInsecure             = "DMPF_GRPC_INSECURE"
	envGRPCCAFile               = "DMPF_GRPC_CA_FILE"
	envGRPCServerName           = "DMPF_GRPC_SERVER_NAME"
	envGRPCClientCertFile       = "DMPF_GRPC_CLIENT_CERT_FILE"
	envGRPCClientKeyFile        = "DMPF_GRPC_CLIENT_KEY_FILE"
	envCORSOrigins              = "DMPF_CORS_ORIGINS"
	envMetricTenants            = "DMPF_METRIC_TENANTS"
	envOrdersContractPath       = "DMPF_OPENAPI_ORDERS_PATH"
	envReservationsContractPath = "DMPF_OPENAPI_RESERVATIONS_PATH"
	envOTLPEndpoint             = "DMPF_OTLP_ENDPOINT"
	envOTLPInsecure             = "DMPF_OTLP_INSECURE"
	envService                  = "DMPF_SERVICE"
	envServiceVersion           = "DMPF_SERVICE_VERSION"
	envInstanceID               = "DMPF_INSTANCE_ID"
)

type Config struct {
	HTTPAddr string

	OrdersTarget       string
	ReservationsTarget string
	GRPCInsecure       bool
	CAFile             string
	ServerName         string
	ClientCertFile     string
	ClientKeyFile      string

	CORSOrigins              []string
	OrdersContractPath       string
	ReservationsContractPath string

	OTLPEndpoint string
	OTLPInsecure bool

	Service  string
	Version  string
	Instance string

	Admission admission.Limit

	// MetricTenants is the allowlist of MET-07: the tenants that keep their own
	// admission bucket and label. Every other tenant shares "other".
	MetricTenants []string
	RouteBudget   deadline.Budget

	Auth authn.Config
}

func Defaults() Config {
	return Config{
		HTTPAddr:  ":8080",
		Service:   "bff",
		Version:   "dev",
		Admission: admission.Limit{PerSecond: 50, Burst: 100, Concurrency: 32},
		RouteBudget: deadline.Budget{
			Dependency:        "contexts",
			Method:            "route",
			Limit:             2 * time.Second,
			Slack:             200 * time.Millisecond,
			EstimatedDuration: 200 * time.Millisecond,
		},
	}
}

func FromEnv(lookup func(string) string) (Config, error) {
	cfg := Defaults()

	cfg.HTTPAddr = envconfig.OrDefault(lookup(envHTTPAddr), cfg.HTTPAddr)
	cfg.OrdersTarget = lookup(envOrdersTarget)
	cfg.ReservationsTarget = lookup(envReservationsTarget)
	cfg.CAFile = lookup(envGRPCCAFile)
	cfg.ServerName = lookup(envGRPCServerName)
	cfg.ClientCertFile = lookup(envGRPCClientCertFile)
	cfg.ClientKeyFile = lookup(envGRPCClientKeyFile)
	cfg.CORSOrigins = envconfig.SplitList(lookup(envCORSOrigins))
	cfg.MetricTenants = envconfig.SplitList(lookup(envMetricTenants))
	cfg.OrdersContractPath = lookup(envOrdersContractPath)
	cfg.ReservationsContractPath = lookup(envReservationsContractPath)
	cfg.OTLPEndpoint = lookup(envOTLPEndpoint)
	cfg.Service = envconfig.OrDefault(lookup(envService), cfg.Service)
	cfg.Version = envconfig.OrDefault(lookup(envServiceVersion), cfg.Version)
	cfg.Instance = envconfig.OrDefault(lookup(envInstanceID), envconfig.Hostname())

	var err error
	if cfg.GRPCInsecure, err = envconfig.ParseBool(envGRPCInsecure, lookup(envGRPCInsecure)); err != nil {
		return Config{}, err
	}
	if cfg.OTLPInsecure, err = envconfig.ParseBool(envOTLPInsecure, lookup(envOTLPInsecure)); err != nil {
		return Config{}, err
	}
	if cfg.Auth, err = authn.ReadEnv(lookup); err != nil {
		return Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate refuses a start the edge could not serve: a missing context target,
// or no transport policy — TLS through a trusted authority or the explicit
// development opt-out (GRP-15).
func (c Config) Validate() error {
	switch {
	case c.OrdersTarget == "":
		return fmt.Errorf("%w: %s", ErrMissingVariable, envOrdersTarget)
	case c.ReservationsTarget == "":
		return fmt.Errorf("%w: %s", ErrMissingVariable, envReservationsTarget)
	case !c.GRPCInsecure && c.CAFile == "":
		return fmt.Errorf("%w: %s=true or %s", ErrMissingVariable, envGRPCInsecure, envGRPCCAFile)
	case c.CAFile != "" && (c.ClientCertFile == "" || c.ClientKeyFile == ""):
		return fmt.Errorf("%w: %s and %s", ErrMissingVariable, envGRPCClientCertFile, envGRPCClientKeyFile)
	}
	if err := c.RouteBudget.Validate(); err != nil {
		return err
	}
	return c.Auth.Validate()
}
