package distkit

import (
	"fmt"
	"strings"
)

// CodeR004 is the stable diagnostic of V32 (RAS-13): the final effect differs
// with redelivery.
const CodeR004 = "DMPF-R004"

// Plan is the deliberate redelivery the harness injects (KIT-06): the same
// envelope published twice and a new message identity for the same effect.
type Plan struct {
	Order      string
	Items      int32
	Deliveries []string
}

// Default is the plan of FND-04 §6.4 (V32): evt-1 twice, then evt-9 for the same order.
var Default = Plan{Order: "o-1", Items: 2, Deliveries: []string{"evt-1", "evt-1", "evt-9"}}

// Effects is the effect edge after every delivery was consumed: the four
// tables and the reservation of the plan's order as it ended up.
type Effects struct {
	Inbox, Reservations, Outbox, Quarantine int64
	Items                                   int32
	Version                                 int64
}

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

// Decide is V32: with redelivery the final effect is the one a single delivery
// produces — one reservation, the plan's items, written once. Anything else is
// DMPF-R004, naming the message identity that was redelivered and the new one
// that carried the same effect.
func Decide(e Effects, p Plan) Verdict {
	var v Verdict
	add := func(format string, args ...any) {
		v.Diagnostics = append(v.Diagnostics, Diagnostic{Code: CodeR004, Detail: fmt.Sprintf(format, args...)})
	}
	dup, fresh := p.redeliveries()
	switch {
	case e.Reservations != 1:
		add("%d reservation(s) for order %s, want 1; redelivered: %s; new identity for the same effect: %s", e.Reservations, p.Order, dup, fresh)
	case e.Items != p.Items || e.Version != 1:
		add("reservation of %s applied %d time(s) (items %d, want %d): redelivered message_id %s and new identity %s must not repeat the effect",
			p.Order, e.Version, e.Items, p.Items, dup, fresh)
	}
	return v
}

// redeliveries names the identities delivered more than once and the ones
// delivered once after the first — the two shapes of redelivery of KIT-06.
func (p Plan) redeliveries() (duplicated, fresh string) {
	seen := map[string]int{}
	var dups, news []string
	for i, id := range p.Deliveries {
		seen[id]++
		if seen[id] == 2 {
			dups = append(dups, id)
		}
		if seen[id] == 1 && i > 0 {
			news = append(news, id)
		}
	}
	return strings.Join(dups, ","), strings.Join(news, ",")
}
