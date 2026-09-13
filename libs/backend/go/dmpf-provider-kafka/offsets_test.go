package dmpfkafka_test

import (
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	dmpfkafka "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-kafka"
)

func records(offsets ...int64) []*kgo.Record {
	out := make([]*kgo.Record, 0, len(offsets))
	for _, o := range offsets {
		out = append(out, &kgo.Record{Topic: "t", Partition: 0, Offset: o})
	}
	return out
}

func TestCursorCommitsOnlyTheContiguousPrefix(t *testing.T) {
	var c dmpfkafka.Cursor
	for _, r := range records(10, 11, 12) {
		c.TrackRecord(r)
	}

	if _, ok := c.NextContiguous(); ok {
		t.Fatal("NextContiguous() advanced with nothing acknowledged")
	}

	c.Mark(11)
	if _, ok := c.NextContiguous(); ok {
		t.Fatal("NextContiguous() advanced past a pending head (TRP-29)")
	}

	c.Mark(10)
	last, ok := c.NextContiguous()
	if !ok || last.Offset != 11 {
		t.Fatalf("NextContiguous() = %v, %v; want the record at 11: 10 and 11 are contiguous, 12 is pending", last, ok)
	}
	if c.Pending() != 1 {
		t.Fatalf("Pending() = %d, want 1", c.Pending())
	}

	c.Mark(12)
	last, ok = c.NextContiguous()
	if !ok || last.Offset != 12 {
		t.Fatalf("NextContiguous() = %v, %v; want 12", last, ok)
	}
	if c.Pending() != 0 {
		t.Fatalf("Pending() = %d, want 0", c.Pending())
	}
}

func TestCursorIgnoresAnUnknownOffsetAndIsIdempotentOnMark(t *testing.T) {
	var c dmpfkafka.Cursor
	for _, r := range records(5, 6) {
		c.TrackRecord(r)
	}
	c.Mark(99)
	c.Mark(5)
	c.Mark(5)
	last, ok := c.NextContiguous()
	if !ok || last.Offset != 5 {
		t.Fatalf("NextContiguous() = %v, %v; want 5", last, ok)
	}
	if _, ok := c.NextContiguous(); ok {
		t.Fatal("a second NextContiguous() without new acks advanced")
	}
}
