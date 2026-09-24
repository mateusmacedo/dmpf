package app

import (
	"context"
	"errors"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	obsusecase "github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// TelemetryOf is what this process declares about itself to the telemetry.
func TelemetryOf(cfg Config) boot.Telemetry {
	return boot.Telemetry{
		Service:  cfg.Service,
		Version:  cfg.Version,
		Instance: cfg.Instance,
		Endpoint: cfg.OTLPEndpoint,
		Insecure: cfg.OTLPInsecure,
		Class:    tracing.ClassWrite,
		Fields:   requestFields,
	}
}

func requestFields(ctx context.Context) logging.Fields {
	fields := logging.Fields{}
	if execution, ok := ports.ExecutionContextFrom(ctx); ok {
		if tenant, scoped := execution.Tenant(); scoped {
			fields[logging.KeyTenantID] = string(tenant)
		}
	}
	if mc, ok := ports.MessageContextFrom(ctx); ok {
		fields[logging.KeyCorrelationID] = mc.CorrelationID
	}
	return fields
}

func classify(err error) string {
	if failure, ok := errors.AsType[*application.Failure](err); ok {
		return string(failure.Category())
	}
	return obsusecase.CategoryUnclassified
}

// subject is read from the carrier, the one source of identity (CTX-03). Over
// gRPC it is usually absent, because the fan-out never carries it (IDN-02), and
// absent stays absent (IDN-20).
func subject(ctx context.Context) string {
	execution, ok := ports.ExecutionContextFrom(ctx)
	if !ok {
		return ""
	}
	who, _ := execution.Subject()
	return string(who)
}
