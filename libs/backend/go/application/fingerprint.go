package application

const (
	fieldString byte = 's'
	fieldInt    byte = 'i'
)

// Fingerprint accumulates the canonical encoding of a command (IDM-04). Every
// field carries its kind and its length, and the encoding never depends on the
// Protobuf serialization, which is not canonical across releases.
type Fingerprint struct{ canonical []byte }

// NewFingerprint starts from the operation, so the same fields under another
// operation never fingerprint alike.
func NewFingerprint(operation string) *Fingerprint {
	return (&Fingerprint{}).String(operation)
}

func (f *Fingerprint) String(value string) *Fingerprint {
	f.append(fieldString, []byte(value))
	return f
}

func (f *Fingerprint) Int(value int64) *Fingerprint {
	f.append(fieldInt, appendVarint(nil, value))
	return f
}

// Canonical is a copy of the encoding; RunIdempotent digests it behind the
// operation it registers.
func (f *Fingerprint) Canonical() []byte {
	return append([]byte(nil), f.canonical...)
}

// WHY: the operation the inbox registers leads the digest, so a fingerprint
// built without NewFingerprint, or for another operation, still collides (R4).
func (f *Fingerprint) under(operation string) []byte {
	scoped := NewFingerprint(operation)
	return append(scoped.canonical, f.canonical...)
}

func (f *Fingerprint) append(kind byte, data []byte) {
	f.canonical = append(f.canonical, kind)
	f.canonical = appendUvarint(f.canonical, uint64(len(data)))
	f.canonical = append(f.canonical, data...)
}
