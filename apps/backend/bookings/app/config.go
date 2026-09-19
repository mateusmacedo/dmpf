package app

import (
	"errors"
	"fmt"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"strings"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
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
	envDSN             = "DMPF_PG_DSN"
	envHTTPAddr        = "DMPF_HTTP_ADDR"
	envMigrate         = "DMPF_MIGRATE"
	envBrokers         = "DMPF_KAFKA_BROKERS"
	envKafkaInsecure   = "DMPF_KAFKA_INSECURE"
	envBookingsTopic   = "DMPF_KAFKA_BOOKINGS_TOPIC"
	envBookingsDLQ     = "DMPF_KAFKA_BOOKINGS_DLQ"
	envGroup           = "DMPF_KAFKA_GROUP"
	envOTLPEndpoint    = "DMPF_OTLP_ENDPOINT"
	envOTLPInsecure    = "DMPF_OTLP_INSECURE"
	envService         = "DMPF_SERVICE"
	envServiceVersion  = "DMPF_SERVICE_VERSION"
	envInstanceID      = "DMPF_INSTANCE_ID"
	defaultHTTPAddr    = ":8080"
	defaultServiceName = "bookings"
)

// Config is every operational value the two roles need, resolved once at
// startup from the environment.
type Config struct {
	Role Role

	DSN      string
	HTTPAddr string
	Migrate  bool

	Brokers       []string
	KafkaInsecure bool
	BookingsTopic string
	BookingsDLQ   string
	Group         string

	OTLPEndpoint string
	OTLPInsecure bool

	Service  string
	Version  string
	Instance string

	Relay relay.Config
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
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingVariable, strings.Join(missing, ", "))
	}
	if c.Role == RoleRelay {
		if err := c.Relay.Validate(); err != nil {
			return err
		}
	}
	if c.Role != RoleAPI && c.Role != RoleRelay {
		return fmt.Errorf("%w: %q (want api|relay)", ErrUnknownRole, c.Role)
	}
	return nil
}
