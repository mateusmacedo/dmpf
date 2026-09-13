// comment-discipline-ok-file: arquivo de contrato interno; cada godoc cita a regra de FND-06 (SQS-08, SQS-08b) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfsqs

import (
	"context"
	"sync"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
)

// heartbeat extends the visibility of one message while it is processed
// (SQS-08), on the injected clock, until stopped, the context ends, or twelve
// hours from receipt pass (SQS-08b); each tick asks for what is left, never more.
type heartbeat struct {
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// startHeartbeat runs tick(ctx, remaining) every interval, each tick bounded by
// one interval so a call stuck at the broker cannot hold the gesture; giveUp
// runs once at the ceiling or on a failed tick: invisibility is no longer sure.
func startHeartbeat(ctx context.Context, c clock.Clock, every time.Duration, ceiling time.Time, tick func(context.Context, time.Duration) error, giveUp func()) *heartbeat {
	h := &heartbeat{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(h.done)
		timer := c.NewTimer(every)
		defer timer.Stop()
		for {
			select {
			case <-h.stop:
				return
			case <-ctx.Done():
				return
			case <-timer.C():
			}
			remaining := ceiling.Sub(c.Now())
			if remaining <= 0 {
				giveUp()
				return
			}
			select {
			case <-h.stop:
				return
			default:
			}
			tickCtx, cancelTick := c.WithTimeout(ctx, every)
			err := tick(tickCtx, remaining)
			cancelTick()
			if err != nil && ctx.Err() == nil {
				giveUp()
				return
			}
			timer.Reset(every)
		}
	}()
	return h
}

// Stop ends the heartbeat and waits for a tick in flight to finish, so no
// extension can land after the gesture that follows; the wait is bounded by
// the tick's own timeout.
func (h *heartbeat) Stop() {
	h.once.Do(func() { close(h.stop) })
	<-h.done
}
