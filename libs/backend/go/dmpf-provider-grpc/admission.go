// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-08 (RES-16, RES-17, MET-12) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfgrpc

import (
	"context"

	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/admission"
)

// TenantFunc resolves the tenant of the call from its context. The identity
// of FND-07 has no realization in the kernel, so the composition root injects
// it; nil means every call is an undeclared tenant.
type TenantFunc func(ctx context.Context) string

// Admission is the server interceptor of RES-16: it asks the controller before
// the handler runs (RES-17), refuses with RESOURCE_EXHAUSTED and counts it by
// route and tenant (MET-12); a route with no declared limit is UNIMPLEMENTED.
func Admission(ctrl *admission.Controller, tenant TenantFunc, instruments *metrics.Instruments) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		who := ""
		if tenant != nil {
			who = tenant(ctx)
		}
		release, reason := ctrl.Admit(info.FullMethod, who)
		defer release()

		if reason == admission.Admitted {
			return handler(ctx, req)
		}

		if instruments != nil {
			labels := metrics.Labels{}.Route(info.FullMethod).TenantWithin(ctrl.Tenants(), who)
			instruments.AdmissionRejections.Add(ctx, 1, metric.WithAttributes(labels.Attributes()...))
		}
		if reason == admission.UndeclaredRoute {
			return nil, status.Errorf(codes.Unimplemented, "dmpfgrpc: %s declares no admission limit (RES-16)", info.FullMethod)
		}
		return nil, status.Errorf(codes.ResourceExhausted, "dmpfgrpc: admission refused: %s (RES-17)", reason)
	}
}
