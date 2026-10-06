package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
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
	ErrInvalidPolicy   = errors.New("bookings: invalid idempotency or purge policy")
)

const (
	envDSN           = "PG_DSN"
	envMigrate       = "MIGRATE"
	envBrokers       = "KAFKA_BROKERS"
	envKafkaInsecure = "KAFKA_INSECURE"
	envGroup         = "KAFKA_GROUP"

	envBookingsTopic = "KAFKA_BOOKINGS_TOPIC"
	envBookingsDLQ   = "KAFKA_BOOKINGS_DLQ"

	envMetricTenants = "METRIC_TENANTS"
)

// Config is every operational value the two roles need, resolved once at
// startup from the environment.
type Config struct {
	Role Role

	DSN     string
	Migrate bool

	API           kernelgrpc.APIEnv
	Admission     admission.Limit
	MetricTenants []string

	Brokers       []string
	KafkaInsecure bool

	// KafkaAuth is the principal this process presents to the
	// broker, required whenever TLS is on (IDN-04).
	KafkaAuth kafka.ClientAuth
	Group     string

	BookingsTopic string
	BookingsDLQ   string

	Signals boot.Signals

	Service string

	Relay relay.Config

	Policies kernelapp.Policies
}

// Defaults are the values a role runs with when the environment says nothing.
func Defaults(role Role) Config {
	return Config{
		Role:    role,
		API:     kernelgrpc.APIEnv{GRPCAddr: ":9090"},
		Service: "bookings",
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
			System:         semconv.MessagingSystemKafka.Value.AsString(),
		},
		Admission: admission.Limit{PerSecond: 50, Burst: 100, Concurrency: 32},
		Policies: kernelapp.Policies{
			IdempotencyWait:      time.Second,
			IdempotencyRetention: 24 * time.Hour,
			OutboxRetention:      168 * time.Hour,
			PurgeInterval:        15 * time.Minute,
			PurgeBatch:           1000,
		},
	}
}

// FromEnv reads the variables through lookup, applies Defaults and validates
// for the role.
func FromEnv(role Role, lookup func(string) string) (Config, error) {
	cfg := Defaults(role)

	var err error
	cfg.DSN = lookup(envDSN)
	if cfg.API, err = kernelgrpc.ReadAPIEnv(lookup, cfg.API.GRPCAddr); err != nil {
		return Config{}, err
	}
	cfg.Brokers = envconfig.SplitList(lookup(envBrokers))
	cfg.KafkaAuth = kafka.ReadClientAuth(lookup)
	cfg.MetricTenants = envconfig.SplitList(lookup(envMetricTenants))
	cfg.BookingsTopic = lookup(envBookingsTopic)
	cfg.BookingsDLQ = lookup(envBookingsDLQ)
	cfg.Group = lookup(envGroup)

	flags := []struct {
		variable string
		into     *bool
	}{
		{envKafkaInsecure, &cfg.KafkaInsecure},
		{envMigrate, &cfg.Migrate},
	}
	for _, flag := range flags {
		if *flag.into, err = envconfig.ParseBool(flag.variable, lookup(flag.variable)); err != nil {
			return Config{}, err
		}
	}

	if cfg.Signals, err = boot.SignalsFromEnv(lookup); err != nil {
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
	missing, err := c.requirements()
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingVariable, missing[0])
	}
	if err := c.Policies.Validate(false, ErrInvalidPolicy); err != nil {
		return err
	}
	return c.Relay.Validate()
}

func (c Config) requirements() ([]string, error) {
	var missing []string
	require := func(variable string, absent bool) {
		if absent {
			missing = append(missing, variable)
		}
	}
	require(envDSN, c.DSN == "")
	switch c.Role {
	case RoleAPI:
		return append(missing, c.API.Missing()...), nil
	case RoleRelay:
		require(envBrokers, len(c.Brokers) == 0)
		require(envBookingsTopic, c.BookingsTopic == "")
		require(envBookingsDLQ, c.BookingsDLQ == "")
		require(envGroup, c.Group == "")
		return append(missing, c.KafkaAuth.Missing(c.KafkaInsecure)...), nil
	case "consumer":
		return nil, fmt.Errorf("%w: %q: the bookings context consumes no channel (use %s)", ErrUnknownRole, c.Role, roleList())
	default:
		return nil, fmt.Errorf("%w: %q (use one of %s)", ErrUnknownRole, c.Role, roleList())
	}
}

func roleList() string {
	names := make([]string, len(Roles))
	for i, role := range Roles {
		names[i] = string(role)
	}
	return strings.Join(names, "|")
}
