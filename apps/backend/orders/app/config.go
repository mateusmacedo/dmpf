package app

import (
	"errors"
	"fmt"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"strings"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

// Role selects which process the binary becomes (BLK-02: the relay never
// shares a process with the request path).
type Role string

const (
	RoleAPI   Role = "api"
	RoleRelay Role = "relay"
)

// Roles lists the accepted values of --role; orders consumes no channel.
var Roles = []Role{RoleAPI, RoleRelay}

var (
	ErrUnknownRole     = errors.New("orders: unknown role")
	ErrMissingVariable = errors.New("orders: required variable is not set")
	ErrInvalidVariable = envconfig.ErrInvalidVariable
)

const (
	envDSN                = "DMPF_PG_DSN"
	envGRPCAddr           = "DMPF_GRPC_ADDR"
	envGRPCInsecure       = "DMPF_GRPC_INSECURE"
	envGRPCCertFile       = "DMPF_GRPC_TLS_CERT_FILE"
	envGRPCKeyFile        = "DMPF_GRPC_TLS_KEY_FILE"
	envGRPCClientCAFile   = "DMPF_GRPC_CLIENT_CA_FILE"
	envGRPCTrustedClients = "DMPF_GRPC_TRUSTED_CLIENTS"
	envMigrate            = "DMPF_MIGRATE"
	envBrokers            = "DMPF_KAFKA_BROKERS"
	envMetricTenants      = "DMPF_METRIC_TENANTS"
	envKafkaInsecure      = "DMPF_KAFKA_INSECURE"
	envOrdersTopic        = "DMPF_KAFKA_ORDERS_TOPIC"
	envOrdersDLQ          = "DMPF_KAFKA_ORDERS_DLQ"
	envGroup              = "DMPF_KAFKA_GROUP"
	envOTLPEndpoint       = "DMPF_OTLP_ENDPOINT"
	envOTLPInsecure       = "DMPF_OTLP_INSECURE"
	envService            = "DMPF_SERVICE"
	envServiceVersion     = "DMPF_SERVICE_VERSION"
	envInstanceID         = "DMPF_INSTANCE_ID"
	envItemLimit          = "DMPF_ITEM_LIMIT"

	// DefaultItemLimit is the ceiling a process takes when it declares none.
	DefaultItemLimit = 10
)

// Config is every operational value the two roles need, resolved once at
// startup from the environment.
type Config struct {
	Role Role

	DSN string

	GRPCAddr     string
	GRPCInsecure bool
	GRPCCertFile string
	GRPCKeyFile  string

	// GRPCClientCAFile and GRPCTrustedClients authenticate the caller (IDN-03):
	// the metadata it propagates is only read from a workload they verified.
	GRPCClientCAFile   string
	GRPCTrustedClients []string

	Service  string
	Version  string
	Instance string

	Brokers       []string
	KafkaInsecure bool

	// KafkaAuth is the principal this process presents to the
	// broker, required whenever TLS is on (IDN-04).
	KafkaAuth   kafka.ClientAuth
	OrdersTopic string
	OrdersDLQ   string
	Group       string

	OTLPEndpoint string
	OTLPInsecure bool

	Migrate   bool
	ItemLimit int

	Relay     relay.Config
	Admission admission.Limit

	// MetricTenants is the allowlist of MET-07: the tenants that keep their own
	// admission bucket and label. Every other tenant shares "other".
	MetricTenants []string
}

// Defaults are the values a role runs with when the environment says nothing.
func Defaults(role Role) Config {
	return Config{
		Role:      role,
		GRPCAddr:  ":9090",
		Service:   "orders",
		Version:   "dev",
		ItemLimit: DefaultItemLimit,
		Relay: relay.Config{
			Source:         "urn:dmpf:reference-orders",
			Interval:       500 * time.Millisecond,
			BatchSize:      50,
			Lease:          30 * time.Second,
			Concurrency:    4,
			MaxAttempts:    10,
			BackoffBase:    200 * time.Millisecond,
			BackoffCeiling: 30 * time.Second,
			ShutdownGrace:  10 * time.Second,
		},
		Admission: admission.Limit{PerSecond: 50, Burst: 100, Concurrency: 32},
	}
}

// FromEnv reads the variables through lookup, applies Defaults and validates
// for the role.
func FromEnv(role Role, lookup func(string) string) (Config, error) {
	cfg := Defaults(role)

	cfg.DSN = lookup(envDSN)
	cfg.GRPCAddr = envconfig.OrDefault(lookup(envGRPCAddr), cfg.GRPCAddr)
	cfg.GRPCCertFile = lookup(envGRPCCertFile)
	cfg.GRPCKeyFile = lookup(envGRPCKeyFile)
	cfg.GRPCClientCAFile = lookup(envGRPCClientCAFile)
	cfg.GRPCTrustedClients = envconfig.SplitList(lookup(envGRPCTrustedClients))
	cfg.Service = envconfig.OrDefault(lookup(envService), cfg.Service)
	cfg.Version = envconfig.OrDefault(lookup(envServiceVersion), cfg.Version)
	cfg.Instance = envconfig.OrDefault(lookup(envInstanceID), envconfig.Hostname())
	cfg.Brokers = envconfig.SplitList(lookup(envBrokers))
	cfg.KafkaAuth = kafka.ReadClientAuth(lookup)
	cfg.MetricTenants = envconfig.SplitList(lookup(envMetricTenants))
	cfg.OrdersTopic = lookup(envOrdersTopic)
	cfg.OrdersDLQ = lookup(envOrdersDLQ)
	cfg.Group = lookup(envGroup)
	cfg.OTLPEndpoint = lookup(envOTLPEndpoint)

	var err error
	flags := []struct {
		variable string
		into     *bool
	}{
		{envGRPCInsecure, &cfg.GRPCInsecure},
		{envKafkaInsecure, &cfg.KafkaInsecure},
		{envOTLPInsecure, &cfg.OTLPInsecure},
		{envMigrate, &cfg.Migrate},
	}
	for _, flag := range flags {
		if *flag.into, err = envconfig.ParseBool(flag.variable, lookup(flag.variable)); err != nil {
			return Config{}, err
		}
	}
	if cfg.ItemLimit, err = envconfig.ParsePositive(envItemLimit, lookup(envItemLimit), cfg.ItemLimit); err != nil {
		return Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate refuses a configuration the role could not start with, naming the
// variable that is missing.
func (c Config) Validate() error {
	required, err := c.requirements()
	if err != nil {
		return err
	}
	for _, r := range required {
		if r.absent {
			return fmt.Errorf("%w: %s", ErrMissingVariable, r.variable)
		}
	}
	if c.ItemLimit <= 0 {
		return fmt.Errorf("%w: %s must be positive", ErrInvalidVariable, envItemLimit)
	}
	return c.Relay.Validate()
}

type requirement struct {
	variable string
	absent   bool
}

func (c Config) requirements() ([]requirement, error) {
	storage := []requirement{{envDSN, c.DSN == ""}}
	switch c.Role {
	case RoleAPI:
		return append(storage, c.transport()...), nil
	case RoleRelay:
		return append(storage,
			requirement{envBrokers, len(c.Brokers) == 0},
			requirement{envOrdersTopic, c.OrdersTopic == ""},
			requirement{envOrdersDLQ, c.OrdersDLQ == ""},
			requirement{envGroup, c.Group == ""},
			c.kafkaClientAuth(),
		), nil
	case "consumer":
		return nil, fmt.Errorf("%w: %q: the orders context consumes no channel (use %s)", ErrUnknownRole, c.Role, roleList())
	default:
		return nil, fmt.Errorf("%w: %q (use one of %s)", ErrUnknownRole, c.Role, roleList())
	}
}

// transport is the api's security policy: TLS by a certificate pair, or the
// explicit development-only opt-out (GRP-15); neither is refused at startup.
func (c Config) transport() []requirement {
	switch {
	case c.GRPCInsecure:
		return nil
	case c.GRPCCertFile == "" && c.GRPCKeyFile == "":
		return []requirement{{envGRPCInsecure + " or " + envGRPCCertFile + " and " + envGRPCKeyFile, true}}
	default:
		return []requirement{
			{envGRPCCertFile, c.GRPCCertFile == ""}, {envGRPCKeyFile, c.GRPCKeyFile == ""},
			{envGRPCClientCAFile, c.GRPCClientCAFile == ""}, {envGRPCTrustedClients, len(c.GRPCTrustedClients) == 0},
		}
	}
}

func roleList() string {
	names := make([]string, len(Roles))
	for i, role := range Roles {
		names[i] = string(role)
	}
	return strings.Join(names, "|")
}

// kafkaClientAuth is the requirement of IDN-04 on a role that talks to the
// broker: with TLS on, the client authenticates; only the opt-out waives it.
func (c Config) kafkaClientAuth() requirement {
	return requirement{"DMPF_KAFKA_SASL_MECHANISM or DMPF_KAFKA_CLIENT_CERT_FILE", !c.KafkaInsecure && c.KafkaAuth.SASL == nil && c.KafkaAuth.CertFile == ""}
}
