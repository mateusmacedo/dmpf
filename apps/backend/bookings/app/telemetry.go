package app

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Tenant attributes every line of this context, which serves one tenancy.
const Tenant = "public"

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
	fields := logging.Fields{logging.KeyTenantID: Tenant}
	if mc, ok := ports.MessageContextFrom(ctx); ok {
		fields[logging.KeyCorrelationID] = mc.CorrelationID
	}
	return fields
}
