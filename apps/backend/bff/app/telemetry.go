package app

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
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
	if call, ok := rpc.CallFrom(ctx); ok {
		fields[logging.KeyCorrelationID] = call.CorrelationID
		fields[logging.KeyRequestID] = call.RequestID
	}
	return fields
}
