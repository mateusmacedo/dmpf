package redis

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
)

var (
	ErrMissingURL = errors.New("redis: REDIS_URL is not set")
	ErrInvalidURL = errors.New("redis: REDIS_URL is not a valid redis URL")

	// ErrDatabaseNotZero refuses a URL that would need SELECT, which the
	// deny-list ACL user is not granted.
	ErrDatabaseNotZero = errors.New("redis: REDIS_URL must select database 0")

	ErrTLSRequired = errors.New("redis: REDIS_REQUIRE_TLS is set and REDIS_URL is not rediss://")
)

const (
	envURL            = "REDIS_URL"
	envDialTimeout    = "REDIS_DIAL_TIMEOUT_MS"
	envCommandTimeout = "REDIS_COMMAND_TIMEOUT_MS"
	envRequireTLS     = "REDIS_REQUIRE_TLS"
)

const (
	defaultDialTimeoutMilliseconds    = 1000
	defaultCommandTimeoutMilliseconds = 500
)

type Config struct {
	Options    *redis.Options
	RequireTLS bool
}

func FromEnv(lookup func(string) string) (Config, error) {
	raw := lookup(envURL)
	if raw == "" {
		return Config{}, ErrMissingURL
	}
	options, err := redis.ParseURL(raw)
	if err != nil {
		return Config{}, ErrInvalidURL
	}

	requireTLS, err := envconfig.ParseBool(envRequireTLS, lookup(envRequireTLS))
	if err != nil {
		return Config{}, err
	}
	if options.DialTimeout, err = parseMilliseconds(envDialTimeout, lookup(envDialTimeout), defaultDialTimeoutMilliseconds); err != nil {
		return Config{}, err
	}
	if options.ReadTimeout, err = parseMilliseconds(envCommandTimeout, lookup(envCommandTimeout), defaultCommandTimeoutMilliseconds); err != nil {
		return Config{}, err
	}
	options.WriteTimeout = options.ReadTimeout
	options.ContextTimeoutEnabled = true

	cfg := Config{Options: options, RequireTLS: requireTLS}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	switch {
	case c.Options == nil:
		return ErrMissingURL
	case c.Options.DB != 0:
		return ErrDatabaseNotZero
	case c.RequireTLS && c.Options.TLSConfig == nil:
		return ErrTLSRequired
	}
	return nil
}

func parseMilliseconds(variable, value string, fallback int64) (time.Duration, error) {
	if value == "" {
		return time.Duration(fallback) * time.Millisecond, nil
	}
	milliseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || milliseconds <= 0 || milliseconds > math.MaxInt64/int64(time.Millisecond) {
		return 0, fmt.Errorf("%w: %s=%q is not a number of milliseconds from 1 up to the largest duration",
			envconfig.ErrInvalidVariable, variable, value)
	}
	return time.Duration(milliseconds) * time.Millisecond, nil
}

func (c Config) LogValue() slog.Value {
	if c.Options == nil {
		return slog.GroupValue(slog.Bool("configured", false))
	}
	return slog.GroupValue(
		slog.String("addr", c.Options.Addr),
		slog.String("username", c.Options.Username),
		slog.Bool("tls", c.Options.TLSConfig != nil),
		slog.String("dial_timeout", c.Options.DialTimeout.String()),
		slog.String("command_timeout", c.Options.ReadTimeout.String()),
	)
}

// NewClient does not reach the server: the first command dials.
func NewClient(cfg Config) (*redis.Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return redis.NewClient(cfg.Options), nil
}
