// comment-discipline-ok-file: arquivo de contrato interno; o godoc cita a regra de FND-06 (TRP-26, TRP-27) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfkafka

import (
	"context"
	"sync"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

type gesture int

const (
	undecided gesture = iota
	acked
	released
)

// acknowledger realizes dmpfports.Acknowledger for one delivery: it records
// the gesture the adapter chose after its transaction (TRP-26) and refuses a
// second one (TRP-27); the worker reads it and applies the broker effect.
type acknowledger struct {
	mu      sync.Mutex
	gesture gesture
}

var _ dmpfports.Acknowledger = (*acknowledger)(nil)

func (a *acknowledger) Ack(context.Context) error     { return a.decide(acked) }
func (a *acknowledger) Release(context.Context) error { return a.decide(released) }

func (a *acknowledger) decide(g gesture) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.gesture != undecided {
		return ErrAlreadyDisposed
	}
	a.gesture = g
	return nil
}

func (a *acknowledger) decision() gesture {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.gesture
}
