package postgres_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func TestNewPoolRefusesAMalformedDSN(t *testing.T) {
	pool, err := postgres.NewPool(context.Background(), "://not-a-dsn", noop.NewTracerProvider().Tracer("test"))

	if err == nil {
		pool.Close()
		t.Fatal("NewPool() accepted a malformed DSN; the configuration is validated at startup")
	}
}
