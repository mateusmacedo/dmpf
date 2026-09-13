// Package payloadhash implements version 1 of the DMPF payload_hash formula
// (docs/dmpf/cloudevents-protobuf-buf.md, ENV-17 to ENV-19; docs/adr/022).
package payloadhash

import (
	"crypto/sha256"
	"encoding/hex"
)

// FormulaVersion is a property of the profile major and never travels in the envelope (ENV-19).
const FormulaVersion = 1

// Sum takes the Any.value bytes exactly as transported: there is deliberately no
// overload for a decoded message, because reserializing does not satisfy ENV-18.
func Sum(anyValue []byte) string {
	digest := sha256.Sum256(anyValue)
	return hex.EncodeToString(digest[:])
}
