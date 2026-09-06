// comment-discipline-ok-file: arquivo de contrato interno; o godoc cita a regra de FND-08 (RES-22, RES-23) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfgrpc

import (
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/compose"
)

const spanPrefix = "dmpf.grpc.client "

// composition is what dmpf-transport/compose needs from this provider: the
// sheet and sinks of the Config, the span prefix and the gRPC status code as
// the failure category (RES-22, RES-23).
func composition(cfg Config) compose.Config {
	return compose.Config{
		Sheet:       cfg.Sheet,
		Service:     cfg.Service,
		SpanPrefix:  spanPrefix,
		Clock:       cfg.Clock,
		Tracer:      cfg.Tracer,
		Instruments: cfg.Instruments,
		Logger:      cfg.Logger,
		Rand:        cfg.Rand,
		Category:    categoryOf,
	}
}
