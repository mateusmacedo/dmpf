package dist_test

import (
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
)

const (
	hashOfOne = "sha-256:aaa"
	hashOfTwo = "sha-256:bbb"
)

func TestOneRecordOnceEachIsTheAcceptingVerdict(t *testing.T) {
	v := dist.DecideRelay(
		map[string]string{"m-1": hashOfOne, "m-2": hashOfTwo},
		[]dist.Published{{MessageID: "m-1", PayloadHash: hashOfOne}, {MessageID: "m-2", PayloadHash: hashOfTwo}})

	if !v.OK() {
		t.Fatalf("DecideRelay() = %v, want no diagnostics", v.Failures())
	}
}

func TestARecordOnTheTopicTwiceIsDrainedTwice(t *testing.T) {
	v := dist.DecideRelay(
		map[string]string{"m-1": hashOfOne},
		[]dist.Published{{MessageID: "m-1", PayloadHash: hashOfOne}, {MessageID: "m-1", PayloadHash: hashOfOne}})

	if v.OK() || !strings.Contains(strings.Join(v.Failures(), " "), dist.CodeDrainedTwice) {
		t.Fatalf("DecideRelay() = %v, want %s: the lease of OBX-08 did not hold", v.Failures(), dist.CodeDrainedTwice)
	}
}

func TestARecordThatNeverReachedTheTopicIsMissing(t *testing.T) {
	v := dist.DecideRelay(
		map[string]string{"m-1": hashOfOne, "m-2": hashOfTwo},
		[]dist.Published{{MessageID: "m-1", PayloadHash: hashOfOne}})

	if v.OK() || !strings.Contains(strings.Join(v.Failures(), " "), dist.CodeMissing) {
		t.Fatalf("DecideRelay() = %v, want %s", v.Failures(), dist.CodeMissing)
	}
}

func TestAPayloadThatArrivedAlteredIsRefused(t *testing.T) {
	v := dist.DecideRelay(
		map[string]string{"m-1": hashOfOne},
		[]dist.Published{{MessageID: "m-1", PayloadHash: hashOfTwo}})

	if v.OK() || !strings.Contains(strings.Join(v.Failures(), " "), dist.CodeHashMismatch) {
		t.Fatalf("DecideRelay() = %v, want %s", v.Failures(), dist.CodeHashMismatch)
	}
}

func closedOrphan() dist.CrashOutcome {
	return dist.CrashOutcome{
		Attempts: []dist.AttemptRow{{N: 1, State: "finished", Class: ports.ClassUnknownSent}},
		Steps:    []application.ExternalStep{{Attempt: ports.Attempt{N: 1}, Class: ports.ClassUnknownSent}},
		Submits:  1,
	}
}

func TestDecideCrash(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*dist.CrashOutcome)
		want   string
	}{
		{name: "the orphan closed once and never resent"},
		{name: "an orphan left begun", mutate: func(o *dist.CrashOutcome) { o.Attempts[0] = dist.AttemptRow{N: 1, State: "begun"} }, want: dist.CodeOrphanNotClosed},
		{name: "an orphan closed with another class", mutate: func(o *dist.CrashOutcome) { o.Attempts[0].Class = ports.ClassAccepted }, want: dist.CodeOrphanNotClosed},
		{name: "a second attempt", mutate: func(o *dist.CrashOutcome) { o.Attempts = append(o.Attempts, dist.AttemptRow{N: 2, State: "begun"}) }, want: dist.CodeOrphanNotClosed},
		{name: "a resend", mutate: func(o *dist.CrashOutcome) { o.Submits = 2 }, want: dist.CodeResent},
		{name: "no step", mutate: func(o *dist.CrashOutcome) { o.Steps = nil }, want: dist.CodeStepNotOnce},
		{name: "the step twice", mutate: func(o *dist.CrashOutcome) { o.Steps = append(o.Steps, o.Steps[0]) }, want: dist.CodeStepNotOnce},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := closedOrphan()
			if tc.mutate != nil {
				tc.mutate(&o)
			}
			v := dist.DecideCrash(o)

			got := strings.Join(v.Failures(), " ")
			if tc.want == "" && !v.OK() || tc.want != "" && (len(v.Diagnostics) != 1 || !strings.Contains(got, tc.want)) {
				t.Fatalf("DecideCrash() = %q, want %q", got, tc.want)
			}
		})
	}
}
