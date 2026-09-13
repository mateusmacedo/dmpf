// comment-discipline-ok-file: arquivo de contrato interno; o godoc cita a regra de FND-06 (TRP-29) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfkafka

import "github.com/twmb/franz-go/pkg/kgo"

// cursor is the contiguous acknowledgement of one partition (TRP-29): tracked
// in delivery order, acknowledged in any order, committed only through the
// acknowledged prefix — a pending record in the middle fixes the ceiling.
type cursor struct {
	tracked []tracked
}

type tracked struct {
	record *kgo.Record
	acked  bool
}

func (c *cursor) Track(record *kgo.Record) {
	c.tracked = append(c.tracked, tracked{record: record})
}

// Mark acknowledges the record at the offset; an unknown offset is ignored.
func (c *cursor) Mark(offset int64) {
	for i := range c.tracked {
		if c.tracked[i].record.Offset == offset {
			c.tracked[i].acked = true
			return
		}
	}
}

// NextContiguous pops the acknowledged prefix and returns its last record —
// the one whose offset+1 is committed — or false when the head is pending.
func (c *cursor) NextContiguous() (*kgo.Record, bool) {
	var last *kgo.Record
	n := 0
	for n < len(c.tracked) && c.tracked[n].acked {
		last = c.tracked[n].record
		n++
	}
	if n == 0 {
		return nil, false
	}
	clear(c.tracked[:n])
	c.tracked = c.tracked[n:]
	return last, true
}

// Pending is how many tracked records are not yet committed.
func (c *cursor) Pending() int { return len(c.tracked) }
