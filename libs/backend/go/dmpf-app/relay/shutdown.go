package relay

import (
	"context"
	"sync"
	"time"

	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

// releaseReason is empty on purpose: a released claim did not fail, and the
// store keeps whatever last_error was already there — usually the reason the
// last delivery attempt did not land, which is the only diagnosis the row has.
const releaseReason = ""

// held tracks the claims one scan owns. A record leaves the set the moment a
// transition lands on it; whatever is left when the scan ends is a claim nobody
// finished, and OBX-13 says it goes back to the pool.
type held struct {
	mu      sync.Mutex
	records map[int64]dmpfpostgres.Claimed
}

func newHeld() *held { return &held{records: make(map[int64]dmpfpostgres.Claimed)} }

func (h *held) take(record dmpfpostgres.Claimed) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records[record.ID] = record
}

func (h *held) settled(id int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.records, id)
}

func (h *held) remaining() []dmpfpostgres.Claimed {
	h.mu.Lock()
	defer h.mu.Unlock()

	remaining := make([]dmpfpostgres.Claimed, 0, len(h.records))
	for _, record := range h.records {
		remaining = append(remaining, record)
	}
	return remaining
}

// release hands every unfinished claim back to the pool at the current instant,
// so the record is drainable again immediately instead of after the lease runs
// out. It writes on a context of its own, because the caller's may already be
// cancelled — that is what ends the loop — and reusing it would fail every
// write, the same reason the unit of work rolls back WithoutCancel.
func (r Relay) release(ctx context.Context, records []dmpfpostgres.Claimed) {
	if len(records) == 0 {
		return
	}

	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.shutdownGrace())
	defer cancel()

	now := r.Clock.Now()
	for _, record := range records {
		// A rejected release means the claim was already replaced, which is
		// the outcome OBX-10 describes and needs no repair.
		_, _ = r.Store.Reschedule(cleanup, record.ID, record.LockedBy, now, releaseReason)
	}
}

func (r Relay) shutdownGrace() time.Duration {
	if r.ShutdownGrace <= 0 {
		return defaultShutdownGrace
	}
	return r.ShutdownGrace
}

// defaultShutdownGrace is a deadline, not an operational tuning knob: it bounds
// the writes that run detached from the caller's cancellation when the caller
// declared nothing, so a stuck database cannot hold the process open forever.
const defaultShutdownGrace = 5 * time.Second
