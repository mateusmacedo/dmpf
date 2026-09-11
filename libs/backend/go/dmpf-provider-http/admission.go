// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-08 (RES-16, RES-17, MET-12) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfhttp

import (
	"net/http"

	"go.opentelemetry.io/otel/metric"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/admission"
)

// TenantFunc resolves the tenant of a request. The identity of FND-07 has no
// realization in the kernel, so the composition root injects it; nil means
// every request is an undeclared tenant.
type TenantFunc func(*http.Request) string

// RouteFunc names the admission route of a request — the declared key, not
// the raw path, so the key space stays bounded (MET-07). The default is METHOD
// + space + URL path; an undeclared route is recorded as UndeclaredRouteLabel.
type RouteFunc func(*http.Request) string

// UndeclaredRouteLabel is the route label of every refusal for a route with no
// declared limit: the request's own path would open the series to any client.
const UndeclaredRouteLabel = "undeclared"

func defaultRoute(r *http.Request) string { return r.Method + " " + r.URL.Path }

// Admission is the middleware of RES-16: it asks the controller before the
// handler and before the body is read (RES-17), answers 429 and counts the
// refusal by route and tenant (MET-12); a route with no declared limit is 404.
func Admission(ctrl *admission.Controller, route RouteFunc, tenant TenantFunc, instruments *metrics.Instruments) func(http.Handler) http.Handler {
	if route == nil {
		route = defaultRoute
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			who := ""
			if tenant != nil {
				who = tenant(r)
			}
			key := route(r)
			release, reason := ctrl.Admit(key, who)
			defer release()

			if reason == admission.Admitted {
				next.ServeHTTP(w, r)
				return
			}

			if instruments != nil {
				label := key
				if reason == admission.UndeclaredRoute {
					label = UndeclaredRouteLabel
				}
				labels := metrics.Labels{}.Route(label).TenantWithin(ctrl.Tenants(), who)
				instruments.AdmissionRejections.Add(r.Context(), 1, metric.WithAttributes(labels.Attributes()...))
			}
			if reason == admission.UndeclaredRoute {
				http.Error(w, "route declares no admission limit", http.StatusNotFound)
				return
			}
			w.Header().Set("Retry-After", "1")
			http.Error(w, "admission refused: "+string(reason), http.StatusTooManyRequests)
		})
	}
}
