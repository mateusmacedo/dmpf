// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (ENV-11, ENV-18, OBX-06), dentro do limite de 3 linhas.

package relay

import "errors"

var (
	// ErrSourceRequired is what Assemble reports without a source. It is
	// configuration, not a column: the URI is derived from the bounded
	// context, never from the host, the pod or the environment (ENV-08).
	ErrSourceRequired = errors.New("relay: source is required")

	// ErrPayloadHashMismatch is what Assemble reports when the digest no longer
	// matches the stored bytes. Corruption at rest is terminal: retrying does
	// not repair bytes, and a new backoff would only delay the diagnosis.
	ErrPayloadHashMismatch = errors.New("relay: stored payload does not match payload_hash")

	// ErrMissingContextAttributes is what Assemble reports when metadata does
	// not carry correlationid, causationid and traceparent as non-empty
	// strings. The claim filters these records out; this is the second guard.
	ErrMissingContextAttributes = errors.New("relay: metadata must carry correlationid, causationid and traceparent")

	// ErrAggregateVersionOutOfRange is what Assemble reports for a version
	// beyond int32, the width of ce_integer. Converting silently would publish
	// a negative version as if it were the real one.
	ErrAggregateVersionOutOfRange = errors.New("relay: aggregate version does not fit the ce_integer range")
)
