package redis_test

import (
	"bytes"
	"errors"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/redis"
)

const secret = "s3cr3t-pass"

func lookupOf(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func TestFromEnvReadsTheURLAndTheDefaults(t *testing.T) {
	cfg, err := provider.FromEnv(lookupOf(map[string]string{
		"REDIS_URL": "redis://revocation:" + secret + "@localhost:16379/0",
	}))
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.Options.Addr != "localhost:16379" || cfg.Options.Username != "revocation" || cfg.Options.Password != secret {
		t.Fatalf("options = %s %s", cfg.Options.Addr, cfg.Options.Username)
	}
	if cfg.Options.DialTimeout != time.Second || cfg.Options.ReadTimeout != 500*time.Millisecond || cfg.Options.WriteTimeout != 500*time.Millisecond {
		t.Fatalf("timeouts = %v %v %v", cfg.Options.DialTimeout, cfg.Options.ReadTimeout, cfg.Options.WriteTimeout)
	}
	if !cfg.Options.ContextTimeoutEnabled {
		t.Fatal("the caller's deadline must bound every command")
	}
}

func TestFromEnvReadsTheTimeouts(t *testing.T) {
	cfg, err := provider.FromEnv(lookupOf(map[string]string{
		"REDIS_URL":                "redis://localhost:6379",
		"REDIS_DIAL_TIMEOUT_MS":    "2000",
		"REDIS_COMMAND_TIMEOUT_MS": "300",
	}))
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.Options.DialTimeout != 2*time.Second || cfg.Options.ReadTimeout != 300*time.Millisecond || cfg.Options.WriteTimeout != 300*time.Millisecond {
		t.Fatalf("timeouts = %v %v %v", cfg.Options.DialTimeout, cfg.Options.ReadTimeout, cfg.Options.WriteTimeout)
	}
}

func TestFromEnvRefuses(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want error
	}{
		{"no url", map[string]string{}, provider.ErrMissingURL},
		{"not a redis url", map[string]string{"REDIS_URL": "http://revocation:" + secret + "@localhost:16379"}, provider.ErrInvalidURL},
		{"malformed url", map[string]string{"REDIS_URL": "redis://revocation:" + secret + "@localhost:16379/zero"}, provider.ErrInvalidURL},
		{"database other than 0", map[string]string{"REDIS_URL": "redis://revocation:" + secret + "@localhost:16379/1"}, provider.ErrDatabaseNotZero},
		{"non-positive timeout", map[string]string{"REDIS_URL": "redis://localhost:6379", "REDIS_COMMAND_TIMEOUT_MS": "0"}, envconfig.ErrInvalidVariable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := provider.FromEnv(lookupOf(tc.env))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "localhost") {
				t.Fatalf("the error leaks the URL: %v", err)
			}
		})
	}
}

func TestFromEnvRequireTLS(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		require string
		want    error
	}{
		{"unset accepts redis://", "redis://revocation:" + secret + "@localhost:16379/0", "", nil},
		{"false accepts redis://", "redis://revocation:" + secret + "@localhost:16379/0", "false", nil},
		{"true refuses redis://", "redis://revocation:" + secret + "@localhost:16379/0", "true", provider.ErrTLSRequired},
		{"true accepts rediss://", "rediss://revocation:" + secret + "@cache.internal:6380/0", "true", nil},
		{"not a boolean", "rediss://revocation:" + secret + "@cache.internal:6380/0", "yes", envconfig.ErrInvalidVariable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := provider.FromEnv(lookupOf(map[string]string{"REDIS_URL": tc.url, "REDIS_REQUIRE_TLS": tc.require}))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if err != nil {
				if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "localhost") {
					t.Fatalf("the error leaks the URL: %v", err)
				}
				return
			}
			if cfg.RequireTLS != (tc.require == "true") {
				t.Fatalf("RequireTLS = %v, want %v", cfg.RequireTLS, tc.require == "true")
			}
		})
	}
}

func TestFromEnvBoundsTheTimeoutsByTheLargestDuration(t *testing.T) {
	largest := strconv.FormatInt(math.MaxInt64/int64(time.Millisecond), 10)
	beyond := strconv.FormatInt(math.MaxInt64/int64(time.Millisecond)+1, 10)
	cases := []struct {
		name  string
		env   string
		value string
		want  error
	}{
		{"largest dial timeout", "REDIS_DIAL_TIMEOUT_MS", largest, nil},
		{"dial timeout beyond the largest duration", "REDIS_DIAL_TIMEOUT_MS", beyond, envconfig.ErrInvalidVariable},
		{"largest command timeout", "REDIS_COMMAND_TIMEOUT_MS", largest, nil},
		{"command timeout beyond the largest duration", "REDIS_COMMAND_TIMEOUT_MS", beyond, envconfig.ErrInvalidVariable},
		{"command timeout beyond int64", "REDIS_COMMAND_TIMEOUT_MS", "9223372036854775808", envconfig.ErrInvalidVariable},
		{"negative dial timeout", "REDIS_DIAL_TIMEOUT_MS", "-1", envconfig.ErrInvalidVariable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := provider.FromEnv(lookupOf(map[string]string{"REDIS_URL": "redis://localhost:6379", tc.env: tc.value}))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if err != nil {
				if !strings.Contains(err.Error(), tc.env) {
					t.Fatalf("err = %v, want the variable named", err)
				}
				return
			}
			if cfg.Options.DialTimeout <= 0 || cfg.Options.ReadTimeout <= 0 {
				t.Fatalf("timeouts = %v %v, want positive durations", cfg.Options.DialTimeout, cfg.Options.ReadTimeout)
			}
		})
	}
}

func TestLogValueOmitsThePassword(t *testing.T) {
	cfg, err := provider.FromEnv(lookupOf(map[string]string{
		"REDIS_URL": "rediss://revocation:" + secret + "@cache.internal:6380/0",
	}))
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	var out bytes.Buffer
	slog.New(slog.NewTextHandler(&out, nil)).Info("start", "redis", cfg)
	line := out.String()
	if strings.Contains(line, secret) {
		t.Fatalf("the log leaks the password: %s", line)
	}
	for _, want := range []string{"redis.addr=cache.internal:6380", "redis.username=revocation", "redis.tls=true", "redis.command_timeout=500ms"} {
		if !strings.Contains(line, want) {
			t.Fatalf("log %q lacks %q", line, want)
		}
	}
}

func TestNewClientRefusesAnUnvalidatedConfig(t *testing.T) {
	if _, err := provider.NewClient(provider.Config{}); !errors.Is(err, provider.ErrMissingURL) {
		t.Fatalf("err = %v", err)
	}
}
