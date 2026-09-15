package dmpfreferencebff

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/deadline"
)

var (
	ErrMissingVariable = errors.New("dmpf-reference-bff: required variable is not set")
	ErrInvalidVariable = errors.New("dmpf-reference-bff: variable has an invalid value")

	// ErrInsecureNotDeclared refuses a clear-text dial that no one asked for:
	// without a CA file, DMPF_GRPC_INSECURE has to say so explicitly.
	ErrInsecureNotDeclared = errors.New("dmpf-reference-bff: no CA file and DMPF_GRPC_INSECURE is not set")
)

const (
	envHTTPAddr                 = "DMPF_HTTP_ADDR"
	envOrdersTarget             = "DMPF_ORDERS_GRPC_TARGET"
	envReservationsTarget       = "DMPF_RESERVATIONS_GRPC_TARGET"
	envGRPCInsecure             = "DMPF_GRPC_INSECURE"
	envGRPCCAFile               = "DMPF_GRPC_CA_FILE"
	envGRPCServerName           = "DMPF_GRPC_SERVER_NAME"
	envCORSOrigins              = "DMPF_CORS_ORIGINS"
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

	CORSOrigins              []string
	OrdersContractPath       string
	ReservationsContractPath string

	OTLPEndpoint string
	OTLPInsecure bool

	Service  string
	Version  string
	Instance string

	Admission   admission.Limit
	RouteBudget deadline.Budget
}

func Defaults() Config {
	return Config{
		HTTPAddr:  ":8080",
		Service:   "dmpf-reference-bff",
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

	cfg.HTTPAddr = orDefault(lookup(envHTTPAddr), cfg.HTTPAddr)
	cfg.OrdersTarget = lookup(envOrdersTarget)
	cfg.ReservationsTarget = lookup(envReservationsTarget)
	cfg.CAFile = lookup(envGRPCCAFile)
	cfg.ServerName = lookup(envGRPCServerName)
	cfg.CORSOrigins = splitList(lookup(envCORSOrigins))
	cfg.OrdersContractPath = lookup(envOrdersContractPath)
	cfg.ReservationsContractPath = lookup(envReservationsContractPath)
	cfg.OTLPEndpoint = lookup(envOTLPEndpoint)
	cfg.Service = orDefault(lookup(envService), cfg.Service)
	cfg.Version = orDefault(lookup(envServiceVersion), cfg.Version)
	cfg.Instance = orDefault(lookup(envInstanceID), hostname())

	var err error
	if cfg.GRPCInsecure, err = parseBool(envGRPCInsecure, lookup(envGRPCInsecure)); err != nil {
		return Config{}, err
	}
	if cfg.OTLPInsecure, err = parseBool(envOTLPInsecure, lookup(envOTLPInsecure)); err != nil {
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
	}
	return c.RouteBudget.Validate()
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func hostname() string {
	if name, err := os.Hostname(); err == nil && name != "" {
		return name
	}
	return "local"
}

func splitList(value string) []string {
	var items []string
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func parseBool(variable, value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%w: %s=%q is not a boolean", ErrInvalidVariable, variable, value)
	}
	return parsed, nil
}
