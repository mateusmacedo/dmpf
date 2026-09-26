package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

// Role selects which process the binary becomes (BLK-02: the relay never
// shares a process with the request path).
type Role string

const (
	RoleAPI   Role = "api"
	RoleRelay Role = "relay"
)

// Roles lists the accepted values of --role; bookings consumes no channel.
var Roles = []Role{RoleAPI, RoleRelay}

var (
	ErrUnknownRole     = errors.New("bookings: unknown role")
	ErrMissingVariable = errors.New("bookings: required variable is not set")
	ErrInvalidVariable = envconfig.ErrInvalidVariable
)

const (
	envDSN            = "PG_DSN"
	envMigrate        = "MIGRATE"
	envBrokers        = "KAFKA_BROKERS"
	envKafkaInsecure  = "KAFKA_INSECURE"
	envGroup          = "KAFKA_GROUP"
	envOTLPEndpoint   = "OTLP_ENDPOINT"
	envOTLPInsecure   = "OTLP_INSECURE"
	envService        = "SERVICE"
	envServiceVersion = "SERVICE_VERSION"
	envInstanceID     = "INSTANCE_ID"

	envBookingsTopic = "KAFKA_BOOKINGS_TOPIC"
	envBookingsDLQ   = "KAFKA_BOOKINGS_DLQ"

	envGRPCAddr           = "GRPC_ADDR"
	envGRPCInsecure       = "GRPC_INSECURE"
	envGRPCCertFile       = "GRPC_TLS_CERT_FILE"
	envGRPCKeyFile        = "GRPC_TLS_KEY_FILE"
	envGRPCClientCAFile   = "GRPC_CLIENT_CA_FILE"
	envGRPCTrustedClients = "GRPC_TRUSTED_CLIENTS"
	envMetricTenants      = "METRIC_TENANTS"
)

// Config is every operational value the two roles need, resolved once at
// startup from the environment.
type Config struct {
	Role Role

	DSN     string
	Migrate bool

	GRPCAddr           string
	GRPCInsecure       bool
	GRPCCertFile       string
	GRPCKeyFile        string
	GRPCClientCAFile   string
	GRPCTrustedClients []string
	Admission          admission.Limit
	MetricTenants      []string

	Brokers       []string
	KafkaInsecure bool

	// KafkaAuth is the principal this process presents to the
	// broker, required whenever TLS is on (IDN-04).
	KafkaAuth kafka.ClientAuth
	Group     string

	BookingsTopic string
	BookingsDLQ   string

	OTLPEndpoint string
	OTLPInsecure bool

	Service  string
	Version  string
	Instance string

	Relay relay.Config
}

// Defaults are the values a role runs with when the environment says nothing.
func Defaults(role Role) Config {
	return Config{
		Role:     role,
		GRPCAddr: ":9090",
		Service:  "bookings",
		Version:  "dev",
		Relay: relay.Config{
			Source:         "urn:dmpf:reference-bookings",
			Interval:       500 * time.Millisecond,
			BatchSize:      50,
			Lease:          30 * time.Second,
			Concurrency:    4,
			MaxAttempts:    10,
			BackoffBase:    200 * time.Millisecond,
			BackoffCeiling: 30 * time.Second,
			ShutdownGrace:  observability.ShutdownGrace,
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
	cfg.BookingsTopic = lookup(envBookingsTopic)
	cfg.BookingsDLQ = lookup(envBookingsDLQ)
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
			requirement{envBookingsTopic, c.BookingsTopic == ""},
			requirement{envBookingsDLQ, c.BookingsDLQ == ""},
			requirement{envGroup, c.Group == ""},
			c.kafkaClientAuth(),
		), nil
	case "consumer":
		return nil, fmt.Errorf("%w: %q: the bookings context consumes no channel (use %s)", ErrUnknownRole, c.Role, roleList())
	default:
		return nil, fmt.Errorf("%w: %q (use one of %s)", ErrUnknownRole, c.Role, roleList())
	}
}

// transport is the api's security policy: mutual TLS, whose client CA and
// allowlist authenticate the caller (IDN-03), or the development-only opt-out
// (GRP-15); neither is refused at startup.
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
	return requirement{"KAFKA_SASL_MECHANISM or KAFKA_CLIENT_CERT_FILE", !c.KafkaInsecure && c.KafkaAuth.SASL == nil && c.KafkaAuth.CertFile == ""}
}
