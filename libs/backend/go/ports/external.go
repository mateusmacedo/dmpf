// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (ERR-10, ERR-11, ERR-18, IDM-10, IDM-11, RES-21), dentro do limite de 3 linhas.

package ports

import "context"

// OutboundKey is the idempotency key a call to an external partner carries; the
// inbound command's key never crosses to the partner (IDM-10, IDM-11).
type OutboundKey string

// ExternalClass is the closed, total outcome of one submission to a partner;
// the zero value is no class, which a caller treats as uncertain (ERR-11).
type ExternalClass uint8

const (
	ClassAccepted ExternalClass = iota + 1
	ClassRetryableTransport
	ClassRetryableUpstream
	ClassFinalValidation
	ClassFinalBusiness
	ClassFinalAuthorization
	ClassUnknownSent
	ClassUnknownAmbiguous
	ClassSchemaMismatch
)

func (c ExternalClass) String() string {
	switch c {
	case ClassAccepted:
		return "accepted"
	case ClassRetryableTransport:
		return "retryable_transport"
	case ClassRetryableUpstream:
		return "retryable_upstream"
	case ClassFinalValidation:
		return "final_validation"
	case ClassFinalBusiness:
		return "final_business"
	case ClassFinalAuthorization:
		return "final_authorization"
	case ClassUnknownSent:
		return "unknown_sent"
	case ClassUnknownAmbiguous:
		return "unknown_ambiguous"
	case ClassSchemaMismatch:
		return "schema_mismatch"
	default:
		return "invalid"
	}
}

// Finding is what an observation of the partner's state concluded about the
// effect; the zero value is no finding, so the outcome class decides alone.
type Finding uint8

const (
	FindingApplied Finding = iota + 1
	FindingAbsent
	FindingRejected
	FindingPending
	FindingInconclusive
)

func (f Finding) String() string {
	switch f {
	case 0:
		return "none"
	case FindingApplied:
		return "applied"
	case FindingAbsent:
		return "absent"
	case FindingRejected:
		return "rejected"
	case FindingPending:
		return "pending"
	case FindingInconclusive:
		return "inconclusive"
	default:
		return "invalid"
	}
}

// KeySemantics is whether the partner deduplicates on the outbound key: only
// KeyNative or KeyHeader makes a resend after an unanswered call verifiable
// (ERR-10, ERR-30).
type KeySemantics uint8

const (
	KeyNative KeySemantics = iota + 1
	KeyHeader
	KeyNone
)

func (k KeySemantics) String() string {
	switch k {
	case KeyNative:
		return "native"
	case KeyHeader:
		return "header"
	case KeyNone:
		return "none"
	default:
		return "invalid"
	}
}

// Efficacy is whether the partner's acceptance is already the effect or only a
// promise an observation has to confirm.
type Efficacy uint8

const (
	EfficacyOnSubmit Efficacy = iota + 1
	EfficacyByObservation
)

func (e Efficacy) String() string {
	switch e {
	case EfficacyOnSubmit:
		return "on_submit"
	case EfficacyByObservation:
		return "by_observation"
	default:
		return "invalid"
	}
}

// Observation is one reading of the partner's state for a key; ID is the
// observation's own identity, so a duplicate delivery is recognized as such.
type Observation struct {
	Key         OutboundKey
	Finding     Finding
	SourceAt    Instant
	Provider    string
	ExternalRef string
	ID          string
}

// ExternalProfile is the partner's declaration, apart from the RES-21 sheet;
// Quiescence is in nanoseconds because this block imports no time.
type ExternalProfile struct {
	KeySemantics         KeySemantics
	Efficacy             Efficacy
	Quiescence           int64
	AuthoritativeAbsence bool
	MaxQueries           int
	KeyAlphabet          string
	KeyMaxLength         int
}

// ExternalCommand submits one command; the class is always set, the error only
// carries the cause for diagnosis (ERR-18), and a zero class is a provider defect.
type ExternalCommand interface {
	Submit(ctx context.Context, key OutboundKey, payload []byte) (ExternalClass, Observation, error)
}

// ExternalState reads the partner's state for a key. It is optional: its absence
// declares "no state query", and a nil value never counts as one.
type ExternalState interface {
	Query(ctx context.Context, key OutboundKey) (Observation, error)
}
