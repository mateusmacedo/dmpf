package memory

import (
	"fmt"
	"sync"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// FixedClock is a clock that never moves, so a test can assert occurred_at.
type FixedClock struct{ At ports.Instant }

// Now returns At.
func (c FixedClock) Now() ports.Instant { return c.At }

// SequenceIDs issues Prefix plus a six-digit counter: "m-000001", "m-000002".
// It guards the counter because one generator is shared by concurrent callers.
type SequenceIDs struct {
	Prefix string

	mu     sync.Mutex
	issued int
}

// NewMessageID issues the next identifier of the sequence.
func (g *SequenceIDs) NewMessageID() ports.MessageID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.issued++
	return ports.MessageID(fmt.Sprintf("%s%06d", g.Prefix, g.issued))
}
