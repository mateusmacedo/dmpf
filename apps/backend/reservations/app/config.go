package app

import (
	"errors"
	"fmt"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"strings"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

// Role selects which of the three processes the binary becomes (BLK-02: the
// relay never shares a process with the request path).
type Role string

const (
	RoleAPI      Role = "api"
	RoleRelay    Role = "relay"
	RoleConsumer Role = "consumer"
)

// Roles lists the accepted values of --role, in the order the usage prints them.
var Roles = []Role{RoleAPI, RoleRelay, RoleConsumer}

var (
	ErrUnknownRole     = errors.New("reservations: unknown role")
	ErrMissingVariable = errors.New("reservations: required variable is not set")
	ErrInvalidVariable = envconfig.ErrInvalidVariable
)

const (
	envDSN               = "DMPF_PG_DSN"
	envGRPCAddr          = "DMPF_GRPC_ADDR"
	envGRPCInsecure      = "DMPF_GRPC_INSECURE"
	envGRPCCertFile      = "DMPF_GRPC_TLS_CERT_FILE"
	envGRPCKeyFile       = "DMPF_GRPC_TLS_KEY_FILE"
	envMigrate           = "DMPF_MIGRATE"
	envBrokers           = "DMPF_KAFKA_BROKERS"
	envKafkaInsecure     = "DMPF_KAFKA_INSECURE"
	envOrdersTopic       = "DMPF_KAFKA_ORDERS_TOPIC"
	envOrdersDLQ         = "DMPF_KAFKA_ORDERS_DLQ"
	envReservationsTopic = "DMPF_KAFKA_RESERVATIONS_TOPIC"
	envReservationsDLQ   = "DMPF_KAFKA_RESERVATIONS_DLQ"
	envGroup             = "DMPF_KAFKA_GROUP"
	envOTLPEndpoint      = "DMPF_OTLP_ENDPOINT"
	envOTLPInsecure      = "DMPF_OTLP_INSECURE"
	envService           = "DMPF_SERVICE"
	envServiceVersion    = "DMPF_SERVICE_VERSION"
	envInstanceID        = "DMPF_INSTANCE_ID"
)

// Config is every operational value the three roles need, resolved once at
// startup from the environment.
type Config struct {
	Role Role

	DSN string

	GRPCAddr     string
	GRPCInsecure bool
	GRPCCertFile string
	GRPCKeyFile  string

	Service  string
	Version  string
	Instance string

	Brokers       []string
	KafkaInsecure bool
	Group         string

	// OrdersTopic and OrdersDLQ address the channel the consumer reads;
	// ReservationsTopic and ReservationsDLQ the one the relay publishes to.
	OrdersTopic       string
	OrdersDLQ         string
	ReservationsTopic string
	ReservationsDLQ   string

	OTLPEndpoint string
	OTLPInsecure bool

	Migrate bool

	Relay     relay.Config
	Wait      time.Duration
	Admission admission.Limit
}

// Defaults are the values a role runs with when the environment says nothing.
func Defaults(role Role) Config {
	return Config{
		Role:     role,
		GRPCAddr: ":9090",
		Service:  "reservations",
		Version:  "dev",
		Relay: relay.Config{
			Source:         "urn:dmpf:reference-reservations",
			Interval:       500 * time.Millisecond,
			BatchSize:      50,
			Lease:          30 * time.Second,
			Concurrency:    4,
			MaxAttempts:    10,
			BackoffBase:    200 * time.Millisecond,
			BackoffCeiling: 30 * time.Second,
			ShutdownGrace:  10 * time.Second,
		},
		Wait:      2 * time.Second,
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
	cfg.Service = envconfig.OrDefault(lookup(envService), cfg.Service)
	cfg.Version = envconfig.OrDefault(lookup(envServiceVersion), cfg.Version)
	cfg.Instance = envconfig.OrDefault(lookup(envInstanceID), envconfig.Hostname())
	cfg.Brokers = envconfig.SplitList(lookup(envBrokers))
	cfg.Group = lookup(envGroup)
	cfg.OrdersTopic = lookup(envOrdersTopic)
	cfg.OrdersDLQ = lookup(envOrdersDLQ)
	cfg.ReservationsTopic = lookup(envReservationsTopic)
	cfg.ReservationsDLQ = lookup(envReservationsDLQ)
	cfg.OTLPEndpoint = lookup(envOTLPEndpoint)

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
		value, err := envconfig.ParseBool(flag.variable, lookup(flag.variable))
		if err != nil {
			return Config{}, err
		}
		*flag.into = value
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
			requirement{envReservationsTopic, c.ReservationsTopic == ""},
			requirement{envReservationsDLQ, c.ReservationsDLQ == ""},
			requirement{envGroup, c.Group == ""},
		), nil
	case RoleConsumer:
		return append(storage,
			requirement{envBrokers, len(c.Brokers) == 0},
			requirement{envOrdersTopic, c.OrdersTopic == ""},
			requirement{envOrdersDLQ, c.OrdersDLQ == ""},
			requirement{envGroup, c.Group == ""},
		), nil
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
		return []requirement{{envGRPCCertFile, c.GRPCCertFile == ""}, {envGRPCKeyFile, c.GRPCKeyFile == ""}}
	}
}

func roleList() string {
	names := make([]string, len(Roles))
	for i, role := range Roles {
		names[i] = string(role)
	}
	return strings.Join(names, "|")
}
