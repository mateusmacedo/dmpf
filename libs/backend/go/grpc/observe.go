// comment-discipline-ok-file: arquivo de contrato interno; o godoc cita a regra de FND-08 (RES-22, RES-23) que o símbolo realiza, dentro do limite de 3 linhas.

package grpc

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/compose"
)

// composition is what transport/compose needs from this provider: the
// sheet and sinks of the Config and the FND-07 category of the status code
// as the failure category (RES-22, RES-23).
func composition(cfg Config) compose.Config {
	return compose.Config{
		Sheet:          cfg.Sheet,
		Clock:          cfg.Clock,
		Tracer:         cfg.Tracer,
		Instruments:    cfg.Instruments,
		LoggerProvider: cfg.LoggerProvider,
		Rand:           cfg.Rand,
		Category:       categoryOf,
		BreakerFailure: dependencyFailure,
	}
}
