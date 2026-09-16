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
// outbox. Every gesture belongs to the transaction whose port made it; in every
// committed transaction state writes and outbox entries come together or not
// at all, and the outbox gained exactly what was enqueued (UOW-07); nothing
// may publish (UOW-08); a use case commits exactly once, and under a refusal
// that commit carries no write, no enqueue and no new outbox entry (UOW-06).
func Decide(f *Fakes, e Expect) Verdict {
	var v Verdict
	add := func(rule string, seq int, format string, args ...any) {
		v.Diagnostics = append(v.Diagnostics, Diagnostic{Rule: rule, Seq: seq, Detail: fmt.Sprintf(format, args...)})
	}

	type tx struct {
		id               int
		begin            int
		writes, enqueues int
		committed        bool
	}
	var (
		entries = f.Ledger.Entries()
		txs     []tx
		cur     *tx
	)
	for _, en := range entries {
		switch en.Gesture {
		case Begin:
			if cur != nil {
				add("UOW-01", en.Seq, "a second begin before the first transaction closed")
			}
			txs = append(txs, tx{id: en.Tx, begin: en.Seq})
			cur = &txs[len(txs)-1]
		case Commit, Rollback:
			if cur == nil {
				add("UOW-07", en.Seq, "%s without an open transaction", en.Gesture)
				continue
			}
			cur.committed = en.Gesture == Commit
			cur = nil
		case Write, Enqueue, Register:
			switch {
			case cur == nil:
				add("UOW-07", en.Seq, "%s outside any transaction", en)
				continue
			case en.Tx != cur.id:
				add("UOW-07", en.Seq, "%s through a port of transaction %d while transaction %d is open — the write lands in a discarded copy", en, en.Tx, cur.id)
				continue
			}
			switch en.Gesture {
			case Write:
				cur.writes++
			case Enqueue:
				cur.enqueues++
			}
		case Publish:
			add("UOW-08", en.Seq, "the use case published to the broker (%s); publication is the relay's, after the commit", en.Detail)
		}
	}
	if cur != nil {
		add("UOW-07", len(entries)-1, "the transaction was never closed")
	}
	if len(txs) > 1 {
		add("UOW-01", -1, "%d transactions opened, want exactly one per use case", len(txs))
	}

	var committed, effects, enqueued int
	for _, t := range txs {
		if !t.committed {
			continue
		}
		committed++
		enqueued += t.enqueues
		if t.writes > 0 || t.enqueues > 0 {
			effects++
		}
		switch {
		case t.writes > 0 && t.enqueues == 0:
			add("UOW-07", t.begin, "the transaction wrote state but enqueued no outbox entry")
		case t.enqueues > 0 && t.writes == 0:
			add("UOW-07", t.begin, "the transaction enqueued an outbox entry but wrote no state")
		}
	}
	// A refusal still commits, empty of effects (UOW-06 rationale): no begin,
	// or a rollback, would make it indistinguishable from a technical failure.
	if committed != 1 {
		add("UOW-06", -1, "%d committed transaction(s), want exactly one — a use case commits once, effect-free under a refusal", committed)
	}

	if e.Accepted {
		if n := f.EntriesSinceBaseline(); n != enqueued {
			add("UOW-07", -1, "the outbox gained %d entr(y/ies) but the ledger enqueued %d — the commit did not carry what the use case wrote", n, enqueued)
		}
	} else {
		if effects != 0 {
			add("UOW-06", -1, "%d committed transaction(s) carried a write or an enqueue under a refusal, want none", effects)
		}
		if n := f.EntriesSinceBaseline(); n != 0 {
			add("UOW-06", -1, "the outbox gained %d entr(y/ies) under a refusal, want none", n)
		}
	}
	return v
}
