package app

import (
	"context"
	"errors"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app/rpc"
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
	fields := logging.Fields{logging.KeyTenantID: rpc.Tenant}
	if mc, ok := ports.MessageContextFrom(ctx); ok {
		fields[logging.KeyCorrelationID] = mc.CorrelationID
	}
	return fields
}

func classify(err error) string {
	if failure, ok := errors.AsType[*application.Failure](err); ok {
		return string(failure.Category())
	}
	return usecase.CategoryUnclassified
}

// subject is absent by design: the identity of FND-07 has no realization in
// the kernel and this context authenticates nobody.
func subject(context.Context) string { return "" }
