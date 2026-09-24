package sqs_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/sqs"
)

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestGroupIDIsAFixedLengthDigestPreservingEquality(t *testing.T) {
	long := strings.Repeat("k", 500)
	a, b, c := sqs.GroupID("k1"), sqs.GroupID("k1"), sqs.GroupID(long)
	if !hex64.MatchString(a) || !hex64.MatchString(c) {
		t.Fatalf("GroupID is not 64 hex: %q %q (SQS-06b)", a, c)
	}
	if a != b {
		t.Fatal("equal keys gave different groups (SQS-05)")
	}
	if a == sqs.GroupID("k2") {
		t.Fatal("distinct keys gave the same group (SQS-05)")
	}
}

func TestDedupIDCombinesSourceIDAndHash(t *testing.T) {
	base := sqs.DedupID("urn:a", "evt-1", "h1")
	if !hex64.MatchString(base) {
		t.Fatalf("DedupID is not 64 hex: %q", base)
	}
	if base != sqs.DedupID("urn:a", "evt-1", "h1") {
		t.Fatal("the same triple gave different ids")
	}
	if base == sqs.DedupID("urn:b", "evt-1", "h1") {
		t.Fatal("another source collided: the id is unique only within a source (SQS-06)")
	}
	if base == sqs.DedupID("urn:a", "evt-1", "h2") {
		t.Fatal("the same id with another payload was suppressed: it must reach the inbox as R4 (SQS-06)")
	}
	if sqs.DedupID("a", "b\x00c", "d") == sqs.DedupID("a\x00b", "c", "d") {
		t.Fatal("the separator lets fields shift into one another")
	}
}
