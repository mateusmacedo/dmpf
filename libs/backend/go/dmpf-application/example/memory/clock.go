package memory

import (
	"fmt"
	"sync"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

// FixedClock is a clock that never moves, so a test can assert occurred_at.
type FixedClock struct{ At dmpfports.Instant }

// Now returns At.
func (c FixedClock) Now() dmpfports.Instant { return c.At }

// SequenceIDs issues Prefix plus a six-digit counter: "m-000001", "m-000002".
// It guards the counter because one generator is shared by concurrent callers.
type SequenceIDs struct {
	Prefix string

	mu     sync.Mutex
	issued int
}

// NewMessageID issues the next identifier of the sequence.
func (g *SequenceIDs) NewMessageID() dmpfports.MessageID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.issued++
	return dmpfports.MessageID(fmt.Sprintf("%s%06d", g.Prefix, g.issued))
}
