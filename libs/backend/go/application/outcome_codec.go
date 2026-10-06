package application

import (
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

const (
	outcomeVersion  byte = 0x01
	branchAccepted  byte = 'a'
	branchRejected  byte = 'r'
	maxDetailsCount      = 1 << 10
)

// ErrOutcomeUnreadable is what DecodeOutcome reports for bytes it cannot read.
// The command is never run again in its place: that could repeat an effect the
// record exists to prevent (IDM-06).
var ErrOutcomeUnreadable = errors.New("application: stored outcome unreadable")

// OutcomeCodec is how one operation's response R becomes the bytes the
// idempotency record keeps. A context declares one per operation, and a release
// has to decode what the previous one encoded.
type OutcomeCodec[R any] struct {
	Encode func(*Encoder, R)
	Decode func(*Decoder) R
}

type Encoder struct{ buf []byte }

func (e *Encoder) String(value string) {
	e.buf = appendUvarint(e.buf, uint64(len(value)))
	e.buf = append(e.buf, value...)
}

func (e *Encoder) Int(value int64) {
	e.buf = appendVarint(e.buf, value)
}

func (e *Encoder) uint(value uint64) {
	e.buf = appendUvarint(e.buf, value)
}

// Decoder reads what an Encoder wrote. A read past the end or a malformed
// length leaves zero values and an error that DecodeOutcome reports.
type Decoder struct {
	buf []byte
	err error
}

func (d *Decoder) String() string {
	n := d.uint()
	if d.err != nil {
		return ""
	}
	if n > uint64(len(d.buf)) {
		d.err = errors.New("string longer than the remaining bytes")
		return ""
	}
	value := string(d.buf[:n])
	d.buf = d.buf[n:]
	return value
}

func (d *Decoder) Int() int64 {
	if d.err != nil {
		return 0
	}
	value, n := readVarint(d.buf)
	if n <= 0 {
		d.err = errors.New("malformed integer")
		return 0
	}
	d.buf = d.buf[n:]
	return value
}

func (d *Decoder) uint() uint64 {
	if d.err != nil {
		return 0
	}
	value, n := readUvarint(d.buf)
	if n <= 0 {
		d.err = errors.New("malformed length")
		return 0
	}
	d.buf = d.buf[n:]
	return value
}

// EncodeOutcome writes a version, the branch and its payload: the accepted
// response through codec, or the rejection's code, message and details.
func EncodeOutcome[R any](codec OutcomeCodec[R], outcome Outcome[R]) []byte {
	e := &Encoder{buf: []byte{outcomeVersion}}
	if rejection, rejected := outcome.Rejection(); rejected {
		e.buf = append(e.buf, branchRejected)
		e.String(string(rejection.Code()))
		e.String(rejection.Message())
		details := rejection.Details()
		e.uint(uint64(len(details)))
		for _, d := range details {
			e.String(d.Key)
			e.String(d.Value)
		}
		return e.buf
	}
	e.buf = append(e.buf, branchAccepted)
	codec.Encode(e, outcome.Response())
	return e.buf
}

func DecodeOutcome[R any](codec OutcomeCodec[R], stored []byte) (Outcome[R], error) {
	var zero Outcome[R]
	if len(stored) < 2 || stored[0] != outcomeVersion {
		return zero, fmt.Errorf("%w: unknown version", ErrOutcomeUnreadable)
	}
	d := &Decoder{buf: stored[2:]}

	var outcome Outcome[R]
	switch stored[1] {
	case branchAccepted:
		outcome = Accepted(codec.Decode(d))
	case branchRejected:
		code, message := d.String(), d.String()
		count := d.uint()
		if count > maxDetailsCount {
			return zero, fmt.Errorf("%w: %d details", ErrOutcomeUnreadable, count)
		}
		details := make([]domain.Detail, 0, count)
		for range count {
			details = append(details, domain.Detail{Key: d.String(), Value: d.String()})
		}
		outcome = Rejected[R](domain.Reject(domain.Code(code), message, details...))
	default:
		return zero, fmt.Errorf("%w: unknown branch %q", ErrOutcomeUnreadable, stored[1])
	}

	if d.err != nil {
		return zero, fmt.Errorf("%w: %w", ErrOutcomeUnreadable, d.err)
	}
	if len(d.buf) != 0 {
		return zero, fmt.Errorf("%w: %d trailing bytes", ErrOutcomeUnreadable, len(d.buf))
	}
	return outcome, nil
}
