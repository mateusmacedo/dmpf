package dmpfreference

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-app/relay"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/admission"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/deadline"
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
	ErrUnknownRole     = errors.New("dmpf-reference: unknown role")
	ErrMissingVariable = errors.New("dmpf-reference: required variable is not set")
	ErrInvalidVariable = errors.New("dmpf-reference: variable has an invalid value")
)

const (
	envDSN               = "DMPF_PG_DSN"
	envHTTPAddr          = "DMPF_HTTP_ADDR"
	envMigrate           = "DMPF_MIGRATE"
	envOpenAPIPath       = "DMPF_OPENAPI_PATH"
	envCORSOrigins       = "DMPF_CORS_ORIGINS"
	envBrokers           = "DMPF_KAFKA_BROKERS"
	envKafkaInsecure     = "DMPF_KAFKA_INSECURE"
	envTopic             = "DMPF_KAFKA_TOPIC"
	envGroup             = "DMPF_KAFKA_GROUP"
	envDLQ               = "DMPF_KAFKA_DLQ"
	envReservationsTopic = "DMPF_KAFKA_RESERVATIONS_TOPIC"
	envReservationsDLQ   = "DMPF_KAFKA_RESERVATIONS_DLQ"
	envOTLPEndpoint      = "DMPF_OTLP_ENDPOINT"
	envOTLPInsecure      = "DMPF_OTLP_INSECURE"
	envService           = "DMPF_SERVICE"
	envServiceVersion    = "DMPF_SERVICE_VERSION"
	envInstanceID        = "DMPF_INSTANCE_ID"
	envItemLimit         = "DMPF_ITEM_LIMIT"
)

// Config is every operational value the three roles need, resolved once at
// startup from the environment (ANC-06: the caller declares, the blocks refuse).
type Config struct {
	Role Role

	DSN      string
	HTTPAddr string

	// OpenAPIPath, when set, makes the api serve the published contract at
	// GET /openapi.yaml; CORSOrigins, when set, lets a browser on one of those
	// origins call it (the Swagger UI of the local stack). Both are off by default.
	OpenAPIPath string
	CORSOrigins []string

	Service  string
	Version  string
	Instance string

	Brokers       []string
	KafkaInsecure bool
	Topic         string
	Group         string
	DLQ           string

	// ReservationsTopic and ReservationsDLQ address the second channel of the
	// example, the one the consumer's own outbox publishes to.
	ReservationsTopic string
	ReservationsDLQ   string

	OTLPEndpoint string
	OTLPInsecure bool

	Migrate   bool
	ItemLimit int

	Relay     relay.Config
	Wait      time.Duration
	Admission admission.Limit
	Budget    deadline.Budget
}

// Defaults are the values a role runs with when the environment says nothing;
// they are the ones the e2e exercises and the README documents.
func Defaults(role Role) Config {
	return Config{
		Role:      role,
		HTTPAddr:  ":8080",
		Service:   "dmpf-reference",
		Version:   "dev",
		ItemLimit: 10,
		Relay: relay.Config{
			Source:         "urn:dmpf:reference",
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
		Budget: deadline.Budget{
			Dependency:        "postgres",
			Method:            "orders",
			Limit:             2 * time.Second,
			Slack:             200 * time.Millisecond,
			EstimatedDuration: 100 * time.Millisecond,
		},
	}
}

// FromEnv reads the variables through lookup (os.Getenv in production, a map
// in tests), applies Defaults and validates for the role.
func FromEnv(role Role, lookup func(string) string) (Config, error) {
	cfg := Defaults(role)

	cfg.DSN = lookup(envDSN)
	cfg.HTTPAddr = orDefault(lookup(envHTTPAddr), cfg.HTTPAddr)
	cfg.OpenAPIPath = lookup(envOpenAPIPath)
	cfg.CORSOrigins = splitList(lookup(envCORSOrigins))
	cfg.Service = orDefault(lookup(envService), cfg.Service)
	cfg.Version = orDefault(lookup(envServiceVersion), cfg.Version)
	cfg.Instance = orDefault(lookup(envInstanceID), hostname())
	cfg.Brokers = splitList(lookup(envBrokers))
	cfg.Topic = lookup(envTopic)
	cfg.Group = lookup(envGroup)
	cfg.DLQ = lookup(envDLQ)
	cfg.ReservationsTopic = lookup(envReservationsTopic)
	cfg.ReservationsDLQ = lookup(envReservationsDLQ)
	cfg.OTLPEndpoint = lookup(envOTLPEndpoint)

	var err error
	if cfg.KafkaInsecure, err = parseBool(envKafkaInsecure, lookup(envKafkaInsecure)); err != nil {
		return Config{}, err
	}
	if cfg.OTLPInsecure, err = parseBool(envOTLPInsecure, lookup(envOTLPInsecure)); err != nil {
		return Config{}, err
	}
	if cfg.Migrate, err = parseBool(envMigrate, lookup(envMigrate)); err != nil {
		return Config{}, err
	}
	if cfg.ItemLimit, err = parsePositive(envItemLimit, lookup(envItemLimit), cfg.ItemLimit); err != nil {
		return Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate refuses a configuration the role could not start with, naming the
// variable that is missing so the process fails at startup with the reason.
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

// requirements is what each role cannot start without: every role reaches
// Postgres, the two Kafka roles need both channels of the catalogue, and only
// the consumer needs a group.
func (c Config) requirements() ([]requirement, error) {
	storage := []requirement{{envDSN, c.DSN == ""}}
	channels := []requirement{
		{envBrokers, len(c.Brokers) == 0},
		{envTopic, c.Topic == ""},
		{envDLQ, c.DLQ == ""},
		{envReservationsTopic, c.ReservationsTopic == ""},
		{envReservationsDLQ, c.ReservationsDLQ == ""},
	}
	switch c.Role {
	case RoleAPI:
		return storage, nil
	case RoleRelay:
		return append(append(storage, channels...), requirement{envGroup, c.Group == ""}), nil
	case RoleConsumer:
		return append(append(storage, channels...), requirement{envGroup, c.Group == ""}), nil
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

func parsePositive(variable, value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%w: %s=%q is not a positive integer", ErrInvalidVariable, variable, value)
	}
	return parsed, nil
}
