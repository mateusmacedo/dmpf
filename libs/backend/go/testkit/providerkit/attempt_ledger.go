package providerkit

import (
	"context"
	"fmt"
	"sync"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// AttemptLedgerSubject is a realization of the attempt ledger over a fresh,
// empty store. Purge removes the attempts finished before the cutoff; a
// realization with no retention leaves it nil and the purge clause is skipped.
type AttemptLedgerSubject struct {
	Ledger ports.AttemptLedger
	Purge  func(before ports.Instant) (removed int64, err error)
}

const (
	kitAttemptKey     = ports.OutboundKey("dmpf-kit-key")
	kitAttemptOther   = ports.OutboundKey("dmpf-kit-other")
	kitAttemptMessage = ports.MessageID("dmpf-kit-message")
	kitBeginCallers   = 16
)

// AttemptLedger certifies ports.AttemptLedger (KIT-04, KIT-12): Begin serializes on
// the key, a begun attempt is never purged, a full row refuses the entry instead of
// growing. newSubject must return a subject over a fresh, empty ledger on every call.
func AttemptLedger(newSubject func() AttemptLedgerSubject) Verdict {
	var v Verdict

	ctx, err := scopedTo(kitTenantA)
	if err != nil {
		v.fail("setup", "KIT-04", "scopedTo(%s): %v", kitTenantA, err)
		return v
	}

	attemptBeginSerializes(newSubject, ctx, &v)
	attemptBegunIsNotPurged(newSubject, ctx, &v)
	attemptObserveSaturates(newSubject, ctx, &v)
	attemptTerminalOnAFullRowApplies(newSubject, ctx, &v)
	attemptFinishSaturatesOnConflicts(newSubject, ctx, &v)
	attemptFinishSaturatesOnObservations(newSubject, ctx, &v)
	return v
}

func attemptBeginSerializes(newSubject func() AttemptLedgerSubject, ctx context.Context, v *Verdict) {
	const clause = "concurrent begins on one key start exactly one attempt"
	s := newSubject()

	var (
		wg      sync.WaitGroup
		release = make(chan struct{})
		starts  = make([]startSeen, kitBeginCallers)
		errs    = make([]error, kitBeginCallers)
	)
	for i := range kitBeginCallers {
		wg.Go(func() {
			<-release
			start, err := s.Ledger.Begin(ctx, kitAttemptKey, kitAttemptMessage, ports.Instant(10+i))
			if err == nil {
				starts[i], err = seeStart(start)
			}
			errs[i] = err
		})
	}
	close(release)
	wg.Wait()

	started := 0
	var first ports.Attempt
	for i := range kitBeginCallers {
		if errs[i] != nil {
			v.fail(clause, "KIT-04", "Begin #%d = %v, want nil", i, errs[i])
			return
		}
		switch starts[i].branch {
		case "started":
			started++
			first = starts[i].attempt
		case "pending":
		default:
			v.fail(clause, "KIT-04", "Begin #%d = %s, want started or pending", i, starts[i].branch)
			return
		}
	}
	if started != 1 {
		v.fail(clause, "KIT-04", "%d of %d concurrent begins started an attempt, want exactly 1", started, kitBeginCallers)
		return
	}
	for i, seen := range starts {
		if seen.attempt != first {
			v.fail(clause, "KIT-04", "Begin #%d saw %+v, want every caller on the one started %+v", i, seen.attempt, first)
			return
		}
	}
}

func attemptBegunIsNotPurged(newSubject func() AttemptLedgerSubject, ctx context.Context, v *Verdict) {
	const clause = "a purge removes finished attempts past the cutoff and never a begun one"
	s := newSubject()
	if s.Purge == nil {
		v.skip(clause)
		return
	}
	if _, err := s.Ledger.Begin(ctx, kitAttemptKey, kitAttemptMessage, 1); err != nil {
		v.fail(clause, "KIT-04", "Begin(begun) = %v, want nil", err)
		return
	}
	if _, err := s.Ledger.Begin(ctx, kitAttemptOther, kitAttemptMessage, 2); err != nil {
		v.fail(clause, "KIT-04", "Begin(finished) = %v, want nil", err)
		return
	}
	if got, err := s.Ledger.Finish(ctx, kitOutcome(kitAttemptOther, ports.ClassAccepted, 10)); err != nil || got.Result != ports.FinishRecorded {
		v.fail(clause, "KIT-04", "Finish = %+v, %v; want recorded", got, err)
		return
	}
	if removed, err := s.Purge(10); err != nil || removed != 0 {
		v.fail(clause, "KIT-04", "Purge(before = finished_at) = (%d, %v), want 0 — the cutoff is exclusive", removed, err)
	}
	if removed, err := s.Purge(1_000_000); err != nil || removed != 1 {
		v.fail(clause, "KIT-04", "Purge(far cutoff) = (%d, %v), want exactly the finished attempt", removed, err)
	}
	start, err := s.Ledger.Begin(ctx, kitAttemptKey, kitAttemptMessage, 20)
	if err == nil {
		var seen startSeen
		if seen, err = seeStart(start); err == nil && (seen.branch != "pending" || seen.attempt.N != 1) {
			v.fail(clause, "KIT-04", "Begin after the purge = %s %+v, want pending on attempt 1 — the begun attempt is still owed a Finish", seen.branch, seen.attempt)
		}
	}
	if err != nil {
		v.fail(clause, "KIT-04", "Begin after the purge = %v, want nil", err)
	}
}

func attemptObserveSaturates(newSubject func() AttemptLedgerSubject, ctx context.Context, v *Verdict) {
	const clause = "an observation past the cap is saturated and records nothing"
	s := newSubject()
	if !fillObservations(s, ctx, v, clause) {
		return
	}
	want := ports.Observed{Result: ports.ObserveSaturated, N: 1, Class: ports.ClassUnknownSent, QueriesDone: ports.MaxAttemptEntries}
	for _, round := range []string{"first", "repeated"} {
		got, err := s.Ledger.Observe(ctx, kitObservation("o-past-the-cap", ports.FindingPending), 600)
		if err != nil || got != want {
			v.fail(clause, "KIT-04", "%s Observe past the cap = %+v, %v; want %+v — a recorded ID would come back a duplicate", round, got, err, want)
			return
		}
	}
	if got, err := s.Ledger.Observe(ctx, kitObservation("o-0", ports.FindingPending), 600); err != nil || got.Result != ports.ObserveDuplicate {
		v.fail(clause, "KIT-04", "Observe of a known ID on a full row = %+v, %v; want duplicate", got, err)
	}
}

func attemptTerminalOnAFullRowApplies(newSubject func() AttemptLedgerSubject, ctx context.Context, v *Verdict) {
	const clause = "a terminal finding on a full row with none terminal yet is applied, not saturated"
	s := newSubject()
	if !fillObservations(s, ctx, v, clause) {
		return
	}
	past := ports.MaxAttemptEntries + 1
	steps := []struct {
		observation ports.Observation
		want        ports.Observed
	}{
		{kitObservation("o-terminal", ports.FindingApplied), ports.Observed{Result: ports.ObserveRecorded, N: 1, Class: ports.ClassUnknownSent, QueriesDone: past}},
		{kitObservation("o-terminal", ports.FindingApplied), ports.Observed{Result: ports.ObserveDuplicate, N: 1, Class: ports.ClassUnknownSent, QueriesDone: past}},
		{kitObservation("o-contradicting", ports.FindingRejected), ports.Observed{Result: ports.ObserveSaturated, N: 1, Class: ports.ClassUnknownSent, QueriesDone: past}},
	}
	for i, step := range steps {
		if got, err := s.Ledger.Observe(ctx, step.observation, 600); err != nil || got != step.want {
			v.fail(clause, "KIT-04", "Observe #%d (%s) on a full row = %+v, %v; want %+v — a saturated terminal leaves the attempt pending forever", i, step.observation.ID, got, err, step.want)
			return
		}
	}
}

func attemptFinishSaturatesOnConflicts(newSubject func() AttemptLedgerSubject, ctx context.Context, v *Verdict) {
	const clause = "a contradicting finish past the cap of conflicts is saturated and keeps the class"
	s := newSubject()
	if !beginAndFinish(s, ctx, v, clause) {
		return
	}
	for i := range ports.MaxAttemptEntries {
		if got, err := s.Ledger.Finish(ctx, kitOutcome(kitAttemptKey, ports.ClassAccepted, 100)); err != nil || got.Result != ports.FinishConflict {
			v.fail(clause, "KIT-04", "contradicting Finish #%d = %+v, %v; want a conflict up to the cap", i, got, err)
			return
		}
	}
	saturated := ports.Finished{Result: ports.FinishSaturated, Recorded: ports.ClassUnknownSent}
	if got, err := s.Ledger.Finish(ctx, kitOutcome(kitAttemptKey, ports.ClassAccepted, 100)); err != nil || got != saturated {
		v.fail(clause, "KIT-04", "contradicting Finish past the cap = %+v, %v; want %+v", got, err, saturated)
		return
	}
	unchanged := ports.Finished{Result: ports.FinishUnchanged, Recorded: ports.ClassUnknownSent}
	if got, err := s.Ledger.Finish(ctx, kitOutcome(kitAttemptKey, ports.ClassUnknownSent, 100)); err != nil || got != unchanged {
		v.fail(clause, "KIT-04", "same-class Finish on a full row = %+v, %v; want %+v", got, err, unchanged)
	}
	expectDone(s, ctx, v, clause)
}

func attemptFinishSaturatesOnObservations(newSubject func() AttemptLedgerSubject, ctx context.Context, v *Verdict) {
	const clause = "a contradicting finish on a row full of observations is saturated"
	s := newSubject()
	if !fillObservations(s, ctx, v, clause) {
		return
	}
	saturated := ports.Finished{Result: ports.FinishSaturated, Recorded: ports.ClassUnknownSent}
	if got, err := s.Ledger.Finish(ctx, kitOutcome(kitAttemptKey, ports.ClassAccepted, 100)); err != nil || got != saturated {
		v.fail(clause, "KIT-04", "contradicting Finish on a full row = %+v, %v; want %+v", got, err, saturated)
		return
	}
	expectDone(s, ctx, v, clause)
}

func beginAndFinish(s AttemptLedgerSubject, ctx context.Context, v *Verdict, clause string) bool {
	if _, err := s.Ledger.Begin(ctx, kitAttemptKey, kitAttemptMessage, 10); err != nil {
		v.fail(clause, "KIT-04", "Begin = %v, want nil", err)
		return false
	}
	if got, err := s.Ledger.Finish(ctx, kitOutcome(kitAttemptKey, ports.ClassUnknownSent, 100)); err != nil || got.Result != ports.FinishRecorded {
		v.fail(clause, "KIT-04", "Finish = %+v, %v; want recorded", got, err)
		return false
	}
	return true
}

func fillObservations(s AttemptLedgerSubject, ctx context.Context, v *Verdict, clause string) bool {
	if !beginAndFinish(s, ctx, v, clause) {
		return false
	}
	for i := range ports.MaxAttemptEntries {
		if got, err := s.Ledger.Observe(ctx, kitObservation(fmt.Sprintf("o-%d", i), ports.FindingPending), 500); err != nil || got.Result != ports.ObserveRecorded {
			v.fail(clause, "KIT-04", "Observe(o-%d) = %+v, %v; want recorded up to the cap", i, got, err)
			return false
		}
	}
	return true
}

func expectDone(s AttemptLedgerSubject, ctx context.Context, v *Verdict, clause string) {
	start, err := s.Ledger.Begin(ctx, kitAttemptKey, kitAttemptMessage, 200)
	if err != nil {
		v.fail(clause, "KIT-04", "Begin after the saturation = %v, want nil", err)
		return
	}
	seen, err := seeStart(start)
	if err != nil || seen.branch != "done" || seen.class != ports.ClassUnknownSent {
		v.fail(clause, "KIT-04", "Begin after the saturation = %s %v (%v), want done with the class first recorded", seen.branch, seen.class, err)
	}
}

type startSeen struct {
	branch  string
	attempt ports.Attempt
	class   ports.ExternalClass
}

func seeStart(start ports.AttemptStart) (startSeen, error) {
	var seen startSeen
	err := start.Match(
		func(a ports.Attempt) error { seen = startSeen{branch: "started", attempt: a}; return nil },
		func(a ports.Attempt) error { seen = startSeen{branch: "pending", attempt: a}; return nil },
		func(a ports.Attempt, c ports.ExternalClass) error {
			seen = startSeen{branch: "done", attempt: a, class: c}
			return nil
		},
	)
	return seen, err
}

func kitOutcome(key ports.OutboundKey, class ports.ExternalClass, at ports.Instant) ports.AttemptOutcome {
	return ports.AttemptOutcome{Key: key, N: 1, Class: class, FinishedAt: at}
}

func kitObservation(id string, finding ports.Finding) ports.Observation {
	return ports.Observation{Key: kitAttemptKey, Finding: finding, ID: id, Provider: "dmpf-kit-partner"}
}
