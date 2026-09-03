package envelope

import "errors"

var (
	ErrModality         = errors.New("envelope: data must be carried as proto_data")
	ErrMissingAttribute = errors.New("envelope: required attribute missing or empty")
	ErrEmptyConditional = errors.New("envelope: conditional attribute present but empty")
	ErrAttributeType    = errors.New("envelope: attribute carried with the wrong value type")
	ErrSpecVersion      = errors.New("envelope: specversion must be 1.0")
	ErrContentType      = errors.New("envelope: datacontenttype must be application/protobuf")
	ErrSchemaMismatch   = errors.New("envelope: Any type URL differs from dataschema")
	ErrDataSchemaForm   = errors.New("envelope: dataschema must be the Any type URL of the payload message")
	ErrMajorMismatch    = errors.New("envelope: dataschema and type disagree on the contract major")
)

// AttributeError names the attribute behind a sentinel so callers can keep errors.Is.
type AttributeError struct {
	Attribute string
	Err       error
}

func (e *AttributeError) Error() string { return e.Err.Error() + ": " + e.Attribute }

func (e *AttributeError) Unwrap() error { return e.Err }

func attributeError(err error, attribute string) error {
	return &AttributeError{Attribute: attribute, Err: err}
}
