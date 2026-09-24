// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (TRP-13, TRP-17, TRP-18) que o símbolo realiza, dentro do limite de 3 linhas.

package kafka

import "github.com/twmb/franz-go/pkg/kgo"

// Header is one operational header beside the envelope (TRP-18).
type Header struct {
	Key   string
	Value []byte
}

// Record is the read-only view an Observer receives of what is about to be
// produced: every slice is a copy, so an observer may read the bytes and can
// never substitute what goes on the wire (TRP-17).
type Record struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers []Header
}

// Observer sees each record before it is produced — for a check, a log, a
// hash — and has no way to alter it.
type Observer func(Record)

func view(r *kgo.Record) Record {
	headers := make([]Header, 0, len(r.Headers))
	for _, h := range r.Headers {
		headers = append(headers, Header{Key: h.Key, Value: append([]byte(nil), h.Value...)})
	}
	return Record{
		Topic:   r.Topic,
		Key:     append([]byte(nil), r.Key...),
		Value:   append([]byte(nil), r.Value...),
		Headers: headers,
	}
}
