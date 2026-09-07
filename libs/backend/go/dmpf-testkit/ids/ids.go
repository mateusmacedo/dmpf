package ids

import (
	"fmt"
	"math/rand/v2"
	"sync"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// Sequence issues Prefix plus a six-digit counter ("m-000001"), the format the
// example providers already assert on. The counter is guarded because one
// generator is shared by concurrent callers.
type Sequence struct {
	Prefix string

	mu     sync.Mutex
	issued int
}

func (g *Sequence) NewMessageID() dmpfports.MessageID {
	return dmpfports.MessageID(g.next())
}

func (g *Sequence) next() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.issued++
	return fmt.Sprintf("%s%06d", g.Prefix, g.issued)
}

// Seeded issues 128-bit hex identifiers from a PCG stream fixed by the seed
// (KIT-07): two runs with the same seed see the same identifiers in the same
// order, which is what a golden comparison across stacks needs.
type Seeded struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func NewSeeded(seed uint64) *Seeded {
	return &Seeded{rng: rand.New(rand.NewPCG(seed, seed))}
}

func (g *Seeded) NewMessageID() dmpfports.MessageID {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dmpfports.MessageID(fmt.Sprintf("%016x%016x", g.rng.Uint64(), g.rng.Uint64()))
}

// ClaimIDs satisfies relay.ClaimIDs of dmpf-app by shape — NewClaimID() string
// — without importing it: a provider-block unit must not depend on app.
type ClaimIDs struct{ seq Sequence }

func NewClaimIDs() *ClaimIDs { return &ClaimIDs{seq: Sequence{Prefix: "claim-"}} }

func (c *ClaimIDs) NewClaimID() string { return c.seq.next() }
