package golden

// Direction of the round trip: the consumer reads the transported bytes, the
// producer writes them from the declared fields (FIX-03).
type Direction string

const (
	DirectionConsumer Direction = "consumer"
	DirectionProducer Direction = "producer"
)

// Oracle is one of the three of ORA-01: semantic equality, hash equality and
// byte identity. They are reported apart, never folded into one verdict.
type Oracle int

const (
	OracleSemantic Oracle = 1
	OracleHash     Oracle = 2
	OracleBytes    Oracle = 3
)

// Code is the stable diagnostic of each oracle (FIX-13).
type Code string

const (
	CodeR001 Code = "DMPF-R001"
	CodeR002 Code = "DMPF-R002"
	CodeR003 Code = "DMPF-R003"
)

func (o Oracle) Code() Code {
	switch o {
	case OracleSemantic:
		return CodeR001
	case OracleHash:
		return CodeR002
	default:
		return CodeR003
	}
}

// Outcome is one oracle in one direction for one case (FIX-12): when it fails,
// Field names what diverged and Expected/Got carry the two values.
type Outcome struct {
	Fixture   string    `json:"fixture"`
	Case      string    `json:"case"`
	Direction Direction `json:"direction"`
	Oracle    Oracle    `json:"oracle"`
	Code      Code      `json:"code"`
	OK        bool      `json:"ok"`
	Field     string    `json:"field,omitempty"`
	Expected  string    `json:"expected,omitempty"`
	Got       string    `json:"got,omitempty"`
}

// recorder keeps exactly one Outcome per oracle: the first failure wins and a
// later check cannot turn it back into a pass.
type recorder struct {
	fixture, name string
	direction     Direction
	failed        map[Oracle]Outcome
}

func newRecorder(f Fixture, c Case, d Direction) *recorder {
	return &recorder{fixture: f.Identity.Fixture, name: c.Name, direction: d, failed: map[Oracle]Outcome{}}
}

func (r *recorder) fail(o Oracle, field, expected, got string) {
	if _, done := r.failed[o]; done {
		return
	}
	r.failed[o] = Outcome{
		Fixture: r.fixture, Case: r.name, Direction: r.direction, Oracle: o, Code: o.Code(),
		Field: field, Expected: expected, Got: got,
	}
}

func (r *recorder) outcomes() []Outcome {
	out := make([]Outcome, 0, 3)
	for _, o := range []Oracle{OracleSemantic, OracleHash, OracleBytes} {
		if f, failed := r.failed[o]; failed {
			out = append(out, f)
			continue
		}
		out = append(out, Outcome{Fixture: r.fixture, Case: r.name, Direction: r.direction, Oracle: o, Code: o.Code(), OK: true})
	}
	return out
}
