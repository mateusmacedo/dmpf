// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (SQS-05, SQS-06, SQS-06b) que o símbolo realiza, dentro do limite de 3 linhas.

package sqs

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

// GroupID derives the FIFO message group from the envelope's partition key
// (SQS-05): equal keys give equal groups, distinct keys distinct groups, and
// the digest keeps it inside the 128-character limit whatever the key (SQS-06b).
func GroupID(partitionKey string) string {
	return digest(partitionKey)
}

// DedupID derives the FIFO deduplication identifier from source, message id
// and payload hash (SQS-06): the id is unique only within a source, and the
// same id with another payload must reach the inbox as R4, not be suppressed here.
func DedupID(source, id, payloadHash string) string {
	return digest(source, id, payloadHash)
}

// digest hashes the fields with their lengths in front, so no field can shift
// into the next and two different triples never share a preimage.
func digest(fields ...string) string {
	h := sha256.New()
	var size [8]byte
	for _, f := range fields {
		binary.BigEndian.PutUint64(size[:], uint64(len(f)))
		h.Write(size[:])
		h.Write([]byte(f))
	}
	return hex.EncodeToString(h.Sum(nil))
}
