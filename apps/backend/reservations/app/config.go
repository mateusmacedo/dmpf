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
	ErrInvalidPolicy   = errors.New("reservations: invalid idempotency or purge policy")
)

const (
	envDSN               = "PG_DSN"
	envMigrate           = "MIGRATE"
	envBrokers           = "KAFKA_BROKERS"
	envMetricTenants     = "METRIC_TENANTS"
	envKafkaInsecure     = "KAFKA_INSECURE"
	envOrdersTopic       = "KAFKA_ORDERS_TOPIC"
	envOrdersDLQ         = "KAFKA_ORDERS_DLQ"
	envOrdersSource      = "ORDERS_SOURCE"
	envReservationsTopic = "KAFKA_RESERVATIONS_TOPIC"
	envReservationsDLQ   = "KAFKA_RESERVATIONS_DLQ"
	envGroup             = "KAFKA_GROUP"
)

// Config is every operational value the three roles need, resolved once at
// startup from the environment.
type Config struct {
	Role Role

	DSN string

	API kernelgrpc.APIEnv

	Service string

	Brokers       []string
	KafkaInsecure bool

	// KafkaAuth is the principal this process presents to the
	// broker, required whenever TLS is on (IDN-04).
	KafkaAuth kafka.ClientAuth
	Group     string

	// OrdersTopic and OrdersDLQ address the channel the consumer reads;
	// ReservationsTopic and ReservationsDLQ the one the relay publishes to.
	OrdersTopic       string
	OrdersDLQ         string
	ReservationsTopic string
	ReservationsDLQ   string

	// OrdersSource is the producer the consumer admits on the orders channel,
	// by the envelope's source attribute (CTX-27, IDN-04).
	OrdersSource string

	Signals boot.Signals

	Migrate bool

	Relay relay.Config
	Wait  time.Duration

	// ConsumerTimeout is this consumer's own time policy, which CTX-28 makes
	// mandatory: the adapter mounts a deadline per attempt from it.
	ConsumerTimeout time.Duration
	Admission       admission.Limit

	// MetricTenants is the allowlist of MET-07: the tenants that keep their own
	// admission bucket and label. Every other tenant shares "other".
	MetricTenants []string

	Policies kernelapp.Policies
}

// Defaults are the values a role runs with when the environment says nothing.
func Defaults(role Role) Config {
	return Config{
		Role:    role,
		API:     kernelgrpc.APIEnv{GRPCAddr: ":9090"},
		Service: "reservations",
		Relay: relay.Config{
			Source:         "urn:dmpf:reference-reservations",
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
		OrdersSource:    "urn:dmpf:reference-orders",
		Wait:            2 * time.Second,
		ConsumerTimeout: 30 * time.Second,
		Admission:       admission.Limit{PerSecond: 50, Burst: 100, Concurrency: 32},

		Policies: kernelapp.Policies{
			IdempotencyWait:      time.Second,
			IdempotencyRetention: 24 * time.Hour,
			OutboxRetention:      168 * time.Hour,
			InboxRetention:       192 * time.Hour,
			PurgeInterval:        15 * time.Minute,
			PurgeBatch:           1000,
		},
	}
}

// FromEnv reads the variables through lookup, applies Defaults and validates
// for the role.
func FromEnv(role Role, lookup func(string) string) (Config, error) {
	cfg := Defaults(role)

	cfg.DSN = lookup(envDSN)
	api, err := kernelgrpc.ReadAPIEnv(lookup, cfg.API.GRPCAddr)
	if err != nil {
		return Config{}, err
	}
	cfg.API = api
	cfg.Brokers = envconfig.SplitList(lookup(envBrokers))
	cfg.KafkaAuth = kafka.ReadClientAuth(lookup)
	cfg.MetricTenants = envconfig.SplitList(lookup(envMetricTenants))
	cfg.Group = lookup(envGroup)
	cfg.OrdersTopic = lookup(envOrdersTopic)
	cfg.OrdersDLQ = lookup(envOrdersDLQ)
	cfg.OrdersSource = envconfig.OrDefault(lookup(envOrdersSource), cfg.OrdersSource)
	cfg.ReservationsTopic = lookup(envReservationsTopic)
	cfg.ReservationsDLQ = lookup(envReservationsDLQ)

	flags := []struct {
		variable string
		into     *bool
	}{
		{envKafkaInsecure, &cfg.KafkaInsecure},
		{envMigrate, &cfg.Migrate},
	}
	for _, flag := range flags {
		value, err := envconfig.ParseBool(flag.variable, lookup(flag.variable))
		if err != nil {
			return Config{}, err
		}
		*flag.into = value
	}

	signals, err := boot.SignalsFromEnv(lookup)
	if err != nil {
		return Config{}, err
	}
	cfg.Signals = signals

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
	if err := c.Policies.Validate(true, ErrInvalidPolicy); err != nil {
		return err
	}
	// INB-14: an entry purged before the broker stops redelivering its message
	// would let that message through as new.
	if window := OrdersChannel(c).Redelivery.UpperBound; c.Role == RoleConsumer && c.Policies.InboxRetention < window {
		return fmt.Errorf("%w: InboxRetention %v is below the redelivery window %v (INB-14)", ErrInvalidPolicy, c.Policies.InboxRetention, window)
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
		require(envReservationsTopic, c.ReservationsTopic == "")
		require(envReservationsDLQ, c.ReservationsDLQ == "")
		require(envGroup, c.Group == "")
		return append(missing, c.KafkaAuth.Missing(c.KafkaInsecure)...), nil
	case RoleConsumer:
		require(envBrokers, len(c.Brokers) == 0)
		require(envOrdersTopic, c.OrdersTopic == "")
		require(envOrdersDLQ, c.OrdersDLQ == "")
		require(envGroup, c.Group == "")
		return append(missing, c.KafkaAuth.Missing(c.KafkaInsecure)...), nil
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
