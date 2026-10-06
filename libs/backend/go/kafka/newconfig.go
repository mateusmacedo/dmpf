package kafka

import (
	"context"
	"crypto/tls"

	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

// NewConfig is the Kafka side of a process. TLS at 1.2 or above is the
// default; insecure opts out for development and CI only, and says so in the
// log, because a process that silently dropped its transport security would
// look identical to one that never had it.
func NewConfig(ctx context.Context, rt *otelboot.Runtime, catalog channel.Catalog, brokers []string, insecure bool, auth ClientAuth) (Config, error) {
	cfg := Config{
		Brokers:        brokers,
		Catalog:        catalog,
		Sheet:          resilience.Defaults("kafka"),
		Clock:          obsclock.System(),
		Tracer:         rt.Tracer(),
		Instruments:    rt.Instruments(),
		LoggerProvider: rt.LoggerProvider(),
	}
	if insecure {
		cfg.logger().WarnContext(ctx, "kafka transport without TLS: KAFKA_INSECURE is set (development and CI only)")
		cfg.InsecureForDevelopmentOnly = true
	} else {
		cfg.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if err := auth.apply(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
