package app

import (
	"errors"
	"fmt"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"slices"
	"strings"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
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
	envHTTPAddr       = "HTTP_ADDR"
	envMigrate        = "MIGRATE"
	envBrokers        = "KAFKA_BROKERS"
	envKafkaInsecure  = "KAFKA_INSECURE"
	envBookingsTopic  = "KAFKA_BOOKINGS_TOPIC"
	envBookingsDLQ    = "KAFKA_BOOKINGS_DLQ"
	envGroup          = "KAFKA_GROUP"
	envOTLPEndpoint   = "OTLP_ENDPOINT"
	envOTLPInsecure   = "OTLP_INSECURE"
	envService        = "SERVICE"
	envServiceVersion = "SERVICE_VERSION"
	envInstanceID     = "INSTANCE_ID"

	envGRPCAddr           = "GRPC_ADDR"
	envGRPCInsecure       = "GRPC_INSECURE"
	envGRPCCertFile       = "GRPC_TLS_CERT_FILE"
	envGRPCKeyFile        = "GRPC_TLS_KEY_FILE"
	envGRPCClientCAFile   = "GRPC_CLIENT_CA_FILE"
	envGRPCTrustedClients = "GRPC_TRUSTED_CLIENTS"
	envMetricTenants      = "METRIC_TENANTS"
	defaultHTTPAddr       = ":8080"
	defaultServiceName    = "bookings"
)

// Config is every operational value the two roles need, resolved once at
// startup from the environment.
type Config struct {
	Role Role

	DSN         string
	HTTPAddr    string
	Migrate     bool
	RouteBudget deadline.Budget

	// GRPCAddr turns the gRPC edge on beside the HTTP one while both coexist;
	// empty keeps the api on HTTP alone.
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
	KafkaAuth     kafka.ClientAuth
	BookingsTopic string
	BookingsDLQ   string
	Group         string

	OTLPEndpoint string
	OTLPInsecure bool

	Service  string
	Version  string
	Instance string

	Relay relay.Config
	Auth  authn.Config
}

// FromEnv resolves the configuration of the role and refuses to start when a
// variable the role needs is missing, so a process never degrades to a value
// nobody declared.
func FromEnv(role Role, lookup func(string) string) (Config, error) {
	cfg := Config{
		Role:          role,
		DSN:           lookup(envDSN),
		HTTPAddr:      envconfig.OrDefault(lookup(envHTTPAddr), defaultHTTPAddr),
		BookingsTopic: lookup(envBookingsTopic),
		BookingsDLQ:   lookup(envBookingsDLQ),
		Group:         lookup(envGroup),
		OTLPEndpoint:  lookup(envOTLPEndpoint),
		Service:       envconfig.OrDefault(lookup(envService), defaultServiceName),
		Version:       envconfig.OrDefault(lookup(envServiceVersion), "0.0.0"),
		Instance:      envconfig.OrDefault(lookup(envInstanceID), envconfig.Hostname()),
		Brokers:       envconfig.SplitList(lookup(envBrokers)),

		GRPCAddr:           lookup(envGRPCAddr),
		GRPCCertFile:       lookup(envGRPCCertFile),
		GRPCKeyFile:        lookup(envGRPCKeyFile),
		GRPCClientCAFile:   lookup(envGRPCClientCAFile),
		GRPCTrustedClients: envconfig.SplitList(lookup(envGRPCTrustedClients)),
		Admission:          admission.Limit{PerSecond: 50, Burst: 100, Concurrency: 32},
		MetricTenants:      envconfig.SplitList(lookup(envMetricTenants)),

		RouteBudget: deadline.Budget{
			Dependency:        "postgres",
			Method:            "route",
			Limit:             2 * time.Second,
			Slack:             200 * time.Millisecond,
			EstimatedDuration: 200 * time.Millisecond,
		},
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
	}

	cfg.KafkaAuth = kafka.ReadClientAuth(lookup)
	var err error
	if cfg.Migrate, err = envconfig.ParseBool(envMigrate, lookup(envMigrate)); err != nil {
		return Config{}, err
	}
	if cfg.KafkaInsecure, err = envconfig.ParseBool(envKafkaInsecure, lookup(envKafkaInsecure)); err != nil {
		return Config{}, err
	}
	if cfg.OTLPInsecure, err = envconfig.ParseBool(envOTLPInsecure, lookup(envOTLPInsecure)); err != nil {
		return Config{}, err
	}
	if cfg.GRPCInsecure, err = envconfig.ParseBool(envGRPCInsecure, lookup(envGRPCInsecure)); err != nil {
		return Config{}, err
	}
	if cfg.Auth, err = authn.ReadEnv(lookup); err != nil {
		return Config{}, err
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	missing := []string{}
	if c.DSN == "" {
		missing = append(missing, envDSN)
	}
	if c.Role == RoleRelay {
		if len(c.Brokers) == 0 {
			missing = append(missing, envBrokers)
		}
		if !c.KafkaInsecure && c.KafkaAuth.SASL == nil && c.KafkaAuth.CertFile == "" {
			missing = append(missing, "KAFKA_SASL_MECHANISM or KAFKA_CLIENT_CERT_FILE")
		}
		if c.BookingsTopic == "" {
			missing = append(missing, envBookingsTopic)
		}
		if c.BookingsDLQ == "" {
			missing = append(missing, envBookingsDLQ)
		}
		if c.Group == "" {
			missing = append(missing, envGroup)
		}
	}
	if c.Role == RoleAPI && c.GRPCAddr != "" {
		missing = append(missing, c.grpcTransport()...)
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingVariable, strings.Join(missing, ", "))
	}
	if c.Role == RoleRelay {
		if err := c.Relay.Validate(); err != nil {
			return err
		}
	}
	if c.Role == RoleAPI {
		if err := c.Auth.Validate(); err != nil {
			return err
		}
	}
	if c.Role != RoleAPI && c.Role != RoleRelay {
		return fmt.Errorf("%w: %q (want api|relay)", ErrUnknownRole, c.Role)
	}
	return nil
}

// grpcTransport is the gRPC edge's security policy: mutual TLS, whose client CA
// and allowlist authenticate the caller (IDN-03), or the development opt-out.
func (c Config) grpcTransport() []string {
	if c.GRPCInsecure {
		return nil
	}
	if c.GRPCCertFile == "" && c.GRPCKeyFile == "" {
		return []string{envGRPCInsecure + " or " + envGRPCCertFile + " and " + envGRPCKeyFile}
	}
	var missing []string
	for name, absent := range map[string]bool{
		envGRPCCertFile:       c.GRPCCertFile == "",
		envGRPCKeyFile:        c.GRPCKeyFile == "",
		envGRPCClientCAFile:   c.GRPCClientCAFile == "",
		envGRPCTrustedClients: len(c.GRPCTrustedClients) == 0,
	} {
		if absent {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}
