package serviceskit

import "fmt"

// Diagnostic names the rule, the position in the ledger and what was seen.
type Diagnostic struct {
	Rule   string
	Seq    int
	Detail string
}

func (d Diagnostic) String() string {
	if d.Seq < 0 {
		return fmt.Sprintf("%s: %s", d.Rule, d.Detail)
	}
	return fmt.Sprintf("%s at ledger position %d: %s", d.Rule, d.Seq, d.Detail)
}

type Verdict struct{ Diagnostics []Diagnostic }

func (v Verdict) OK() bool { return len(v.Diagnostics) == 0 }

func (v Verdict) Failures() []string {
	out := make([]string, 0, len(v.Diagnostics))
	for _, d := range v.Diagnostics {
		out = append(out, d.String())
	}
	return out
}

// Expect is what the test declares about the outcome the use case reached:
// whether it was accepted, so the verdict knows if writes may persist.
type Expect struct{ Accepted bool }

// Decide reads the ledger against UOW-06..UOW-08 and the fakes' committed
// state. Every write, enqueue and registration must sit between a begin and
// its commit (UOW-07); nothing may publish (UOW-08); and under a refusal the
// store must hold no state and no outbox entry (UOW-06).
func Decide(f *Fakes, e Expect) Verdict {
	var v Verdict
	add := func(rule string, seq int, format string, args ...any) {
		v.Diagnostics = append(v.Diagnostics, Diagnostic{Rule: rule, Seq: seq, Detail: fmt.Sprintf(format, args...)})
	}

	entries := f.Ledger.Entries()
	open := false
	var writes, enqueues, begins, commits int
	for _, en := range entries {
		switch en.Gesture {
		case Begin:
			if open {
				add("UOW-01", en.Seq, "a second begin before the first transaction closed")
			}
			open = true
			begins++
		case Commit, Rollback:
			if !open {
				add("UOW-07", en.Seq, "%s without an open transaction", en.Gesture)
			}
			open = false
			if en.Gesture == Commit {
				commits++
			}
		case Write, Enqueue, Register:
			if !open {
				add("UOW-07", en.Seq, "%s outside any transaction", en)
			}
			switch en.Gesture {
			case Write:
				writes++
			case Enqueue:
				enqueues++
			}
		case Publish:
			add("UOW-08", en.Seq, "the use case published to the broker (%s); publication is the relay's, after the commit", en.Detail)
		}
	}
	if open {
		add("UOW-07", len(entries)-1, "the transaction was never closed")
	}
	if begins > 1 {
		add("UOW-01", -1, "%d transactions opened, want exactly one per use case", begins)
	}

	if e.Accepted {
		if writes > 0 && enqueues == 0 {
			add("UOW-07", -1, "state was written but no outbox entry was enqueued in the same transaction")
		}
		if enqueues > 0 && writes == 0 {
			add("UOW-07", -1, "an outbox entry was enqueued but no state was written in the same transaction")
		}
		if commits != 1 {
			add("UOW-07", -1, "%d commits, want exactly one carrying steps 6 and 7 together", commits)
		}
	} else {
		if writes > 0 || enqueues > 0 {
			add("UOW-06", -1, "%d write(s) and %d enqueue(s) under a refusal, want none", writes, enqueues)
		}
		if n := f.EntriesSinceBaseline(); n != 0 {
			add("UOW-06", -1, "the outbox gained %d entr(y/ies) under a refusal, want none", n)
		}
	}
	return v
}
