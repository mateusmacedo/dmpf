package serviceskit

import (
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/stable"
)

// Gesture is one thing the service did to a port, as the fakes observe it.
type Gesture string

const (
	Begin    Gesture = "begin"
	Write    Gesture = "write"
	Enqueue  Gesture = "enqueue"
	Register Gesture = "register-inbox"
	Commit   Gesture = "commit"
	Rollback Gesture = "rollback"
	Publish  Gesture = "publish"
)

// Entry is one gesture at its position in the ledger; Detail names the target
// (an aggregate identifier, a message identifier, a destination).
type Entry struct {
	Seq     int
	Gesture Gesture
	Detail  string
	// Tx is the transaction the gesture belongs to — the one whose ports made
	// it — so a port that escaped an earlier transaction is caught even when it
	// is used between the current begin and commit.
	Tx int
}

func (e Entry) String() string {
	if e.Detail == "" {
		return string(e.Gesture)
	}
	return string(e.Gesture) + "(" + e.Detail + ")"
}

// Ledger is the ordered record of gestures a use case made through the fakes:
// position, not time, is what decides UOW-06..UOW-08 (KIT-08).
type Ledger struct{ seq stable.Sequence[Entry] }

func (l *Ledger) record(tx int, g Gesture, detail string) {
	l.seq.Append(Entry{Gesture: g, Detail: detail, Tx: tx})
}

// Entries returns the gestures in order, each numbered by its position.
func (l *Ledger) Entries() []Entry {
	items := l.seq.Items()
	for i := range items {
		items[i].Seq = i
	}
	return items
}

func (l *Ledger) String() string {
	parts := make([]string, 0, l.seq.Len())
	for _, e := range l.Entries() {
		parts = append(parts, e.String())
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// Reset forgets what was recorded, so a fixture's own transactions do not count
// against the use case under test.
func (l *Ledger) Reset() { l.seq = stable.Sequence[Entry]{} }
