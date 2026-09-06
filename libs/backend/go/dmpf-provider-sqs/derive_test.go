package dmpfsqs_test

import (
	"regexp"
	"strings"
	"testing"

	dmpfsqs "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-sqs"
)

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestGroupIDIsAFixedLengthDigestPreservingEquality(t *testing.T) {
	long := strings.Repeat("k", 500)
	a, b, c := dmpfsqs.GroupID("k1"), dmpfsqs.GroupID("k1"), dmpfsqs.GroupID(long)
	if !hex64.MatchString(a) || !hex64.MatchString(c) {
		t.Fatalf("GroupID is not 64 hex: %q %q (SQS-06b)", a, c)
	}
	if a != b {
		t.Fatal("equal keys gave different groups (SQS-05)")
	}
	if a == dmpfsqs.GroupID("k2") {
		t.Fatal("distinct keys gave the same group (SQS-05)")
	}
}

func TestDedupIDCombinesSourceIDAndHash(t *testing.T) {
	base := dmpfsqs.DedupID("urn:a", "evt-1", "h1")
	if !hex64.MatchString(base) {
		t.Fatalf("DedupID is not 64 hex: %q", base)
	}
	if base != dmpfsqs.DedupID("urn:a", "evt-1", "h1") {
		t.Fatal("the same triple gave different ids")
	}
	if base == dmpfsqs.DedupID("urn:b", "evt-1", "h1") {
		t.Fatal("another source collided: the id is unique only within a source (SQS-06)")
	}
	if base == dmpfsqs.DedupID("urn:a", "evt-1", "h2") {
		t.Fatal("the same id with another payload was suppressed: it must reach the inbox as R4 (SQS-06)")
	}
	if dmpfsqs.DedupID("a", "b\x00c", "d") == dmpfsqs.DedupID("a\x00b", "c", "d") {
		t.Fatal("the separator lets fields shift into one another")
	}
}
