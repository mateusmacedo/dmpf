package dist

const (
	// CodeDrainedTwice is the record that reached the topic more than once:
	// two relays claimed it, and the lease of OBX-08 did not hold.
	CodeDrainedTwice = "DMPF-P001"
	CodeMissing      = "DMPF-P002"
	CodeHashMismatch = "DMPF-P003"
)

type Diagnostic struct {
	Code   string
	Detail string
}

func (d Diagnostic) String() string { return d.Code + ": " + d.Detail }

type Verdict struct{ Diagnostics []Diagnostic }

func (v Verdict) OK() bool { return len(v.Diagnostics) == 0 }

func (v Verdict) Failures() []string {
	out := make([]string, 0, len(v.Diagnostics))
	for _, d := range v.Diagnostics {
		out = append(out, d.String())
	}
	return out
}

// Published is one envelope as it reached the topic. PayloadHash is recomputed
// over the bytes the topic carried, never read from the envelope: the digest
// does not travel, it is what the outbox stored and the relay checked.
type Published struct {
	MessageID   string
	PayloadHash string
}

func DecideRelay(settled map[string]string, published []Published) Verdict {
	var v Verdict
	seen := map[string]int{}
	for _, p := range published {
		seen[p.MessageID]++
		stored, known := settled[p.MessageID]
		if known && stored != p.PayloadHash {
			v.Diagnostics = append(v.Diagnostics, Diagnostic{
				Code:   CodeHashMismatch,
				Detail: "message " + p.MessageID + " arrived hashing to " + p.PayloadHash + ", stored " + stored,
			})
		}
	}
	for id := range settled {
		switch n := seen[id]; {
		case n == 0:
			v.Diagnostics = append(v.Diagnostics, Diagnostic{
				Code:   CodeMissing,
				Detail: "message " + id + " settled in the outbox and never reached the topic",
			})
		case n > 1:
			v.Diagnostics = append(v.Diagnostics, Diagnostic{
				Code:   CodeDrainedTwice,
				Detail: "message " + id + " reached the topic more than once",
			})
		}
	}
	return v
}
