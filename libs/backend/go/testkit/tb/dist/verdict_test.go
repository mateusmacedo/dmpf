package dist_test

import (
	"strings"
	"testing"

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
