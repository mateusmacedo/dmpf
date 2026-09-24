package relay

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		Source:         testSource,
		Interval:       time.Second,
		BatchSize:      50,
		Lease:          time.Minute,
		Concurrency:    4,
		MaxAttempts:    5,
		BackoffBase:    time.Second,
		BackoffCeiling: time.Minute,
		ShutdownGrace:  5 * time.Second,
	}
}

func TestConfigRefusesValuesTheLoopCannotRunWith(t *testing.T) {
	cases := []struct {
		field  string
		break_ func(c *Config)
	}{
		{"source", func(c *Config) { c.Source = "" }},
		{"interval", func(c *Config) { c.Interval = 0 }},
		{"interval", func(c *Config) { c.Interval = -time.Second }},
		{"batch size", func(c *Config) { c.BatchSize = 0 }},
		{"batch size", func(c *Config) { c.BatchSize = -1 }},
		{"lease", func(c *Config) { c.Lease = 0 }},
		{"lease", func(c *Config) { c.Lease = -time.Minute }},
		{"concurrency", func(c *Config) { c.Concurrency = 0 }},
		{"concurrency", func(c *Config) { c.Concurrency = -4 }},
		{"backoff base", func(c *Config) { c.BackoffBase = -time.Second }},
		{"backoff ceiling", func(c *Config) { c.BackoffCeiling = time.Millisecond }},
		{"shutdown grace", func(c *Config) { c.ShutdownGrace = -time.Second }},
	}

	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			config := validConfig()
			c.break_(&config)

			err := config.Validate()
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Validate() = %v, want ErrInvalidConfig", err)
			}
			if !strings.Contains(err.Error(), c.field) {
				t.Fatalf("Validate() = %q, want it to name %q", err, c.field)
			}
		})
	}
}

func TestConfigAcceptsTheValuesTheCallerMayLeaveOut(t *testing.T) {
	config := validConfig()
	config.MaxAttempts = 0
	config.BackoffBase = 0
	config.BackoffCeiling = 0
	config.ShutdownGrace = 0

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil: these four are the caller's to omit", err)
	}
}

func TestNewFailsAtStartupRatherThanAtTheFirstScan(t *testing.T) {
	config := validConfig()
	config.Lease = 0

	if _, err := New(newFakeStore(), &fakePublisher{}, &countingIDs{}, fixedClock(0), config); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("New() = %v, want ErrInvalidConfig", err)
	}
}

func TestNewRefusesAMissingCollaborator(t *testing.T) {
	cases := []struct {
		name  string
		build func() (Relay, error)
	}{
		{"no store", func() (Relay, error) {
			return New(nil, &fakePublisher{}, &countingIDs{}, fixedClock(0), validConfig())
		}},
		{"no publisher", func() (Relay, error) {
			return New(newFakeStore(), nil, &countingIDs{}, fixedClock(0), validConfig())
		}},
		{"no claim ids", func() (Relay, error) {
			return New(newFakeStore(), &fakePublisher{}, nil, fixedClock(0), validConfig())
		}},
		{"no clock", func() (Relay, error) {
			return New(newFakeStore(), &fakePublisher{}, &countingIDs{}, nil, validConfig())
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := c.build(); !errors.Is(err, ErrIncompleteRelay) {
				t.Fatalf("New() = %v, want ErrIncompleteRelay", err)
			}
		})
	}
}

func TestNewCarriesTheConfigurationOntoTheRelay(t *testing.T) {
	config := validConfig()

	relay, err := New(newFakeStore(), &fakePublisher{}, &countingIDs{}, fixedClock(0), config)
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}

	if relay.Source != config.Source || relay.Interval != config.Interval || relay.BatchSize != config.BatchSize {
		t.Errorf("relay = %+v, want the declared source, interval and batch size", relay)
	}
	if relay.Lease != config.Lease || relay.Concurrency != config.Concurrency || relay.MaxAttempts != config.MaxAttempts {
		t.Errorf("relay = %+v, want the declared lease, concurrency and ceiling", relay)
	}
	if relay.ShutdownGrace != config.ShutdownGrace {
		t.Errorf("ShutdownGrace = %v, want %v", relay.ShutdownGrace, config.ShutdownGrace)
	}

	// The backoff has to be inside the band the base and the ceiling describe;
	// its jitter makes an exact assertion meaningless.
	if got := relay.Backoff(1); got < config.BackoffBase/2 || got > config.BackoffBase {
		t.Errorf("Backoff(1) = %v, want between %v and %v", got, config.BackoffBase/2, config.BackoffBase)
	}
}
