package application

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func (s Service) instrumentation() ports.Instrumentation {
	if s.Instrumentation == nil {
		return ports.NoInstrumentation()
	}
	return s.Instrumentation
}
