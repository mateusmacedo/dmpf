package distkit_test

import (
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/distkit"
)

const (
	hashOfOne = "sha-256:aaa"
	hashOfTwo = "sha-256:bbb"
)

func TestOneRecordOnceEachIsTheAcceptingVerdict(t *testing.T) {
	v := distkit.Decide(
		map[string]string{"m-1": hashOfOne, "m-2": hashOfTwo},
		[]distkit.Published{{MessageID: "m-1", PayloadHash: hashOfOne}, {MessageID: "m-2", PayloadHash: hashOfTwo}})

	if !v.OK() {
		t.Fatalf("Decide() = %v, want no diagnostics", v.Failures())
	}
}

func TestARecordOnTheTopicTwiceIsDrainedTwice(t *testing.T) {
	v := distkit.Decide(
		map[string]string{"m-1": hashOfOne},
		[]distkit.Published{{MessageID: "m-1", PayloadHash: hashOfOne}, {MessageID: "m-1", PayloadHash: hashOfOne}})

	if v.OK() || !strings.Contains(strings.Join(v.Failures(), " "), distkit.CodeDrainedTwice) {
		t.Fatalf("Decide() = %v, want %s: the lease of OBX-08 did not hold", v.Failures(), distkit.CodeDrainedTwice)
	}
}

func TestARecordThatNeverReachedTheTopicIsMissing(t *testing.T) {
	v := distkit.Decide(
		map[string]string{"m-1": hashOfOne, "m-2": hashOfTwo},
		[]distkit.Published{{MessageID: "m-1", PayloadHash: hashOfOne}})

	if v.OK() || !strings.Contains(strings.Join(v.Failures(), " "), distkit.CodeMissing) {
		t.Fatalf("Decide() = %v, want %s", v.Failures(), distkit.CodeMissing)
	}
}

func TestAPayloadThatArrivedAlteredIsRefused(t *testing.T) {
	v := distkit.Decide(
		map[string]string{"m-1": hashOfOne},
		[]distkit.Published{{MessageID: "m-1", PayloadHash: hashOfTwo}})

	if v.OK() || !strings.Contains(strings.Join(v.Failures(), " "), distkit.CodeHashMismatch) {
		t.Fatalf("Decide() = %v, want %s", v.Failures(), distkit.CodeHashMismatch)
	}
}
