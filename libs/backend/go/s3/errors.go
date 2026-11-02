// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (DAT-14, IDN-15, RES-27, SPEC-CWBA4BPV), dentro do limite de 3 linhas.

package s3

import (
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/worm"
)

var (
	// ErrIncompleteConfig is a configuration without region, bucket or clock,
	// or one that asks for path-style addressing without an explicit endpoint.
	ErrIncompleteConfig = errors.New("s3: configuration is incomplete")

	// ErrTLSRequired is an endpoint that is not https without the explicit
	// development-only opt-out: the SigV4 signature travels in the headers.
	ErrTLSRequired = errors.New("s3: endpoint must be https outside development")

	// ErrBucketNotImmutable is a bucket that does not keep versions under
	// Object Lock: the store refuses to operate on it.
	ErrBucketNotImmutable = errors.New("s3: bucket does not keep versions under object lock")

	// ErrNotReady is a store whose Ready has not confirmed the bucket yet; it
	// is a refusal of the same kind, so it matches ErrBucketNotImmutable.
	ErrNotReady = fmt.Errorf("%w: Ready has not confirmed the bucket", ErrBucketNotImmutable)

	// ErrTenantUnresolved reports an operation of a tenant store whose carrier
	// resolved no tenant, or one with a slash (IDN-15).
	ErrTenantUnresolved = worm.ErrTenantUnresolved

	// ErrPutOutcomeUnknown is a write that may have created a version the
	// caller holds no reference to: it is never repeated (RES-27), and the
	// version is found by listing the key's versions.
	ErrPutOutcomeUnknown = errors.New("s3: put outcome unknown, a version may exist without a reference")

	// ErrUnexpectedResponse is a response without what the port promises
	// (version, checksum, the store's instant): the store fails closed.
	ErrUnexpectedResponse = worm.ErrUnexpectedResponse
)
